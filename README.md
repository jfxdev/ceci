# LeaFlag

Feature-flag platform compatible with [OpenFeature](https://openfeature.dev) (via [OFREP](https://github.com/open-feature/protocol)), with a Consul-style key/value parameter store. Permissions are scoped per **project** — a project groups both feature flags and parameters, and members hold a role (`owner` > `admin` > `editor` > `viewer`).

## Stack

- Backend: Go, [gin](https://github.com/gin-gonic/gin), [GORM](https://gorm.io) + PostgreSQL
- Frontend: React, Vite, TypeScript, [shadcn/ui](https://ui.shadcn.com) (Tailwind), light theme by default
- Auth: argon2id password hashing, JWT access token + rotating opaque refresh token (httpOnly cookie)
- Single Docker image: the frontend build is embedded into the Go binary via `go:embed`

## Control Plane e Data Plane

O mesmo artefato aceita três modos via `LEAFLAG_MODE`:

- `all-in-one` (padrão): painel, administração e runtime no mesmo processo; indicado para desenvolvimento e POCs.
- `control-plane`: painel, usuários e API administrativa. Requer `RUNTIME_SYNC_TOKENS` e expõe `GET /internal/v1/runtime/snapshot` somente para Data Planes autenticados.
- `data-plane`: não abre conexão com PostgreSQL; recebe snapshots do Control Plane em `CONTROL_PLANE_URL`, autenticado por `RUNTIME_SYNC_TOKEN`, e os atualiza a cada `EDGE_SYNC_INTERVAL` (padrão: `5s`).

Em produção, a URL usada por workloads é a do Data Plane. O endpoint OFREP continua em `/ofrep/v1`, e parâmetros de runtime usam a leitura compatível com Consul em `GET /v1/kv/:key`. Este último aceita a API key de ambiente em `Authorization: Bearer leaflag_sk_…` ou `X-Consul-Token: leaflag_sk_…`, com `raw`, `recurse`, `keys` e `separator`.

O endpoint interno de sincronização deve ficar atrás de um ingresso HTTPS; o Data Plane aceita somente URLs `https://` para o Control Plane.

Exemplo de implantação:

```sh
# Control Plane
LEAFLAG_MODE=control-plane RUNTIME_SYNC_TOKENS="$SYNC_TOKEN" /leaflag

# Data Plane (sem DATABASE_URL)
LEAFLAG_MODE=data-plane CONTROL_PLANE_URL=https://control.example.com \
  RUNTIME_SYNC_TOKEN="$SYNC_TOKEN" /leaflag
```

## Backend architecture

DDD-lite layering under `backend/internal/`:

```
constants/    application-wide constants (roles, reason codes, ttls)
domain/       pure domain entities, no framework dependencies
model/        GORM persistence structs (gorm tags)
dto/          HTTP request/response structs (json tags)
repository/   all database access and external API calls, behind interfaces
service/      business logic / use cases, depends on repository interfaces
routes/       one file per route group, each with its own *_test.go
middleware/   gin.HandlerFunc auth/role/api-key guards
web/          embeds the built frontend and serves it as an SPA fallback
```

`cmd/leaflag/main.go` only wires dependencies and starts the server — no routes are registered there.

## Login corporativo com OIDC

LeaFlag aceita um provedor OpenID Connect (OIDC) por instância. A integração
usa Authorization Code com PKCE, `state` assinado em cookie httpOnly e `nonce`
validado no ID token. Os usuários recebem a sessão própria do LeaFlag (JWT +
refresh cookie); tokens do IdP não são usados para chamar as APIs do produto.

O administrador de bootstrap, criado pelo comando `seed`, é marcado de forma
permanente como conta local. Mesmo que o IdP retorne seu e-mail ou subject, o
login OIDC é recusado; essa conta continua acessível somente pelo formulário
de e-mail e senha.

### Configuração persistida

OIDC é configurado em **Administration → Access groups → OIDC single sign-on**
e fica persistido no PostgreSQL. O client secret nunca volta pela API e é
cifrado antes de ser salvo. Defina uma chave mestra geral fora do banco — ela
também será usada por parâmetros secretos:

```sh
ENCRYPTION_KEY=...
```

O campo **Create users on first login** permite criar usuários na primeira autenticação. Com
o valor `false`, somente identidades previamente vinculadas podem entrar. O
vínculo usa `issuer` configurado + `sub`; uma conta local existente não é
vinculada automaticamente por e-mail, prevenindo takeover por colisão de
e-mail. A criação/vinculação administrativa de identidades existentes será
adicionada antes de usar JIT desligado em produção.

A URL de callback deve apontar para o Control Plane (ou all-in-one), não para
o Data Plane. HTTPS é obrigatório, exceto `http://localhost` no
desenvolvimento. Depois de configurar, o login exibe o botão **Sign in with
SSO** e o callback redireciona de volta para `/projects`.

### Microsoft Entra ID (Azure AD)

1. Crie um **App registration** do tipo Web no tenant desejado.
2. Adicione exatamente o valor preenchido em **Redirect URL** no LeaFlag em **Authentication →
   Web → Redirect URIs**.
3. Em **Certificates & secrets**, crie um client secret e use o seu valor em
   **Client secret**.
4. Use o Application (client) ID no campo **Client ID** e o tenant ID no campo **Issuer URL**:

   ```sh
   https://login.microsoftonline.com/<tenant-id>/v2.0
   ```

5. Atribua usuários ou grupos ao Enterprise Application conforme a política
   de acesso da organização.

Para usar grupos em permissões de projeto posteriormente, configure o claim
`groups` como **Groups assigned to the application**, de preferência emitindo
Object IDs imutáveis. Isso evita o limite de grupos excessivos no token.

### Authentik

1. Crie uma Application com Provider **OAuth2/OpenID Connect**.
2. Cadastre uma Redirect URI **Strict** igual ao campo **Redirect URL** do LeaFlag.
3. Copie o Client ID e Client Secret do provider.
4. Use como issuer/discovery base:

   ```sh
   https://authentik.example.com/application/o/<slug>
   ```

5. Mantenha os scopes `openid`, `profile` e `email` habilitados. Se a política
   exigir e-mail verificado, configure um property mapping confiável para o
   claim `email_verified` no Authentik.

Os grupos de acesso internos do LeaFlag já podem ser associados a projetos.
Em **Administration → Access groups**, abra o gerenciamento do grupo e inclua
o valor que o IdP emite no claim `groups`: use o Object ID do grupo no Entra
ou o nome/valor configurado no Authentik. A cada login, o LeaFlag sincroniza
essas associações; uma associação criada manualmente no painel nunca é
removida pelo IdP.

## Development

```sh
cp .env.example .env
make dev
```

`make dev` starts Postgres (docker compose), the Go backend on **:8110** (hot reload via [air](https://github.com/air-verse/air)), and the Vite dev server on **:8111** (proxies `/api` and `/ofrep` to :8110). It runs in the default `all-in-one` mode.

```sh
make test    # go test ./... -cover  +  vitest run --coverage
make build   # builds the frontend, embeds it into the backend, outputs bin/leaflag
make docker-build
```

## OFREP

Flags are evaluated through the OpenFeature Remote Evaluation Protocol:

- `POST /ofrep/v1/evaluate/flags/{key}` — single flag
- `POST /ofrep/v1/evaluate/flags` — bulk (all flags in the project)

Both require an environment-scoped API key: `Authorization: Bearer leaflag_sk_...` (create one from a project's API keys settings). Targeting rules use a small JSONLogic-style condition tree (`==`, `!=`, `>`, `<`, `in`, `contains`, `and`, `or`) plus optional deterministic percentage rollouts keyed on `context.targetingKey`.

## Parameters

The administrative KV store is scoped per project/environment. Keys may contain `/` for hierarchical organization (e.g. `service/db/host`) and support prefix filtering, versioning, and history. The Data Plane offers its read-only runtime projection under `/v1/kv/:key`; writes and history remain in the Control Plane.
