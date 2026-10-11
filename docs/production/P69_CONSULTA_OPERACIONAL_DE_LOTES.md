# P69 — cinco consultas operacionais de lotes

Base: `81af95e`, branch `feat/producao-receitas`.

Nova seção em `/local/production`, usando a API de consulta já existente:

1. **Produto**: encontra os lotes declarados para um produto, com vínculo à ordem
   e ao resultado preservados.
2. **Local**: filtra pelo local do resultado de produção. Não infere localização
   atual depois de transferências ou vendas.
3. **Intervalo de validade**: limites opcionais e inclusivos, em datas de
   calendário fornecidas pelo operador. Não converte fuso ou presume “hoje”.
4. **Validade não informada**: consulta explícita dos lotes sem data, separada
   dos datados. Sem data não significa prazo ilimitado. A seleção limpa e
   desabilita os campos de intervalo para não produzir filtros contraditórios.
5. **Última avaliação humana**: separa lotes sem avaliação, com parecer favorável
   ou desfavorável. Mostra revisão, operador, data, critério e motivo existentes.

Os filtros combinam-se. A seção inclui seleção de lotes registrados, anulados ou
ambos; inicia com registrados. Anulados são históricos, e um parecer original
não os transforma em ativos. Cada nova consulta começa na primeira página;
paginar mantém os filtros. Mudanças de campos limpam os resultados anteriores.
O total é global da consulta, e o tamanho da página é indicado separadamente.
A ordenação existente é validade crescente, datas ausentes depois das presentes.

## Contratos e limites

Sem mudança de backend, migração ou backup; SQLite permanece 44. O novo leitor
usa somente GET `/local/v1/production/lot-search`, com `status`, `quality`,
`product_id`, `location_id`, `expiry`, `expires_from`, `expires_to` e `offset`.
As permissões de produção e isolamento de empresa/loja continuam na API. Os
testes existentes incluem leitura histórica com contrato expirado, restauração
com datas/qualidade e concorrência de avaliação versus consulta.

Nenhuma reserva, baixa, movimentação, avaliação ou autorização de venda é
criada. Quantidade do lote é a classificação histórica do resultado, não o
saldo físico atual. A ordenação não constitui fila de retirada ou regra FEFO.

Entradas recusam campos desconhecidos, enums indevidos, IDs fora do limite de
128 bytes UTF-8, datas inexistentes, intervalos invertidos e mistura de “sem data”
com intervalo. Nenhuma empresa, operador ou aparelho é enviada como parâmetro
extra. O leitor exige eco exato dos filtros, offset, total/quantidade da página e
`has_more`, identidades únicas, lotes válidos e itens que realmente satisfaçam
os filtros. Estado e snapshot da avaliação humana devem corresponder ao lote,
incluindo a identidade/metadados originais quando ele foi posteriormente anulado.

Reutiliza os validadores de lotes/qualidade existentes. Não presume parecer
positivo para lotes sem avaliação nem corrige resposta inconsistente. O hook
existente protege token e respostas atrasadas. Erros não viram página vazia.

Arquivos compartilhados: `LocalProduction.tsx` recebe a seção e `package.json`
recebe o teste novo. Leitores anteriores, filas e escritores não são alterados.

## Verificação

Testes dos cinco filtros, combinações, eco, referências, ausência de data,
calendário/bissexto, limites inclusivos, UTF-8, paginação, pareceres, snapshot
imutável, anulações, unidades/quantidades e recusas de sessão/permissão. Frontend
completo e TypeScript/build. Backend completo, vet e corrida de consultas de
lotes. Testes em instalações temporárias, sem bancos comerciais/demonstração.
Sem aceite visual de navegador nesta entrega.
