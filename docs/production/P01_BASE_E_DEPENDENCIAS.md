# Produção — P01: base e dependências

Inspeção de 07/10/2026. Base informada pelo proprietário:
`8fdf5fe77ed11360df56a4f4c569bd8a8621e402`, branch `main`, árvore limpa.
O pacote foi criado com `git archive HEAD`; não contém `.git`. Não há
histórico Git local para confirmar o hash por conta própria neste ambiente.
O código contém a entrega de sessões online e a migração PostgreSQL 000005.

## Escopo e regras

Lido `docs/architecture/PROJECT_RULES.md`. Árvores oficiais: `backend/` e
`frontend-web/`. Nenhuma alteração nas árvores duplicadas, autenticação,
sessões, banco real, migrações, rotas ou frontend nesta entrega.

Produção será implementada primeiro no núcleo SQLite. A API PostgreSQL
exige implementação e aceite próprios; não será considerada atendida pelo
serviço local. O esquema histórico `item_recipes` em
`backend/db/migrations/000001_init.sql` não é serviço de receitas versionadas:
não foi encontrada implementação Go que o utilize. Essa migração contém
comandos destrutivos e NÃO deve ser executada.

## Inventário fundamentado no código

| Dependência | Arquivos atuais | Comportamento e consequência |
| --- | --- | --- |
| Banco e transação | `backend/internal/localdb/localdb.go`, `migrate.go` | SQLite privado, WAL, FULL, chaves estrangeiras e uma conexão por processo. Cada migração tem checksum; não editar arquivos já aplicados. |
| Catálogo | `backend/internal/localdb/catalog/catalog.go`, `migrations/0007_product_details.sql` | Unidades `unit`, `kg`, `g`, `liter`, `ml`, `meter`; produtos por empresa, locais por empresa/loja. Local `production` já existe como tipo, sem implementar produção. |
| Movimentos | `backend/internal/localdb/stock/stock.go`, `migrations/0006_stock_operations.sql` | Quantidades `int64` em milésimos da unidade; saldo por soma de movimentos. Operações entry/transfer/loss, vínculo dos movimentos e outbox na mesma transação. |
| Autorização | `backend/internal/localdb/identity/authorize.go`, `operate.go`, `effective.go` | `ManageProduction` já existe. Revalidar vínculo, loja, dispositivo aprovado e política na transação. IDs de contexto vêm da sessão, nunca do pedido. |
| Contrato | `backend/internal/core/modules/modules.go`, `backend/internal/localdb/entitlementstore/store.go` | `Production` depende de `Core` e `Inventory`. `RequireTx` já verifica permissão, assinatura, vigência e recuo do relógio. Reutilizar, sem caminho novo que dispense contrato. |
| Perfil de produção | `backend/internal/localdb/identity/authorize.go` | Papel production pode gerir produção, consultar catálogo/pedidos; não recebe ManageStock automaticamente. O consumo autorizado pela ordem deve ter regra específica de produção, sem ampliar o papel globalmente. |
| API local | `backend/internal/localapi/server.go`, `capabilities.go` | Contexto de sessão e capacidades já disponíveis. Não há montagem de rotas de produção. A futura chamada mountProduction é alteração compartilhada a coordenar. |
| Histórico de negócio | `backend/internal/localdb/purchases/purchases.go`, `sale/sale.go`, `sale/cancel.go` | Referências de operações estáveis, snapshots, autorização transacional e compensação. Reutilizar padrões, não assumir que atendem produção. |
| Transporte | `backend/internal/localdb/outgoing/outgoing.go`, `incoming/protocol.go`, `incoming/receive.go` | knownType aceita eventos de venda/estoque/caixa, não produção; receiver registra recebimento, sem aplicar o módulo de produção. Outbox não significa reconciliação concluída. |
| Migrações | `backend/internal/localdb/migrations/`, `backend/db/migrations/` | SQLite termina em 0027_access_policy.sql; PostgreSQL contém 000005_online_sessions.sql. Próximo número NÃO reservado por esta inspeção. |

Não foram encontrados no núcleo local serviços/tabelas de receitas versionadas,
ordens de produção, reservas de ingredientes, etapas, resultados ou lotes.
Isso foi conferido no código e migrações, não deduzido de telas ou nomes de pastas.

## Quantidades e transações

`quantity_milli` representa milésimos da unidade cadastrada, não gramas
universais. Exemplos: produto em kg com quantity_milli=4000 representa 4 kg;
produto em g com quantity_milli=4000 representa 4 g.

Proposta para P02: ingredientes de massa expressos em gramas inteiras positivas,
com entrada em kg convertida exatamente para gramas. Para produto em kg,
G gramas correspondem a G unidades de quantity_milli; para produto em g,
correspondem a G*1000, verificando overflow antes de multiplicar. Saldo de
produto em g pode conter frações de grama; não arredondar para cima a
disponibilidade utilizável. Validar limites também nos futuros contratos JSON.
Óleo em liter/ml não vira massa sem densidade explícita; inicialmente rejeitar
essa conversão. A unidade deve ficar preservada na versão da receita.

Rendimento terá unidade explícita (unidades inteiras de produto ou gramas para
produto de massa), separado do peso dos ingredientes. Escalonamento e eventual
arredondamento conservador serão definidos/testados na P03; não perder frações
silenciosamente. Alternativas de capacidade são simulações independentes; só
reserva transacional compromete saldo.

`stock.RecordWithContract` abre e confirma sua própria transação. Não chamá-lo
em laço para consumir ingredientes de uma ordem: isso permitiria consumo
parcial. A P05 precisa confirmar reserva/consumo, movimentos, vínculo à ordem,
histórico/auditoria e outbox em uma transação, mediante interface transacional
coordenada ou implementação própria revisada. Não chamar db.Query/Exec dentro
de transação ativa: há apenas uma conexão e isso pode bloquear.

Reservas só serão efetivas se todas as saídas concorrentes relevantes
(venda, perda, transferência, contagem e outras ordens) respeitarem o saldo
reservado. Logo, reserva exige coordenação com estoque/PDV; tabela isolada não
constitui proteção. P08 depende de lotes no estoque, ainda ausentes no núcleo.

## Contrato proposto para a próxima etapa (ainda não implementado)

- Receita identificada por empresa/loja/ID, com produto de saída.
- Cada alteração cria versão imutável; ordem futura referencia exatamente a
  versão utilizada. Guardar unidade, quantidades e rendimento daquela versão.
- ID estável de operação e revisão esperada; repetição idêntica devolve resultado
  durável, conteúdo diferente conflita. Duas atualizações da mesma revisão não
  podem se confirmar como uma só versão.
- Cadastro autorizado por ManageProduction e Production via RequireTx; produto
  e ingredientes conferidos na empresa autenticada.
- Versão, ingredientes, registro de operação, auditoria e evento versionado de
  produção confirmados juntos; falha reverte tudo.
- Consulta de resultado para resolver perda de resposta sem repetir escrita
  automaticamente. Política de leitura após vencimento será explícita.
- Sem API publicada nem mudanças no frontend nesta P01. URLs, DTOs e códigos
  de erro serão documentados quando o serviço correspondente estiver pronto.

## Coordenação com a fundação

Solicitação para o outro chat/integrador: informar a base atual e reservar o
número de uma nova migração SQLite para receitas versionadas. 0028 é apenas o
próximo número observado, NÃO uma reserva. Não usar PostgreSQL 000005, já
pertencente às sessões online.

| Etapa | Arquivos/contratos compartilhados a coordenar |
| --- | --- |
| P02 | Nova migração em backend/internal/localdb/migrations; compatibilidade com migrate.go/backup. Proposta de tabelas de receita, versões, ingredientes, operações e auditoria. Nomes finais ainda não definidos. |
| P05 | Estoque, venda, contagem e saldo reservado; interface de escrita com sql.Tx; eventos de consumo/resultado. |
| P08 | Modelo de lotes, validade, saldo por lote e rastreabilidade de movimentos. |
| P10 | localapi/server.go e documentação OpenAPI; capabilities.go só se houver necessidade real. |
| Transporte futuro | incoming/protocol.go, permissões de pares, recepção e aplicação de eventos de produção; coordenar antes de afirmar sincronização. |

Área própria prevista: `backend/internal/localdb/production/`, handlers/testes
em novos arquivos `backend/internal/localapi/production*.go`, documentação
`docs/production/` e OpenAPI próprio. Não alterar middleware, identity,
entitlementstore, autenticação ou sessões para liberar operações.

## Verificações e estado

P01 é inspeção e documentação; nenhuma funcionalidade de produção foi criada.
Go não está disponível no ambiente de inspeção, portanto go test/go vet/race
e builds Go NÃO foram executados. Nenhum teste PostgreSQL real ou demonstração
de API foi executado. A conferência adicional do esquema usa SQLite Python
em memória; não substitui o migrador Go nem testes dos serviços.

| Etapa | Estado | Commit | Testes/pendência |
| --- | --- | --- | --- |
| P01 | Inventário concluído; aplicação no PC pendente | Ainda não criado | Conferir patch, branch e base no PC; confirmar coordenação da migração. |
| P02 | Próxima etapa | — | Persistência depende de migração coordenada; versões, isolamento, conflito, replay, rollback e reinício terão testes. |
| P03–P12 | Pendentes | — | Nenhuma capacidade, ordem, reserva, etapa, resultado, lote, sugestão, API ou UI anunciada como concluída. |

Aplicar em worktree/branch própria ancorada na revisão indicada, sem push.
O pacote de fonte não traz mudanças posteriores: se a base mudar, inspecionar
antes de integrar. Utilizar apenas bancos descartáveis e caminhos de teste.
