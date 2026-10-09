# P51 — Historico auditado das perdas
Continua P50. /local/production consulta registro e GET /production/losses/{id}/history existentes. Mostra resultado/produto, quantidade/unidade original, estado atual e eventos recorded/revisao1 e voided/revisao2 com motivo, operador, aparelho, operacao e data originais. Nao apaga ou sobrescreve o motivo de declaracao quando anulado.

Historico tem no maximo dois eventos, sem paginacao ficticia. Cliente confere IDs do registro selecionado, sequencia de revisoes/estados, identidade/motivo/data da declaracao original, data do estado atual e identidade composta aparelho/operacao sem duplicidade. Respostas fora de ordem, desconhecidas ou inconsistentes nao viram auditoria fabricada. Como registro e historico sao consultas separadas, alteracao concorrente pode ser detectada como incompatibilidade: usuario consulta novamente, sem gravacao automatica.

Leitura exige manage_production, empresa/loja/aparelho autenticados pelo backend, mas nao contrato vigente. Historico pode mostrar outros operadores da loja conforme essa permissao. Recibo GET da propria operacao continua scoped tambem ao operador/aparelho original. Troca de ID/sessao invalida consulta antiga via useReadTask existente.

Nenhum POST, retry, baixa/estorno de estoque, reserva ou alteracao de resultado. Schema SQLite 44 preservado; sem migracoes/backup/endpoint novo. Testes frontend cobrem historico original apos anulacao, registro selecionado, motivos/identidades conflitantes, datas/revisoes, anulação concorrente, duas consultas apenas GET e falhas 401/409. aplicar.sh testa frontend/build antes do commit. Backend da P49 com suite, vet/race e backup/restauracao. Sem merge/push/bancos reais/servidores.

Continuidade: este lote entrega declaracao, anulacao e auditoria de perdas. Restam interfaces de lotes/qualidade, demais acoes de catalogo/estoque/recebimento e validacao integrada com main; nao declara toda a interface concluida.
