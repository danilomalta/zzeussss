# P54 — consulta de qualidade humana do lote na interface

Após P53, consulta o lote, último parecer e página do histórico. Mantém schema 44 e não modifica backend, backup, auth ou infraestrutura. Não registra pareceres nesta entrega.

Contratos existentes: GET /local/v1/production/lots/{id}, /quality e /quality/history?offset=N. Paginação em revisões crescentes, até 50 avaliações, começando em offset+1. Última avaliação é global, independente da página. Estado não avaliado exige revisão zero e latest=null. Ausência de avaliação não significa aprovação.

Cada retrato histórico deve ser lote declarado revisão 1 com os mesmos metadados imutáveis do registro consultado: ID, resultado, produto, unidade, quantidade, código, fabricação, validade, motivo, criador e data original. O lote pode estar anulado atualmente; avaliações anteriores continuam preservadas. Estado atual deve coincidir entre lote e qualidade. Quando a página inclui a última avaliação, seu conteúdo completo deve coincidir com latest. Número de linhas deve corresponder à revisão global e ao offset. Conflitos entre consultas recusam exibição, solicitando nova consulta.

Critério e motivo explícitos de até 255 bytes, operador, aparelho, operação e data por avaliação. Revisões até 2147483647, sequência íntegra e operações distintas na página. Não se cria parecer a partir de validade, nem se transforma parecer humano em liberação sanitária ou bloqueio de venda. Sem reservas ou movimentos de estoque.

Arquivos compartilhados: LocalProduction inclui painel; package.json inclui teste; productionQuality é cliente novo de leitura. Tests cobrem retratos divergentes, anulação, não avaliado, paginação de 51 avaliações, precisão, revisão/parecer adulterados, concorrência e recusas. Suíte frontend e build necessários antes de commit. Aceitação visual no PC ainda deve ser realizada.

Continuidade: operações de declarar/anular lotes e registrar avaliações pela interface ainda requerem entregas com fila persistida e recibos originais. Cadastro e recebimento avançados, aceitação visual e integração revisada com main também não estão concluídos. Nenhum percentual total é inferido apenas da quantidade de etapas.
