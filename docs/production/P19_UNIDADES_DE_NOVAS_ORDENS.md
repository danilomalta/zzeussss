# P19 — Unidade de novas ordens

Base a53207e, branch feat/producao-receitas. Nenhuma migracao: SQLite 35.

POST /production/orders passa a conferir, na transacao autorizada existente,
a unidade atual do produto resultante e de todos os ingredientes contra a
versao preservada. Divergencia ou produto ausente retorna 409 antes de inserir
ordem, auditoria ou outbox. Nao converte nem reinterpreta estoque.

Replay de uma ordem existente continua sendo resolvido antes dessa verificacao.
Consultar uma ordem antiga continua retornando seu snapshot historico. Uma
mudanca posterior do catalogo nao reescreve ou invalida a leitura da ordem.
Reserva e execucao mantem suas validacoes existentes.

Compartilhado: production/orders.go, contrato de criacao de novas ordens.
Autorizacao, idempotencia, quantidades, estoques e demais escritores preservados.
Testes novos conferem divergencias de resultado/ingredientes, rollback sem
efeitos, replay historico e bloqueio de um segundo plano. Sem banco real.
Backup e migracoes nao alterados. Sem merge ou push.
