# P32 — Recebimento local parcial e diferencas declaradas

Base: commit P31 gerado no PC; ancestral ea15711. SQLite 0042 livre apos
0041 na branch. Migracao 0042_production_purchase_receipts.sql. Backup
acrescenta explicitamente 42; preserva 25..41, checksums, ValidateSchema,
integrity_check, foreign_key_check e aparelho. Formato/criptografia intactos.

POST /local/v1/purchase-orders/:id/receipts. Campos obrigatorios:
operation_id, delivery_reference, location_id, unit, delivered_milli,
accepted_milli, reason. Proibidos campos extra/duplicados, empresa/loja/pedido
no corpo, query parameters e fracao numerica. 4096 bytes de limite.

Requer autorizacao local P31 e, dentro da transacao, manage_replenishment
+ Orders e manage_stock + Inventory vigentes, identidade/aparelho/loja validos.
Quantidades inteiras 1..9007199254740991, na escala milli da unidade EXPLICITA:
1000 g = 1 grama; 1000 kg = 1 kg; nao confundir a escala com gramas.
Unidade deve coincidir exatamente com o item congelado E catalogo atual.
Nao converte g/kg ou ml/l automaticamente e nunca volume/massa. Divergencia
apos edicao de catalogo exige resolucao humana fora desta etapa: 409.

Aceita entregas parciais. accepted_milli <= delivered_milli e <= restante do
pedido. Diferenca (delivered - accepted) declarada no motivo nao entra em
estoque nem reduz restante. Totais entregues acumulados tambem limitados a
MaxQuantity para representacao exata em JSON. Excesso aceito nao e permitido.
Recusa integral (accepted=0), devolucao, estorno, preco, financeiro e fiscal
nao sao implementados; nao apresentar esta etapa como fechamento desses fluxos.

Delivery_reference identifica UMA entrega do fornecedor na loja. Unique por
empresa/loja/fornecedor/referencia, inclusive entre pedidos; impede lancamento
duplo com outra operacao. Novas entregas usam referencias diferentes. IDs 1..128
bytes e motivo 1..255 sem espacos nas bordas/NUL/CR/LF. Datas sao locais UTC,
nao prova de emissao do fornecedor. Referencias nao sao documentos fiscais.

Replay exato autorizado retorna o recibo ORIGINAL com repeated=true, status200.
Nao consulta unidade atual nem recalcula resultado de uma operacao ja registrada.
Nova operacao retorna 201. Modificacao do corpo/autor/loja/aparelho conflita.

Estoque aumenta so pela quantidade aceita, no local da loja autenticada.
Operacao entry, movimento, vinculo, recibo, auditoria e DUAS outboxes
(stock.operation e purchase.received schema1) gravados atomicos. Quantidade
recusada e documental. IDs de estoque gerados, nao reutilizam operation_id do
usuario. FreeTx verifica saldo/reservas anteriores; nenhuma reserva muda e
entrada nao pode mascarar saldo/reservas inconsistentes. Overflow recusa tudo.
Produtos/fornecedores posteriormente inativos nao impedem cumprir pedido
ja autorizado; nao cria nova compra/receita com produto inativo.

receiving_status de consultas anteriores: authorized, partially_received ou
received. Estado comercial status permanece local_not_sent. Sem transmissao
ou aceite externo real. 400 entrada, 401 sessao, 403 papel/modulo, 404 pedido/local
fora do escopo, 409 saldo/restante/unidade/evidencia, 413 tamanho, 503 emissor.
Compartilhados: server.go monta rota; orderStatusTx calcula estado de recebimento
com registros novos. Nenhum escritor antigo de estoque/auth/infra e modificado.

Testes: parciais/diferencas, replay apos completar e trocar unidade, limites,
referencias duplicadas, overflow, permissao, falta de autorizacao, rollback
em cada tabela/outbox, disputa pelo restante, retries e backup/restauracao.
Todos os bancos sao descartaveis; nenhum servidor iniciado.
