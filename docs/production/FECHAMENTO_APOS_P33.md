# Estado apos P31–P33

Base P30 ea15711. Tres commits separados; somente contam como aceitos no PC
apos a saida com testes e registro de cada etapa. Lote implementa autorizacao
humana LOCAL, recebimento parcial com diferencas e rastreabilidade verificavel.
Nao implementa envio ou confirmacao externa, financeiro/fiscal, recusa integral,
devolucao ou estorno de recebimento. Nenhuma tela nova neste lote.

Recebimento do pedido pode avancar sem protocolo entre empresas, pela decisao
explicita do comprador. Essa decisao permanece separada de status comercial
local_not_sent. Nao preencher este status como sent/confirmed por inferencia.
Somente quantidades aceitas entram no estoque; reservas ficam preservadas.

Restam quatro frentes de backend, agora com parte de recebimento entregue:
1. Transmissao/confirmacao reais e vinculo verificado entre empresas/fornecedores.
2. Completar excecoes do recebimento: recusa integral, excessos com aprovacao,
   devolucao/estorno e regras de conversao explicitamente aprovadas, se exigidas
   pelo fluxo comercial final. O lote atual rejeita excessos e unidade diferente.
3. Aceite integrado global com todos os escritores, rollback e restauracao;
   ciclo recebimento/reserva/consumo ja testado neste lote nao fecha essa frente.
4. Revisao de contratos, colisoes SQLite e branches em integracao separada.

Tres interfaces completas continuam sem entrega/aceite: producao,
catalogo/estoque e compras/recebimento. Contagem nao e percentual de esforco,
nem promessa de quatro patches. Infra/auth/recuperacao pertencem ao outro chat.

SQLite0041 e0042 estavam livres na base efetiva desta branch, reservados pelas
novas migracoes deste lote. Backup mantem lista explicita 25..42 e verificacoes.
Nao aceita schema futuro. Nenhuma migracao antiga alterada. Sem main/merge/push.
