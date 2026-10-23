# Painel de recebimentos pendentes — backend — 22/10/2026

Complemento após Conferência Integrada do Recebimento. Sem migração, frontend,
Electron ou alteração de decisões comerciais. Dois GETs em JSON/CSV.

## Coorte e filtros

Período semiaberto `[from,to)` pela CRIAÇÃO do pedido, não pela chegada, previsão
ou última conferência. Até 366 dias, limites em segundos inteiros RFC3339,
normalizados UTC. Inclui frações de segundo do início; exclui todo o segundo
do limite final. A criação persistida é canônica UTC RFC3339Nano; registros
selecionados com timestamp não canônico falham em vez de inventar uma data.
Ordenação cronológica real por criação e ID, sem erro da ordenação textual
entre timestamps com fração e sem fração.

Filtros opcionais por `supplier_id`, `product_id`. IDs distintos não são unidos
por terem nomes iguais. `state` padrão `pending`; opções `all`, `received`,
`cancelled`, `review`. Pendentes são não cancelados com saldo a receber;
recebidos são não cancelados com saldo zero. Review indica qualquer aviso
local, inclusive ausência de doca: não é prova de erro, culpa ou dívida.

## Totais

Resumo por fornecedor + produto + unidade dos pedidos que passaram no filtro
de situação. Não mistura unidades, produtos, moedas ou outros estabelecimentos.
Os agregados em milésimos são strings decimais exatas e podem ultrapassar o
limite seguro de Number/JavaScript. Valores individuais preservam inteiros
validados do núcleo. Pedido atual continua com UM produto/unidade por pedido.

`remaining_milli_exact` preserva inclusive o saldo histórico de cancelados;
`pending_milli_exact` exclui os cancelados. Não deduz nova compra, cobrança ou
reposição a partir desse total. Accepted exclui entradas estornadas conforme
as regras oficiais anteriores. Não representa valor monetário ou pagamento.

`cohort_orders` e `source_evidence` contam os candidatos antes do filtro de
situação. `matched_orders`, grupos e seus totais contam apenas os selecionados.
Lista paginada `items` não muda os grupos. CSV: cabeçalho, summary, todos os
grupos selecionados e somente os pedidos da página. Proteção contra fórmulas
nos campos textuais. Não exporta o histórico integral dos recibos.

## Transação, limites e integridade

Reutiliza `overviewTx` interno para conferir vários pedidos na MESMA transação
autorizada: sessão, view_orders + manage_stock, aparelho aprovado e empresa/loja.
A consulta antiga `Overview` mantém sua assinatura e comportamento, passando a
delegar ao helper e ainda sendo responsável pelo próprio commit/rollback.
Não altera regra de recebimento, estorno, autorização ou licença.

Limites antes de paginação e do filtro de situação: até 200 pedidos na coorte,
2.000 fontes somadas (recibos + recusas + estornos + reservas de doca), e até
1.000 fontes em cada pedido. Acima: 422 sem resumo parcial. Reduzir página ou
trocar situação não contorna o limite; estreitar período, fornecedor ou produto
pode reduzir a coorte. Um único pedido acima de 1.000 exige tratar seu volume,
não apenas trocar o período do painel. Sem arquivamento automático.

Valida todos os candidatos antes de filtrar situação/página. Evidência quebrada
em pedido que ficaria fora da página também falha 409, sem descartar o pedido.
Quantidades em branco/ausentes não são inventadas como zero.

## Rotas e arquivos

GET `/local/v1/management/purchases/receiving-board` e variante `.csv`.
Obrigatórios from/to; opcionais supplier_id, product_id, state, limit, offset.
limit 1–100, offset 0–1.000.000. Rejeita query desconhecida/duplicada, opção
vazia e corpo GET. Sem cache. Contrato em `docs/api/receiving-board.openapi.json`.

Novos: purchases/receiving_board*.go, localapi/receiving_board*.go, contrato e
este documento. Modifica somente receiving_overview.go na biblioteca para
compartilhar a transação, montagem no handler anterior e mapas de rota presentes.
Aplicador confere os hashes anteriores e para se houver alteração incompatível.

## Pendências preservadas

Sem aceite remoto do fornecedor, pagamento, fiscal, coleta, notificação enviada
ou decisão automática. Avisos são locais e para revisão humana. Ainda requer
tela, demonstração com serviço real/B13 do usuário e medição em computador antigo.
