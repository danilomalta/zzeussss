# Devolução em dinheiro em turno posterior — schema66

Data: 26/10/2026. Base: `6ff0d2f255477aee324c4338c80106bf6734cce2`, branch `feat/backend-b02a-compat`. Unidade backend independente; não altera telas, Electron nem serviços existentes.

## Problema e comportamento

Uma venda não podia ser devolvida depois do encerramento do caixa original. Agora o operador autorizado pode escolher explicitamente um turno aberto do mesmo aparelho, empresa e loja para pagar a devolução. O turno da venda precisa ter fechamento oficial. O fechamento, venda e pagamento originais permanecem históricos e não são reabertos ou reescritos.

As rotas antigas conservam seu corpo e comportamento. Elas não escolhem o caixa atual quando o caixa original está fechado. As novas rotas são:

- `POST /local/v1/sales/{id}/returns/cash-session/quote`: `items` e `refund_cash_session_id` obrigatórios.
- `POST /local/v1/sales/{id}/returns/cash-session`: `operation_id`, `reason`, `expected_refund_cents`, `items` e `refund_cash_session_id` obrigatórios.

Veja o [contrato completo](../api/customer-returns.openapi.json). Seleção vazia/nula, campos extras, JSON duplicado e cotação divergente são recusados. O serviço deriva empresa, loja, usuário e aparelho da sessão. Não aceita esses contextos no corpo.

## Autorizações e efeitos

Exige `CancelSale` e `ManageCash`, licença POS para gravar, dono/gerente autorizado e vínculo do turno escolhido com o próprio operador. Não empresta caixa de outro usuário, aparelho ou loja. Reembolso exige dinheiro físico suficiente. Não seleciona modalidade alternativa nem gera compromisso automático.

Caixa, itens devolvidos, entrada física opcional, livro de caixa, contexto imutável e outbox são gravados na mesma transação. Mercadoria avariada usa `no_restock`. Quantidades e centavos permanecem inteiros; devoluções acumuladas nunca ultrapassam a venda e preservam a alocação de centavos do preço original.

A chave de idempotência inclui o turno escolhido. Repetir a mesma operação recupera o recibo original mesmo após o fechamento do turno de reembolso. Mudar o destino ou qualquer dado com a mesma chave causa conflito. Uma resposta perdida deve ser consultada em `GET /local/v1/sale-return-operations/{operation_id}`; este recurso não cria retry automático, não apaga operações incertas e não autoriza operar depois de revogação.

## Relatórios e compatibilidade

`cash_session_id` no recibo identifica de onde saiu o dinheiro; `original_cash_session_id` aparece somente se a venda for de outro turno. O histórico continua identificando o turno da venda.

O relatório oficial antigo permanece idêntico. `returned_sales_cents` e `net_sales_cents` continuam sendo métricas das vendas e devoluções no mesmo turno. O novo `cash.prior_session_refund_cents` identifica dinheiro pago neste turno por vendas anteriores; já está incluído em `cash.customer_refund_cents`. Não deve ser somado novamente nem interpretado como venda negativa deste turno.

A exportação contábil por período inclui `prior_session_refund_cents` apenas quando positivo, separando-o dos totais de vendas do turno. Esses totais operacionais não substituem DRE ou apuração econômica por data da venda. A ausência do novo campo significa zero; os relatórios e CSV antigos sem o novo efeito preservam o formato anterior. O cliente da interface precisa aceitar o campo opcional antes de expor a nova ação; a interface não foi alterada nesta unidade.

Schema66 cria somente `customer_return_cross_shifts`, índice e proteções de proveniência. SQL1–65 não muda. Banco65 preenchido é migrado preservando recibo, fechamento e checksums históricos. Binário65 deve recusar banco66 como versão futura. Não há downgrade automático; rollback operacional exige cópia/backup separado e o binário compatível, nunca edição da tabela de migrações.

## Verificações

Testes específicos cobrem cotação e escrita reais, fechamento original idêntico, relatório do turno pagador, CSV autorizado, corpo estrito, caixa inexistente/fechado/sem saldo/de outro aparelho, falta de vínculo, permissão e revogação, conflito de chave, disputa de duas devoluções, devolução prévia parcial, proveniência imutável e corrupção de caixa. Injeções ABORT/IGNORE em sete pontos verificam rollback de caixa/estoque/registro/outbox.

O ensaio `TestHTTPCustomerReturnLaterShiftRestartAndEncryptedRestore` usa handlers de produção sobre SQLite em três processos separados do executável de testes e HTTP real de loopback. Confere recibos após reinício e restauração em caminho novo, reenvio sem duplicação e encerramento com saldo401 centavos. A flag `-pilot-backup-cli=/caminho/titan-backup` executa o CLI de backup real; sem ela o teste usa a biblioteca e não deve ser descrito como prova do executável.

Os resultados consolidados e comandos observados são registrados no documento único de continuidade da branch `feat/backend-checkout-authority` e em sua evidência `docs/evidence/backend-refund-next-shift66-20261026/acceptance.json` após a execução. CI só é comprovado pelo run do commit correspondente.

## Limites e operação

Esta unidade entrega somente devolução em dinheiro no mesmo aparelho, usando turno escolhido. Não entrega devolução entre lojas/aparelhos, fiscal, TEF/Pix, vínculo financeiro bancário ou uma nova tela. Não altera processos8182/8282 nem migra seus bancos. Instalar executáveis em versão isolada não atualiza esses serviços automaticamente. Esta entrega não equivale a concluir todo o backend do ERP.
