# Coletas e devoluções ao fornecedor — 20261022

## Base e dependências

Entrega incremental sobre Ciclo Compras57, oito pacotes20261022 e B02B14, HEAD do usuário58a26bb, branchfeat/backend-b02a-compat. Reconstrução localcef21df com conteúdos verificados; esta referência não é um commit do usuário. Trabalho isolado emfeat/backend-supplier-returns, preservando as entregas anteriores.

Migração0058_supplier_returns.sql nova, imutável depois de aplicada. Não renumera SQL antigo. O aplicador recusa qualquer ocupação inesperada do número58 ou mudança nos pré-requisitos. Banco da demonstração não é aberto. A migração só roda na futura inicialização normal do backend.

Compartilhados: montagem de rotas emserver.go; serviço de estoque separado em operação com transação fornecida pelo chamador; guarda de estorno de recebimento; contadores de migração, backup e inventários de API. Não altera autenticação, sessões, frontend, Electron, perfis ou chaves. Record/RecordWithContract conservam o caminho e o contrato legados; RecordInTxWithContract não confirma a transação do chamador.

## Fluxo real

1. Operador comViewOrders eManageStock abre pedido de devolução ligado a tratativa ativa, recebimento íntegro e mesmo produto/unidade/pedido. Quantidade em milésimos da unidade existente, sem float/conversão implícita. O recebimento original nunca é apagado.
2. Gerente comManageReplenishment registra evidência manual de autorização, parcial se necessário, ou recusa. External_verified permanecefalse. Não envia mensagem ao fornecedor.
3. Gerente agenda coleta:next_delivery exige outra entrega do mesmo fornecedor;relationship_end exige referência de rota e prazo. Reagendamento preserva versões anteriores. Prazo é informação de acompanhamento, sem bloqueio automático por atraso. A entrega de origem não pode servir como sua própria próxima coleta.
4. Operador despacha exatamente a quantidade autorizada, somente com entrega vinculada chegada/recebida, ou exceção de encerramento. Valida saldo livre, reservas, produto e unidade atuais. Registro, movimento, outbox e histórico são uma única transação. Falha final desfaz todos os efeitos.
5. Gerente registra quantidade recebida pelo fornecedor, acumulando confirmações parciais. É atestado local manual; não é resposta assinada do fornecedor.
6. Quantidade ainda não recebida pode voltar à loja, parcial ou integralmente, com nova entrada e histórico. A soma recebida no fornecedor+recuperada nunca excede a despachada. Completed exige conciliação física integral.

A saída usa o tipo legado stock.loss, pois o esquema de estoque existente só distingueentry/transfer/loss. O evento de devolução e o vínculo da operação indicam a natureza da saída. Isso NÃO registra despesa, perda de produção, crédito ou quitação. Relatórios que agrupem todo loss como desperdício precisam considerar esse vínculo; migração de taxonomia de estoque fica pendente para coordenação.

## Consistência e autorização

IDs estáveis, expected_revision obrigatório nas transições e recibos por operação. Reenvio idêntico recupera o resultado original; troca de conteúdo conflita; consulta de operação exige usuário e dispositivo originais. Listagem/histórico validam a fonte completa antes de paginar. Limites explícitos:1000 devoluções na seleção,10000 eventos agregados e4096 revisões por devolução; fonte maior retorna422, sem total parcial utilizável.

Solicitações concorrentes não comprometem a mesma quantidade do recebimento: requested compromete pedido integral, approved/scheduled a quantidade autorizada, dispatched/completed o líquido despachado menos recuperado. Cancelamento/recusa liberam compromisso. Saldo físico só é movimentado no despacho/recuperação; pedido/agendamento não reservam estoque. Pode faltar saldo no despacho por operações legítimas concorrentes, retornando409.

Estorno legado do recebimento fica bloqueado enquanto houver mercadoria fora da loja, inclusive já recebida no fornecedor. Recuperação integral permite estorno normal; recibos de operações já concluídas continuam recuperáveis. Cancelar é permitido somente antes do despacho. Não existe apagamento nem cancelamento fictício de mercadoria em trânsito.

A API física não expõe valores financeiros. Consulta financeira separada exigeViewAccounting e vínculo real da conta ao recebimento. Projeta total/aplicado/saldo oficiais sem recalculá-los por devolução, comfinancial_effect_recorded=false. review_required sinaliza revisão humana, inclusive após conciliação física; não é autorização de desconto nem trava automática de pagamento.

## APIs

Contrato estrito emdocs/api/supplier-returns.openapi.json; nove operações reais:
POST purchase-receiving-cases/:id/returns; GET purchase-supplier-returns com filtroscase_id/order_id/supplier_id/state e paginação; GET purchase-supplier-returns/:id; GET :id/history; POST :id/actions; POST :id/dispatch; POST :id/recover; GET purchase-supplier-return-operations/:id; GET finance/receiving-return-review/:id. Prefixo/local/v1. Ações:approve,decline,schedule,cancel,supplier_receive. Corpos exigem todos os campos do respectivo contrato; campos não usados usam vazio/zero. Sem tenant/store/actor no corpo. Cache-Control:no-store.

## Pendências explícitas

Não implementa crédito comercial, abatimento, reembolso, substituição automática, nota fiscal de devolução, autenticação remota do fornecedor, GPS, smartphone offline, rastreamento por lote/serial da mercadoria devolvida ou interface. Uma devolução possui um despacho; menores quantidades exigem autorização menor ou outra solicitação, não múltiplas saídas na mesma devolução. Não resolve tratativa nem cancela janela de doca automaticamente. Evidência externa manual pode ser inexata; protocolo textual não comprova autenticidade documental.

Fontes completas no ZIP são referência única para revisão; o aplicador grava apenas arquivos enumerados. Sem enchimento, binários, bancos, segredos ou cópias deZIPs. O tamanho não é critério de qualidade; funcionalidade e evidência estão discriminadas no pacote.
