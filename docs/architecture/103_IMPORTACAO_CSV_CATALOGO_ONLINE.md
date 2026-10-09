# Entrega 28 — Importação CSV do catálogo online

Base de aplicação: main 3360f4d, limpa, após entrega 27. A frente de produção, SQLite, frontend, instalação e bancos comerciais não são alterados.

## Funções entregues

| Função | Comportamento real |
|---|---|
| Validação de planilha | CSV UTF-8 delimitado por ponto e vírgula; cabeçalho e campos estritos; preço exato e versão obrigatória |
| Prévia | Resolve SKUs exatos da empresa; mostra antes/depois; vincula planilha, motivo, usuário e versões por hash |
| Aplicação | Produtos, auditoria individual, outboxes, lote e metadados da importação na mesma transação |
| Recuperação/consulta | Recibo estável do aplicador; consulta administrativa da origem do lote com motivo, ator e hash |

## Formato aceito

```csv
sku;expected_version;preco;ativo
ARROZ-1;1;7,90;
CAFE-2;3;;false
```

Uma linha por SKU e exatamente um dos campos preco/ativo preenchido. Não muda preço e status do mesmo produto em uma única planilha. É necessário usar os SKUs e versões reais consultados na API; o exemplo é ilustrativo.

- 1–100 registros, até 65536 bytes de CSV, UTF-8. Cabeçalho exato. Envelope JSON até 524288 bytes para comportar escapes do CSV; limite de requisição do servidor também continua aplicável.
- SKU aparado, até 100 caracteres, sem controles, obrigatório e exato. Não resolve por nome, código de barras, abreviação, acento aproximado ou fornecedor. SKU ausente: 404; ambiguidade: 409.
- expected_version inteiro positivo canônico, sem sinal, expoente ou zero inicial, menor que 9007199254740991. Não infere versão.
- preco em decimal exato não negativo, até 9999999999.99, com ponto OU vírgula e até duas casas. Sem milhares, moeda, sinais, expoente, espaços ou fração vazia; valores convertidos a centavos inteiros.
- ativo aceita somente true ou false; não aceita sim/não, 0/1, vazios ou null como mudança.
- Campos CSV entre aspas seguem encoding/csv; aspas inválidas e quantidade errada de colunas recusadas. Linhas fisicamente vazias são ignoradas pelo leitor padrão; linhas de dados nunca são descartadas. Linhas duplicadas, ausência de alteração ou versão antiga recusam toda a planilha.
- CSV bruto não é persistido. Guarda fingerprint SHA-256, motivo e snapshots de todas as alterações; não existe endpoint de download da planilha original. Guarde a planilha conferida separadamente se precisar de seu original.

## APIs e fluxo

POST /api/v1/catalog/imports/preview: JSON com operation_id, reason e csv. Prévia em transação de leitura repetível, sem gravação de produtos, auditoria, lote, importação ou outbox. Resultados ordenados por ID do produto, não por linha da planilha.

POST /api/v1/catalog/imports/apply: envie exatamente os mesmos campos e bytes, mais preview_hash retornado. A nova sessão deve ser do mesmo ator/empresa autorizado. Conferência sob bloqueios dos produtos; qualquer mudança desde a prévia recusa o conjunto. Lote normal não pode reutilizar identidade já ocupada para ser rotulado como importação.

GET /api/v1/catalog/imports/{operation_id}: recuperação pelo próprio aplicador da empresa. Depois de perder resposta, consulte antes de repetir o POST. Resultado incerto/503 não autoriza novo UUID; preservar ID, motivo, CSV e hash. Replay de pedido igual devolve snapshots originais mesmo após mudança de SKU, edição posterior ou reversão.

GET /api/v1/catalog/batches/{operation_id}/import: owner/admin/manager da empresa consultam motivo, ator, hash de origem e recibo de lote, inclusive de outro operador autorizado. Outra empresa recebe 404. Não há listagem adicional de importações; histórico de lotes existente e esta consulta permitem localizar a origem.

Autorização owner/admin/manager derivada da sessão, confirmada no banco. Stock, cashier, accountant e employee não importam. JSON desconhecido, duplicado, null, query strings extras, MIME incorreto ou tentativa de informar tenant/ator recusados. Erros públicos preservam a mensagem padronizada, sem conteúdo da planilha/SQL.

## Auditoria, atomicidade e reversão

Todas as operações individuais continuam usando a implementação de lotes: IDs filhos determinísticos, bloqueios em ordem, comparação da versão e preço exato, auditoria e intenção de envio durável. Registro adicional da importação inclui ator, motivo, hash da planilha, hash do pedido, recibo e timestamp real. Evento de importação é persistido com o lote; falha ou inserção suprimida impede commit.

Replay verifica pedido/ator antes de consultar o SKU atual. Conteúdo, motivo ou ator diferente com mesmo UUID: 409. Mesma importação concorrente incrementa versões uma única vez. Outro lote concorrente não ganha atualização parcial.

Importações são lotes normais e podem ser revertidas pela API oficial da entrega 27, sob as mesmas restrições: motivo, preview, valores exatamente iguais ao resultado original, nova versão, original preservado e uma reversão por original. Não há remoção de histórico ou sobrescrita do estado posterior.

Outbox é intenção durável, não confirmação de envio para caixas ou outras empresas. Não existe interface, upload multipart, Excel/XLSX, XML, cadastro de novos produtos, alteração de estoque, fiscal, custo ou fornecedores nesta entrega.

## Migração e verificação

PostgreSQL 000012_online_catalog_imports.sql adiciona imports e import_outbox. Histórico separado versão 12 com checksum, bloqueio transacional, verificação de tabelas e recusa de versão futura/adulterada. Requer esquema de reversão 11. `titan-online migrate-catalog` explicitamente aplica 8→9→10→11→12; `check-catalog` verifica toda a sequência. Inicialização da API não migra automaticamente. Nenhuma SQL SQLite nova.

Testes incluem formato, dinheiro, limites, entrada ambígua, SKU ausente/duplicado, versão antiga, autorização, prévia sem escrita, metadados/outbox/commit falhos, replay sem consulta do SKU atual e isolamento do recibo. Suite PostgreSQL opt-in existente acrescenta ciclo HTTP CSV, reversão, rename após perda de resposta, rollback tardio, replay concorrente, conflito de identidade e proteção de migração.

A preparação do ZIP valida testes Go, vet, race, builds, mapa de 90 rotas e sintaxe do instalador. PostgreSQL real não está disponível no ambiente de preparação; o instalador exige a suite catalog em banco novo isolado no PC antes de commit. Novo app HTTP de fixture por fase mantém o limite 100/min ativo sem excedê-lo artificialmente pela soma das fases antigas.

Se qualquer etapa falhar, preservar alterações e saída; não reaplicar ou descartar dados para liberar commit. Aplicador não executa migração comercial, push ou servidor. A ativação do esquema comercial permanece uma manutenção explícita com backup e procedimento de recuperação verificado.
