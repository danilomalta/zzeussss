# Conferência integrada por pedido — backend — 22/10/2026

Complemento após Utilização de Docas, sem migração. Duas consultas JSON/CSV
reúnem pedido, autorização local de recebimento, quantidades conferidas,
recusas integrais, estornos de entrada e agenda em uma única transação de leitura.

## Semântica

O núcleo atual preserva UM produto/unidade por pedido. Esta entrega usa essa
regra existente: não anuncia pedidos com múltiplas linhas. Milésimos inteiros
da unidade original, sem conversões ou soma de unidades diferentes.

`effective_accepted_milli` e `effective_delivered_milli` excluem entradas
estornadas. `remaining_milli` é planejado menos aceito efetivo. Recusas parciais
são a diferença entregue/aceito das entradas efetivas. Recusas integrais são
tentativas separadas, somadas em string decimal exata; não consomem o saldo do
pedido. O total recusado não é uma parcela adicional da quantidade planejada:
uma mercadoria pode ser apresentada novamente em entregas diferentes.

`receipt_count` inclui originais estornados; `voided_receipt_count` os identifica.
`voided_accepted_milli_exact` preserva o histórico, sem recolocar aquele recebimento
no total efetivo. Estorno de entrada não comprova coleta, devolução física ou
reembolso. Nada é reprocessado por esta consulta.

As reservas de doca se vinculam pelo pedido, NÃO por placa, motorista ou
referência individual de entrega. Se existe uma descarga concluída e quantidade
a receber, gera um aviso para revisão, não acusa falta/erro de alguém.

Autorização local não confirma fornecedor. `supplier_confirmation` é sempre
`not_proven_by_local_records`. `payment_evaluation` é sempre `not_evaluated`.
Não lê saldo financeiro nem deduz pagamento ou débito da quantidade recebida.

## Avisos de revisão

Lista estável `review_flags`, sem gravar/enviar notificações: autorização local
ausente, saldo pendente, recebimento parcial, recusas registradas, estorno
registrado, descarga concluída com saldo pendente, pedido cancelado com agenda
ativa e ausência de agendamento local. Pedido cancelado não gera sugestão de
nova compra e não cancela a doca automaticamente. A política comercial é humana.

## Acesso, integridade e limites

Exige sessão, `view_orders`, `manage_stock` e aparelho aprovado no contexto
empresa/loja. Revalida na transação; não renova contrato para ler histórico.
Reutiliza a validação oficial de recibos, movimentos, estornos, autorização e
eventos de recusa. Alteração inconsistente fora da página solicitada falha 409,
em vez de produzir resumo aparentemente correto. Dados de agenda passam pelo
validador existente e o fornecedor deve coincidir com o pedido.

Limite 1.000 registros-fonte SOMADOS por pedido: recebimentos + recusas integrais
+ estornos + reservas de doca, inclusive históricas/canceladas. Verifica antes
da validação detalhada. Acima responde 422, sem totais parciais; reduzir a página
não contorna esse limite. Não há arquivamento automático.

Pagina apenas reservas de doca, `limit` 1–100, `offset` 0–1.000.000. Quantidades,
contagens e avisos sempre representam todo o conjunto. CSV contém cabeçalho,
uma linha `summary` e as reservas da página; protege campos textuais de fórmulas.

## Contrato e aplicação

GET `/local/v1/purchase-orders/{id}/receiving-overview` e variante `.csv`.
Aceita somente `limit`/`offset`, sem corpo GET ou query duplicada/desconhecida.
Contrato em `docs/api/receiving-overview.openapi.json`. Sem cache.

Novos fontes: `dockagenda/order_snapshot.go`, `purchases/receiving_overview*.go`,
`localapi/receiving_overview*.go`; modifica apenas montagem em `dock_utilization.go`
e os dois mapas de rotas quando existirem. Frontend/Electron preservados.

Pendentes: ligação da tela, confirmação comercial entre empresas, associação
explícita entrega–veículo–recibo, política de crédito/coleta e operação com serviço
real do usuário. Os testes não comprovam essas funções ainda não implementadas.
