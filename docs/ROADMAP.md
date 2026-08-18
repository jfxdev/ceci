# LeaFlag — Roadmap

Levantamento derivado de um deep dive no código (não hipótese). Cada item cita
o arquivo/linha que motiva o trabalho. Prioridade em três horizontes: destravar
produção/compliance, change management, plataforma.

> Nota: `docs/enterprise.md` e `docs/todo.md` estão desatualizados — listam como
> "faltando" recursos já implementados (environments, OIDC/SSO, cache via Data
> Plane). Este arquivo reflete o estado real do código.

## Estado atual (implementado)

- **Auth** — argon2id, JWT access + refresh token rotativo (cookie httpOnly).
- **RBAC** — 4 papéis por projeto (`owner` > `admin` > `editor` > `viewer`).
- **Projects / Environments** — environments por projeto, context fields.
- **Access Groups + OIDC SSO** — Authorization Code + PKCE, `state` assinado,
  `nonce` validado, client secret cifrado, admin seed travado como conta local,
  mapeamento de grupos do IdP para grupos internos.
- **Parameters** — KV hierárquico, versionado, com histórico (`ParameterVersion`,
  append-only). Leitura runtime compatível com Consul (`/v1/kv/:key`).
- **Feature flags** — variants, config per-environment, targeting rules
  (JSONLogic-style), rollout % determinístico, prerequisites com guard de
  ciclo/profundidade, archive, kill switch.
- **OFREP** — eval single/bulk, ETag/304, SSE (`/evaluate/flags/stream`),
  `/configuration`.
- **Control Plane / Data Plane** — split via `LEAFLAG_MODE`. Data Plane compila
  snapshot em memória (swap atômico), avalia sem tocar o Postgres, replica via
  poll com `If-None-Match`. Resolve throughput e HA de leitura.
- **Playground**, **maintenance mode**.

## Curto prazo — destrava produção/compliance

### 1. Build de teste quebrado (bloqueador imediato)
`go test ./...` falha ao compilar o pacote `service`:

```
maintenance_service_test.go:39: *fakeInstanceSettingsRepository does not
implement repository.InstanceSettingsRepository (missing method SetOIDC)
```

O fake não acompanhou a interface `repository.InstanceSettingsRepository`
(`instance_settings_repository.go:17`). WIP não commitado. Adicionar `SetOIDC` ao
fake. Sem isto, `make test` não passa.

### 2. Audit log / versionamento de flags (bloqueador de compliance)
`Parameter` tem `ParameterVersion` (append-only, history completo). `FeatureFlag`
/ `FlagRule` / `FlagVariant` não têm nada — só `UpdatedAt` (`model/flag.go`).
Sem quem-mudou-o-quê-quando-valor-antes/depois não passa certificação (SOX,
PCI-DSS, regs de banco central) e não há rollback. Copiar o padrão de
`ParameterVersion`: `FlagVersion` append-only + endpoint de history + UI de
rollback.

### 3. Pool de conexão Postgres não configurado
`db.go:11` — `gorm.Open(postgres.Open(dsn), &gorm.Config{})` sem
`SetMaxOpenConns` / `SetMaxIdleConns` / `SetConnMaxLifetime`. Sob carga usa
defaults do driver, sem limite superior explícito. Fix pequeno, alto impacto.

### 4. Rate limit em `/ofrep/v1/*`
Hoje só CORS + auth por API key (`ofrep_routes.go`). Rate limiting existe só em
`/auth/login` e `/auth/register`. Um client em retry loop / poll agressivo satura
sem barreira. Estender `middleware.RateLimitPerIP` (ou por API key) ao OFREP.

### 5. `projectChanged` ignora o projectID
`runtimeplane/snapshot.go:158` — compara só `previous.etag == next.etag`, o
parâmetro `projectID` não é usado. Qualquer mudança em qualquer projeto acorda
**todos** os subscribers SSE de todos os projetos. Fanout desnecessário.
Comparar etag/hash por escopo de projeto.

## Médio prazo — change management

### 6. Aprovação em duas etapas (four-eyes)
Hoje `editor` aplica direto em prod (`RequireProjectRole(..., RoleEditor)` em
`flag_routes.go`), sem gate. Editor propõe → admin aprova antes de efetivar.
Exigência comum em banco.

### 7. Pipeline de promoção entre environments
Environments já existem, mas sem promoção controlada dev → staging → prod.
Adicionar diff + promoção de config de flag entre ambientes.

### 8. Mudanças agendadas
Ligar/desligar flag ou trocar rollout em horário definido. Requer scheduler
(cron in-process ou tabela de jobs) e integra com o audit log (item 2).

### 9. Webhooks / notificações
Sem notificação em mudança de flag crítica. Slack / PagerDuty / webhook genérico
disparado no `broadcaster.Publish` (o mesmo pub/sub que já alimenta o SSE).

### 10. API key: expiração, rotação, escopo
Hoje bearer simples por projeto+environment (`leaflag_sk_...`), sem expiração
nem rotação automática, escopo = projeto inteiro. Adicionar TTL, rotação e
escopo mais granular (read-only, por flag prefix).

## Longo prazo — plataforma

### 11. Flags-as-code / Terraform provider
Mudança via PR/git com review externo ao sistema (change management auditável
fora do próprio painel). Comum em banco.

### 12. Analytics de avaliação
Quais flags avaliadas, com que frequência, por qual variant. Detecção de flag
stale (nunca avaliada / rollout 100% há muito tempo → candidata a remoção).

### 13. SSE por-subscriber com contexto real
`ofrep_routes.go` reavalia o stream com `evalCtx == nil` — manda resultado
genérico, não segmenta por usuário (documentado em `docs/ofrep-realtime.md`).
Guardar o contexto do subscriber e reavaliar com ele.

### 14. Redis pub/sub para SSE multi-réplica do Control Plane
O Data Plane já resolve HA de SSE via snapshot poll. O `flag_broadcaster.go` do
Control Plane ainda é in-process (`map[uuid][]chan`) — múltiplas réplicas sem
sticky session perdem eventos entre si. Redis pub/sub resolve.

### 15. OpenTelemetry — traces, métricas, logs

Hoje a observabilidade é só `gin.Logger()` (`router.go:37,46,56`) — log de
acesso, sem trace/métrica estruturada. Instrumentar com OTel (SDK Go +
OTLP exporter) para produção.

**Setup**
- Provider global inicializado em `cmd/leaflag/main.go` (após `config.Load()`,
  antes de montar o router): `TracerProvider` + `MeterProvider` com OTLP gRPC/HTTP
  exporter, `Resource` com `service.name=leaflag`, `service.version`, e
  `leaflag.mode` (all-in-one/control-plane/data-plane) como atributo — separar
  telemetria por plane é essencial já que compartilham binário.
- Shutdown com flush no encerramento (o server já tem hook de shutdown).
- Config nova: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SDK_DISABLED` (default
  desligado para não quebrar `make dev`). Adicionar em `config.go` + `Validate`.

**Traces (spans)**
- **HTTP**: `otelgin` middleware no lugar/junto de `gin.Logger()`, nos três
  routers (`NewAllInOneRouter`/`NewControlPlaneRouter`/`NewDataPlaneRouter`).
  Propaga `traceparent` de entrada.
- **DB**: plugin `otelgorm` no `db.Connect` (`db.go:11`) — cada query vira span
  filho. Casa com o achado do pool não configurado: os spans de DB expõem
  contenção de conexão direto.
- **Eval**: span manual em `FlagService.Evaluate`/`EvaluateAll`
  (`flag_service.go:196,205`) e no path data-plane
  (`Store.Evaluate`/`EvaluateAll`, `snapshot.go:180,188`) com atributos
  `flag.key`, `project.id`, `environment.id`, `result.reason`,
  `result.variant`. Instrumentar os DOIS paths, senão o data-plane (que serve o
  tráfego de produção) fica cego.
- **Runtime sync**: span em `Syncer.SyncOnce` (`sync.go:75`) cobrindo o
  `HTTPSource.Load` + `Store.Replace`, com `snapshot.etag`, `notModified`,
  duração. Propagar contexto do Data Plane → Control Plane no header do
  `/internal/v1/runtime/snapshot` para trace distribuído entre planes.

**Métricas**
- `leaflag.flag.evaluations` (counter) por `reason`/`variant`/`project` — resolve
  o item 12 (analytics/stale-flag detection) com a mesma instrumentação.
- `leaflag.flag.evaluation.duration` (histogram).
- `leaflag.ofrep.not_modified` (counter) — razão de acerto do ETag/304.
- `leaflag.runtime.snapshot.age` (gauge) — segundos desde o último sync com
  sucesso (`Store.lastSuccess`, `snapshot.go:92`); alarme de Data Plane
  servindo snapshot velho.
- `leaflag.runtime.sync.errors` (counter) — do `Store.lastError`.
- `leaflag.sse.subscribers` (up/down counter) — nas `Subscribe`/cancel
  (`ofrep_routes.go:84`, `snapshot.go:246`).
- DB pool stats (open/idle/wait) via `sql.DBStats` — pré-requisito é configurar
  o pool (item 3).

**Logs**
- Migrar `gin.Logger()` para logs estruturados correlacionados por `trace_id`
  (o exporter OTel de logs ou slog + bridge). Fecha o loop trace↔log.

**Nota de precedência**: instrumentar eval e runtime sync dá o maior retorno
(hot path + saúde da replicação Data Plane). HTTP/DB genérico vem de brinde com
os middlewares. Não bloqueia nada, mas é pré-requisito prático para operar
multi-réplica com confiança.

## Segurança & correção (deep dive)

Achados de auditoria de código, citando arquivo:linha. Ordenados por gravidade.

### Segurança

- **S1 (crítico) — secret default em prod sem guarda.** `config.go:28,34` —
  `JWT_SECRET` e `ENCRYPTION_KEY` caem para `"dev-secret-change-me"` se não
  setados, e `Validate()` (`config.go:40`) não rejeita o default em
  `control-plane`/`data-plane`. Deploy que esqueça de setar roda com secret
  público → forja de JWT (account takeover) e decifração do client secret OIDC.
  Fix: `Validate()` recusa o default quando `Mode != all-in-one`.
- **S2 (alto) — "parâmetros secretos" não existem.** README promete que
  `ENCRYPTION_KEY` cifra parâmetros secretos, mas `Parameter.Value` é
  `string not null` em plaintext e `grep aes/cipher` só bate em
  `oidc_configuration_service.go`. KV vai receber senha de banco gravada em
  claro. Implementar param cifrado com flag `secret`, ou corrigir o README.
- **S3 (médio) — user enumeration por timing no login.** `auth_service.go:117`
  — email inexistente retorna sem rodar argon2; email existente roda o hash
  caro. Diferença de tempo revela contas. Rodar hash dummy no ramo de usuário
  inexistente.
- **S4 (médio) — refresh rotation sem theft detection.** `auth_service.go:195`
  — replay de refresh token já revogado só retorna `ErrInvalidToken`, sem
  revogar a família da sessão. Melhor prática: token revogado reapresentado =
  sinal de roubo → invalidar sessão inteira.
- **S5 (médio) — single-flight ausente no refresh do frontend.** `api.ts:28`
  — cada request que toma 401 chama `tryRefresh()` sozinho. Como o backend
  **rotaciona** o refresh, N requests paralelos disparam refreshes concorrentes;
  o primeiro rotaciona o cookie, os demais mandam o token já revogado → falham →
  logout espúrio. Compartilhar uma única promise de refresh in-flight.
- **S6 (baixo) — registro aberto.** `Register` público — qualquer um cria
  conta (sem acesso a projeto). Confirmar se o produto quer invite-only.

### Correção

- **C1 — lógica de prerequisite duplicada.** `flag_service.go:225`
  (`resolveWithPrerequisites`, path DB) e `flag_evaluate_loaded.go:36`
  (`resolveLoadedFlag`, path data-plane) são cópias da mesma recursão →
  divergência entre control-plane e data-plane. Extrair função única
  parametrizada pelo lookup.
- **C2 — `ReplaceVariantsAndRules` reescreve o catálogo de variants inteiro.**
  `flag_repository.go:132` — deleta variants por `flag_id` (não por
  environment, pois são compartilhados) e recria. Save de rules no env A
  reescreve o catálogo que o env B usa; last-write-wins sob edição concorrente.
- **C3 — `resultFor` erra a flag inteira se o variant do rollout não existe.**
  `flag_evaluate.go:94` — rollout apontando para variant inexistente cai em
  `ErrCodeGeneral` em vez de usar o default. Misconfig derruba o eval.
- **C4 — `==`/`!=` comparam via `fmt.Sprint`.** `flag_evaluate.go:221` —
  comparação stringificada, type-loose (`true == "true"`, `1 == "1"`). Pode dar
  match de targeting inesperado.
- **C5 — modulo bias no rollout.** `flag_evaluate.go:124` — `Sum32() % 100`
  tem viés leve (2^32 não divisível por 100). Distribuição de rollout % não
  exatamente uniforme.
- **C6 (alto) — race de versão em parâmetro corrompe o audit.**
  `parameter_service.go:43` calcula `version = existing.Version + 1` **fora** da
  transação do repo, e `ParameterVersion` (`model/parameter.go:21`) não tem
  índice único em `(ParameterID, Version)`. Dois writers concorrentes leem a
  versão N e ambos gravam N+1 → duas linhas na mesma versão, `Parameter.Value`
  em last-write-wins silencioso. É justamente a trilha de auditoria (history de
  parâmetro). Fix: unique `(parameter_id, version)` + calcular a versão dentro da
  transação (ou upsert com lock).

### Notas de escopo (não-bug, decisões a confirmar)

- **Snapshot de runtime é global por sync token.** `runtime_snapshot_routes.go`
  — qualquer token válido recebe o snapshot inteiro (todos os projetos, todas
  as API keys em hash, todos os parâmetros). Um Data Plane de uma região/cliente
  vê tudo. Sem multi-tenant real, ok; com isolamento por cliente, escopo por
  token seria necessário.
- **OIDC não valida `email_verified`.** `oidc_service.go:148` — usa `email` (ou
  `preferred_username`) sem checar `email_verified`. Com JIT ligado e IdP que
  emite email não verificado, cria conta com email arbitrário. Impacto baixo
  (sem auto-link por email), mas para JIT em prod, exigir o claim.

### Positivo confirmado

- Motor de targeting compartilhado entre control/data plane (`evaluateFlag`/
  `toEnvFlag`/`resultFor`) — sem divergência exceto o wrapper de prereq (C1).
- API key: sha256 de chave 256-bit random, `FindActiveByHash`, sem plaintext.
- Password: argon2id + `subtle.ConstantTimeCompare`, formato PHC.
- OIDC: sem link automático por email, bootstrap admin travado como conta local.
- `RequireProjectRole` retorna 404 (não 403) a não-membro — não vaza existência
  de projeto.
- Preloads de flag são escopados por environment, sem N+1.

## Referências de infra

`docs/enterprise.md` tem um diagrama de deploy AWS (ECS Fargate + RDS Multi-AZ +
ElastiCache + WAF + Secrets Manager). Continua válido como alvo de produção,
mas note que a seção de "gaps de throughput/cache" já foi majoritariamente
resolvida pelo Data Plane em memória.
