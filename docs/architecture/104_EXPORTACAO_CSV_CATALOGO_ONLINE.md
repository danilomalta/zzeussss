# Entrega 29 — Exportação CSV e ciclo de edição do catálogo online

Base main 87e26fc limpa. Complementa importação 28. Sem migração nova; não toca frontend, SQLite, produção ou banco comercial.

## O que funciona

| Função | Resultado |
|---|---|
| Prévia JSON | CSV e produtos exatos com SKU, preço em centavos e versão real |
| Filtros e paginação | Nome/SKU, ativo/inativo/todos, limite, offset, total, indicador e próximo offset |
| Formatos de trabalho | Preço OU ativação por linha, no cabeçalho compatível com importação 28 |
| Download | CSV UTF-8, aspas/delimitadores escapados, hash dos bytes e metadados nos headers |

GET /api/v1/catalog/exports/preview? action=price & q=... & ativo=all & limit=50 & offset=0 (sem espaços no URL real) retorna metadados e CSV. GET /api/v1/catalog/exports/csv usa os mesmos filtros e entrega text/csv em attachment com nome fixo. Exemplo real de formato de URL: `/api/v1/catalog/exports/preview?action=price&ativo=active&limit=50&offset=0`.

Sessão confirmada no banco; owner/admin/manager da empresa. Não permite tenant/ator por query e não habilita exportação para stock/cashier/accountant/employee. Headers Cache-Control: no-store e X-Content-Type-Options: nosniff. Mantém guardas de sessão e limite HTTP. O download autenticado precisa usar Bearer da sessão; não há link público de exportação.

## CSV e fluxo de trabalho

```csv
sku;expected_version;preco;ativo
000123;2;7.90;
```

No modo action=active, a linha é `000123;2;;true` ou false. Versão é a do produto; preço em decimal exato com duas casas e ponto, sem float. Aspas e ponto e vírgula dentro de SKU são escapados pelo encoding/csv. Nenhum nome, descrição, custo, dado fiscal, estoque ou fornecedor entra no CSV; JSON da prévia inclui os campos básicos de Product já autorizados.

1. Consulte/exporte uma página e confira os produtos/versões.
2. Edite somente a coluna preco ou ativo do modo escolhido, preservando SKU e expected_version. Em editor de planilha importe SKU como texto, para não perder zeros iniciais nem converter códigos; o CSV preserva bytes, mas não controla a inferência de tipo de programas externos.
3. Remova as linhas que não serão alteradas. A importação rejeita linhas sem mudança: enviar exportação intacta não cria atualização fictícia.
4. Envie o CSV editado e motivo à prévia da importação 28, com novo UUID para esta operação, e confira antes/depois.
5. Aplique com o hash retornado. Alteração posterior à exportação provoca conflito de versão; nunca force o CSV com versões inventadas.
6. Se perder resposta, consulte o recibo da importação. Reversão segue a entrega 27.

A página vazia contém somente o cabeçalho e items=[], total/has_more correspondentes ao filtro. Não é uma planilha importável até conter registros de alterações. Download retorna X-Catalog-Source-Hash, Total, Row-Count, Limit, Offset, Has-More e Next-Offset (último somente quando há mais). Hash identifica os bytes exportados; não é assinatura, aprovação, ID de operação ou recibo persistido. Não reutilize esse hash como preview_hash da importação: a prévia de importação vincula os valores editados e o ator.

## Limites e consistência

- 1–100 produtos por página (padrão 50), offset 0–1000000000, total filtrado até 1000000000. Cada CSV até 65536 bytes. Uma página corresponde a até um lote importável; não junta páginas ilimitadas na memória.
- Contagem e produtos lidos na mesma transação de snapshot repetível; ordem ascendente por ID. Verifica tamanho/ordem dos resultados antes de entregar e não retorna CSV parcial em falha.
- Snapshots são por página. Páginas consultadas em momentos diferentes podem mudar de posição/conteúdo se houver cadastro/edição; não representa exportação global congelada. O controle de versão da importação protege a edição posterior. Nenhum cursor/snapshot persistente criado.
- q até 120 caracteres, pesquisa ILIKE literal por nome/SKU, escapando %, _ e barra. Sem aproximação por erro/acento. ativo=all/active/inactive; action=price/active. Unknown/duplicate query, action/ativo explicitamente vazios, corpo em GET e números fora do limite são recusados, sem fallback silencioso.
- SKU exato, aparado, até 100 caracteres, sem controles; versão menor que o máximo seguro (9007199254740991). SKU começando com =,+,-,@ é recusado em vez de exportar fórmula ou adicionar prefixo que alteraria seu código. A página inteira retorna 409; nada é omitido silenciosamente. Ajustar o cadastro sob autorização antes de exportar, ou filtrar outra página; não há endpoint para burlar a validação.
- Leitura não grava produtos, auditoria de alteração, recibos ou outbox; generated_at vem do banco. Não afirma entrega de dados aos caixas, backup, Excel/XLSX, XML ou sincronização.

## Verificação e ativação

Não adiciona SQL. Depende do catálogo e da importação anteriores; manutenção comercial continua explícita. Preparação confere testes Go completos, vet, race, builds, mapa de 92 rotas, patch contra base equivalente e sintaxe do instalador. PostgreSQL real não está disponível na preparação: instalador exige a suite catalog em banco novo isolado no PC antes de commit.

Testes cobrem CSV exato, valor máximo, acentos/aspas/semicolon/zeros iniciais, round trip de parser, fórmulas recusadas, empty/beyond pages, contagem inconsistente, erro nas linhas/timestamp/commit, guardas e query estrita. Ciclo PostgreSQL inclui exportar páginas e download, isolamento, editar/reimportar preço e status, no-op recusado, filtros reais e ausência de escrita pela exportação.

Aplicador exige main limpa na base 87e26fc e SHA-256 do patch. Não faz push, migração comercial ou servidor. Em falha preserve alterações e envie saída; não reaplique automaticamente.
