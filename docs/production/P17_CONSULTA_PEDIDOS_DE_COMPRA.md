# P17 — Consulta de pedidos de compra preservados

Base efetiva: `f832420`, branch `feat/producao-receitas`, pasta
`~/Downloads/zzeus-frontend-producao`. Esta entrega inicia a consulta operacional
do modulo de compras existente. Nao integra main nem o trabalho de autenticacao.

## Contrato

`GET /local/v1/purchase-order-search?supplier_id=ID&product_id=ID&offset=0`

Todos os filtros sao opcionais. Fornecedor e produto usam igualdade de IDs,
combinados por AND. Produto e consultado no item preservado, sem depender do
nome/unidade atual no catalogo. ID inexistente retorna 200 com `items: []` e
`total_count: 0`, depois da autorizacao. Nao ha pesquisa por nome nem filtros de
estado: o estado real deste modulo e somente `local_not_sent`.

Resultado: `filters`, `total_count`, `offset`, `limit: 50`, `has_more`, `items`.
Cada pedido inclui ID, operacao, fornecedor ID/nome preservado, sugestao,
aprovador, data de aprovacao, data de criacao, estado e item historico completo:
produto ID, SKU, nome, unidade e `quantity_milli`. Cada pedido existente tem
exatamente um item, vindo da sugestao aprovada. Nenhum preco/custo e exposto.

Ordem deterministica: `created_at DESC, id DESC`. Offset entre zero e
9007199254740991 inclusive; somente digitos decimais na API. IDs de 1 a 128 bytes,
sem espacos nas bordas, NUL, CR ou LF. Query vazia, desconhecida ou duplicada e
rejeitada com 400. Valores sao vinculados ao SQL, nunca interpolados.
`total_count` e global dentro dos filtros, independente da pagina. Offset alem
do fim retorna pagina vazia com a mesma contagem. `has_more` usa subtracao para
nao somar inteiros com risco de overflow.

Quantidade e inteira exata na escala milli da unidade registrada: 5000 `unit`
representa 5 unidades. Nao e grama. Unidade `g`, `kg`, `liter`, `ml`, `meter` ou
`unit` permanece como registrada, sem conversao ou soma entre unidades.
Quantidade entre 1 e 9007199254740991 inclusive. Estado desconhecido, unidade
invalida, quantidade invalida ou pedido sem seu unico item retorna 409, sem
publicar uma pagina parcial. O erro reutiliza o contrato de conflitos de compras.

## Autorizacao e consistencia

A rota protegida deriva empresa, loja, identidade e aparelho da sessao.
Reutiliza `purchases.ReadTx` e a permissao `view_orders`, com verificacao atual
de vinculos e aparelho dentro da transacao. Nao muda papeis ou politicas.
Os mesmos papeis do leitor anterior podem consultar seus registros locais,
sujeitos aos bloqueios efetivos. Um usuario com papel `supplier` e um membro
local autorizado; isso nao cria acesso entre empresas.

Preserva a leitura de registros proprios apos vencimento do contrato, como
`GET /purchase-orders/:id`. Consulta nao avanca o relogio de licenca. Contagem,
cabecalhos e itens usam uma transacao SQLite, inclusive perante uma criacao
concorrente. Cursores fecham antes da leitura seguinte. Limite de 50 pedidos e
um item por pedido evita resposta sem limite. Nao ha cache.

Cada resposta e consistente, mas paginacao por offset entre requisicoes nao e
um cursor de exportacao imutavel: novas compras entre paginas podem deslocar
posicoes. Nenhum snapshot persistente e criado.

## Limites da entrega e arquivos compartilhados

Fornecedores continuam referencias comerciais locais da migracao 0023.
`local_not_sent` significa pedido local nao enviado. Esta consulta nao transmite,
confirma, recebe, paga ou autoriza automaticamente pedidos. Nao modifica
compras, aprovacoes, auditoria, estoque, reservas, outbox ou contratos.

O unico compartilhado alterado e `backend/internal/localapi/server.go`: adiciona
`mountPurchaseSearch(protected)` ao lado da montagem de compras. Uma nova rota,
sem mudar `/purchase-orders`, `/purchase-orders/:id`, criacao, autorizacao ou
idempotencia existentes. Conflito nessa linha deve ser revisado na integracao.

Nenhuma migracao nova. SQLite continua 35. Migracoes antigas, backup.go,
checksums, whitelist, ValidateSchema, integrity_check, foreign_key_check,
validacao do aparelho, criptografia e recuperacao permanecem intactos.

## Validacao

Testes em SQLite descartavel e Fiber App.Test, sem servidores ou bancos reais:
filtros isolados/combinados, IDs desconhecidos e texto SQL como valor, nomes e
unidades historicos apos alteracao do catalogo/fornecedor, quantidade maxima,
todas as seis unidades existentes, 53 resultados/duas paginas/offset maximo,
consulta sem efeitos/sem atualizacao do relogio, expiracao de contrato,
autorizacao, isolamento de loja na mesma base, escopo de empresa/aparelho,
inconsistencia de item, criacao concorrente e backup/restauracao do pedido/item.

Fixtures de paginacao populam dados historicos validos em banco temporario.
Nao alteram a regra existente que bloqueia nova aprovacao pendente do mesmo
produto. A criacao concorrente usa outro produto e aprovacao humana normal.

O instalador verifica branch, base, arvore limpa e checksum. Depois testa
backup/CLI, nucleo de compras e API P17, suite completa, vet, race e build do
executavel local com CGO desativado, sem executa-lo. Faz commit somente dos sete
arquivos P17 depois de sucesso. Nao executa merge, push nem reaplica P16.

API completa: `docs/api/purchase-search.openapi.json`.
