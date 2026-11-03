# B02B6 — malotes persistidos e conferência da custódia

Base no PC: B02B5 `79799ab`, branch `feat/backend-b02a-compat`, worktree
`zzeus-frontend-backend-b02a`. O log enviado confirmou testes, build e commit
limpo. Este pacote reúne duas frentes funcionais do backend e nove operações HTTP.
Não modifica frontend, autenticação, sessões ou bancos PostgreSQL.

## Frente1 — preparação, despacho e histórico de malotes

Um malote referencia uma sangria existente em `cash_adjustments`, pelo ID da
operação. O valor vem do backend e corresponde à sangria inteira. O cliente não
informa nem altera o valor declarado. Valida loja, aparelho, turno, autor,
movimento negativo e lançamento do journal, incluindo operação, autor e instante.
Texto livre do motivo não é prova de sangria. Fonte divergente é recusada.

Somente o autor da sangria pode preparar o malote e despachá-lo/cancelá-lo,
sempre com permissão ManageCash e contrato POS válido. O lacre e o ID do malote
são estáveis. Um lacre não é reutilizado no mesmo escopo loja/aparelho. Cada
sangria tem no máximo um malote ativo ou recebido, garantido também por índice
único parcial. Não há divisão parcial da sangria entre vários malotes.

Transições:

| Estado | Ação | Próximo estado | Responsável |
| --- | --- | --- | --- |
| inexistente | create | prepared | autor da sangria |
| prepared | dispatch | in_transit | remetente |
| prepared | cancel com motivo | cancelled | remetente |
| in_transit | receive com contagem e lacre | received | outro gerente autorizado |
| in_transit | reject com motivo | rejected | outro gerente autorizado |

Cancelamento e recusa preservam registro e histórico. Permitem preparar um novo
malote para a mesma sangria, com novo ID, operação e lacre. Não devolvem dinheiro
automaticamente ao caixa. Recebido é terminal e não libera a origem para outro
malote. Estados terminais e identidade/valor/lacre/origem têm proteção SQL contra
alteração; não há rota DELETE ou sobrescrita de histórico.

## Frente2 — recebimento, divergência e conciliação

O recebedor precisa de ManageCash e POS válido, e deve ser diferente tanto do
remetente quanto do operador original do turno. O backend oculta o declarado
nas consultas desse recebedor até o recebimento ser gravado. Lista, histórico
e recusa não revelam o valor declarado; recibos de operação só podem ser lidos
pelo próprio autor. Isso é controle da API local, não garantia contra quem já
conheça o valor fora do sistema ou administre o arquivo SQLite.

`receive` aceita somente operação estável, lacre e valor contado. Não aceita
valor esperado, aprovação própria, campo de diferença, estado ou outro autor.
Lacre diferente retorna409. A diferença é calculada como contado−declarado.
Contagem zero, a menor ou a maior pode ser registrada. Divergência fica explícita;
não é despesa automática, estorno da sangria, ajuste de estoque ou depósito
bancário. Recebimento é a declaração da pessoa autenticada sobre a entrega física,
não comprovação pela adquirente, pelo banco ou por hardware de contagem.

A conciliação de um turno encerrado retorna:

- total e quantidade de sangrias auditadas;
- valores sem malote, preparados, em trânsito e recebidos;
- valor contado recebido, diferença total e quantidade divergente;
- quantidades históricas de malotes cancelados e recusados.

`unassigned + prepared + in_transit + received_declared = withdrawn`.
Valores em centavos inteiros, somados com inteiros arbitrários; totais acima do
inteiro exato do cliente retornam422, sem resposta parcial. Exige o operador
original do turno com ManageCash, respeitando também o vínculo humano original
e o registro oficial de encerramento. Outro gerente não usa esse endpoint para
descobrir a declaração antes de contar. Histórico não exige licença POS atual.

O escopo é `audited_local_withdrawals`: sangrias formalmente auditadas deste
banco/dispositivo. Não representa todos os fluxos de caixa, movimento legado
não classificado, banco ou outras lojas. Journal/relatório B02B2–B02B5 continuam
necessários para o fechamento completo. Uma auditoria removida não faz uma
sangria desaparecer da conciliação: a referência do journal é conferida.
Malotes ativos e terminais são validados para detectar origem ausente/divergente
e estado sem histórico. Leitura nunca repara o passado.

## APIs e exemplos

Todas exigem sessão e ManageCash. Escritas também exigem POS; leituras permanecem
acessíveis sem licença vigente. Empresa, loja, aparelho e usuário vêm da sessão.

| Método | Rota em `/local/v1` | Resultado |
| --- | --- | --- |
| POST | `/cash/bags` | preparar malote |
| POST | `/cash/bags/{id}/dispatch` | registrar despacho |
| POST | `/cash/bags/{id}/cancel` | cancelar preparação com motivo |
| POST | `/cash/bags/{id}/receive` | gravar contagem e recebimento |
| POST | `/cash/bags/{id}/reject` | recusar com motivo |
| GET | `/cash/bags/{id}` | estado atual e histórico |
| GET | `/cash/bags/operations/{id}` | resposta original da operação do próprio autor |
| GET | `/cash/bags` | lista paginada, filtros de estado e turno |
| GET | `/cash/sessions/{id}/custody` | conciliação das sangrias do turno encerrado |

Preparar: `{"operation_id":"prepare-one","bag_id":"bag-one","withdrawal_operation_id":"withdraw-one","seal_id":"seal-one"}`.
Despachar: `{"operation_id":"dispatch-one"}`.
Receber: `{"operation_id":"receive-one","seal_id":"seal-one","counted_cents":290}`.
Cancelar/recusar: `{"operation_id":"cancel-one","reason":"Lacre incorreto"}`.

JSON estrito: sem campos adicionais, ausentes, duplicados ou nulos; limite8192
bytes de corpo. Novos IDs de malote/operação começam com letra/dígito ASCII e
usam somente letras/dígitos ASCII, ponto, dois-pontos, hífen ou `_`, até128 bytes,
para serem consultáveis como um único segmento de URL. GET não aceita corpo. Query somente na lista: `state`,
`session_id`, `limit` de1 a100 (padrão50), `offset` de0 a1000000000 (padrão0).
Parâmetros desconhecidos/duplicados são recusados. OpenAPI detalhado acompanha
schemas reais, verificados contra respostas dos testes.

## Repetição segura, transações e eventos

ID da operação único no escopo empresa/aparelho. Cada operação grava estado,
histórico, resposta original e evento durável na mesma transação. Falha, inclusive
INSERT/UPDATE ignorado por trigger, desfaz tudo. A origem não pode ser usada
simultaneamente por dois malotes. Concorrência não usa última gravação vence.

Reenvio com mesmo autor, ID e conteúdo retorna o recibo original com
`repeated=true`; outro conteúdo/ação/malote gera409. Reenvio de preparação depois
do recebimento retorna o snapshot original prepared, sem reabrir o malote:
o estado atual vem de GET `/cash/bags/{id}`. Leitura de recibo alheio retorna404.
Não tratar um recibo histórico como consulta ao estado atual.

Nenhuma dessas ações insere novo movimento em `cash_movements`, estoque ou
journal. A sangria já retirou o valor do caixa. Receber o malote não retira de
novo, não reabre fechamento e não altera a receita da venda.

O protocolo `incoming` existente não reconhece `cash.bag.*`. Os eventos ficam
em `cash_bag_outbox`, na transação, com schema1, payload, autoria/recibo vinculado
e proteção contra edição/exclusão. A fila compartilhada do protocolo antigo não
recebe esses novos tipos. Não há transporte, ack remoto, consumidor comercial
ou endpoint de confirmação entre aparelhos entregue aqui. O estado in_transit
é o despacho declarado por uma pessoa, não um recibo de rede. Fila própria e
histórico são preservados por backup/restauração.

## Migração e arquivos compartilhados

Nova `0048_cash_bags.sql`, reservada nesta linha do backend depois de0045–0047.
Nenhum script0001–0047 foi alterado. Migração48 cria tabelas de malotes,
operações e eventos, índice de referência da sangria e guardas de histórico.
Suporta migração de banco47 com dados existentes sem fabricar malotes antigos.

Backup passa de25–47 para25–48, preservando ValidateSchema, checksums,
integridade, FKs, chave, dispositivo e revogação de sessões. Teste de restauro
inclui o esquema47 real com seus triggers, executando os scripts completos com
o parser SQLite. Fixtures de futura migração usam49. Não há downgrade destrutivo.

Integração deve coordenar48, `server.go`, `backup/backup.go`, testes de migração,
inventários de API e matriz de progresso. O aplicador exige a base79799ab e
preimagens exatas. Confere a branch da produção, quando existente, para detectar
numeração posterior a44 ou migrações pendentes antes de tocar a entrega.
Nada é aplicado à worktree da produção nem à pasta com frontend97–100.

## Validação e limites do aceite

Testes de integração cobrem ciclo completo, contagem cega pela API, diferença
a menor, zero, agregado fora de faixa, estados, lacres, autoria, permissões,
licença, revogação, consulta/lista/recibos e schema. Cenário de625 em sangrias
reconcilia todos os estados e preserva cancelamentos, recusa e substituição.
Testes concorrentes disputam a mesma origem e o mesmo recebimento divergente:
somente uma operação vence. Falhas em estado, histórico e eventos preservam
tudo ou nada; retry funciona após corrigir a falha. Backup/reabertura e resposta
perdida mantêm IDs, recibos e filas. Resultados executados constam das evidências.

Isso ocorre em SQLite temporário e servidor HTTP de teste no mesmo processo.
Não demonstra queda física, entrega real de cédulas, conectividade entre
aparelhos ou integração visual. Frontend completo ausente no ambiente de
preparação; testes/build dele são exigidos no PC antes do commit.

Permanecem pendentes: pagamento misto R$315, cartão/Pix e confirmação eletrônica,
financeiro contábil completo, conta bancária/cofre de destino, transporte comercial
de malotes, SEFAZ e conexão com as telas97–100. Nenhum conector simulado foi
habilitado. Sem servidor iniciado, banco da demonstração, merge ou push.
