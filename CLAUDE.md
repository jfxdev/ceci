# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

ceci is a feature-flag platform compatible with [OpenFeature](https://openfeature.dev) (via [OFREP](https://github.com/open-feature/protocol)), plus a Consul-style key/value parameter store. Permissions are scoped per **project** — a project groups both feature flags and parameters, and members hold a role (`owner` > `admin` > `editor` > `viewer`).

Stack: Go/gin/GORM+PostgreSQL backend, React/Vite/TypeScript/shadcn (Tailwind) frontend, argon2id + JWT/rotating-refresh-cookie auth. Single Docker image — the frontend build is embedded into the Go binary via `go:embed`.

## Commands

```sh
cp .env.example .env
make dev            # postgres (docker compose) + backend :8110 (air hot reload) + vite :8111 (proxies /api, /ofrep)
make test           # go test ./... -cover  +  vitest run --coverage
make build           # builds frontend, embeds into backend, outputs bin/ceci
make docker-build
make seed            # creates admin@ceci.local / admin123 (idempotent)
```

Single test/package:
```sh
cd backend && go test ./internal/service/... -run TestFlagService_Evaluate -v
cd frontend && npx vitest run src/components/shared/data-table.test.tsx
```

Frontend lint/typecheck (not wired into `make test`):
```sh
cd frontend && npx oxlint
cd frontend && npx tsc --noEmit -p tsconfig.app.json
```

Postgres runs on port **5433** locally (5432 is often taken on dev machines) — see `docker-compose.yml` / `.env.example`.

## Backend architecture

DDD-lite layering under `backend/internal/`:

```
constants/    application-wide constants (roles, reason codes, ttls)
model/        GORM persistence structs (gorm tags)
dto/          HTTP request/response structs (json tags)
repository/   all database access and external API calls, behind interfaces
service/      business logic / use cases, depends on repository interfaces
routes/       one file per route group, each with its own *_test.go
middleware/   gin.HandlerFunc auth/role/api-key guards
web/          embeds the built frontend and serves it as an SPA fallback
```

`backend/cmd/ceci/main.go` only wires dependencies (config → db → repositories → services → router) and starts the server — no routes are registered there, no business logic. `backend/cmd/seed/main.go` is the standalone seed script.

Each route file declares a narrow local interface for the service(s) it depends on (e.g. `authService` in `auth_routes.go`) rather than depending on the concrete `*service.XService` type — this is what lets route tests use hand-rolled fakes instead of a real DB. `RegisterXRoutes(rg, concreteService)` is a thin public wrapper around the real, interface-typed `registerXRoutes(rg, service)` that's exercised in tests.

Repository tests run against real sqlite (in-memory, `github.com/glebarez/sqlite`), not mocks — see `repository/testdb_test.go`. Route/service tests use fakes.

Flag targeting conditions (`model.FlagRule.ConditionJSON`, `dto.FlagRuleInput.ConditionJSON`) are stored and passed around as opaque JSON — there's no Go struct for the condition tree. It's a JSONLogic-style structure interpreted recursively at eval time by `service/flag_evaluate.go`: `{"var": "attr"}` for context lookups, `==`/`!=`/`>`/`<`/`in`/`contains` for comparisons, and `and`/`or` for arbitrarily nested boolean combination. Percentage rollouts are deterministic, keyed on `context.targetingKey`.

## Frontend architecture

`frontend/src/pages/` — one file per route (flags list, flag editor, parameters browser, members, project settings, login/register). `frontend/src/lib/api.ts` is a thin fetch wrapper: it attaches the in-memory access token, retries once through `/auth/refresh` on a 401, and throws `ApiError`. `frontend/src/lib/auth.tsx` is the `AuthProvider`/`useAuth()` context — access token lives only in memory, session is restored on load via the httpOnly refresh cookie.

`frontend/src/components/shared/` holds cross-page primitives (`DataTable`, `ConfirmDialog`, `RoleBadge`, `AppLayout`, `ProtectedRoute`) — reuse these instead of building ad-hoc tables/dialogs per page. `frontend/src/components/ui/` is shadcn/ui, generally not hand-edited.

Because the flag condition tree is opaque JSON on the backend, `flag-editor.tsx` has its own parse/serialize layer (`conditionToRule`/`ruleToCondition` and friends) that converts between the JSONLogic shape and the editor's flat `RuleRow`/`ConditionRow` state. The editor currently supports one combinator (`and`/`or`) per rule across a flat list of leaf conditions; conditions nested deeper than that (e.g. an `and` containing an `or`) round-trip lossily — a nested sub-condition collapses to a blank leaf when loaded into the editor. This is a known, intentional limit, not a bug.

Radix Select in this UI needs a `key` prop tied to data-readiness on the trigger when the selected value depends on async-loaded options — otherwise the trigger can render blank because Radix only registers an item's label once it has mounted (see the `Default variant` select in `flag-editor.tsx` for the pattern).

## Testing conventions

- Backend: table-driven `go test`, fakes over mocks, sqlite in-memory for repository-layer tests.
- Frontend: vitest + Testing Library, colocated `*.test.tsx`.
- Coverage floors are not enforced by tooling but the project targets ~75-100% per backend package and high-80s% on the frontend; check `make test` output before considering backend/service or frontend/component changes done.

## Known env quirk

The browser automation tooling used in agent sessions doesn't reliably dispatch synthetic `click` events to Radix UI (Select/Dialog) — pointer events aren't fired, so the component doesn't open/select. A programmatic `element.click()` via JS in the page context works normally. This is a tooling limitation, not an app bug — don't "fix" a Radix component because it didn't respond to an automated click.
