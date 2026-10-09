# P34 — Recusa integral local de entrega

Base ab96a99, feat/producao-receitas. SQLite0043 livre nesta base da branch:
0043_production_delivery_rejections.sql. Nenhuma migracao antiga alterada.
Backup acrescenta explicitamente43, preservando25..42, verificacoes de esquema,
checksums, integridade, FKs, aparelho e formato/criptografia.

POST /local/v1/purchase-orders/:id/rejections
Campos obrigatorios operation_id, delivery_reference, unit, delivered_milli,
reason. Sem accepted_milli/local/produto/empresa/loja no corpo. Recusa integral
significa quantidade aceita ZERO; nenhum estoque, reserva ou pagamento muda.
Quantidade inteira1..9007199254740991 na escala milli da unidade congelada
no pedido; sem conversao. Unidade diferente do pedido:409.

Requer autorizacao local P31, manage_replenishment/Orders e manage_stock/
Inventory vigentes. Dados derivados da sessao/aparelho. IDs1..128bytes e
motivo1..255bytes sem bordas/NUL/CR/LF. Max4096bytes e JSON estrito.
Referencia de entrega unica por empresa/loja/fornecedor ENTRE recebimentos
P32 e recusas P34. Disputa entre aceitar e recusar a mesma referencia so
permite uma decisao. A referencia permanece ocupada; nova entrega usa outra.

201 novo;200 replay EXATO por autor/aparelho/loja. Divergencia:409. Recusa
nao preenche pedido, nao reduz restante e nao desfaz aprovacao. Pode documentar
nova entrega totalmente recusada mesmo quando ja houve recebimento completo.
Estado comercial continua local_not_sent: nenhuma confirmacao remota inferida.
Linha imutavel com autor/data/corpo, evento de excecao e outbox
purchase.delivery.rejected schema1 atomicos. Triggers que ignoram escrita
provocam rollback. 400 entrada;401sessao;403papel/modulo;404pedido;409conflito;
413tamanho;503sem emissor;500falha inesperada.

Compartilhados: server.go monta rota; receipts.go passa a conferir referencia
contra as duas tabelas. A auditoria de excecoes e nova; nao altera o CHECK da
purchase_receiving_events antiga. GET consolidado sera acrescentado na P36.
Testes de recusa/replay/sem estoque, limites, permissao, disputa aceitar/recusar,
rollback e backup/restauracao. Somente bancos descartaveis/Fiber App.Test.
