# ceci — próximos passos

Status atual: auth, projects, parameters (KV hierárquico+versionado), feature flags (CRUD+evaluator+FlagEditor com rules/rollout), OFREP, sidebar layout (shadcn sidebar-07), seed, rate limit no login, CORS no OFREP — tudo testado (backend ~75-100% coverage por pacote, frontend 88.5%) e validado em browser real contra Postgres.

## Feito recentemente
- [x] Coverage em `repository` (87.8%), `db` (100%), `config` (100%), `constants` (100%) — testes com sqlite in-memory via `github.com/glebarez/sqlite`
- [x] Removido `default:gen_random_uuid()` (postgres-specific) dos models, trocado por `BeforeCreate` hooks GORM — portável entre bancos, testado em sqlite e Postgres real
- [x] Deletado `internal/domain` (código morto, nunca usado — services trabalham direto com `model`)
- [x] FlagEditor completo: kill switch, variants, targeting rules (attr/op/value), rollout %
- [x] Bug conhecido do FlagEditor corrigido: Select do shadcn ficava em branco no "Default variant" ao editar flag existente (Radix só registra label do item quando ele monta — corrigido com `key` forçando remount quando dados chegam)

## Urgente
0. **Repo sem nenhum commit ainda** — `git log` vazio, tudo untracked. Fazer primeiro commit antes de qualquer outra coisa (checar `.env` não vai junto — já tem `.gitignore`, confirmar que cobre `.env`).

## Alta prioridade
1. **Registro de usuário** — hoje só existe `make seed` (1 admin fixo). Sem tela nem rota de signup. Confirmado: não existe `POST /api/v1/auth/register` em `auth_routes.go`. Se precisar múltiplos usuários reais, falta esse endpoint (ou fluxo de admin convidar via UI/email).
2. **Audit log de flags** — `parameters` já tem `ParameterVersion` (histórico completo, append-only). `feature_flags`/`flag_rules`/`flag_variants` não têm nada — mudança de rule/variant não fica rastreada. Copiar o mesmo padrão (`FlagAudit` ou `FlagVersion`).

## Média prioridade
3. **Members UI incompleta** — falta editar role de membro existente e remover membro (rotas do backend já existem: `PATCH`/`DELETE /projects/:id/members/:userId`, só falta botão na página).
4. **Dockerfile produção nunca testado de verdade** — só validei o binário local (`make build`) servindo frontend embutido. Falta rodar `make docker-build` e subir o container real de ponta a ponta.
5. **FlagEditor: condição só suporta single attr/op/value na UI** — AND/OR aninhado só dá pra configurar via API direto (evaluator já suporta). Documentar essa limitação pro usuário final, ou expandir a UI se for necessário no dia a dia.

## Baixa prioridade
6. **Rate limiting só no `/auth/login`** — dá pra estender pra outras rotas sensíveis (criação de project/api-key/member) se abuso virar problema real. Adiado a pedido do usuário.
7. **CLAUDE.md** do projeto pra onboarding de outros devs/agentes (README.md raiz já existe e cobre stack+dev setup).

## Notas técnicas pra quem pegar isso depois
- Login seed: `admin@ceci.local` / `admin123` (via `make seed`, idempotente)
- Postgres local roda na porta **5433** (5432 já ocupada por outro projeto na máquina de dev) — ver `docker-compose.yml` e `.env.example`
- Backend :8110, frontend :8111 (Vite proxy `/api` e `/ofrep` pro backend)
- Ferramenta de automação de browser usada nas sessões tem um bug conhecido: clique sintético não dispara o pointer event do Radix UI (Select/Dialog) — clique programático via `element.click()` no JS funciona normal. Não é bug do app.
