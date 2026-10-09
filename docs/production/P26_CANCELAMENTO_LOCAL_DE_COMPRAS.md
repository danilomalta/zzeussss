# P26 — Cancelamento auditado de pedido ainda local

SQLite 0039_production_purchase_cancellations.sql, livre na branch apos P25.
Sem reconstruir a tabela antiga ou alterar migracoes anteriores. Backup
admite explicitamente schema 39 com validacoes anteriores preservadas.

POST /local/v1/purchase-orders/:id/cancel: operation_id, expected_status
(obrigatoriamente local_not_sent), reason. Corpo estrito ate 4096 bytes,
IDs 1..128 e motivo aparado 1..255 bytes sem NUL/CR/LF. Queries rejeitadas.
Exige manage_replenishment e contrato Orders vigente. Retorna 200 com
order_id, status=cancelled, repeated e evidencia cancellation (ator,
aparelho, operacao, motivo, instante). Replay igual devolve a decisao original;
replay divergente e segundo cancelamento com outra operacao retornam 409.

Registro original permanece local_not_sent. A decisao unica em tabela
separada determina o ESTADO EFETIVO mostrado por Get, Orders, Search e Trace.
Trace acrescenta cancellation opcional, validada contra o request salvo.
Contrato compartilhado dessas leituras: agora admitem cancelled e preservam
os itens, fornecedor, aprovacao e auditoria da criacao. Consumers que
conheciam apenas local_not_sent devem reconhecer cancelled antes de integrar.
API antiga recebe atualizacao documental. server.go monta a nova rota.

Cancelamento e outbox purchase.order.cancelled sao atomicos. Nunca apaga o
pedido, altera a aprovacao, muda estoque ou financeiro. A sugestao continua
ocupada por esse pedido: nao pode criar substituto usando a mesma aprovacao.
Replay da criacao retorna o ID existente e nao reabre um pedido cancelado.
Nao permite reativar, cancelar pedido remoto ou simular envio/confirmacao.
Operacoes remotas ficam para o contrato entre empresas. Outbox so pendente.

Testes: leituras concordantes, evidencia/snapshots, replay e concorrencia,
nao reutilizacao de aprovacao, rollback, validacao, permissoes e backup/restauro.
