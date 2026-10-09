# P45 — Resultado medido na interface
Continua P44. /local/production consulta a ordem, consumo e etapas pelo trace existente. Só oferece conclusão para ordem aprovada com uma reserva consumida e todas as etapas existentes concluídas. Sem plano de etapas, o requisito de etapas é satisfeito. Resultado existente é apresentado em consulta.

Operador informa quantidade medida, motivo e confirmação explícita. Não presume rendimento planejado. Quantidades exatas em milésimos da unidade explícita, até três decimais; unit deve ser inteiro. Zero informado explicitamente é aceito e encerra sem entrada física. Sobreprodução, overflow, decimais excessivos, notação exponencial e massa/volume incompatíveis não são inferidos. Diferença prevista x medida é documental, não classifica perda.

POST /production/results preserva versão/plano/local da ordem e expected_revision consultada. O servidor confere consumo e movimentos negativos, responsabilidade vigente, etapas e saldo/overflow na transação, e entra produto acabado positivo uma única vez. Interface não chama consumo nem reserva. Resultado conserva op/result/order IDs e quantidade/motivo na fila compartilhada antes do POST. Resposta perdida mantém pendência; GET confirma o recibo original antes de liberar nova gravação. Nenhum envio automático ao montar a tela.

Schema SQLite 44 e backup preservados, sem novos endpoints. Campos dos contratos compartilhados não mudam nesta entrega. Testes de preparo e pendência cobrem zero, precisão, unidades, revisão, bloqueios de estado, resposta perdida/repetição e concorrência; testes backend e backup da P43 continuam garantindo recibos.

aplicar.sh executa todos os testes frontend e build antes do commit. A validação local não abre servidor nem banco real. Sem merge/push.

Ainda fora deste lote: telas de execução de etapas, perdas, lotes/qualidade, demais ações de catálogo/estoque/recebimento e revisão integrada com main. Backend existente permanece disponível; não declara interface inteira finalizada.
