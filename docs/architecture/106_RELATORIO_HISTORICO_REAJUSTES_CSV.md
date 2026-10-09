# Entrega 31 — Relatório histórico de reajustes percentuais

Base PC main b83bf30, entrega 30 validada em PostgreSQL real. Dois GETs completam prévia JSON e download CSV dos reajustes percentuais. Não alteram produto, auditoria, estoque, SQLite, produção ou frontend. Não adicionam migração; dependem do esquema PostgreSQL 13 anterior.

## Consulta e filtros

- GET /api/v1/catalog/adjustments/exports/preview: página JSON com CSV, recibos originais, contagem de operações/linhas, hash e timestamp da geração.
- GET /api/v1/catalog/adjustments/exports/csv: os bytes do CSV como anexo, metadados em headers X-Catalog-*; cache no-store, nosniff e nome fixo de arquivo.
- Owner/admin/manager da empresa autenticada podem consultar todos os responsáveis dessa empresa. A API revalida sessão/papel/empresa dentro da transação. Stock/cashier/employee/accountant não exportam. Não aceita tenant fornecido pelo cliente.
- actor_id opcional é UUID canônico não nulo. product_id opcional é inteiro positivo canônico. Nenhum filtro é SQL concatenado.
- from/until devem ser enviados juntos, em RFC3339 com fuso, ano 2000..9999, limite máximo 366 dias. from é inclusivo; until exclusivo. Sem período, todas as operações são elegíveis, mas somente a página limitada é devolvida.
- limit padrão 10, máximo 10 operações. offset padrão 0, máximo 1000000000. Até 100 produtos por operação: máximo 1000 linhas CSV por página. Campos vazios, repetidos, desconhecidos e corpo GET são recusados.

## Valores e significado

Cada linha inclui UUID de reajuste e da alteração filha, UTC da operação, UUID do responsável, produto, SKU/nome antes e depois, versões, preços inteiros em centavos e decimais exatos, percentual em basis points, motivo e regra half_up. Não consulta o preço atual para reescrever o passado. Uma reversão posterior ou edição não modifica os valores do recibo original.

Com product_id, total conta apenas as operações que continham aquele produto; CSV mostra somente as linhas dele. JSON items preserva o recibo completo de cada operação para não mutilar a evidência histórica. Row_count é linhas de produto; limit/offset/next_offset são operações. Não confundir as duas medidas.

Escopo: somente reajustes percentuais da entrega 30. Alterações individuais, criação, importação CSV, lotes comuns e compensações não ganham rótulo fictício de reajuste. O relatório preserva o reajuste original mesmo compensado, sem indicar que o preço continua em vigor. Não é fechamento financeiro, documento fiscal ou confirmação de sincronização com caixas.

## CSV e segurança

UTF-8, separador ponto e vírgula, LF, aspas/duplicações RFC do encoding/csv, cabeçalho fixo. Todo campo textual recebe um apóstrofo inicial, inclusive SKU, nome, motivo, UUID e data; evita execução de fórmulas e preserva zeros em planilhas. Metadado text_prefix explica a transformação; JSON mantém os textos originais. IDs de produto e versões também recebem prefixo, pois planilhas podem truncar inteiros de 16 dígitos. Preços e percentual permanecem numéricos. CSV é relatório para consulta e não deve ser enviado à API de importação de cadastro. Não promete comportamento idêntico em todos os editores de planilha; importar como UTF-8/ponto e vírgula e conferir o prefixo.

Hash SHA256 identifica somente os bytes do CSV daquela página; não é assinatura nem prova contra adulteração externa. Teto 4 MiB, nomes/SKUs/motivos válidos, recibos e versões conferidos. Linha inválida ou leitura parcial recusa todo o relatório com 503, sem devolver CSV truncado.

## Consistência e testes

Contagem, recibos e timestamp são obtidos em uma transação repeatable read. Ordem estável: created_at DESC, operation_id DESC. Contagem deve bater com o número esperado da página. Estado vazio tem items=[] e apenas cabeçalho CSV. Commit ou leitura incerta não publica um relatório inventado.

Snapshot é por requisição; prévia e download separados podem refletir novas operações. Páginas separadas também podem mudar se houver escrita concorrente, pois não há cursor/snapshot durável de exportação. Comparar source_hash quando precisar saber se os bytes coincidem. Não anunciar exportação global congelada.

Testes de biblioteca cobrem centavos e texto de planilha, filtros de tempo, autorização/revogação, isolamento dos argumentos SQL, paginação, ordem, recibos malformados, contagem divergente, erro de leitura/commit e nenhum resultado parcial. Rotas cobrem filtros estritos e headers. Suite PostgreSQL isolada cobre recibos históricos após reversão, filtro produto/responsável/período, paginação, isolamento de empresa, CSV HTTP e nenhum efeito sobre produtos/outbox.

Aplicador exige Go/vet/race/build, contratos das 98 rotas e PostgreSQL real isolado no PC antes de commit. Preparação local não dispõe de PostgreSQL real; não anunciar teste real local. Não executa push, migração comercial, frontend ou branch de produção. Em falha preserve mudanças e relatório, não reaplique automaticamente.
