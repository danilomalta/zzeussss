# P44 — Materiais na interface
Continua P43. /local/production consulta GET /production/orders/{id}/trace?lot_offset=0 e mostra versão, local, ingredientes planejados/reservados/consumidos e reserva atual. Somente ordem aprovada pode reservar; reserva ativa oferece liberar ou consumir. Consumo exige confirmação explícita da baixa física. Razão obrigatória, IDs gerados uma vez e fila compartilhada preservida. Ação confirmada limpa a consulta; nova decisão exige nova leitura.

Contratos existentes: POST /production/material-reservations e /production/material-reservations/state. Não adiciona endpoint ou migração; schema 44. Não há revisão fictícia para materiais. Servidor arbitra saldo e transições atomicamente; criação de ordem continua sem reservar. Liberar não repõe estoque consumido e concluir é outra operação.

Validação de trace confere plano exato com BigInt, contexto, quantidades e reserva atual; respostas incoerentes bloqueiam ações. Permissão manage_production e contrato vigente nas gravações continuam no servidor e na interface. Consulta somente lê; não há envio automático. Etapas são apenas consultadas para prontidão da P45.

Teste de cliente rejeita empresa/contexto trocados via identificadores, quantidades inconsistentes, duplicatas, reserva divergente e ações fora de estado; teste de receipt P43 assegura perda de resposta sem baixa duplicada. aplicar.sh executa testes frontend e build antes do commit. Sem banco real, servidor, merge ou push.
