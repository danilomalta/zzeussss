# Inventario de fechamento — backend e interfaces

Base comercial confirmada: P18, a53207e. P19-P21 sao correcoes, nao novas
consultas. O aceite das tres depende dos testes e commits no PC.

## O que esta comprovado

Produção: receita/versao imutavel, ingredientes, capacidade por local,
alternativas independentes, ordens com snapshot, responsavel, estados,
reserva/consumo atomico, resultado, perdas, etapas, lotes e qualidade.
Catalogo: criar/listar produtos e locais, permissao e consulta de vinculos.
Estoque: movimentos, contagens, protecao de saldo livre, reservas e consultas.
Compras: referencia de fornecedor local, aprovacao de reposicao, pedido com
item preservado, idempotencia, consultas e rastreabilidade de aprovacao/auditoria.
Testes de backend e backup foram executados. Interface completa deste fluxo
nao foi demonstrada. Esses itens nao representam o sistema inteiro concluido.

## Oito blocos de trabalho ainda abertos no backend

1. Ativacao/inativacao autorizada e auditada de receitas; sem alterar versoes,
   sem impedir leitura historica; definir efeito em NOVAS ordens e ordens existentes.
2. Edicao de produto no catalogo com autorizacao, auditoria, idempotencia e
   bloqueio de alteracao de unidade quando houver dados que dependem dela.
3. Inativacao de produto com preservacao de historico e regras explicitas
   para novas receitas, compras, producao e operacoes comerciais compartilhadas.
4. Edicao/inativacao de fornecedores LOCAIS, preservando snapshots de compras.
5. Ciclo de compra: cancelar e registrar confirmacao autorizada; confirmar
   um fornecedor remoto de verdade depende da integracao entre empresas,
   nao deve ser simulado como enviado por uma API puramente local.
6. Recebimento conferido vinculado a compra: parcial, diferencas, unidades,
   duplicidade e movimentos atomicos. Movimentacao generica existente nao
   demonstra esse ciclo de compra completo.
7. Aceite integrado: producao, venda, ajuste, contagem, reservas e recebimento
   concorrentes, rollback, isolamento e backup de todo o ciclo.
8. Fechamento do contrato/API/documentacao com limites conhecidos e verificacao
   conjunta com a outra branch. Merge/push exigem etapa separada; nao sao feitos aqui.

Este e um backlog de oito BLOCOS, nao promessa de oito patches. Cada bloco pode
exigir mais de uma entrega. Os itens 5/6/8 dependem de contratos compartilhados
que devem ser descritos antes da integracao. Financeiro e outras aplicacoes do
roteiro global nao estao inclusos nessa contagem.

## Tres interfaces ainda sem aceite nesta frente

- Producao: receitas, capacidade, ordens, materiais, resultados, perdas,
  etapas, lotes e qualidade no fluxo real de operacao.
- Catalogo/estoque: cadastros, edicao, saldos, reservas e historico.
- Compras/recebimento: fornecedor local, aprovacao, pedido e conferencia.

A existencia de API nao equivale a interface pronta. Desktop/mobile, instalacao,
autenticacao, recuperacao de senha e infraestrutura pertencem a outra frente.
Integracao dessas interfaces e teste nos aparelhos ainda precisam de aceite.

## Como medir

Ainda nao ha percentual auditado de conclusao: faltam criterios detalhados e
pesos de esforco para os oito blocos e tres interfaces. Contar 21 patches como
21 funcionalidades concluidas daria medida enganosa. A lista acima explicita o
trabalho restante sem chamar planejamento de teste aprovado. Proximas entregas
priorizam os itens 1 e 2, com reserva/coordenacao de migracao quando necessária.
