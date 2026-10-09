# P50 — Anulacao auditada de declaracao
Continua P49. /local/production consulta GET /production/losses/{id} e mostra resultado, produto, quantidade/unidade, motivo original, criador e revisao. Apenas recorded/revisao1 permite anular. Motivo novo obrigatorio e confirmacao explicita da classificacao, sem estorno de estoque. POST /production/losses/void existente preserva expected_revision consultada; retorno voided/revisao2.

Anular devolve quantidade a diferenca sem classificacao do resultado, sem repor produto acabado, restaurar ingredientes ou alterar ordem. Registro original, motivo e eventos permanecem; nao reabre perda anulada. Outra classificacao exige nova declaracao autorizada. Autorizacao/contrato/aparelho conferidos pelo backend. Atualizacao concorrente provoca conflito, nunca revisao nova ou reenvio automatico.

Fila compartilha bloqueio entre abas e armazenamento separado por empresa/loja/aparelho/operador. Op/loss ID, revisao e motivo preservados antes do POST. Resultado original confirmado via GET de loss_void. Sucesso limpa consulta; nova decisao requer leitura nova. Resposta perdida nao permite descarte incerto.

Schema SQLite 44 mantido, sem migracao/backup/endpoint novo. Backend e restauracao validados na P49. Testes frontend cobrem input fechado, revisao/estado, payload sem estoque, corrupcao de recibo, resposta perdida e conflito sem substituicao de revisao. aplicar.sh testa frontend/build antes de commit. Sem merge/push/servidor/banco real.
