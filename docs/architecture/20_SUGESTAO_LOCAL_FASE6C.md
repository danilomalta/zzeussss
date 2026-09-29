# Fase 6C — Sugestão calculada localmente

`replenishment.Suggest` soma gôndola, depósito e recebimento da mesma loja no dispositivo, compara com o limite mínimo e, quando abaixo dele, sugere a quantidade para atingir o alvo. Guarda ID, política e saldo observados. Uma repetição devolve o mesmo resultado mesmo que o estoque tenha mudado. Para produto já acima do limite, guarda `not_needed` sem enfileirar pedido.

Uma sugestão não equivale a aprovação, pedido enviado, reserva do fornecedor ou saldo global entre aparelhos. O saldo precisa ser conciliado antes de tomar decisão entre PC e celular desconectados. Nenhum fornecedor tem acesso automático aos dados desta etapa.
