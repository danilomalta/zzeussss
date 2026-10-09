# P38 — Interface de saldo e origem das reservas

Rota /local/stock abre consulta por ID de produto/local. Cadastros, locais e
entradas existentes continuam acessiveis por /local/catalog, com link na pagina
para quem possui view_catalog. Nova rota permanece sujeita a area stock.

Saldo fisico, reservado global e livre apresentados separadamente, unidade
explicita, sem confundir quantidade_milli com gramas. Detalhes exigem tambem
manage_production, como a API. Cada pagina tem ate50 reservas e saldo global;
nao somar a pagina como total. Exibe ordem, versao, responsavel e data.
Todas as chamadas GET. Sem nova migracao ou mudanca de backend/schema44.

Mudancas compartilhadas: router/index.tsx troca componente da rota stock;
package.json acrescenta teste sem dependencia. Integracao deve revisar destino
stock e preservar acesso ao catalogo. Nenhuma alteracao em login ou sessoes.

Testes frontend e build TypeScript/Vite executados localmente. Validacao inclui
contexto, unidades, fisico=reservado+livre, overflow, origem duplicada, totais
paginados, permissao recusada e consulta sem POST. Sem DB/servidor reais.
Aceite visual e tarefas manuais continuam pendentes. Movimentacoes e cadastros
avancados do catalogo nao sao declarados completos por esta consulta.
