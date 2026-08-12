# ceci

Feature-flag platform compatible with [OpenFeature](https://openfeature.dev) (via [OFREP](https://github.com/open-feature/protocol)), with a Consul-style key/value parameter store. Permissions are scoped per **project** — a project groups both feature flags and parameters, and members hold a role (`owner` > `admin` > `editor` > `viewer`).

## Stack

- Backend: Go, [gin](https://github.com/gin-gonic/gin), [GORM](https://gorm.io) + PostgreSQL
- Frontend: React, Vite, TypeScript, [shadcn/ui](https://ui.shadcn.com) (Tailwind), light theme by default
- Auth: argon2id password hashing, JWT access token + rotating opaque refresh token (httpOnly cookie)
- Single Docker image: the frontend build is embedded into the Go binary via `go:embed`

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

`cmd/ceci/main.go` only wires dependencies and starts the server — no routes are registered there.

## Development

```sh
cp .env.example .env
make dev
```

`make dev` starts Postgres (docker compose), the Go backend on **:8110** (hot reload via [air](https://github.com/air-verse/air)), and the Vite dev server on **:8111** (proxies `/api` and `/ofrep` to :8110).

```sh
make test    # go test ./... -cover  +  vitest run --coverage
make build   # builds the frontend, embeds it into the backend, outputs bin/ceci
make docker-build
```

## OFREP

Flags are evaluated through the OpenFeature Remote Evaluation Protocol:

- `POST /ofrep/v1/evaluate/flags/{key}` — single flag
- `POST /ofrep/v1/evaluate/flags` — bulk (all flags in the project)

Both require a project-scoped API key: `Authorization: Bearer ceci_sk_...` (create one from a project's API keys settings). Targeting rules use a small JSONLogic-style condition tree (`==`, `!=`, `>`, `<`, `in`, `contains`, `and`, `or`) plus optional deterministic percentage rollouts keyed on `context.targetingKey`.

## Parameters

Consul-style KV store scoped per project. Keys may contain `/` for hierarchical organization (e.g. `service/db/host`) and support prefix filtering, versioning, and history.
