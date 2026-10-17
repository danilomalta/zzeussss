# B02B3 — finalização transacional de planos em dinheiro

## Base e coordenação

Base de aplicação no PC: B02B2 `5dabd4a`, branch `feat/backend-b02a-compat`,
worktree `zzeus-frontend-backend-b02a`. A produção conferida permanece em
`ba29cbd`, com migrações até 44. A 0045 do checkout já foi incorporada nesta linha.

Esta entrega acrescenta **0046_checkout_cash_sales.sql** somente no backend
isolado. Não criar outra 0046 na integração da produção. Não reescreve 0001–0045
nem aplica este patch na branch de produção. O instalador verifica a referência
e alterações pendentes no backend da produção antes de aplicar.

Arquivos compartilhados: o núcleo de venda `sale.go` é extraído para aceitar
uma transação externa, preservando o contrato antigo; a montagem de planos recebe
uma rota nova; backup e fixtures passam a conhecer46. Autenticação e sessões não
são modificadas. Frontend97–100 em outra worktree não é escrito.

## Operação implementada

POST `/local/v1/checkout/plans/{id}/finalize-cash`:

```json
{"operation_id":"checkout-id-estavel","received_cents":4000}
```

O operador declara o dinheiro recebido nesta requisição, como no fluxo de venda
em dinheiro já existente. Não existe verificação física da cédula pelo servidor.
O ID da operação precisa ser persistido pelo cliente antes do envio e repetido
com o mesmo recebido. Pode coincidir com o ID do checkout e com sua própria
operação de criação; não pode reutilizar operação de outro plano ou outra venda.

O serviço exige sessão, operador/empresa/loja/aparelho originais, permissão de
venda, contrato POS, caixa aberto e plano preparado composto só por dinheiro.
Revalida produto ativo, unidades/preços por linha, arredondamento, saldo livre
e reservas da produção. Mudança de preço/unidade não é aceita só porque o total
coincide. A cotação continua sem reserva ou preço travado.

Uma única transação grava:

- venda com `sale_id` igual ao `checkout_id` estável;
- pagamentos confirmados em dinheiro com os mesmos IDs das parcelas do plano;
- consumo de estoque com movimentos rastreáveis;
- entrada no caixa apenas pelo total líquido da venda;
- operação de venda e evento `sale.committed` existente;
- vínculo imutável do checkout com venda, operação, recebido, troco e data.

Recebido deve cobrir o total e ser inteiro seguro JavaScript. Troco é recebido
menos total. Por exemplo, total 3198, recebido 4000, troco 802: o caixa recebe 3198,
não 4000. O serviço não cria duas entradas, uma para recebido e outra para receita.

Se qualquer etapa falhar, inclusive inserção do vínculo ignorada por trigger,
tudo é revertido. Plano, estoque, caixa, venda e outbox não ficam parcialmente
concluídos. O caminho legado `/sales` mantém campos, hash e evento v1 anteriores,
com até 8 pagamentos de entrada. A nova finalização aceita até 20 parcelas em
dinheiro já persistidas no plano, sem alterar o payload do evento legado.

## Estados e recuperação

O plano passa a `completed`, com `completion` contendo venda, operação, recebido,
troco, data e estado atual da venda. Parcelas passam a `confirmed`, mantendo
`provider_confirmed=false`: dinheiro declarado não é aprovação de adquirente.

O GET do plano recupera esses dados após reinício ou perda da resposta. Reenvio
exato retorna200 com `repeated=true`; primeira conclusão retorna201. Outro
recebido ou outra operação para um checkout já concluído gera409. Venda que
reutilize os IDs sem vínculo do checkout não é adotada silenciosamente.

A repetição de uma conclusão já gravada não precisa reabrir o caixa nem
revalidar preços antigos. O vínculo existente é recuperado com autorização
histórica; o endpoint HTTP ainda exige configuração de verificador. O GET do
plano permanece disponível sem contrato/verificador vigente, sob autorização.

Cancelamento do plano é bloqueado após conclusão. O cancelamento real da venda
em dinheiro usa a rota existente, enquanto permitido pelas regras dela. Nesse
caso o plano preserva `state=completed`, porque houve conclusão no passado,
mas `completion.sale_status=cancelled` e parcelas `reversed` mostram o estorno.
Repetir a finalização não vende novamente nem desfaz o estorno.

`can_finalize` passa a ser uma indicação observada de prontidão para dinheiro:
plano preparado, somente cash, turno aberto, preços/unidades iguais e saldo
suficiente na consulta. Não é autorização de licença nem reserva; a operação
revalida tudo dentro da transação. Após conclusão/cancelamento ou num plano
misto, permanece falso. Histórico de preços/quantidades do plano é preservado.

## Fechamento e limites

A consulta oficial B02B2 passa a enxergar a venda e as parcelas efetivamente
gravadas. Um estorno real aparece como cancelamento/reversão e recompõe estoque
e caixa segundo o serviço existente. O troco não aumenta receita ou esperado.

Nenhuma aprovação pode vir do navegador: o endpoint recebe apenas operação e
recebido. Cartão/Pix/creditAccount ou qualquer distribuição mista continuam
recusados sem efeito comercial. O exemplo R$315 com 100+96+119 continua pendente.
Não há conector simulado habilitado nem banco/adquirente configurado.

Este incremento não é razão financeiro, agenda de recebíveis, crédito concedido,
emissão fiscal ou homologação de equipamento. O recibo operacional continua em
GET `/local/v1/sales/{checkout_id}` com `fiscal_authorized=false`. A ligação ao
adapter Checkout96/frontend97–100 não foi feita. A API não afirma integração visual.

O evento de venda v1 mantém seu contrato; a consulta local preserva os IDs das
parcelas. Isso não demonstra preservação desses IDs em outro aparelho: o
protocolo legado não transporta os novos vínculos de checkout/recebido/troco.
Os eventos de criação/cancelamento de plano continuam sem aplicação comercial
no transporte. Não anunciar sincronização de checkout como concluída.

## Backup e verificação

Backup aceita agora 46, preservando 25–45, ValidateSchema, checksums, integridade,
FKs, verificação do aparelho e revogação de sessões na restauração. Não há
downgrade automático para executáveis de esquema 44/45. O instalador executa
somente testes com bancos descartáveis; não abre o banco da demonstração.

Testes exercitam conclusão real, IDs estáveis, troco e caixa líquido, repetição
após encerramento, reinício com resposta perdida, 20 parcelas, concorrência,
autorização, contrato, mudança de preço/unidade/estoque/turno, plano cancelado,
recusa de mistos, colisão com venda alheia e rollback induzido em pagamento,
outbox e vínculo final. Backup/restauração preservam o vínculo e as parcelas.
O cancelamento posterior da venda e o relatório B02B2 são exercitados de ponta
a ponta no backend. Resultados efetivos acompanham EVIDENCIAS.txt do pacote.

| Etapa | Estado após esta entrega | Pendência |
|---|---|---|
| B02A/B02B1/B02B2 | Aplicadas e testadas no PC até o commit 5dabd4a | Integração visual separada |
| B02B3 — dinheiro | Código e testes desta entrega | Aplicação no PC |
| Confirmação eletrônica/mistos | Pendente | Conector autorizado e estados de cobrança |
| Financeiro completo e R$315 misto | Pendente | Lançamentos, recebíveis, liquidação e fechamento conciliado |
