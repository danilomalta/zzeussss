# P27 — Historico autorizado e paginado do fornecedor local

Sem migracao; SQLite permanece 39. GET
/local/v1/purchase-suppliers/:id/history?offset=0 retorna current (cadastro,
revision), origin (snapshot inicial + auditoria de criacao), items (alteracoes
com revisao, ator, aparelho, antes/depois, motivo e instante), total_count,
offset, limit=50 e has_more. Ordem crescente de revisao; offset inteiro
0..9007199254740991. Query desconhecida, repetida ou nao numerica: 400.

A leitura exige view_orders no escopo empresa/loja/aparelho/membro. Pode ler
apos expirar contrato, sem modificar relogio, outbox, cadastro ou estoque.
Nao e permissao para editar. Todas as consultas usam a mesma transacao.
Falta de fornecedor: 404; falta/inconsistencia da evidencia: 409 sem resposta
parcial. Confere origem/auditoria, total/min/max/revisao atual e ultimo evento;
para cada item da pagina confere payload canonico e antes contra o evento
anterior (ou origem). Nao e assinatura criptografica de logs nem auditoria
de alteracoes manuais antigas: para registros anteriores a 38, origin foi
capturado no upgrade. Nao interpreta esse baseline como prova remota.

Compartilhado: server.go monta a rota. Nenhuma mudanca em autenticacao,
infraestrutura, formato/criptografia de backup ou importador de eventos.
Contrato API em purchase-supplier-history.openapi.json.

Testes: 51 alteracoes/paginacao, escopos, dados corrompidos, leitura concorrente
com edicao, expiry, e fluxo combinado fornecedor editado/inativo + pedido
cancelado, snapshots e backup/restauracao. O aceite integrado global com
venda/contagem/recebimento concorrentes continua aberto.
