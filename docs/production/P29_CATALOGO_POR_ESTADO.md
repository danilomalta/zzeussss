# P29 — Identificacao e filtro do estado do produto

Sem migracao; SQLite 40. Contratos de leitura compartilhados:
GET /local/v1/products e GET /local/v1/catalog/search acrescentam status e
state_revision em cada produto. state_revision pertence ao estado P28,
nao a edicao de metadados P23. Campos anteriores e visibilidade de custo
preservados. As rotas continuam exigindo view_catalog; nenhuma escrita.

/catalog/search aceita novo filtro status=active|inactive|all. Ausencia
significa all, preservando o conjunto de resultados antigo. Valor vazio,
invalidos, parametros repetidos ou desconhecidos: 400. Filtro de estado
combina com busca textual/unidade/pendencia ANTES da contagem e paginacao.
50 linhas por pagina; ordenacao sku/id. Busca de custo continua autorizada.
/products continua listando todos para preservar compatibilidade historica;
nao ganha filtro de estado. Interfaces comerciais devem selecionar active
em /catalog/search ou interpretar o estado retornado. A API de venda P28
bloqueia nova venda inativa mesmo se um cliente antigo ignorar o novo campo.

Estado se junta ao produto por tenant/product, nunca por loja; politica de
reposicao continua da loja atual. Leitura nao altera relogio de contrato,
estoque, estado ou auditoria. Consultas de capacidade continuam historicas;
capacidade positiva nao autoriza novo pedido com produto inativo.

Testes: sem estado salvo active/0, filtros combinados, 52 inativos, paginacao,
lista antiga, parametros, custo oculto de caixa e IDs iguais noutra empresa.
Sem frontend nesta entrega: novas telas/fluxo completo continuam sem aceite.
