# P36 — Historico consolidado e validacao com reservas da producao

Base: commitP35 no PC; ancestral ab96a99. SQLite44; nenhuma migracao nem
alteracao de backup neste passo. Toda leitura permanece disponivel apos
vencimento do contrato, sujeita a view_orders e manage_stock vigentes no papel,
identidade/aparelho/loja validos. Nenhuma escrita ou avanco de relogio de licenca.

GET /local/v1/purchase-orders/:id/receiving-history?offset=0
Parametro unico offset decimal0..9007199254740991. Vazio/duplicado/sinal/fracao/
empresa/loja extra:400. IDs1..128bytes sem bordas/NUL/CR/LF. Pagina50 itens,
ordenacao deterministica created_at,id,kind (nao prova de causalidade remota).

Retorna order_id, commercial_status, receiving_status, product_id, unit,
totals, total_count, offset, limit=50, has_more, items. Cada item tem kind,
id, created_at e exatamente um objeto receipt/rejection/void. Kind received
inclui o recibo original e, se aplicavel, void; kind voided apresenta a decisao
compensatoria separada. Nao somar esses dois como duas entradas de mercadoria.

Totais GLOBAIS do pedido, independentemente da pagina:
- planned_milli, effective_accepted_milli, effective_delivered_milli,
  partial_rejected_milli, remaining_milli: inteiros exatos limitados por MaxQuantity.
- fully_rejected_milli_exact: SOMA de entregas totalmente recusadas (P34),
  decimal string, podendo exceder a faixa segura de numeros JSON/JavaScript.
- recorded_accepted_milli_exact: SOMA de aceitos ORIGINAIS, incluindo anulados.
- voided_accepted_milli_exact: SOMA de compensacoes autorizadas.
As ultimas duas tambem sao strings decimais. Effective = recorded - voided.
remaining = planned - effective. Recusa integral nao diminui remaining.
Mesmo numeral nao identifica a mesma dimensao em unidades diferentes: nao
somar pedidos g com kg, ml com massa etc. Nao ha conversao implicita.

Valida o conjunto inteiro ANTES de paginar: recibos e entradas originais,
anulacoes e movimentos negativos, corpo canonico, autoria/aparelho/data,
auditoria, pedido/produto/unidade/fornecedor, autorizacao humana. Corrupcao
fora da pagina produz409; nenhuma resposta parcial mascara evidencia faltante.
Custo linear no historico com leituras adicionais por evento; pagina limita
resposta, nao custo de validacao. Quantidades de historico nao substituem
saldo ATUAL P15, pois outras vendas/producoes podem movimentar os mesmos produtos.

Compartilhados: server.go monta rota; receiving_trace.go extrai helper que
permite as leituras no MESMO sql.Tx, preservando autorizacao e sem transacoes
aninhadas. P33 continua acessivel com sua ampliacao documentada na P35.
Nao expõe custo, chave, token, documento completo ou payload da outbox.

Testes: 52 eventos, totais fora da pagina, strings acima de2^53-1, replay,
sem escrita, corrupcao fora da pagina, backup/restauracao; receita+ordem+compra+
recebimento com reserva ativa bloqueando anulacao; liberacao e compensacao
posterior; corrida reservar vs anular em saldo limite, permitindo um unico
vencedor sem saldo negativo nem reserva descoberta. Bancos descartaveis.
