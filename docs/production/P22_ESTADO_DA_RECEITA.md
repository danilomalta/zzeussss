# P22 — Ativacao e inativacao de receitas

Base bb6f1e7, feat/producao-receitas. SQLite 0036_production_recipe_state.sql,
numero livre na base desta branch; registrado para revisao na futura integracao.
Backup aceita explicitamente 36 alem de 25-35, sem alterar formato/criptografia.
Checksums, ValidateSchema, integrity_check, foreign_key_check e aparelho preservados.

GET /local/v1/production/recipes/:id/state: estado e revisao operacional.
POST /local/v1/production/recipe-state: operation_id, recipe_id,
expected_revision, status (active/inactive), reason. Sem tenant/loja no corpo.
Receitas existentes/novas comecam active, revisao operacional 0 ate a primeira
mudanca. Esta revisao NAO e a revisao da versao imutavel da receita.
Estado repetido com nova operacao ou revisao obsoleta = 409. Replay identico
retorna o resultado original com repeated=true. Operacao divergente = 409.
Mudanca exige manage_production e contrato ativo Production; leitura exige
permissao atual, preservada apos expiracao como os leitores anteriores.

Estado, auditoria e outbox sao atomicos. O evento pendente nao demonstra
replicacao entre aparelhos: importacao desses novos eventos permanece fora
 desta entrega. Motivo, autor, aparelho, operacao e revisao ficam registrados.

Inativacao bloqueia NOVAS ordens de qualquer versao dessa receita na mesma
loja. Nao altera versoes, ordens existentes, replay, reservas ou historico.
Uma ordem existente pode continuar seu fluxo autorizado; nao e cancelada.
Publicar nova versao de receita inativa nao reativa a receita. Consulta de
capacidade continua sendo leitura historica, nao autorizacao para criar ordem.

Compartilhados: server.go monta as rotas; production/orders.go confere estado
na transacao de criacao depois de resolver replay. Backup whitelist e testes
de migracao/compatibilidade recebem apenas o incremento necessario.
Testes: ciclo, replay, revisao, autorizacao, rollback de auditoria, ordem antiga,
bloqueio de nova ordem e backup/restauracao do estado com auditoria.
Nao muda main, bancos reais, autenticacao ou infraestrutura. Sem merge/push.
