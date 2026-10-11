# P66 — cinco funções de consulta do estoque

Base autorizada: `1016d7a`, branch `feat/producao-receitas`.

A tela `/local/stock` amplia a consulta existente sem criar operações de estoque:

1. Comparação do mesmo produto em até dez locais distintos, com físico, reservado e livre por local. Não há soma entre locais. As consultas GET são independentes, podem representar instantes diferentes e não prometem um snapshot único.
2. Conferência opcional de uma quantidade necessária contra o saldo **livre** de cada local. A necessidade é digitada com vírgula, até três casas decimais e unidade explícita.
3. Falta exata por local, preservando conversões compatíveis e frações menores que a escala do estoque, sem arredondar para zero.
4. Agrupamento das reservas **da página atual** por ordem, preservando as versões referenciadas.
5. Agrupamento das reservas **da página atual** por responsável. Não se confunde o subtotal da página com o saldo global.

## Contratos e limites

Sem modificação de backend, migração ou backup. SQLite permanece em 44.
Usa os contratos existentes GET `/local/v1/stock/availability` e GET
`/local/v1/stock/reservations`. As permissões e isolamento de loja são conferidos
pela API existente; detalhes de reservas continuam exigindo produção.

Arquivo compartilhado `stockRead.mjs`: reforça o contrato de paginação para exigir
a quantidade exata de registros esperada pelo total/offset/limite e `has_more`
consistente. Nenhum campo ou formato do backend foi alterado.

Todos os saldos de entrada são inteiros seguros e satisfazem físico = reservado +
livre. Quantidade planejada e cálculo de faltas usam BigInt intermediário. Não
há conversão volume/massa. Conversões aceitas: unidade idêntica, kg/g e liter/ml.
Exemplo: necessidade `0,001 g` com estoque em kg equivale a `0,001 ÷ 1000 kg`.
Essa representação é uma fração exata em unidades, não miligramas presumidos.

Falha de um local permanece explícita: nenhum saldo zero é inventado. Uma
resposta 401 encerra a comparação e segue a invalidação existente da sessão.
Mudanças nos parâmetros limpam resultados antigos; o hook existente rejeita
respostas atrasadas e alterações de token.

Não cria ordem, não reserva, baixa, transfere ou sugere produção simultânea.
Não consulta bancos comerciais ou demonstração nos testes.

## Verificação

Testes unitários de seleção, paginação, escopo retornado, perdas de resposta,
GET somente leitura, unidades, limite seguro e frações exatas. Testes locais do
frontend, TypeScript/build, suíte completa do backend, vet e corrida dos módulos
de estoque/API. Sem aceite visual em navegador.
