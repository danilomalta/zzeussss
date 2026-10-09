# Entrega 30 — Reajuste percentual versionado do catálogo online

Base main 5fb271d limpa. Não altera SQLite, produção, frontend ou bancos comerciais. Quatro APIs completam prévia, aplicação, recuperação e histórico do reajuste.

## Seleção e percentual

Owner/admin/manager autenticado da empresa fornece operation_id, reason, rate_basis_points e items com product_id/expected_version. A lista é explícita, até 100 produtos, sem duplicatas. Pode usar a consulta/exportação anterior para escolher produtos e conferir versões; não recalcula a seleção por filtro na hora de aplicar.

100 basis points = 1%. Exemplos: 500 = +5%, -125 = -1,25%, -10000 = -100%, 10000 = +100%. Apenas inteiro entre -10000 e 10000, excluindo zero. Não aceita preço novo, float, percentual decimal em string, ator, tenant, status ou categoria no pedido.

Preço final = (preço_em_centavos × (10000 + rate_basis_points) + 5000) / 10000, com divisão inteira. Arredonda metade para cima sobre o preço final, não sobre o desconto isolado. R$1,01 com +50% vira R$1,52; com -50% vira R$0,51. Redução -100% de preço positivo vira zero. Limites garantem multiplicação segura em int64; preço final não pode ultrapassar 999999999999 centavos.

Se um produto não muda por arredondamento, inclusive reajuste de preço zero, todo o lote é recusado como entrada sem efeito; não omite itens nem aumenta versões fictícias. Nome, SKU, ativação e demais campos não são alterados. Um produto inexistente na empresa recusa tudo; versão diferente recusa tudo.

## APIs

| Rota | Resultado |
|---|---|
| POST /api/v1/catalog/adjustments/preview | Antes/depois calculados, motivo, percentual, rounding=half_up e hash, sem escrita comercial |
| POST /api/v1/catalog/adjustments/apply | Mesma seleção/percentual/motivo mais preview_hash; transação única com lote e metadados |
| GET /api/v1/catalog/adjustments/{operation_id} | Recibo original pelo próprio ator/empresa; outro ator recebe 404 |
| GET /api/v1/catalog/adjustments?limit=10&offset=0 | Histórico administrativo da empresa, incluindo outros atores autorizados; até 10 recibos por página |

Prévia usa snapshot repetível. Aplicação confirma sessão no banco, bloqueia identidades filhas e produtos em ordem de ID, confere versões e recalcula os preços. Hash vincula seleção normalizada, motivo, percentual, ator e snapshots. Ordem diferente na lista do mesmo pedido não cria conteúdo diferente, pois os IDs são normalizados em ordem. Resultado JSON também é ordenado por ID.

Replay idêntico confere identidade/ator/hash e devolve o recibo antes de consultar preços atuais. Funciona após edição posterior ou reversão; não aplica percentual novamente. Outro motivo, percentual, seleção ou ator com mesmo UUID gera conflito. Identidade ocupada por lote não relacionado não pode ser rotulada como reajuste.

Em timeout/perda da resposta, preservar UUID e pedido; consultar o recibo antes de repetir. 503 não comprova ausência. Commit incerto não devolve sucesso inventado. Histórico guarda percentual, motivo, ator, timestamp e snapshots de cada item; não soma preços ou recalcula valores com base no cadastro atual.

## Auditoria, reversão e limites reais

Reusa transação dos lotes: preço exato, nova versão, auditoria e outbox individual. Reajuste e seu evento adicional são gravados na mesma transação. Falha tardia/supressão de outbox impede commit e deixa todos os produtos e versões intactos.

Reajuste é lote normal e pode ser compensado pela API de reversão da entrega 27. Preserva original, exige motivo e prévia, incrementa versão, não apaga auditoria nem sobrescreve edição posterior. Outbox é intenção durável; não afirma que os preços chegaram aos caixas.

Autorização atual é do usuário owner/admin/manager que solicita e aplica. Não implementa aprovação independente por uma segunda pessoa, aprovação automática por percentual, validação de margem negativa, custo, promoção ou agendamento. Essas políticas exigem dados/regras específicos e não devem ser anunciadas como concluídas. Até -100% é aceito explicitamente por ator autorizado e motivo obrigatório; prévia deve ser conferida antes de aplicar.

Corpo JSON até 65536 bytes; motivo até 500 caracteres, sem controles; IDs canônicos e versões seguras; campos duplicados/desconhecidos/null são recusados no objeto e nos itens. Sessão/empresa derivadas do servidor. Stock/cashier/accountant/employee não reajustam. GET histórico aceita somente limit/offset, padrão 10, máximo 10, offset até 1000000000; não existe exportação ilimitada do histórico.

## Migração e testes

PostgreSQL 000013_online_catalog_adjustments.sql cria adjustments e adjustment_outbox com vínculo ao lote, ator da empresa, motivo, percentual e snapshots; índice do histórico. Histórico próprio versão 13/checksum, transação, bloqueio e proteção contra futuro/adulteração. Requer esquema 12. Manutenção explícita `titan-online migrate-catalog` agora aplica 8→9→10→11→12→13; check-catalog verifica sequência. Inicialização da API não migra.

Testes: matemática exata e limites, no-op, seleção repetida/sem versão, entrada ambígua, prévia sem escrita, autorização/revogação, hash, identidade, falhas de metadados/outbox/commit, replay e recibo isolado. Suite PostgreSQL inclui ciclo HTTP, histórico, reversão, resposta perdida, rollback tardio, replay concorrente, redução exata a zero e proteção da migração; fases usam novo app HTTP de fixture, mantendo limite ativo.

Preparação verifica suite Go, vet, race, builds, contratos de 96 rotas e patch na base exata. PostgreSQL real não está disponível aqui; aplicador exige suite catalog em banco novo isolado no PC antes de commit. Não faz push, migração comercial ou servidor. Em falha preserve alterações e saída; não reaplique automaticamente.
