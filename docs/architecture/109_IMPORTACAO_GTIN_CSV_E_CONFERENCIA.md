# Entrega 34 — GTIN CSV e conferência

Base PC main 5b2e252, entrega 33 aprovada em PostgreSQL isolado. Quatro APIs, parser estrito, diagnóstico seguro, CSV de conferência, testes e documentação. Sem nova migração, sem frontend, sem branch de produção ou banco comercial.

## APIs

| Método e rota | Uso |
|---|---|
| GET /api/v1/catalog/barcodes/import/template | Download do modelo vazio UTF-8 com cabeçalho product_id;expected_version;code |
| POST /api/v1/catalog/barcodes/import/preview | JSON operation_id, reason e csv como string; prévia normalizada sem escrita |
| POST /api/v1/catalog/barcodes/import/apply | Mesmo JSON com preview_hash da prévia; todas as linhas gravadas ou nenhuma |
| GET /api/v1/produtos/{id}/barcodes/export.csv | Conferência completa dos códigos ativos/inativos de um produto em snapshot único |

Owner/admin/manager/stock autorizados; cashier/employee/accountant não. Tenant/ator vêm da sessão. Query extra/repetida e corpo GET são recusados. O modelo exige sessão vigente e não consulta produtos. Aplicação e leituras revalidam sessão no banco.

## Formato de importação

CSV UTF-8 separado por ponto e vírgula, cabeçalho exatamente product_id;expected_version;code. BOM no início é opcional. LF/CRLF, campos quoted corretamente e linhas físicas vazias são aceitos. Não aceita XLS/XLSX, XML, multipart nem text/csv direto na API: o corpo é application/json com csv string.

Até 16384 bytes de CSV, 32768 bytes de JSON e de 1 a 50 produtos distintos. Um novo GTIN por produto no lote. Nenhuma linha de produto inválida é ignorada. Não converte decimal, notação científica, fórmulas, espaço ou sinal em identificador/código. IDs/versões são inteiros decimais canônicos, sem zero inicial; versões de entrada inferiores ao limite seguro. GTIN é string ASCII 8/12/13/14 com dígito verificador; preserva seus zeros.

Produto e chave canônica GTIN repetidos geram erro de linha. Cabeçalho incorreto, coluna extra/faltante, aspas malformadas e arquivo sem itens são recusados. Limites adicionais da entrega 32 permanecem: até 50 aliases por produto incluindo inativos, GTIN reservado por empresa mesmo inativo e lookup apenas quando código e produto ativos.

O formulário JSON também recusa chaves extras/repetidas, null, valores de tipo incorreto, conteúdo posterior e UTF-8 inválido. Motivo obrigatório até 500 caracteres. CSV inválido retorna 400 com csv_error: line, field e code enumerado. line é a linha física inicial do registro; 0 quando não se aplica. Não retorna a linha, GTIN, SQL ou erro bruto do parser. Conflitos de banco continuam 409 genérico e não expõem códigos/produtos de outra empresa.

## Identidade, prévia e aplicação

CSV é uma entrada alternativa do lote da entrega 33, não uma segunda operação comercial. Após validar, converte às mesmas entradas normalizadas por product_id. A prévia revalida catálogo, versões, capacidade e GTINs existentes; retorna o hash vinculado à empresa, ator, UUID, motivo e snapshots antes/depois.

Apply usa a mesma implementação atômica de cadastro GTIN em lote, com bloqueios, UUIDs filhos determinísticos, incrementos de versão, auditorias, recibos e outboxes individuais e do lote. Erro no último evento desfaz todos os filhos. Não cadastra produtos, altera preços/estoque ou corrige GTIN silenciosamente.

CSV com BOM/CRLF, aspas válidas, linhas físicas vazias ou ordem diferente dos mesmos itens tem a mesma identidade semântica. O hash protege a operação normalizada, não os bytes do arquivo; não persiste a planilha original nem comprova sua procedência. O recibo conserva os valores reais aplicados, responsável e motivo. Alterar motivo, versão, produto, código ou hash com mesmo UUID conflita.

Para recuperar resposta perdida use GET /api/v1/catalog/barcodes/batches/{operation_id}, existente da entrega 33. Após 503 preserve entrada/UUID/hash; não crie outro UUID nem repita automaticamente. Replay CSV ou JSON do mesmo lote devolve os snapshots originais e não publica outro evento. Ator/empresa diferentes não consultam seu recibo. Cadastro individual prévio com o UUID de um filho não pode ser anexado como novo lote.

## Conferência por produto

Exportação inclui até 50 códigos, ativos e inativos, com product_id, product_version, barcode_id, barcode_version, code, canonical_code, barcode_active e product_active. Lista e versão do produto vêm do mesmo snapshot e sessão, sem escrita. Produto ausente/estrangeiro retorna 404. Produto sem códigos retorna somente o cabeçalho, não inventa registros.

CSV servido como attachment com nome constante, text/csv UTF-8, no-store e nosniff. Headers identificam produto, versão, contagem e SHA-256 do CSV; hash detecta alteração, não é assinatura nem garantia de origem. Campos exportados são IDs, versões, GTINs validados e booleanos; valores de fórmula/inválidos não são exportados. Estados do código e produto aparecem separadamente.

Este arquivo é relatório de conferência, não arquivo pronto para recadastro. Já cadastrado continua ocupado inclusive inativo; importá-lo novamente não cria cópia nem reativa. Para montar entrada de novos códigos use o modelo, IDs/versões atuais e novos GTINs conferidos. Inativar/reativar continua pelo lifecycle auditado individual da entrega 32.

Ao abrir no LibreOffice/Excel importe a coluna code/canonical_code como TEXTO. CSV conserva zeros no arquivo, mas não controla a inferência da planilha. Não use fórmula ="código" para tentar preservar zeros: a API recusa. O modelo vazio não contém exemplos que possam ser cadastrados por engano.

## Exemplo

```json
{"operation_id":"11111111-1111-4111-8111-111111111111","reason":"Etiquetas conferidas","csv":"product_id;expected_version;code\n10;2;036000291452\n20;4;4006381333931\n"}
```

IDs/versões são ilustrativos. Envie preview e revise antes/depois. Apply envia o mesmo JSON com preview_hash recebido. Consulte o recibo e confira o produto/exportação. Caso um GTIN ou versão esteja ocupado/obsoleto, corrija a origem e produza nova prévia; não bypassa o hash.

## Testes e estado real

Unidades cobrem zeros/equivalência, BOM/CRLF/quoted, limites 50/51 linhas, fórmulas/coerções, diagnósticos seguros, JSON estrito, template com sessão revogada, estados separados, snapshot completo e tenant. Testes das rotas cobrem papéis, query, corpo e caminho inválidos antes de escrita. Suíte PostgreSQL amplia o fluxo HTTP com download protegido, prévia sem efeitos, CSV→lote, replay entre entradas CSV/JSON, variante de formatação, conflito por conteúdo, recibo isolado, exportação/inativação e rollback tardio de todos os itens.

Preparação executa Go, vet, race, builds, contratos e patch em base limpa. PostgreSQL real não disponível na preparação: aplicar.sh exige a suíte catalog em banco novo isolado no PC antes de commit. Não executa migração comercial, servidor ou push. Esquema necessário permanece o 15 da entrega 33.

Pendentes: importação XLSX/XML, múltiplos novos códigos do mesmo produto em um único lote, edição/transferência/liberação de GTIN, ativação/undo de lote, embalagens e peso/preço de balança, fiscal, frontend, PDV SQLite e sincronização real. Não anuncia backend completo ou prontidão comercial por testes isolados.
