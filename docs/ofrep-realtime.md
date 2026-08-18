# OFREP real-time updates (SSE)

## Visão geral

O protocolo OFREP padrão é request/response (poll). LeaFlag implementa, além
disso, uma extensão própria — **não parte da spec OFREP** — de push via
Server-Sent Events (SSE): `GET /ofrep/v1/evaluate/flags/stream`.

Não é WebSocket. É SSE: conexão HTTP única, long-lived, server empurra
eventos texto, client nunca escreve de volta na mesma conexão. Motivo: SSE
reconecta sozinho no browser (`EventSource`), passa por proxy/LB HTTP normal
sem upgrade de protocolo, e o caso de uso (push unidirecional de "flags
mudaram") não precisa de canal bidirecional.

Implementação: `backend/internal/routes/ofrep_routes.go` (handler
`/evaluate/flags/stream`) + `backend/internal/service/flag_broadcaster.go`
(pub/sub in-process, por `projectID`). **Single-instance only** — não tem
fan-out entre múltiplas réplicas do backend (sem Redis/NATS por trás). Se
rodar `leaflag` com >1 réplica atrás de LB, cada réplica só vê os `Publish()`
que ela mesma disparou (writes que caíram nela). Rodar múltiplas réplicas
sem sticky sessions faz o cliente perder updates de outras réplicas.

## Contrato do endpoint

```
GET /ofrep/v1/evaluate/flags/stream
Authorization: Bearer leaflag_sk_xxx
```

- Auth: mesma project API key do resto do OFREP (`RequireProjectAPIKey`).
- Response: `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
  `Connection: keep-alive`.
- **Evento inicial**: assim que conecta, manda o estado atual completo —
  não espera uma mudança pra mandar o primeiro evento.
- **Evento `flags`**: disparado no connect e a cada mudança (create/update/
  delete de qualquer flag do projeto). Payload é o mesmo shape do bulk eval:

  ```
  event:flags
  data:{"flags":[{"key":"new-checkout","value":true,"reason":"STATIC","variant":"on"}]}

  ```

  Sem contexto de avaliação por-subscriber — reavalia com `evalCtx == nil`
  (equivalente ao contexto vazio). Se sua flag depende de `targetingKey`/
  atributos de contexto pra segmentar, o SSE não segmenta por usuário: ele
  manda o resultado "genérico" e cabe ao client re-derivar localmente ou
  tratar o evento como só um "invalidation signal" (recarregar via bulk POST
  com o contexto real).
- **Evento `heartbeat`**: a cada 15s, `data:` vazio, só pra manter proxies/
  LBs de matarem a conexão idle. Ignorar no client.
- Encerra quando o client desconecta (`ctx.Done()`) ou quando `EvaluateAll`
  falha (erro 500 antes do primeiro evento, conexão simplesmente cai depois).
- Sem reconexão automática do lado servidor — cabe ao client reconectar.
  `EventSource` do browser já faz isso sozinho; em Go/Python é preciso
  implementar retry manual (exemplos abaixo já incluem).

## Providers oficiais OpenFeature não sabem consumir isso

O `OFREPWebProvider`/providers oficiais fazem só polling (respeitam ETag/
304, mas não abrem SSE). Pra aproveitar push real, é preciso:
1. Manter o provider OFREP normal pra avaliação (`getBooleanValue` etc).
2. Escutar o SSE em paralelo e, a cada evento `flags`, forçar o provider a
   revalidar (na prática: refetch manual + `OpenFeature.setContext(...)` de
   novo, ou simplesmente invalidar seu próprio cache e reconsultar via bulk
   POST com o contexto do usuário).

## Exemplo TypeScript (frontend, browser)

```ts
const es = new EventSource("/ofrep/v1/evaluate/flags/stream", {
  // EventSource nativo não manda headers customizados (sem Authorization).
  // Duas opções: (a) usar um polyfill tipo `event-source-polyfill` que aceita
  // headers, ou (b) expor a key via query string num endpoint que aceite
  // isso (LeaFlag hoje só aceita Bearer header — use (a) em produção).
})

es.addEventListener("flags", (e) => {
  const { flags } = JSON.parse(e.data) as {
    flags: { key: string; value: unknown; reason: string; variant?: string }[]
  }
  console.log("flags changed:", flags)
  // invalide seu cache local / force um re-eval do provider aqui
})

es.addEventListener("heartbeat", () => {
  // no-op, só mantém viva a conexão
})

es.onerror = () => {
  // EventSource já reconecta sozinho por padrão; só logar se quiser observabilidade
  console.warn("SSE connection dropped, browser will retry")
}
```

Com `event-source-polyfill` (pra mandar o Bearer token):

```ts
import { EventSourcePolyfill } from "event-source-polyfill"

const es = new EventSourcePolyfill("/ofrep/v1/evaluate/flags/stream", {
  headers: { Authorization: "Bearer leaflag_sk_xxx" },
  heartbeatTimeout: 30000, // > 15s do heartbeat do servidor
})
```

## Exemplo Go

```go
package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"strings"
	"time"
)

func streamFlags(ctx context.Context, baseURL, apiKey string, onFlags func(raw string)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/ofrep/v1/evaluate/flags/stream", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var event string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimPrefix(line, "event:")
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimPrefix(line, "data:")
			if event == "flags" {
				onFlags(data)
			}
			event = ""
		}
	}
	return scanner.Err()
}

func main() {
	ctx := context.Background()
	for {
		err := streamFlags(ctx, "https://seu-leaflag-host", "leaflag_sk_xxx", func(raw string) {
			log.Println("flags changed:", raw)
			// re-avalie/invalide cache local aqui
		})
		if err != nil {
			log.Printf("stream dropped: %v, reconnecting in 3s", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}
```

## Exemplo Python

```python
import time
import httpx

def stream_flags(base_url: str, api_key: str, on_flags):
    headers = {"Authorization": f"Bearer {api_key}"}
    with httpx.Client(timeout=None) as client:
        with client.stream("GET", f"{base_url}/ofrep/v1/evaluate/flags/stream", headers=headers) as resp:
            event = None
            for line in resp.iter_lines():
                if line.startswith("event:"):
                    event = line[len("event:"):]
                elif line.startswith("data:"):
                    data = line[len("data:"):]
                    if event == "flags":
                        on_flags(data)
                    event = None

if __name__ == "__main__":
    while True:
        try:
            stream_flags(
                "https://seu-leaflag-host",
                "leaflag_sk_xxx",
                lambda raw: print("flags changed:", raw),
            )
        except httpx.HTTPError as e:
            print(f"stream dropped: {e}, reconnecting in 3s")
        time.sleep(3)
```

(Alternativa mais simples em Python: lib `sseclient-py`, que já faz o parse
de `event:`/`data:` — mas não vem com o header de auth por padrão, precisa
passar via `requests.get(..., stream=True, headers=...)` e envolver com
`sseclient.SSEClient(response)`.)

## Resumo prático

| Mecanismo | Latência | Complexidade client | Quando usar |
|---|---|---|---|
| Poll simples (bulk eval) | intervalo fixo | trivial | maioria dos casos |
| Poll + ETag/304 | intervalo fixo, banda menor | trivial (já é o provider oficial) | default recomendado |
| SSE (`/evaluate/flags/stream`) | quase instantâneo | manual, sem contexto por-subscriber, single-instance only | kill-switch urgente, dashboards internos |
