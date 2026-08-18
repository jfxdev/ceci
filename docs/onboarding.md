# Plano de onboarding — LeaFlag

## Objetivo

Levar uma pessoa que acabou de criar uma conta até a primeira configuração utilizável de um projeto LeaFlag: um projeto, ambientes, uma equipe opcional, uma credencial de integração e uma primeira flag ou parâmetro.

O fluxo deve reduzir a tela vazia inicial sem esconder a possibilidade de o usuário explorar o produto depois de configurar apenas o mínimo necessário.

## Princípios

- O onboarding é **opcional e retomável**: o usuário pode pular uma etapa e voltar pela checklist da visão geral do projeto.
- A experiência é centrada no **projeto**. Flags, parâmetros e chaves de API dependem de um projeto e de um ambiente selecionado.
- Campos técnicos permanentes (slug do projeto e chave do ambiente) deixam clara sua imutabilidade antes da criação.
- Ações de produção recebem contexto e confirmação, sem bloquear a criação de projetos menores ou de prova de conceito.
- As ações respeitam os papéis já existentes: `owner`, `admin`, `editor` e `viewer`.

## Entrada e redirecionamento

Após `POST /api/v1/auth/register`, a pessoa já está autenticada. Em vez de redirecionar diretamente para `/projects`, encaminhar para `/onboarding/project` **quando não houver nenhum projeto acessível**.

Se já houver projeto, manter o redirecionamento atual para `/projects`. Usuários autenticados que interromperem o fluxo encontram o ponto de retorno na checklist da visão geral do primeiro projeto criado.

## Fluxo de telas

### 1. Boas-vindas e primeiro projeto

**Rota proposta:** `/onboarding/project`

**Objetivo:** criar o espaço onde a configuração será organizada.

**Campos**

- Nome do projeto — obrigatório.
- Chave do projeto (slug) — preenchida automaticamente a partir do nome, editável antes de salvar e apresentada como identificador usado em integrações.
- Ambientes iniciais — seleção por caixas de escolha; modelos obrigatórios da instância já vêm selecionados.

**Ações**

- Primária: `Criar projeto e continuar`.
- Secundária: `Ir para meus projetos`.

**Estados e validações**

- Slug vazio ou inválido impede o envio.
- Erro de conflito informa que a chave já está em uso e preserva os dados.
- Enquanto salva, impedir novo envio e mostrar progresso.

**Integração atual:** `POST /api/v1/projects`, com `name`, `slug` e `environmentTemplateKeys`.

### 2. Ambientes

**Rota proposta:** `/onboarding/projects/:projectId/environments`

**Objetivo:** confirmar os contextos nos quais a configuração será usada.

Mostrar os ambientes criados pelo template como cartões. A pessoa pode criar outros antes de seguir.

**Campos do novo ambiente**

- Chave técnica (`staging`) — obrigatória e imutável depois de criada.
- Nome exibido (`Homologação`) — obrigatório.

**Orientação de interface**

- Explicar que flags, parâmetros e chaves serão isolados por ambiente.
- Exibir aviso destacado para `prod`/`production`: mudanças nesse contexto afetam workloads reais.
- Não apresentar exclusão como ação de onboarding; a gestão completa continua na página de ambientes.

**Ações**

- Primária: `Continuar`.
- Secundária: `Adicionar ambiente`.
- Terciária: `Pular por enquanto`.

**Integração atual:** `POST /api/v1/projects/:projectId/environments`.

### 3. Equipe (opcional)

**Rota proposta:** `/onboarding/projects/:projectId/team`

**Objetivo:** conceder acesso inicial sem transformar esse passo em barreira.

**Campos por convite**

- E-mail.
- Papel, com `viewer` como padrão.

**Regras de experiência**

- Explicar os quatro níveis de acesso em linguagem simples: `viewer` consulta; `editor` altera configuração; `admin` gerencia projeto; `owner` controla membros e chaves de API.
- Permitir adicionar mais de uma pessoa numa única sessão.
- Sem convite pendente no backend atual, a mensagem deve esclarecer que a pessoa precisa já ter uma conta. O fluxo definitivo de convite por e-mail depende de API própria de convites.

**Ações**

- Primária: `Continuar`.
- Secundária: `Convidar mais alguém`.
- Terciária: `Fazer depois`.

**Integração atual:** `POST /api/v1/projects/:projectId/members`.

### 4. Credencial de integração (opcional)

**Rota proposta:** `/onboarding/projects/:projectId/api-key`

**Objetivo:** conectar o primeiro serviço ao endpoint OFREP ou à leitura de parâmetros.

**Campos**

- Ambiente associado — seletor obrigatório, sem pré-selecionar produção se houver outro ambiente disponível.
- Rótulo da chave — obrigatório; exemplo: `checkout-api-staging` ou `CI`.

**Após criar**

- Mostrar a chave uma única vez, com ação de copiar.
- Exibir um exemplo de cabeçalho `Authorization: Bearer` e link para a documentação OFREP.
- Explicar que revogar a chave interrompe imediatamente os clientes que a utilizam.

**Ações**

- Primária antes da criação: `Criar chave`.
- Primária depois da criação: `Copiei a chave e quero continuar`.
- Secundária: `Configurar depois`.

**Permissão:** apenas `owner` pode criar e listar chaves; para os demais papéis, ocultar a etapa e manter a checklist com a indicação de que um owner precisa concluí-la.

**Integração atual:** `POST /api/v1/projects/:projectId/environments/:envKey/api-keys`.

### 5. Primeira configuração

**Rota proposta:** `/onboarding/projects/:projectId/first-config`

**Objetivo:** orientar a pessoa a perceber valor antes de chegar ao painel.

Apresentar duas escolhas equivalentes:

| Escolha | Quando usar | Destino |
| --- | --- | --- |
| Criar uma feature flag | Ativar, desativar ou liberar uma funcionalidade gradualmente | Editor de flag, pré-preenchido no ambiente escolhido |
| Criar um parâmetro | Guardar configuração de runtime, como `service/db/host` | Navegador de parâmetros, com diálogo de criação aberto |

**Ações**

- `Criar feature flag`.
- `Criar parâmetro`.
- `Explorar o painel`.

O fluxo não precisa duplicar os editores existentes. Deve apenas contextualizar a escolha, definir o ambiente e redirecionar para a tela já disponível.

### 6. Conclusão e checklist persistente

**Rota proposta:** `/projects/:projectId/overview`

Adicionar um cartão `Preparação do projeto` no topo da visão geral, visível até todos os itens estarem concluídos ou serem dispensados explicitamente.

Itens da checklist:

- Projeto criado.
- Ao menos um ambiente configurado.
- Equipe revisada.
- Chave de integração criada.
- Primeira flag ou parâmetro criado.

Cada item aponta para a respectiva tela e mostra o estado atual pela API, não apenas por estado local do navegador. A checklist pode ser recolhida depois da conclusão.

## Regras de retomada

- O estado de conclusão é derivado do backend sempre que possível: existência de ambientes, membros, chaves e configurações.
- A opção `Pular` não deve ser interpretada como conclusão. Ela só impede o redirecionamento automático nesta sessão; a checklist permanece disponível.
- Se o navegador fechar, o usuário volta à lista de projetos. Ao abrir o projeto recém-criado, a checklist oferece a próxima ação pendente.

## Escopo técnico por fase

### Fase 1 — fluxo guiado sem alteração de backend

1. Criar as rotas e páginas de onboarding no frontend.
2. Reutilizar os endpoints e diálogos já existentes para projetos, ambientes, membros e chaves.
3. Alterar o redirecionamento pós-registro para iniciar o fluxo quando não há projetos.
4. Adicionar a checklist derivada no `ProjectOverviewPage`.
5. Cobrir os redirects, permissões e ações de pular com testes de frontend.

### Fase 2 — onboarding completo para equipes

1. Criar modelo e endpoints de convite pendente, com expiração e aceite.
2. Permitir escolher vários ambientes padrão na criação, sem depender apenas de templates administrativos.
3. Guardar preferências explícitas de onboarding dispensado/concluído por usuário e projeto, se a checklist derivada não for suficiente.

### Fase 3 — requisitos corporativos

1. Inserir aprovação em duas etapas para mudanças em produção.
2. Relacionar justificativa e ticket de mudança à criação/alteração de flags.
3. Exibir auditoria, versões e rollback na configuração inicial de produção.
4. Acrescentar expiração, rotação e responsável às chaves de API.
5. Preparar uma entrada de SSO/OIDC quando o provedor de identidade for implementado.

## Critérios de aceite da Fase 1

- Uma pessoa recém-registrada consegue criar um projeto e chegar à visão geral sem depender de conteúdo pré-existente.
- Cada etapa pode ser pulada sem causar bloqueio no produto.
- Nenhuma rota de onboarding permite ação acima do papel atual do membro.
- A chave bruta nunca volta a ser exibida após o fechamento da tela de criação.
- A checklist reflete dados reais após atualizar a página.
- Em telas pequenas, as etapas continuam legíveis e o botão principal fica visível sem exigir interação complexa.

## Referências

- `README.md`: projeto, ambientes, OFREP e parâmetros.
- `docs/todo.md`: estado atual de registro, membros e ambientes.
- `docs/enterprise.md`: requisitos futuros de aprovação, auditoria, ambientes e segurança corporativa.
