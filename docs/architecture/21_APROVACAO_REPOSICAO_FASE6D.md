# Fase 6D — Aprovação humana de sugestão

`replenishment.Review` exige gerente ou dono com aparelho aprovado. A decisão, motivo, responsável e evento outbox entram em uma transação. Repetir o mesmo ID é seguro. Para aprovar, a revisão da política e o saldo local devem continuar iguais aos observados na sugestão. Uma aprovação pendente do mesmo produto impede outra; sugestões defasadas podem ser rejeitadas e refeitas.

O estado `approved` **não é pedido enviado**. Ainda não existe fornecedor vinculado, confirmação B2B, liberação de dados autorizada nem recebimento. Até haver ciclo de pedido e baixa da aprovação pendente, não há segunda aprovação do mesmo produto. O saldo observado continua sendo local ao dispositivo e requer conciliação entre aparelhos.
