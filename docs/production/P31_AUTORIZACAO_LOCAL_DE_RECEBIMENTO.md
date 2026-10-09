# P31 — Autorizacao humana local para recebimento

Base efetiva ea15711, feat/producao-receitas. SQLite 0041 estava livre na
base desta branch: 0041_production_receiving_authorizations.sql. Conferir
colisao na futura integracao, sem alterar migracoes antigas.

POST /local/v1/purchase-orders/:id/authorize-receiving
JSON obrigatorio: operation_id, reference, reason. IDs 1..128 bytes sem
espacos nas bordas/NUL/CR/LF; motivo 1..255 bytes com as mesmas restricoes.
Referencia identifica a documentacao conferida pelo comprador. Nao ha
verificacao criptografica de documento nem confirmacao do fornecedor.

Requer manage_replenishment e contrato Orders vigente, aparelho e vinculos
validos na transacao. Replay exige mesmo autor/loja/aparelho e corpo exato.
Referencia e motivo nao sao normalizados silenciosamente. Retorna 200 com
order_id, repeated e authorization (operation_id, device_id, actor_id,
reference, reason, created_at). Novas decisoes com outro ID para o mesmo
pedido conflitam. Pedido cancelado nao pode ser autorizado; pedido autorizado
nao pode ser cancelado pela rota de cancelamento local. Decisao nao reversivel
neste lote. Nenhum estoque, reserva, caixa ou pagamento e criado.

GETs anteriores acrescentam receiving_status: not_authorized ou authorized.
Trace acrescenta receiving_authorization quando presente. status original
permanece local_not_sent: autorizar recebimento NAO envia pedido nem comprova
aceite do fornecedor. A integracao externa continua pendente.

Decisao, purchase_receiving_events e outbox purchase.receiving.authorized schema1 sao
atomicos. Falha/RAISE(IGNORE) desfaz tudo. Leitores continuam autorizados apos
vencimento conforme contrato anterior. Escritores/replays continuam bloqueados.
400 entrada invalida; 401 sessao; 403 papel/contrato; 404 pedido fora do escopo;
409 cancelamento/decisao/idempotencia/evidencia inconsistente; 413 tamanho;
503 emissor nao configurado. Erros inesperados permanecem 500.

Compartilhados: server.go monta rota; Order acrescenta receiving_status;
orderStatusTx e Trace expõem decisao; Cancel recusa pedido autorizado.
Esses contratos precisam de revisao na integracao. Whitelist de backup
acrescenta explicitamente 41, preservando 25..40 e todas as verificacoes.
Testes de migracao atualizam apenas expectativa atual/proxima versao.

Testes SQLite descartaveis/Fiber: replay, cancelamento antes/depois, snapshot,
entrada estrita, papeis, concorrencia, rollback da decisao/auditoria/outbox,
e backup/Verify/restauracao dos novos dados. Sem servidores/bancos reais.
