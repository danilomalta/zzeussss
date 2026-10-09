# P57 — registrar parecer humano pela interface

Consulta registro atual do lote, qualidade e primeira página do histórico antes da decisão. Parecer começa sem seleção; aprovação/reprovação, critério e motivo são explícitos e requerem confirmação humana. Revisão congelada é a da qualidade, independente da revisão do lote. Lote anulado ou dados inconsistentes bloqueiam nova avaliação.

POST /local/v1/production/quality-reviews com operation_id, lot_id, expected_revision, status, criterion e reason. Fila existente persiste payload antes do envio e confirma GET de recibo quality original. Resposta perdida nunca produz reavaliação automática. Nova decisão é outro evento/revisão, inclusive quando o parecer é igual; avaliações anteriores não são sobrescritas.

Sem liberação sanitária, bloqueio automático de venda ou escrita de estoque. Autorização, contrato e concorrência continuam validados no servidor existente. Schema 44 preservado, backend/backup da P55 mantidos. LocalProduction inclui tela e package.json registra teste.

Testes cobrem ausência de decisão, critério UTF8, lote anulado, revisão congelada, payload/recibo original, resposta perdida e conflito. Suíte frontend e build antes de commit. Aceitação visual, fluxos avançados de catálogo/estoque/compras e integração revisada com main continuam pendentes; este lote não declara conclusão do sistema inteiro.
