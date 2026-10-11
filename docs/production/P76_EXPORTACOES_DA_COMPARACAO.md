# P76 — Cinco exportações da comparação de receitas

Base: `ab9acd5`, branch `feat/producao-receitas`. SQLite permanece 44.

1. CSV de nomes, produtos resultantes e rendimentos por batida.
2. CSV dos ingredientes adicionados, com quantidade da versão de comparação.
3. CSV dos ingredientes removidos, com quantidade da versão de origem.
4. CSV dos ingredientes alterados, com ambas as quantidades/unidades e diferença exata.
5. CSV dos ingredientes preservados, com mesma referência, quantidade e unidade nas duas versões.

Os cinco botões ficam no painel da P75. Exportação vazia de ingredientes fica desabilitada; os geradores puros retornam somente o cabeçalho quando não há linhas, sem registros inventados. O resumo de rendimento sempre tem uma linha. Arquivos indicam receita, IDs/revisões das duas versões e direção origem → comparação em cada linha. Comparar com revisão anterior continua permitido e explícito.

## Exatidão e integridade

Usa exclusivamente os dois snapshots já autorizados e carregados na P75. Cada gerador revalida as versões e recalcula a comparação, sem confiar nos arrays ou diferenças derivados fornecidos pelo chamador. Sem HTTP, credenciais, escrita, reserva ou baixa de estoque.

Diferenças usam direção (`increase`, `decrease`, `same`, `incompatible` ou `produto_substituido`), magnitude como numerador decimal textual e denominador decimal textual. Fração representa milésimos da unidade da origem: magnitude física = numerador / denominador / 1000. Reduções não são transformadas em aumentos; não há arredondamento. Mudança equivalente de unidade permanece no CSV de alterados, enquanto preservados exige unidade idêntica.

Incompatibilidade de unidades e produto resultante substituído deixam numerador/denominador vazios, com motivo explícito. Vazio não é zero; zero só aparece em diferença equivalente. Conversão preserva a regra P75: unidade igual ou kg/g e liter/ml, sem massa-volume ou normalização pelo rendimento. Intermediários maiores que o inteiro seguro são preservados como strings BigInt.

CSV UTF-8 com BOM, ponto e vírgula, campos entre aspas e CRLF. Quantidades legíveis com vírgula decimal e três casas, além dos inteiros originais em milésimos. Escape de aspas, proteção existente contra fórmulas de planilha e recusa de NUL preservados. Identificadores não são reconstruídos a partir de nomes. Nenhum campo não selecionado (inclusive token/credenciais) é exportado. O arquivo preserva todos os dígitos; inteiros extensos devem ser importados como texto em planilhas, cuja conversão numérica automática pode arredondá-los.

## Contratos compartilhados

`operationsExport.mjs`: acrescenta cinco nomes explícitos à lista do download local; nomes anteriores e formato permanecem aceitos. `ExportCSV.tsx`: amplia apenas a união dos tipos de exportação. `RecipeComparison.tsx`: acrescenta botões com escopo explícito. `package.json`: adiciona o teste. Sem mudança em backend, banco, migração ou backup.

Nome de arquivo mantém o contrato `TitanSystem_comparacao_<tipo>_pagina_0.csv`; 0 é compatibilidade do componente de download, não paginação de receitas. O conteúdo identifica precisamente as duas versões. O navegador administra o destino/renomeação de downloads repetidos. Blob e link temporários usam o mecanismo local existente, com revogação do URL.

## Verificações

Testes dos cinco CSVs: campos, classificação, frações, extremos, reversão, incompatibilidade, produto substituído, derivações adulteradas, fontes inválidas, caracteres/fórmulas, linhas vazias, nomes e limpeza do download. Frontend completo e build. Backend: testes HTTP de receitas, suíte completa uma vez, vet e corrida dos testes HTTP de receitas. Patch reaplicado em cópia limpa e script verificado para base incorreta/entrega já registrada.

Sem merge/push, alteração de main ou bancos comerciais/demonstração. Sem aceite visual em navegador.
