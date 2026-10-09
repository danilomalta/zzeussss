# P48 — Historico auditado das etapas
Continua P47. /local/production consulta GET /production/orders/{id}/stages/history?offset=N existente. Paginas de ate 50 eventos em ordem da sequencia global; intervalos sao normais. Mostra configuracao com nomes/responsaveis/ordem dos itens, inicio/conclusao com revisao anterior/resultante, operador, aparelho, motivo, operacao e instante original. Eventos posteriores nao substituem anteriores.

Leitura por manage_production, empresa e loja autenticadas; pode consultar decisoes de outros operadores autorizadas pelo backend. Isso e diferente do recibo da propria operacao, scoped tambem ao operador/aparelho original. Consultas historicas nao precisam de contrato vigente, gravacoes continuam sujeitas ao contrato. Sem POST, retry, reserva, consumo ou conclusao automaticos.

Cliente valida tipo do evento, sequencia positiva e crescente, entrada estrita, motivo e operacao coerentes, order_id selecionado e resultado/revisao correspondentes. Dados malformados, fora do intervalo exato, futuros ou de outra ordem nao viram eventos fabricados. Falha 401 invalida sessao via fluxo existente; outras recusas sao exibidas. Consulta antiga nao reaparece depois de trocar ID ou sessao.

Schema SQLite 44 preservado; sem novos endpoints/migracoes/alteracoes de backup. Testes frontend verificam consulta somente GET, paginacao/limites, ordem trocada, revisao e quantidade de eventos falsas, sequencia insegura, erros e pagina vazia. aplicar.sh executa testes e build antes do commit. Sem merge/push/servidores/bancos reais.

Continuidade: este lote entrega plano, execucao sequencial e auditoria de etapas. Ainda faltam telas para perdas, lotes/qualidade, outras acoes de catalogo/estoque/recebimento e validacao integrada com main. Nao declara interface inteira finalizada.
