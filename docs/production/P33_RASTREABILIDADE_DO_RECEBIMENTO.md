# P33 — Rastreabilidade de recebimento e ciclo com reservas

Base: commit P32 gerado no PC, ancestral ea15711. Nenhuma migracao nova:
SQLite 42. Arquivos de backup inalterados nesta etapa.

GET /local/v1/purchase-orders/:id/receiving-trace?offset=0
Requer view_orders e manage_stock, sem exigir contrato vigente para leitura.
Escopo deriva da sessao e aparelho; ambos verificados na transacao. IDs 1..128
bytes sem bordas/NUL/CR/LF. Unico parametro permitido: offset decimal
0..9007199254740991; campo vazio, duplicado, fracao, sinal ou desconhecido:400.

Retorna order_id, commercial_status, receiving_status, authorization (ou null),
product_id, unit, planned_milli, delivered_milli, accepted_milli,
rejected_milli, remaining_milli, total_count, offset, limit=50, has_more e items.
Items sao recibos originais P32 em ordem crescente created_at/id. Totais sao
GLOBAIS do pedido, nunca apenas a pagina. Quantidades exatas na unidade
congelada do pedido: recusado=entregue-aceito; restante=planejado-aceito.
Recebimentos nunca demonstram transmissao, aceite de fornecedor nem pagamento.
commercial_status permanece local_not_sent ou cancelled; separado do fluxo
local autorizado/partially_received/received. Nenhum custo, segredo ou
credencial retornado. Consulta nao grava auditoria/outbox nem altera relogio.

Verifica todos os recibos, inclusive fora da pagina: corpo canonico,
produto/unidade/fornecedor do pedido, autorizacao, auditoria com mesmo
pedido/autor/aparelho/data/corpo, operacao de estoque entry, unico movimento
vinculado com quantidade aceita/local/produto/autor/data/motivo coerentes.
Ausencia/divergencia produz 409, nunca uma pagina parcial aparentemente valida.
Leitura faz trabalho linear no numero de recibos para validar o historico:
paginacao limita resposta, nao custo de verificacao. Otimizacao futura exige
preservar essa verificacao. Saldo ATUAL consultar endpoint P15: vender/consumir
apos receber nao reescreve recibos originais.

Replay P32 agora tambem valida autorizacao e ausencia de cancelamento
inconsistente antes de devolver recibo. Dados corrompidos nao sao tratados
como retry bem-sucedido. Sem alterar a idempotencia de um historico valido.
Compartilhados: server.go monta GET; receipts.go reforca coerencia do replay;
comentario do pacote purchases passa a mencionar recebimento local.

Testes: 51 recibos, paginas/totais completos, sem escrita, limites, vinculos
invalidos de empresa/loja/aparelho/identidade, papel, leitura apos vencimento,
bloqueio do escritor vencido, corrupcao fora da pagina e restauracao do historico.
Ciclo integrado REAL em SQLite descartavel: receita+ordem+reserva ativa;
compra criada a partir de sugestao aprovada; autorizacao humana; recebimento;
entrada concorrente com consumo da reserva; saldo livre/exato e backup
restaurado com recibos, movimentos e reserva consumida. Fiber App.Test, sem
servidores. Nao substitui testes de venda/contagem/ajuste/recebimento juntos.
