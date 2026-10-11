# Entrega45 — propostas humanas de reposição online

Base PC: `ad8ad9e`, main. Cinco funcionalidades:

1. Criar proposta por produto, com quantidade inteira explícita, motivo e snapshot do cálculo44.
2. Consultar a proposta e sua revisão/estado atuais.
3. Aprovar a proposta planejada; rejeitar qualquer mudança de estoque, versão, preço, estado ou planejamento em relação ao snapshot.
4. Cancelar proposta planejada ou aprovada, mantendo motivo, quantidade e snapshot.
5. Recuperar o recibo da própria operação após perda da resposta, inclusive depois de decisões posteriores.

## Transação e concorrência

A empresa vem da sessão. Apenas owner/admin/manager; sessão, vínculo e papel são verificados novamente dentro da transação, com locks compartilhados. Reutilização do operation_id com outro ator, conteúdo ou ação retorna409. Replay devolve o recibo original antes de comparar a revisão atual; nenhuma duplicação de eventos. O ID da criação é o ID da proposta. Revisão inicial1, incremento em cada decisão. Revisão concorrente não sobrescreve outra decisão.

A criação bloqueia e lê o produto, valida expected_version e exige recomendação needs_replenishment. O gerente pode escolher quantidade diferente da sugestão, de1 até2147483647 unidades, com motivo. A quantidade escolhida não muda nas decisões. Aprovação bloqueia proposta/produto e compara todo o StockPlan; divergência exige nova proposta. Cancelamento não depende do produto estar ativo ou não excluído logicamente. Recibos históricos preservam snapshots; a consulta da proposta informa o estado atual. Recuperação do recibo é restrita ao seu ator, mesmo dentro da mesma empresa.

Proposta, operação auditada e outbox são escritos na mesma transação. RowsAffected, leitura de volta e conteúdo do outbox são conferidos; falha, inserção ignorada ou dados alterados provocam rollback. Outbox é registro durável pendente, não comprovação de envio externo.

## Esquema

PostgreSQL incremental000019, dependente do planejamento18. Tabelas online_replenishment_proposals, online_replenishment_operations, online_replenishment_outbox e ledger online_replenishment_migrations com checksum19. Histórico divergente/futuro é recusado. Manutenção explícita `titan-online migrate-replenishment-proposals` / `check-replenishment-proposals`; API não aplica DDL na inicialização. Installer executa migração apenas em banco novo *_test. SQLite, backup local e branch de produção não são alterados.

## Limites do aceite

Não há pedido ao fornecedor, reserva, compra, baixa/entrada de estoque nem cancelamento de execução externa. Propostas independentes do mesmo produto são permitidas; antes de futura integração com compra será necessário controlar duplicação de execução. Não se implementou dupla aprovação por pessoas distintas. Interface e fluxo mercado–fornecedor ainda dependem de integração específica.

## Verificação

Testes de parser estrito, snapshot adulterado, transições/revisão, replay, permissões, sessão revogada, rollback de outbox e leitura de volta. Rotas reais montadas e contratos OpenAPI conferidos. O cenário PostgreSQL do installer cobre migração idempotente/checksum, ciclo criar→aprovar→cancelar, recuperação histórica, isolamento por empresa/ator, concorrência entre aprovação/cancelamento, mudanças do produto, outbox ignorado e preservação de catálogo/custos. Gatilho de falha fica restrito ao próprio cenário. O teste real exige PostgreSQL isolado; testes com sqlmock não o substituem.

## Correção do cenário PostgreSQL45

A sessão stock de teste omitira created_at, obrigatório na migração5 e sem valor padrão. A fixture passa a registrar clock_timestamp() para criação e expiração uma hora depois. Nenhuma restrição de sessão, autorização, DDL ou função comercial foi flexibilizada. Todos os cenários posteriores continuam obrigatórios, incluindo concorrência, outbox ignorado, exclusão lógica e ausência de alteração de estoque.
