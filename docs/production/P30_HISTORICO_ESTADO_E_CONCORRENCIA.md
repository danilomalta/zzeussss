# P30 — Historico gerencial do estado e inativacao concorrente com venda

Sem migracao; SQLite 40. GET
/local/v1/catalog/products/:id/state-history?offset=0 devolve current,
offset, limit=50, total_count, has_more e items ordenados por revisao.
Itens incluem loja da decisao, aparelho, ator, operacao, antes/depois,
motivo e instante. Estado e historico pertencem ao produto na EMPRESA,
nao sao limitados a loja atual: manage_stock possui o mesmo alcance da
edicao/inativacao. view_catalog apenas nao da acesso aos motivos/atores.
Exige membro/aparelho/loja atuais e manage_stock; leitura apos expiry
continua permitida sem atualizar o relogio do contrato.

Offset inteiro 0..9007199254740991; query desconhecida, repetida, vazia ou
fracionaria: 400. Produto inexistente: 404. Inconsistencia: 409 sem resposta
parcial. Confere total/min/max/revisao atual e ultimo payload; cada item da
pagina confere JSON canonico, motivo/estado e estado anterior contra o
antecessor ou baseline active. Nao e assinatura criptografica de auditoria.
Snapshots das operacoes comerciais e permissoes nao sao alterados.

server.go apenas monta nova rota. Backup permanece 40, sem mudanca de
formato/criptografia/recuperacao. Testes de 51 mudancas e duas lojas,
paginacao, expiry/permissoes, corrupcao, backup/restauracao da auditoria.
Teste concorrente inativacao/venda aceita SOMENTE venda completamente
registrada antes da decisao OU recusa sem consumo; nova venda apos a decisao
e recusada. Restaura e compara estado, auditoria, vendas e saldo de estoque.

Teste restrito a este contrato nao fecha o aceite integrado global de
producao/contagem/ajuste/recebimento concorrentes. Nenhum servidor ou banco
real/demonstracao foi aberto; testes usam fixtures temporarias.
