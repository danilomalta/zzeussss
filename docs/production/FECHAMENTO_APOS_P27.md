# Inventario apos P25-P27

Base do lote: P24 0b71562. Testes locais do lote nao equivalem a aceite no PC.
Apos os commits do PC: manutencao dos fornecedores locais passa a ter
edicao, ativacao/inativacao, revisao, replay, auditoria e historico paginado.
Cancelamento local de compras implementado, sem simular envio/confirmacao.

Restam CINCO blocos de backend; nao significa cinco patches nem percentual:
1. Inativacao de produtos, com reglas dos escritores comerciais e producao.
2. Completar ciclo de compra: confirmacao autorizada e contrato externo real
   entre empresas. Cancelamento ainda local ja coberto pelo lote.
3. Recebimento conferido: parciais/diferencas/unidades/duplicidade/movimentos
   atomicos associados ao pedido, preservando estoque reservado.
4. Aceite integrado global: producao/venda/contagem/ajuste/recebimento
   concorrentes, isolamento, rollback e restauracao do ciclo completo.
5. Contratos/API e revisao conjunta com outra branch, sem integrar em silencio.

Tres interfaces desta frente continuam sem implementacao/integracao e aceite:
producao; catalogo/estoque; compras/recebimento. API nao equivale a tela pronta.
Infra/auth/recuperacao pertencem ao outro chat. Financeiro/fiscal/mobile e o
roteiro global nao estao incluidos no numero de blocos. Sem percentual
confiavel antes de definir criterios e pesos de esforco.

Schema 38 usado pela manutencao; 39 pelo cancelamento; P27 nao cria migracao.
Revisao posterior de integracao precisa verificar colisoes dos numeros com a
outra branch e contratos de outbox pendente. Sem merge/push neste lote.
