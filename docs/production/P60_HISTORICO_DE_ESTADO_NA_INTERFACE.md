# P60 — histórico auditado do estado do catálogo na interface

Após P59, consulta GET /local/v1/catalog/products/{id}/state-history?offset=N. Backend já existente devolve estado atual, revisão, total_count, limit=50, offset, has_more e itens na mesma transação. Interface preserva totais globais em página parcial, verifica quantidade da página, sequências e alternância entre active/inactive desde estado inicial active revisão zero. Exibe motivo, operador, loja, aparelho, operação e data. Eventos de outras lojas da mesma empresa não são descartados, porque estado do produto é compartilhado pela empresa.

Sem escrita, cadastro, estoque ou cancelamento de operações. Histórico inexistente não é aprovação de qualidade/comercial. Respostas recusadas não viram listas vazias bem-sucedidas; token ou produto alterados descartam resposta antiga. Não há total calculado a partir somente da página.

LocalCatalog inclui painel, package.json registra testes; novos cliente e tipos não mudam backend ou fila. Schema 44 e backup P58 preservados. Testes cobrem histórico vazio, 51 revisões, página fora do total, respostas inconsistentes, duplicidade, contexto e recusas; suíte frontend/build antes de commit.

Situação: operações e consultas principais da produção agora têm interface com proteção de resposta perdida. Este lote acrescenta edição/estado/histórico ao catálogo. Ainda restam operações avançadas de estoque e recebimento pela interface, exceções comerciais de compras quando definidas, aceitação visual no PC e integração revisada das branches. Não se infere percentual de esforço pelos números Pxx.
