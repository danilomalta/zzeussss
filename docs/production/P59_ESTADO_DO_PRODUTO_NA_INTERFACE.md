# P59 — ativação/inativação do produto na interface

Após P58, consulta estado atual e revisão independente da edição. Motivo explícito limitado a 255 bytes UTF8 e sem NUL/CR/LF, confirmação humana e decisão oposta ao estado consultado. POST /local/v1/catalog/products/{id}/state conserva operação e revisão congeladas na fila CatalogPending. Consulta recibo original state; inativação anterior continua inactive no recibo mesmo após reativação posterior.

Estado é do produto na empresa inteira. Não apaga catálogo, receitas, saldo ou histórico. Os bloqueios de novos usos são os já implementados pelo backend; não se presume cancelamento de ordens ou documentos. Em conflito, nenhuma revisão é substituída automaticamente. Operação incerta é preservada, sem reenvio na montagem. Permissão manage_stock e contrato inventory ativo para novas decisões; consultas sujeitas à autorização da API.

Só interface/testes/documentação; LocalCatalog inclui formulário sob fila existente e package.json registra testes. Schema 44, backend e backup P58 mantidos. Testar motivo/estado, revisão independente, payload/recibo imutáveis, resposta perdida e recusa concorrente; suíte frontend/build antes do commit.
