# Entrega41 — histórico auditável de custos online

Base de aplicação: main em d3f0b90, entrega40 validada no PC. A frente de produção permanece separada.

## Cinco funcionalidades

1. `GET /api/v1/catalog/cost-history/summary`: total de alterações, custos iniciais, aumentos, reduções, valores mantidos e quantidade distinta de produtos/responsáveis.
2. `GET /api/v1/catalog/cost-history/entries`: lista paginada com recibo, responsável, motivo, estados anterior/posterior e snapshot do produto.
3. `GET /api/v1/catalog/cost-history/entries/:operation_id`: detalhe da alteração para gestão/auditoria dentro da empresa.
4. `GET /api/v1/catalog/cost-history/products/:id`: histórico paginado e filtrado de um produto, com total e próximo offset.
5. `GET /api/v1/catalog/cost-history/export.csv`: exportação de uma página com filtros, snapshots e proteção de células de texto.

## Semântica e filtros

- A fonte é `online_cost_operations`, criada pela migração17 da entrega39. Nenhum schema ou fluxo de escrita é alterado.
- Nome/SKU vêm do snapshot salvo, não do produto atual. Renomear, inativar ou excluir logicamente um produto não apaga seu histórico.
- `q`: texto de até120 caracteres, busca literal sem curingas sobre nome/SKU históricos. `%`, `_` e barra invertida são escapados; o handler decodifica a URL uma vez.
- `actor_id`: UUID canônico de um responsável. `product_id`: inteiro positivo seguro, apenas na consulta geral. A consulta por produto recebe o ID na rota e rejeita sobrescrita por query.
- `change`: all (padrão), initial, increased, decreased ou unchanged.
- `from` e `until`: RFC3339/RFC3339Nano, anos2000–9999; início inclusivo, fim exclusivo. Ambos presentes exigem from anterior a until.
- O custo inicial tem valor anterior e delta nulos. Custo inicial declarado como zero continua sendo conhecido. Alteração de valor igual é unchanged, com delta0; não é descartada.
- Aumentos/reduções usam subtração de centavos inteiros. Não há cálculo por ponto flutuante nem agregado monetário que ultrapasse a precisão do cliente.
- Papéis permitidos: owner, admin e manager. Cada acesso verifica a sessão persistida, empresa e papel antes da consulta.
- O detalhe de auditoria permite gestores consultar alterações de outros responsáveis da mesma empresa. A recuperação/replay preexistente em `/catalog/costs/operations/:operation_id` continua restrita ao próprio responsável.
- Recibo de outra empresa ou inexistente:404 no detalhe. Lista de produto sem histórico acessível retorna total0, inclusive IDs de outra empresa; não revela existência externa.
- Queries extras, duplicadas, inválidas, valores vazios nos filtros tipados e corpo em GET são recusados antes do acesso de negócio ao banco.

## Consistência, paginação e CSV

Resumo/lista/detalhe usam transação repeatable-read, autorização revalidada e consultas parametrizadas com tenant obrigatório. Lista ordenada por created_at DESC e operation_id DESC. Total e itens pertencem ao mesmo snapshot da requisição. Resultados parciais/inconsistentes são recusados.

Limit padrão50, máximo100; offset de0 até1bilhão. `has_more` e `next_offset` permitem continuação. Página vazia contém items vazio. Cada página seguinte é nova leitura: novas alterações podem deslocar offsets. Não há garantia de snapshot global de exportação entre requisições.

`source_hash` é SHA256 do filtro, total e itens da página, sem timestamp de geração. Serve para identificar o conteúdo; não é assinatura, prova fiscal ou hash global.

CSV UTF-8 com separador ponto e vírgula, aspas padrão CSV, máximo256KiB por página. Colunas: operation_id, product_id, sku_snapshot, nome_snapshot, actor_id, created_at, change, before_cost_cents, after_cost_cents, delta_cents, revision, reason. Custo/delta desconhecido fica em branco. Textos que poderiam ativar fórmulas recebem apóstrofo. Centavos e datas não são convertidos para valores aproximados.

Download inclui no-store, nosniff, nome fixo, total/count/has-more/next-offset, source-hash e escopo online-cost-history. CSV não modifica catálogo, custos, recibos ou outbox.

## Verificação e limites

- Testes unitários: classificação das quatro situações; limites de filtros/datas; autorização; resumo particionado; detalhe ausente; erro de banco; sessão revogada; página parcial; CSV e células especiais.
- Testes HTTP: cinco rotas montadas, papéis negados, duplicação/query extra, ID não canônico e busca literal decodificada uma vez.
- PostgreSQL isolado: duas empresas, dois responsáveis, cinco alterações com todas as classificações, paginação, produto, recibo externo, filtros de ator/datas, limites inclusivo/exclusivo, busca com `%` e produto de controle, CSV, ausência de mutações, snapshot após exclusão e sessão revogada.
- O instalador executa suite Go, vet, race, builds, verificação JSON/rotas, testes do runner Python e integração PostgreSQL em banco novo. Commit somente após aprovação de todas essas etapas.
- Neste ambiente de criação não há PostgreSQL para executar a integração real; essa etapa é obrigatória no PC pelo instalador. Testes que ignoram integração não são evidência de aceite real.
- Custos são valores unitários declarados: não representam custo médio de estoque, CMV, escrituração ou apuração fiscal. Nenhuma interface foi adicionada nesta entrega.
