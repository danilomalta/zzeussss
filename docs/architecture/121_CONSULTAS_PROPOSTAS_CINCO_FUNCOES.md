# Entrega46 — cinco consultas das propostas online

Base PC main397fcd1, PostgreSQL19 da entrega45. Sem nova migração, SQLite ou mudanças na frente de produção.

1.Resumo: total de propostas e contagens planned/approved/cancelled, filtráveis por busca literal e produto. Não soma unidades de produtos distintos.
2.Lista paginada: estados, busca em nome/SKU histórico e motivo atual, produto, limit1..100 e offset. Ordenação por criação desc e IDdesc; total e próximo offset na mesma transação.
3.Histórico auditado por proposta: recibos imutáveis em ordem decrescente da revisão, com paginação, motivo e responsáveis. Gestores da mesma empresa podem consultar a auditoria, enquanto o recibo individual45 permanece restrito ao próprio ator.
4.Conferência: comparação integral do StockPlan atual com o snapshot, status matches/changed/product_unavailable e indicação can_approve somente quando planned e sem mudanças. Não toma decisão, não atualiza snapshot e não reserva estoque. Exclusão lógica mantém consulta da proposta/histórico e resulta em product_unavailable. Corrupção de dados resulta503.
5.CSV: uma página limitada, UTF8, ponto e vírgula, escape de texto perigoso por apóstrofo, contagens/offset/hash/escopo nos headers. Dados de identificação vêm do snapshot; quantidade escolhida e estado/revisão vêm da proposta atual. Não é pedido nem documento fiscal.

## Consistência e permissões

Somente owner/admin/manager, empresa da sessão e revalidação persistente dentro de transações repeatable-read. Uma chamada de lista conta e lê no mesmo snapshot. Páginas distintas são chamadas independentes; mudanças concorrentes podem deslocar registros. Nenhum cursor/snapshot global foi criado e não se promete exportação completa imutável. Conferência é informativa: aprovação45 continua conferindo revisão e produto sob locks.

Rotas de summary/export são registradas antes do parâmetro proposal_id. Sem corpo nas consultas, sem campos de query desconhecidos/repetidos, sem números vazios nem product_id com zeros à esquerda. Busca usa escaping literal de%/_/barra invertida. Nenhum dado de empresa alheia é retornado; proposta não localizada retorna404.

## Verificação

Testes de paginação com total e hash, proteção CSV e snapshot adulterado, histórico, estados de conferência, dados ausentes, papéis e parser de query. Cenário PostgreSQL real obrigatório no installer: contagens/estados, páginas sem duplicação em conjunto estável, busca literal com controle exclusivo46, histórico após cancelamento e exclusão, sessão/empresa/papéis, CSVHTTP e ausência de mutações em catálogo/custos/operações/outbox. Criações preparatórias usam as funções45 já existentes; consultas46 não gravam propostas.

Ainda não cria vínculo/pedido ao fornecedor, agenda de doca, compra, recebimento ou interface de uso. Não envia eventos de outbox.
