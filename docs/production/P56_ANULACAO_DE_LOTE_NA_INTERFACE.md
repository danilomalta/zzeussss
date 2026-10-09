# P56 — anulação de lote pela interface

Após P55, consultar lote, conferir identificação e quantidade original, informar motivo novo e confirmar explicitamente anulação. POST /local/v1/production/lots/void preserva operation_id, lot_id, expected_revision=1 e reason na fila existente. GET do recibo lot_void confirma evento original revisão 2.

Lotes anulados não podem ser reabertos ou anulados novamente. Código original continua reservado; nova declaração exige outro lote/código. Atribuição ao resultado é liberada, mas não há retirada, reposição ou estorno de estoque, nem exclusão das avaliações anteriores. Conflitos não atualizam revisão ou enviam nova operação automaticamente.

Somente interface/testes/documentação: LocalProduction inclui tela, package.json inclui testes. Schema 44, backend e backup da base P55 preservados. Testar revisão congelada, recusas, limites UTF8, payload sem ajuste de estoque e recibo original; suíte frontend/build antes do commit.
