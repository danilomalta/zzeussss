# P35 — Anulacao integral de recebimento por erro de lancamento

Base: commitP34 no PC, ancestral ab96a99. SQLite0044 estava livre apos0043;
0044_production_receipt_voids.sql. Backup inclui explicitamente44, preserva
25..43 e checksums/ValidateSchema/integrity/FKs/aparelho/formato/criptografia.
Nao modifica migracoes antigas nem tabelas originais por rebuild.

POST /local/v1/purchase-orders/:id/receipts/:receipt_id/void
JSON obrigatorio operation_id e reason. Corpo estrito4096bytes, IDs1..128bytes
motivo1..255bytes sem bordas/NUL/CR/LF. Nao aceita quantidade, unidade,
local/produto/empresa/loja substitutos. Anula integralmente a quantidade ACEITA
no recibo original, com autoria/aparelho/data/motivo e vinculos preservados.

Requer manage_replenishment/Orders e manage_stock/Inventory vigentes, sessao/
aparelho/loja validos. Unidade atual do catalogo deve coincidir com original.
Sem conversao. Replay exato200, mesmo resultado original e repeated=true;
outra operacao para o recibo ou corpo/autor/loja divergente:409. Referencia de
entrega original PERMANECE ocupada mesmo anulada; corretivo usa nova referencia
identificavel de correcao. Replay P32 permanece o recibo original, nao relanca.

Compensacao negativa no local/produto originais somente se FreeTx comporta
quantidade aceita. Produto depois inativo nao impede corrigir lancamento.
Nao apaga entrada, recibo, auditoria ou reserva. Se mercadoria ja foi consumida,
vendida ou reservada e falta saldo livre:409, nenhum efeito. Nao promete rastrear
unidades fisicas fungiveis ou lotes individuais; confere saldo disponivel.

Usa kind='loss' na operacao GENERICA de estoque, que ja representa baixa.
O vinculo com purchase_receipt_voids e outbox purchase.receipt.voided distingue
correcao de recebimento de uma perda comercial. Nao classificar a compensacao
como perda financeira. Evento stock.operation schema1 acompanha movimento.
Correcao, movimento, vinculo, auditoria e outboxes sao atomicos; ignore/falha
em qualquer ponto desfaz tudo. Nada de pagamento, devolucao fisica ao fornecedor
ou confirmacao remota: essa rota corrige registro local lançado por engano.

Total ACEITO efetivo/restante do pedido exclui recibos anulados, reabrindo saldo
para receber corretamente. GET P33 items preserva originais e acrescenta void
quando anulados; cabecalho acrescenta voided_count e
voided_accepted_milli_exact (string decimal: ciclos de correcao podem acumular
acima de Number.MAX_SAFE_INTEGER). accepted_milli/delivered_milli/rejected_milli
passam a representar recibos VALIDOS nao anulados. remaining=planned-accepted.
Nao somar quantidades anuladas com efetivas. Total_count continua contar originais.

Compartilhados: server.go monta rota; receipts.go verifica anulacoes na soma
transacional antes de aceitar outra entrega; receiving_trace.go expõe compensacoes
e totais efetivos. A soma valida recibos e suas anulacoes, nao ignora registro
inconsistente. Historico comercial permanece local_not_sent. Contratos de
leitura estao ampliados explicitamente: revisar clientes na integracao.

Testes: snapshot/replay, saldo pendente/correcao, referencia ocupada, autorizacao,
unidade divergente, rollback de todas as escritas, corrida com outra baixa,
nenhum saldo negativo, backup/restauracao com entradas e compensacoes.
Sem servidores/bancos reais/main/merge/push.
