# Requisitos para automações internas de um banco internacional

Levantamento do que falta em LeaFlag pra sustentar uso interno num contexto de
banco internacional (alto compliance, alta disponibilidade, auditoria
forte). Estado atual: auth argon2id+JWT, RBAC 4 níveis (owner/admin/editor/
viewer) por projeto, kill-switch (`Enabled`), targeting rules, OFREP
(eval, ETag/304, SSE — ver `docs/ofrep-realtime.md`). Nada abaixo está
implementado ainda — é levantamento de gap, não plano de execução.

## Compliance / auditoria (bloqueador)

- **Audit log imutável** — quem mudou o quê, quando, valor antes/depois.
  Hoje só existe `FeatureFlag.UpdatedAt`, sem histórico. Sem isso não passa
  certificação (SOX, PCI-DSS, regs locais de banco central).
- **Aprovação em duas etapas (four-eyes principle)** — editor propõe, outra
  pessoa aprova antes de ir pra prod. Hoje `editor` já aplica direto
  (`middleware.RequireProjectRole(roleResolver, constants.RoleEditor)` em
  `flag_routes.go`), sem gate de aprovação.
- **Versionamento/rollback** — reverter uma flag pra config anterior sem
  reconstruir manualmente. Não existe hoje.

## Isolamento e ambientes

- **Ambientes (dev/staging/prod) por projeto** com promoção controlada.
  Hoje só existe o conceito de "projeto" — sem separação nativa de
  ambiente, sem pipeline de promoção entre eles.
- **Data residency** — banco internacional pode exigir dado em região
  específica (GDPR/LGPD/regs locais). Hoje é um Postgres único,
  single-region.

## Throughput (req/s) e alta disponibilidade

Estado atual verificado no código (não hipótese):

- **Pool de conexão Postgres não configurado** — `backend/internal/db/db.go`
  chama `gorm.Open(postgres.Open(dsn), &gorm.Config{})` sem
  `SetMaxOpenConns`/`SetMaxIdleConns`/`SetConnMaxLifetime`. Sob carga, usa
  defaults do driver — sem limite superior explícito nem tuning pro volume
  esperado.
- **Zero cache** — todo eval OFREP (`Evaluate`/`EvaluateAll` em
  `flag_service.go`) bate direto no Postgres via `FindByKey`/`List`
  (`flag_repository.go`), com preload de Variants+Rules a cada chamada. Pra
  automação interna de banco (potencialmente milhares de serviços internos
  checando flag em hot path), isso vira gargalo cedo. Grep confirma: nenhum
  Redis ou cache de query existe no repo hoje.
- **ETag/304 ainda bate no banco** — `notModified()` em `ofrep_routes.go`
  chama `Version()`, que faz `COUNT(*)` + query do `updated_at` mais recente
  **antes** de decidir se retorna 304. Mesmo uma resposta "nada mudou" custa
  2 queries — em alto volume de poll, isso praticamente anula o ganho do
  304.
- **Sem rate limit em `/ofrep/v1/*`** — só CORS + auth por API key
  (`middleware.OFREPCors()` + `middleware.RequireProjectAPIKey`). Rate
  limiting hoje só existe em `/auth/login`/`/auth/register`
  (`middleware.RateLimitPerIP`). Um client mal configurado (poll agressivo,
  retry loop, bug de integração) satura sem barreira nenhuma.
- **SSE é single-instance** (`backend/internal/service/flag_broadcaster.go`
  é pub/sub in-process, `map[uuid.UUID][]chan struct{}` em memória). Rodando
  múltiplas réplicas atrás de LB sem sticky session, cada réplica só vê
  `Publish()` de writes que caíram nela mesma — outras réplicas perdem o
  evento.
- `/healthz` existe (`internal/routes/health_routes.go`) — dá pra usar como
  liveness/readiness probe; vale confirmar se ele testa a conexão com o
  Postgres antes de reportar saudável (senão o orquestrador manda tráfego pra
  instância com DB inacessível).
- HA/DR do Postgres em si está fora do escopo do código LeaFlag, mas é
  pré-requisito operacional pro deploy bancário.

**Recomendação de maior impacto**: cache de flags em memória por processo,
invalidado via o `flag_broadcaster.go` que já existe (o mesmo pub/sub que
hoje só alimenta o SSE passa a alimentar também a invalidação de cache) —
troca "1+ query por request" por "query só quando algo muda". Em
multi-réplica, esse cache precisa ser compartilhado (Redis) — que é também a
peça que resolve o problema de SSE single-instance, então é o mesmo
componente resolvendo os dois problemas (throughput e HA) ao mesmo tempo.

## Segurança corporate

- **SSO/SAML/OIDC** — hoje é email/senha argon2id
  (`backend/internal/constants/constants.go` tem os parâmetros Argon2, auth
  em `service/auth_service.go`). Banco vai exigir integração com IdP
  corporativo.
- **Rate limiting incompleto** — hoje só cobre `/auth/login` e
  `/auth/register` (`constants.LoginRateLimitPerMinute`/`Burst`). Falta
  estender pra criação de project/api-key/member — já listado como
  pendência em `docs/todo.md` ("Rate limiting só no /auth/login").
- **Rotação e escopo de API key** — hoje é bearer simples por projeto
  (`leaflag_sk_...`, ver `middleware/project_api_key.go`), sem expiração/
  rotação automática nem escopo mais granular que "todo o projeto".

## Operacional

- **Webhook/notificação** (Slack, PagerDuty, etc) em mudança de flag
  crítica — não existe.
- **Flags-as-code / provider Terraform** — mudança via PR/git com review,
  não só UI direta. Comum em bancos por exigência de change management
  auditável fora do próprio sistema.

## Infraestrutura AWS recomendada

LeaFlag é um único binário Go (frontend embutido via `go:embed`, `make build`
gera `bin/leaflag`, `make docker-build` empacota tudo numa imagem só) — isso
simplifica bastante a infra: não precisa hospedar frontend separado (S3+
CloudFront), é um container só, stateless por request (exceto SSE, tratado
abaixo).

```
                                   Route53 (DNS)
                                        │
                                  ACM (cert TLS)
                                        │
                                   ┌────▼────┐
                                   │   WAF   │  (rate-based rules, geo-block se exigido)
                                   └────┬────┘
                                        │
                                  ┌─────▼─────┐
                                  │    ALB    │  (subnets públicas, 2+ AZs)
                                  └─────┬─────┘
                                        │
                        ┌───────────────┼───────────────┐
                        │        subnets privadas        │
                  ┌─────▼─────┐                   ┌─────▼─────┐
                  │ ECS Fargate│  ...autoscaling... │ ECS Fargate│
                  │  task (leaflag)│                   │  task (leaflag)│
                  └─────┬─────┘                   └─────┬─────┘
                        │                                │
                ┌───────┴────────────────────────────────┴───────┐
                │                                                  │
          ┌─────▼─────┐                                    ┌───────▼──────┐
          │ RDS Postgres│ Multi-AZ, encriptado (KMS)        │ ElastiCache  │
          │  (primary + │ backups automáticos + PITR         │ Redis        │
          │  standby)   │                                    │ (cluster mode,│
          └─────────────┘                                    │ Multi-AZ)     │
                                                               └──────────────┘
```

- **Compute — ECS Fargate** (em vez de EKS): não há necessidade de
  orquestração k8s pra um binário único sem sidecars — Fargate remove a
  gestão de nodes e escala mais simples. Task definition com o container da
  imagem ECR; healthcheck apontando pro `/healthz` existente.
  - Autoscaling do ECS Service por `ALBRequestCountPerTarget` (ou CPU) —
    como a camada HTTP é stateless (exceto SSE), escalar horizontalmente é
    seguro assim que o cache/pub-sub compartilhado (Redis) estiver no lugar.
  - Mínimo 2 tasks em AZs diferentes, mesmo fora de pico, pra não ter ponto
    único de falha na camada de compute.
- **Load balancer — ALB**: TLS termination com certificado ACM, health
  check em `/healthz`, sticky session **desligada** (não deve ser necessária
  uma vez que SSE passe a usar Redis pub/sub em vez de estado in-process).
- **WAF na frente do ALB**: regras rate-based (mitiga a ausência de rate
  limit em `/ofrep/v1/*` mencionada acima, pelo menos na borda), managed
  rule sets (SQLi/XSS — defesa em profundidade, mesmo com queries
  parametrizadas via GORM), geo-restriction se data residency exigir bloqueio
  de origem.
- **Banco — RDS Postgres, Multi-AZ**: failover automático, backups
  automáticos + point-in-time recovery, storage encriptado via KMS. Read
  replica só se o cache (abaixo) não for suficiente pra tirar carga de leitura
  do primary — com cache bem feito, provavelmente não precisa a princípio.
- **Cache/pub-sub — ElastiCache Redis, cluster mode enabled, Multi-AZ com
  failover automático**: resolve throughput (cache de flags, evita
  round-trip ao Postgres por eval) e HA do SSE (pub/sub compartilhado entre
  réplicas ECS) com o mesmo componente — ver seção anterior.
- **Segredos — Secrets Manager**: credenciais do Postgres, secret de
  assinatura JWT, chave de rotação de refresh token. Rotação automática pra
  credenciais de banco. Nunca em env var plana na task definition.
- **Rede — VPC**: ALB em subnets públicas; ECS tasks, RDS e ElastiCache em
  subnets privadas; NAT Gateway pra saída (pulls de imagem, etc). Security
  groups restritos por porta/origem entre as camadas.
- **Observabilidade — CloudWatch Logs** (driver `awslogs` no ECS task),
  CloudWatch Alarms em métricas de ALB (5xx, latência) e RDS (CPU,
  conexões, storage), X-Ray opcional se quiser tracing distribuído.
- **CI/CD**: build da imagem via `make docker-build` (ou pipeline
  equivalente), push pro ECR, deploy no ECS via CodePipeline/CodeBuild ou
  GitHub Actions com OIDC (sem chave de longa duração).
- **Multi-região**, só se data residency realmente exigir: stack replicado
  por região, Route53 com roteamento geo/latency-based — adiciona
  complexidade operacional significativa, não fazer sem requisito concreto.

## Prioridade sugerida

1. Audit log + aprovação em duas etapas — trava certificação/compliance,
   maior bloqueador pra qualquer avaliação de segurança.
2. Cache de flags (Redis) — resolve throughput e HA do SSE ao mesmo tempo,
   pré-requisito técnico pra qualquer rollout multi-réplica.
3. SSO — mais trabalho de integração que desenvolvimento, mas exigido por
   política corporate antes de qualquer rollout real.
4. Resto (webhooks, flags-as-code, ambientes, rotação de key) — melhora
   operação, não bloqueia adoção inicial.
