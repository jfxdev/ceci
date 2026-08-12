# ceci — próximos passos

Status atual: auth, projects, parameters (KV hierárquico+versionado), feature flags (CRUD+evaluator+FlagEditor com rules/rollout), OFREP, sidebar layout (shadcn sidebar-07), seed, rate limit no login, CORS no OFREP — tudo testado (backend ~75-100% coverage por pacote, frontend 88.5%) e validado em browser real contra Postgres.

## Feito recentemente
- [x] Coverage em `repository` (87.8%), `db` (100%), `config` (100%), `constants` (100%) — testes com sqlite in-memory via `github.com/glebarez/sqlite`
- [x] Removido `default:gen_random_uuid()` (postgres-specific) dos models, trocado por `BeforeCreate` hooks GORM — portável entre bancos, testado em sqlite e Postgres real
- [x] Deletado `internal/domain` (código morto, nunca usado — services trabalham direto com `model`)
- [x] FlagEditor completo: kill switch, variants, targeting rules (attr/op/value), rollout %
- [x] Bug conhecido do FlagEditor corrigido: Select do shadcn ficava em branco no "Default variant" ao editar flag existente (Radix só registra label do item quando ele monta — corrigido com `key` forçando remount quando dados chegam)

## Feito nesta sessão
- [x] **Repo inicializado e no GitHub** — commit inicial em `repo-init`, branch `main` criado, PR draft #1 aberto (`repo-init` → `main`). `.env`/`node_modules`/`coverage` de fora do versionamento.
- [x] **Registro de usuário** — `POST /api/v1/auth/register` (o `AuthService.Register` já existia, faltava só a rota + wiring). Retorna 409 (`ErrEmailTaken`) em email duplicado, auto-login na conta recém-criada. Página `/register` no frontend, link cruzado com `/login`. Testado via curl e end-to-end em browser real.
- [x] **Members UI completa** — role do membro agora é um `Select` inline (dispara `PATCH` ao trocar) e apareceu botão "Remove" com `ConfirmDialog` (dispara `DELETE`). Validado em browser real: trocar viewer→editor refletiu no banco e na tela; remover tirou da listagem.

## Alta prioridade
1. **Audit log de flags** — `parameters` já tem `ParameterVersion` (histórico completo, append-only). `feature_flags`/`flag_rules`/`flag_variants` não têm nada — mudança de rule/variant não fica rastreada. Copiar o mesmo padrão (`FlagAudit` ou `FlagVersion`).

## Média prioridade
2. **Dockerfile produção nunca testado de verdade** — só validei o binário local (`make build`) servindo frontend embutido. Falta rodar `make docker-build` e subir o container real de ponta a ponta.
3. **FlagEditor: condição só suporta single attr/op/value na UI** — AND/OR aninhado só dá pra configurar via API direto (evaluator já suporta). Documentar essa limitação pro usuário final, ou expandir a UI se for necessário no dia a dia.

## Baixa prioridade
4. **Rate limiting só no `/auth/login`** (e agora `/auth/register`) — dá pra estender pra outras rotas sensíveis (criação de project/api-key/member) se abuso virar problema real. Adiado a pedido do usuário.
5. **CLAUDE.md** do projeto pra onboarding de outros devs/agentes (README.md raiz já existe e cobre stack+dev setup).

## Notas técnicas pra quem pegar isso depois
- Login seed: `admin@ceci.local` / `admin123` (via `make seed`, idempotente)
- Postgres local roda na porta **5433** (5432 já ocupada por outro projeto na máquina de dev) — ver `docker-compose.yml` e `.env.example`
- Backend :8110, frontend :8111 (Vite proxy `/api` e `/ofrep` pro backend)
- Ferramenta de automação de browser usada nas sessões tem um bug conhecido: clique sintético não dispara o pointer event do Radix UI (Select/Dialog) — clique programático via `element.click()` no JS funciona normal. Não é bug do app.
