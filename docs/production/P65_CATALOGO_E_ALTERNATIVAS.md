# P65 — Catálogo e planejamento com cinco funções

Base de aplicação: `44c075b`, branch `feat/producao-receitas`.

## Cinco funções

Na página `/local/catalog`:

1. Filtrar produtos por estado: todos, ativos ou inativos. A tabela exibe o
   estado efetivo e sua revisão, sem presumir que todos os produtos estão ativos.
2. Consultar versões de receitas vinculadas ao produto nesta loja: ingrediente
   ou produto resultante, quantidade por lote, unidade histórica e última versão.
3. Consultar ordens vinculadas ao produto: versão imutável, papel, quantidade
   planejada total, local e estado efetivo. Não representa saldo ou consumo atual.
4. Comparar de uma a vinte versões preservadas no mesmo local: uma consulta
   obtém um snapshot único do estoque livre após reservas. Cada versão permanece
   alternativa independente, sem total simultâneo somado.
5. Inspecionar materiais de cada alternativa: estoque livre na unidade do
   catálogo, exigência por lote na unidade da receita, razão exata de conversão,
   lotes completos possíveis e todos os ingredientes limitantes, incluindo empates.

Vínculos podem fornecer IDs para a comparação; também é possível inserir IDs de
versões explicitamente. Remover uma versão ou alterar o local invalida o resultado.
IDs duplicados ou mais de vinte versões são recusados. A seleção é limpa ao trocar
sessão/token/loja. Consultas obsoletas não substituem resultados de consultas novas.

Última versão não significa receita ativa. Uma ordem cancelada ou concluída continua
sendo vínculo histórico. A unidade histórica e sua quantidade são preservadas;
unidade diferente do catálogo aparece como aviso, sem conversão silenciosa.
Totais de vínculos são globais; itens são paginados em blocos de cinquenta.
Não são somados como demanda, estoque reservado ou capacidade simultânea.

## Contratos existentes

Não altera backend, autenticação, infraestrutura, escritor, outbox, migração ou backup.
SQLite permanece em **44**. Novas funções de interface usam apenas rotas existentes:

- `GET /local/v1/catalog/search?...&status=all|active|inactive`: permissão
  `view_catalog`, visibilidade do custo conferida no servidor. Filtros por texto,
  unidade e pendência continuam disponíveis.
- `GET /local/v1/catalog/products/{id}/production-usage?kind=versions|orders&offset=N`:
  exige `manage_production` e `view_catalog`; empresa/loja/aparelho vêm da sessão.
- `POST /local/v1/production/capacity/alternatives`: **consulta somente leitura**,
  apesar do verbo POST, com `location_id` e `version_ids`. Exige `manage_production`.
  Nenhuma ordem é criada e nenhum ingrediente é reservado ou consumido.

APIs documentadas em `production-product-usage.openapi.json`,
`production-capacity.openapi.json` e `catalog-status-search.openapi.json`.

## Arquivos compartilhados

`catalogSearch.mjs` e sua declaração recebem `status` opcional. Clientes antigos
que o omitem mantêm o contrato. Quando informado, valida o estado aplicado na
resposta, estado/revisão de cada linha e correspondência do filtro. Custo oculto
continua sendo recusado se aparecer indevidamente. Unidade filtrada é confrontada
com as linhas. `LocalCatalog.tsx` recebe filtro, coluna e componente de planejamento;
`package.json` inclui testes novos. Não modifica as filas de operações pendentes.

## Quantidades e validação

Quantidades inteiras são milésimos da unidade explícita, não gramas universais.
Frontend confere a aritmética com BigInt e a resposta segura sem arredondamento:

`lotes = floor(saldo_milli × numerador / (necessário_milli × denominador))`.

Conversões admitidas são identidade, kg↔g e litro↔ml, com as razões exatas existentes.
Não há densidade presumida, volume→massa, embalagem→unidade ou unidade→massa.
Os lotes da alternativa são o mínimo dos materiais; todos os empates devem aparecer
como limitantes. Produto de lotes por rendimento deve igualar a produção informada.
Saldos compartilhados entre alternativas devem coincidir no mesmo snapshot.
Resposta com flags de produção simultânea, saldo inconsistente, overflow, limitante
incompatível, registro duplicado ou página fora do contexto é recusada.

## Verificação e limites

Testes frontend novos cobrem estado do catálogo, privacidade do custo, empresa/loja,
páginas e vínculos históricos, quantidades planejadas, razões exatas, saldo zero,
empates, duplicidade, limite de vinte, aritmética segura e alternativas independentes.
Testes HTTP existentes reexecutados cobrem filtro de estado antes da paginação,
permissões/custo, vínculos preservados, backup/restauração, snapshot concorrente,
capacidade exata e ausência de escrita em estoque/outbox/relógio do contrato.

Aplicador testa backend específico, suíte completa, vet, corrida, frontend e build
antes de registrar somente a P65. Usa bancos temporários e Fiber em memória; não
abre servidor nem altera bancos reais ou da demonstração. Sem merge ou push.
Aceitação visual em navegador não foi realizada nesta entrega. Comparação não é
planejamento simultâneo nem autorização para executar ordens.
