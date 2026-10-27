# B02B12 — aplicação e devolução de dinheiro de fornecedores

Base do PC: B02B11 `d176beb`, branch `feat/backend-b02a-compat`, worktree
`zzeus-frontend-backend-b02a`. O log confirma Go/race,89 testes locais do frontend,
build e árvore limpa. Não comprova a integração visual dos serviços desta etapa.

## Comportamento implementado

Dois fluxos: registrar pagamento declarado em espécie a fornecedor, vinculando
uma sangria auditada a uma parcela aprovada; registrar devolução integral desse
pagamento, vinculando um suprimento auditado de igual valor. Os movimentos de
caixa já existem: estes comandos classificam sua finalidade e atualizam o saldo
documental, sem criar outra saída/entrada, venda, estoque ou lançamento de cofre.

O responsável da fonte declara a entrega/recebimento físico. Beneficiário e
referência são informados manualmente. Não há comprovante eletrônico, aprovação
do fornecedor, validação fiscal, banco ou adquirente. `basis` é sempre
`declared_cash_audited_sources`. A referência não é uma autorização bancária.

## Pagamento e devolução

Documento precisa estar `approved`. Cada comando usa todo o valor de uma sangria
em uma única parcela. Uma parcela aceita várias sangrias e pagamentos parciais,
até seu valor exato; a mesma sangria não é dividida entre parcelas/documentos.
Valor vem do registro auditado, nunca do cliente. Comando exige ManageCash e
Finance + Core verificados dentro da transação; usuário deve ser o ator da
sangria. Nenhuma permissão ou licença nova é concedida. O caixa pode já estar
encerrado: esse registro classifica uma fonte histórica, sem reabrir o turno.
Não exige licença POS atual para classificar dinheiro previamente movimentado;
criar o movimento de caixa continua exigindo as regras e licença POS existentes.

Sangria com qualquer histórico de malote, inclusive cancelado/rejeitado, não pode
virar pagamento. Sangria usada em pagamento nunca pode virar malote, mesmo depois
da devolução: o dinheiro devolvido tem outro movimento, não restaura a origem.
Índices e triggers protegem exclusividade nas duas ordens de gravação.

Devolução exige suprimento posterior à sangria original, do mesmo valor do
pagamento, e usuário igual ao ator desse suprimento. Não é preciso ser o mesmo
ator do pagamento. Não aceita fonte ausente, de outro tipo, valor diferente,
origem incompatível ou suprimento financiado por cofre. Um suprimento só devolve
um pagamento. Devolução parcial de um pagamento não é suportada; pagamentos
parciais distintos podem ser devolvidos individualmente. O operador continua
responsável pela correspondência física do dinheiro e documento.

Não há apagar/editar pagamento nem devolução. Cada pagamento aceita uma única
devolução. IDs e referência do recibo continuam reservados depois disso. Nova
entrega física requer nova sangria, novo pagamento e referência própria. Não
contornar a proteção reusando uma referência para outra entrega.

Documento com valor aplicado não pode ser cancelado, também por SQL. Após
devolver todos os pagamentos, o cancelamento documental segue a autorização
anterior: responsável diferente do criador no documento aprovado. Histórico
financeiro continua preservado. Cancelamento não cria um suprimento automático
e não prova estorno de qualquer pagamento externo desconhecido.

## Consultas, saldo e compatibilidade

Contrato: `docs/api/payable-cash.openapi.json`. Quatro operações novas:

| Método | Caminho em /local/v1 | Função |
| --- | --- | --- |
| POST | /finance/payables/{id}/cash-payments | Aplicar sangria na parcela |
| GET | /finance/payables/{id}/cash-payments | Pagamentos, devoluções e saldo atuais |
| POST | /finance/payable-cash/{id}/return | Registrar suprimento de devolução |
| GET | /finance/payable-cash/operations/{id} | Recibo original do próprio ator |

Escritas não aceitam query; corpo até8KiB, campos exatos, sem duplicatas/extra/nulo.
Consultas exigem ViewAccounting, sem licença Finance atual. Escopo empresa/loja/
aparelho/usuário vem da sessão. GET não aceita corpo. Lista aceita somente
limit1–100 (padrão50), offset0–1000000000; query vazia/extra/duplicada é recusada.
Todo o documento/fontes/recibos/eventos é validado antes da paginação, mesmo
quando a página é vazia. Fonte danificada ou inconsistência retorna409, sem
reparar o passado ou fornecer resultados parciais.

GET documento/lista B02B11 passa a incluir `cash` com `paid_cents` bruto histórico,
`returned_cents`, `applied_cents` líquido e `outstanding_cents`. Aplicado é
bruto menos devolvido. Saldo é principal menos aplicado somente quando o
documento está aprovado; pendente/rejeitado/cancelado tem saldo ativo zero.
Isso não desconta pagamentos externos desconhecidos. `settled_cents` continua
null e `payment_execution_available` continua false: o sistema não transfere
dinheiro nem executa pagamento por esses comandos. O valor conhecido de espécie
está nos campos explícitos `cash`, sem atribuir liquidação a banco/cartão/Pix.

Recibos documentais B02B11 permanecem byte a byte como foram gravados, sem `cash`.
Recibo da aplicação original conserva `return:null`, mesmo depois da devolução;
GET de pagamentos mostra a devolução atual. Replay idêntico do mesmo ator retorna
recibo original e repeated=true; mesma chave com outro conteúdo409, chave de
outro ator404. A recuperação verifica histórico atual sem reexecutar dinheiro.

Agenda JSON/CSV acrescenta `cash_applied_cents`, saldo aprovado restante e faixas
por vencimento; valores documentais antigos não são substituídos. Colunas
`cash_applied_cents` e `outstanding_cents` são anexadas ao CSV **formato2**, mantendo
as posições antigas. Resumo e detalhe são seções distintas. Saldo é zero nos
itens não aprovados. Não somar principal novamente com aplicado/saldo, nem com
suas faixas. Os saldos são atuais na hora da leitura. `as_of` classifica somente
a data de vencimento; não reconstrói saldo em uma data histórica anterior.

Conciliação de malotes acrescenta `supplier_paid_cents` e
`supplier_returned_cents`. A sangria usada em pagamento sai de `unassigned` e
entra integralmente em `supplier_paid_cents`, incluindo quando foi devolvida.
Devolvido é informativo e não se subtrai do total de sangrias desse turno. A
devolução já aparece no caixa como suprimento, possivelmente em outro turno.
Journal e relatório contábil continuam mostrando apenas os movimentos originais,
sem nova dupla contagem. A conciliação não presume saldo bancário ou cofre.

## Integridade e limites

Movimento auditado, sessão original e vínculo do operador devem conferir com a
fonte do journal e lançamento imutável. Conferem tipo, ator, operação, valor,
data e referências. Esquema53 possui FKs e triggers imutáveis. Registro, operação
e evento privado são atômicos: falha no último passo reverte os vínculos. Não
desfaz o movimento de caixa criado anteriormente, que continua existindo e pode
estar sem classificação; isso é mostrado como pendência, não apagado.

Até1000 pagamentos históricos por documento, incluindo devolvidos. Bruto
histórico e cada total até9007199254740991 centavos; reuso ilimitado do mesmo
valor por ciclos pagamento/devolução não pode ultrapassar esse bruto. Valores
inteiros exatos. Limites B02B11 permanecem:1000 documentos locais,120 parcelas
por documento,10000 parcelas por agenda,CSV4MiB. A agenda valida a fonte completa.

Nova `0053_payable_cash.sql`; SQL0001–0052 preservados byte a byte. Backup25–53,
incluindo restauração de snapshot real52 para53 em arquivo novo. Fixtures futuras
passam a54. Nenhuma mudança em autenticação/sessões, PostgreSQL ou frontend.
Eventos `payable_cash_events` são locais, privados e duráveis; não são enviados
no protocolo legado, que não possui esse contrato comercial.

Compartilhados: montagem/inventário de APIs, contratos de documentos e malotes,
leituras/agenda/cancelamento de contas a pagar, exclusividade/conciliação de
malotes, suporte de versão no backup e expectativas dos testes de migração.
Não foram modificados serviços de lançamento de caixa ou de cofre. O instalador
confere colisões de53 em outras branches/worktrees e produção28–44 antes de
aplicar e commitar; conflito exige coordenar, sem renumerar ou descartar arquivos.

Pendentes: executar pagamento diretamente com débito transacional na fonte,
pagamento direto de cofre, transferência/boleto/Pix/cartão, devolução parcial de
um pagamento, reconciliação externa, substituição versionada do documento,
vínculo fiscal/recebimento, contabilidade estatutária, frontend e outros aparelhos.
Não conclui venda mistaR$315 ou processamento eletrônico. Ensaios concorrentes
usam requisições no mesmo processo; não demonstram sincronização comercial entre
bancos independentes. Resultados observados constam em EVIDENCIAS.txt do pacote.
