# P53 — histórico auditado do lote na interface

Após P52, adiciona consulta explícita de lote e histórico. Nenhuma escrita ou migração. Schema 44 mantido.

GET /production/lots/{id} seguido de GET /production/lots/{id}/history, ambos sob /local/v1. A interface exige IDs vinculados, uma declaração revisão 1 e, quando anulado, uma anulação revisão 2. Motivo, operador e data originais devem coincidir com o lote; último evento deve coincidir com estado, revisão e atualização atuais. Duplicidade de operação por aparelho é recusada.

O histórico mostra código, quantidade e datas declaradas originais. Não altera metadados, estoque ou avaliações. Se uma anulação ocorrer entre as duas consultas, os dados incoerentes são recusados; o usuário deve consultar novamente. Token ou seleção alterados descartam respostas atrasadas pelo useReadTask existente. Erros 401 invalidam sessão; 403/404/409 não são apresentados como históricos vazios.

Arquivos compartilhados: LocalProduction inclui painel; productionLots adiciona audit e validadores sem modificar métodos existentes; package.json acrescenta teste. Backend, backup, auth, infra e migrações preservados.

Testes: eventos originais após anulação, motivo separado, operador estrangeiro, revisão faltante, operação duplicada, somente GET, anulação concorrente e recusas. Executar suíte local frontend e build antes do commit. Testes automatizados não substituem aceitação visual.
