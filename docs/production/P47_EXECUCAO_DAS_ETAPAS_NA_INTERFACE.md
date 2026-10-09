# P47 — Progresso das etapas
Continua P46; /local/production. Exibe sequencia, estado, revisao e responsavel de cada etapa da ordem consultada. Somente o responsavel da primeira etapa nao concluida pode decidir: pending/revisao1 -> running/revisao2 -> completed/revisao3. Etapas anteriores precisam estar concluidas; ordem aprovada e consumo previo obrigatorios. Nao permite pular, cancelar, reabrir ou editar etapas.

POST /production/stages/state existente. expected_revision sempre congelada da consulta, motivo e IDs persistidos no bloqueio comum. Depois de confirmacao limpa consulta; proxima decisao exige nova leitura. Resposta perdida conserva mesma operacao. Conflito nao troca revisao automaticamente. GET de recibo original permanece running/revisao2 mesmo quando etapa atual esta completed/revisao3.

Nenhuma baixa de estoque, entrada ou conclusao de ordem nesta acao. Todas as etapas completas apenas habilitam conclusao medida separada. Autorizacao, responsabilidade, contrato vigente, comprovacao de consumo e sequencia conferidos pelo servidor na transacao. Historico continua leitura autorizada sem exigir contrato vigente.

Schema 44 preservado; sem endpoints novos, migracoes ou alteracao do backup. Testes frontend cobrem responsavel/consumo/sequencia, revisao original, POST exato, confirmacao atrasada e recusa concorrente sem nova revisao. Backend da P46 validado com suite, vet/race e restauracao de recibos. aplicar.sh desta entrega testa frontend e build antes do commit. Sem merge/push/servidor/banco real.
