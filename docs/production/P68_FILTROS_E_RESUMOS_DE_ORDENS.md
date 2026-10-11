# P68 — cinco funções de consulta operacional de ordens

Base: `7ac2e8e`, branch `feat/producao-receitas`.

## Funcionalidades

1. Filtrar ordens por **versão imutável da receita**. Também há atalho na versão
   consultada para preencher o filtro; não executa consulta automática.
2. Filtrar por **local de estoque**.
3. Filtrar por **responsável preservado na ordem**.
4. Resumir **produtos planejados** das ordens desta página, por estado efetivo,
   local, produto e unidade, com IDs das ordens e das versões originais.
5. Resumir **ingredientes previstos** desta página a partir das quantidades por
   batida da versão preservada × batidas planejadas, com os mesmos agrupamentos.

Os três filtros são combináveis entre si e com o estado já existente. Uma nova
consulta começa no offset zero; mudar qualquer filtro limpa a resposta anterior.
Paginar preserva os filtros selecionados e validados. Os filtros são opcionais:
campos vazios não são enviados. Quantidades dos resumos são string decimal
inteira em milésimos, acumuladas e multiplicadas com BigInt, sem arredondamento.
Não se juntam kg/g, liter/ml, locais ou estados diferentes no mesmo grupo.

Os resumos indicam claramente página atual versus contagem global da consulta.
Ordens canceladas e concluídas continuam com seu planejamento histórico separado:
isso não demonstra produção atual ou consumo efetivo. Resumos não são capacidade
alternativa, estoque livre, reservas, movimentações ou autorização para executar
produção simultânea. Não há consulta adicional de estoque nem criação de ordem.

## Contrato compartilhado

Sem modificação de backend, migração ou backup. SQLite permanece 44.
O endpoint GET `/local/v1/production/order-search` já aceita `version_id`,
`location_id` e `responsible_id`, além de `status` e `offset`. O backend mantém
permissão de produção, isolamento de empresa/loja e leitura autorizada em contrato
expirado. Sua contagem e itens já usam a mesma transação e predicado.

`productionRead.mjs` amplia `orders(token,status,offset)` com quarto argumento
opcional de filtros. Os três argumentos anteriores permanecem compatíveis.
Só aceita as três chaves previstas; IDs são texto íntegro, sem espaços externos,
NUL/CR/LF e até 128 bytes UTF-8. Contexto de empresa, loja, operador ou aparelho
não entra no corpo nem na URL como novo campo.

O leitor exige eco exato de filtros e offset, itens que correspondam aos filtros,
IDs únicos, ingredientes únicos, tamanho esperado da página e `has_more`
consistente com o total. Uma resposta incoerente é recusada, sem corrigir ou
presumir registros. O hook existente impede resultado atrasado ou de outro token.

`operationsExport.mjs` mantém a exportação da P67 e acrescenta quatro colunas
no final do CSV de ordens: filtros de estado, versão, local e responsável,
retirados da resposta validada, preservando o contexto da página exportada. Os
campos anteriores, quantidades e proteção textual de fórmulas permanecem.

`LocalProduction.tsx`, os tipos e `package.json` recebem a integração dos filtros,
resumos e testes. Não há gravação no navegador nem mudança nas filas de operações.

## Verificação

Testes dos filtros independentes/combinados, UTF-8, recusa de campos indevidos,
eco e referências incompatíveis, paginação, duplicidade, unidades, estados,
locais, versões preservadas e aritmética acumulada acima do limite seguro de JS.
Teste do contexto exportado em CSV. Frontend completo e TypeScript/build.
Backend completo, vet e corrida de consultas filtradas, que abrangem isolamento,
estados efetivos, restauração e concorrência. Testes usam bancos temporários,
sem banco comercial/demonstração. Sem aceite visual em navegador nesta entrega.
