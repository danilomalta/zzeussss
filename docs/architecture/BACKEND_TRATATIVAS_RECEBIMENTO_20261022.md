# Tratativas humanas de recebimento — 22/10/2026

Unidade operacional de backend após Painel de Recebimentos. Nova migração
incremental: 0047 na base de produção46, ou 0056 na base B13+Agenda55. Mesmo
SQL verificado, sem renumeração livre. Cria casos, operações e eventos; não
modifica registros comerciais antigos nem remove tabelas. Frontend preservado.

## Decisões humanas e significado

Abre uma tratativa vinculada ao pedido e fornecedor já registrados. Tipo:
falta, dano, produto incorreto ou outro. Requested outcome é SOLICITAÇÃO:
revisão, reposição, crédito, reembolso, coleta ou outra. Não aprova política do
fornecedor nem comprova contato/aceite externo. Referência é texto informado,
não documento fiscal ou prova assinada por terceiro. Sem anexos nesta unidade.

Snapshot inicial preserva produto, unidade, planejado, aceito, saldo e avisos
da conferência oficial no momento de abertura, com limite1000fontes por pedido.
Não muda quando novos recebimentos chegam; consultar Overview para situação atual.
Recebimento posterior não encerra o caso sozinho.

Estados: open → in_progress → resolved/cancelled. Dono/gerente pode resolver
ou cancelar diretamente um caso aberto, e reabrir caso encerrado. Resolved é
ENCERRAMENTO ADMINISTRATIVO LOCAL, não reembolso pago, crédito lançado, coleta
realizada, nota cancelada ou aceite do fornecedor. Razão obrigatória em cada ação.

Operador com view_orders+manage_stock pode abrir sem responsável ou para si.
Pode iniciar/anotar se responsável, ou criador de caso ainda sem responsável.
Iniciar caso não atribuído o atribui ao próprio operador. Gerente/dono podem
atuar e atribuir outro usuário atualmente ativo/autorizado naquela loja.
Resolver, cancelar, reabrir, atribuir e mudar prazo exigem gerente/dono. Casos
fechados não recebem novas notas sem reabertura. Nenhum privilégio depende da UI.

Prazo opcional manual: RFC3339 em segundos inteiros normalizado UTC, ano2000–2200,
vazio para nenhum/limpar. Pode estar no passado. Overdue calculado em leitura
apenas para casos ativos; não muda estado, envia cobrança ou estabelece contrato.

## Atomicidade e recuperação

Cada comando usa operation_id persistente e revisão esperada. Contexto vem da
sessão: empresa/loja/aparelho/usuário. Escritas requerem contrato Orders assinado.
Mesma operação+payload normalizado retorna snapshot original, repeated=true,
mesmo após novas revisões. Outra carga com mesmo ID falha409. Não gerar novo ID
após resposta incerta; consultar operação ou repetir o comando original.

Mudança de caso, recibo imutável, evento e outbox `purchases.receiving_case` são
uma transação. RowsAffected deve ser1; INSERT/UPDATE ignorado aborta tudo.
Revisão impede sobrescrita concorrente. Lease clock do contrato adquire o writer
SQLite antes da verificação/execução. Não é protocolo distribuído de consenso.

Outbox é registro local; envio/aplicação comercial em outro aparelho/empresa
não foram implementados nesta entrega. Não alterar saldo, financeiro ou fiscal
ao consumir esses eventos sem contrato específico futuro.

## Leituras e auditoria

Detalhe, lista, histórico e operação exigem permissões/aparelho atuais. Não
requerem renovação de contrato para consultar histórico autorizado. Operação
fica limitada ao ator/aparelho originais; histórico do caso é acessível aos
perfis de estoque/compras autorizados na mesma loja.

Detalhe compara estado atual com último snapshot, hash do pedido da operação,
vínculos de evento e sequência contínua de revisões. Histórico valida recibos
da página. Lista valida casos retornados, não anuncia auditoria integral de
casos fora da página. Protege contra inconsistência, não contra administrador
do banco reescrevendo coordenadamente todos os registros: não há assinatura
criptográfica de acordo comercial com fornecedor nesta unidade.

Lista por ID, filtros order_id/status/assigned_to, limit1–100, offset0–1000000.
Histórico por revisão. Sem CSV/anexos nesta unidade. Leituras sem efeitos
comerciais; retorno de operação é snapshot histórico, não estado atual do caso.

## Contrato e implantação

Seis operações em `docs/api/receiving-cases.openapi.json`: abrir, listar,
detalhar, agir, histórico e recuperar operação. JSON estrito, todos os campos
de comando presentes; campos auxiliares irrelevantes devem ser vazios. Sem
query em POST, corpo GET, duplicados/null/desconhecidos. Corpo até8192bytes.

Aplicador testa candidata, migrações e backup em bancos descartáveis antes de
copiar fontes. Atualiza contadores/probes de migração e permite backup apenas
da nova versão conhecida além das anteriores. SQL só será aplicada ao banco do
usuário quando o novo backend for iniciado normalmente; não inicia serviço.
Backup de fontes do aplicador não substitui backup do banco antes de atualização.

Ainda pendentes: tela nos idiomas/temas existentes, integração com serviço real
do usuário/B13, políticas comerciais aprovadas, anexos, comunicação remota,
coleta e liquidação financeira. Nada disso é inferido pelo encerramento local.
