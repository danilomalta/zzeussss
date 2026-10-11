# P72 — Cinco resumos operacionais do recebimento

Base: `019486b`; branch `feat/producao-receitas`.

Acrescenta ao histórico de recebimentos em `/local/receiving`:

1. Cobertura exata: aceite efetivo versus planejado e pendente, com fração reduzida por máximo divisor comum usando BigInt. Não arredonda percentuais nem altera o estado autorizado do pedido.
2. Conferência das recusas parciais efetivas: entregue efetivo = aceito efetivo + recusado parcial efetivo. Estes totais globais excluem recebimentos anulados e não incorporam recusas integrais.
3. Recusas integrais: total global acumulado separado do total da página, agrupamento dos registros da página pela referência da entrega, contagem de eventos e IDs de seus operadores. Não reduz o pendente nem representa entrada de estoque.
4. Anulações: aceite original global − aceite anulado global = aceite efetivo global; quantidade e contagem de eventos de anulação da página separados dos originais da página marcados como anulados. Os dois lados podem aparecer em páginas diferentes, sem presumir vínculos ausentes nem contar duas entradas de mercadoria.
5. Aceite ativo por local: agrupa somente recebimentos não anulados da página, com contagem, quantidade exata e IDs dos operadores dos registros. Não representa saldo atual, estoque livre nem distribuição do aceite de todas as páginas.

## Contrato e validação

Deriva os resumos do objeto já carregado pelo contrato existente `GET /local/v1/purchase-orders/:id/receiving-history?offset=N`. Nenhuma nova chamada HTTP, escrita, dependência, armazenamento ou regra de autorização. A consulta original permanece protegida por sessão e permissão de estoque; troca de sessão ou de pedido usa a limpeza de resultados existente.

A validação adicional verifica dimensões, unidades, estados, paginação de 50 eventos, duplicidade por tipo/ID, referências de pedido/produto/unidade e balances globais. Verifica que cada soma da página não excede o total global correspondente. Quando o histórico inteiro cabe na primeira página, exige igualdade das somas e presença dos dois lados das anulações. Quando original e anulação estão na mesma página, seus metadados de correção devem coincidir. Inconsistências tornam o resumo indisponível com mensagem explícita; os formulários existentes não são alterados.

Quantidades acumuladas históricas são strings decimais e podem exceder o limite seguro do JavaScript; BigInt mantém a precisão. Grandezas medidas em unidades inteiras mantêm a granularidade de 1000 milésimos. Não há conversão de massa em volume. A fração mede apenas quantidade aceita no pedido, não prazo, pagamento ou progresso físico da produção.

## Arquivos compartilhados

`LocalReceivingHistory.tsx`: inclui o componente de resumos abaixo dos totais globais existentes. Mantém os eventos individuais, paginação, CSVs e ações de recebimento.

`frontend-web/package.json`: acrescenta o novo arquivo de testes ao comando `test:local`; dependências e lockfile permanecem iguais.

SQLite continua no schema 44, última migração `0044_production_receipt_voids.sql`. Nenhuma alteração de API, migração ou backup. Os testes backend existentes do histórico verificam backup/restauração e disputa entre anulação e reserva de produção.

## Verificações e entrega

Testes novos: cobertura exata, máximo inteiro seguro, recusas parciais/integral, acumulados acima do limite seguro, agrupamento por entrega/local/operador, anulações sem duplicidade, páginas diferentes, correspondência de originais, igualdade do histórico completo, página vazia com totais globais não vazios, referências/estados/paginação inválidos e pureza sem alteração dos objetos.

Aplicador valida base, árvore limpa, checksum e patch; executa testes específicos `TestHTTPPurchaseReceivingHistory`, suíte Go completa uma vez, vet, corrida, testes locais da interface e build. Confere os sete arquivos e registra somente após aprovação dos testes. Reconhece entrega já registrada. Sem merge, push, servidores ou bancos comerciais/demonstração. Não houve aceite visual no navegador.
