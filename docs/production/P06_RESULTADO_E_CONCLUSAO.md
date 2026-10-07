# P06 — resultado medido e conclusão da ordem

Base obrigatória no computador do usuário: `491a1f9`, branch
`feat/producao-receitas`, árvore limpa, pasta `~/Downloads/zzeus-frontend-producao`.
A saída recebida confirmou P05 registrada nessa base e testes aprovados.

## Escopo e decisões

A ordem aprovada, com ingredientes efetivamente consumidos pela P05, pode ter
um único resultado imutável. O operador informa a quantidade boa realmente
produzida e um motivo. A quantidade planejada e a receita original continuam
preservadas; rendimento real não reescreve o planejamento.

A entrada do produto acabado usa o **produto, unidade e local da ordem**, sem
aceitar substituições no corpo. Ingredientes não são baixados novamente.
Produto em área de produção pode ser transferido depois pela operação de estoque
existente, respeitando as reservas ativas.

`produced_milli` aceita inteiros exatos de 0 até o total planejado. Zero registra
um resultado sem produto bom, concluindo a ordem sem movimento zero fictício.
Na unidade `unit`, a produção boa deve ser múltipla de 1000: não dá entrada em
fração de peça. Outras unidades mantêm precisão de milésimos. Quantidades acima
do plano são recusadas nesta fase; não se presume material extra disponível.

`shortfall_milli = planned_milli - produced_milli` indica **diferença de rendimento**
na unidade do produto acabado. Não é uma baixa de estoque nem um cálculo de
custo. Descartes físicos de material extra, sobreprodução, fracionamento da ordem,
consumo parcial, reversão e devolução de ingrediente consumido permanecem para
entregas próprias. Nenhuma reposição fictícia é feita ao concluir.

Antes de publicar, a transação confere aprovação/revisão, responsável autorizado,
permissão atual do autor/aparelho, contrato de produção, snapshot válido, unidade
atual do produto acabado e capacidade do saldo de entrada. Confere também que
há exatamente um grupo consumido da ordem, com quantidades e unidades iguais
à receita × lotes, e movimentos negativos correspondentes de produto/local/
empresa/loja corretos. Apenas marcar o grupo como `consumed` não prova consumo.

A mesma transação grava entrada positiva (quando houver produto bom), resultado,
revisão/data da ordem e outbox. Cada escrita exige uma linha afetada; falha,
INSERT/UPDATE ignorado ou erro na outbox desfaz o conjunto. IDs únicos impedem
conclusões duplicadas e duas chamadas concorrentes não duplicam saldo.

## Estado explícito e migração

Nova migração **0031_production_results.sql**. O número 0031 estava livre na
base P05; o script confirma a ausência de qualquer `0031_*.sql` antes de aplicar.
Nenhuma migração 0001–0030 foi editada.

A tabela `production_results` guarda o estado terminal `completed`, quantidades,
consumo vinculado, movimento de entrada, responsável pela ação, aparelho, motivo,
request/result canônicos e data. Um resultado por ordem e por grupo consumido.
Ela também é o registro imutável de auditoria/idempotência da conclusão.

O CHECK histórico de `production_orders.status` aceita apenas os estados da
P04. Foi preservado, sem reconstrução de tabelas: o resultado imutável é a
extensão normalizada do estado. **GetOrder e ListOrders projetam `completed`**
quando existe esse resultado e mostram `completion_id`. A revisão sobe de 2
para 3 no fluxo comum. SQL que ler diretamente apenas a coluna histórica
`production_orders.status` continuará vendo `approved`; consumidores de estado
atual devem usar a API ou combinar as duas tabelas conforme `orderColumns`.

O histórico da ordem combina eventos P04 e conclusão, com paginação por revisão,
`kind=completed`, antes `approved`, depois `completed`, motivo e autor. O endpoint
antigo `/orders/state` continua aceitando somente aprovação/cancelamento; não
permite saltar consumo e conclusão. Ordens concluídas continuam protegidas contra
cancelamento, nova reserva e novo consumo pelos contratos P04/P05.

## Mudanças compartilhadas

- `localapi/server.go`: duas rotas de resultados no router de sessão existente.
- `localapi/production.go`: conflito de resultado retorna 409.
- `production/orders.go`: estado projetado, `completion_id` e histórico combinado;
  snapshot/plano continuam intactos. Esse contrato afeta consultas P04 e leituras
  de ordem feitas pela P05.
- `docs/api/production-orders.openapi.json`: registra a extensão do contrato de
  leitura; enum das ações do endpoint antigo continua o mesmo.
- `backup/backup.go`: lista explicitamente schemas **25, 26, 27, 28, 29, 30, 31**.
  Autorização continuada do usuário cobre esse ajuste mínimo. Mantidos validação
  do aparelho, ValidateSchema, checksums, integrity_check, foreign_key_check,
  formato e criptografia. Versões futuras continuam recusadas.
- Testes de migração passam a esperar 31; os testes fictícios de falha usam 32.

Outbox `production.result.completed` descreve resultado e movimento, mas esta
entrega não aplica eventos remotos. Não altera autenticação/sessões, infraestrutura
operacional ou migrações PostgreSQL. Não abre servidor nem usa banco real.

## API

POST `/local/v1/production/results`, objeto exato, sem query:
```json
{"operation_id":"complete-op","result_id":"result-1","order_id":"order-1","expected_revision":2,"produced_milli":27000,"reason":"27 unidades boas; rendimento menor que o plano"}
```

Retorna 201 com resultado, produto/local/unidade, reserva consumida, planejamento,
quantidade real, diferença, revisão e `status=completed`. Replay autorizado do
mesmo autor, aparelho, empresa/loja e corpo normalizado retorna o resultado
original, 200 e `repeated=true`; não cria entrada/evento. Reutilizar operation_id
com outro corpo ou publicar segundo resultado para a ordem: 409.

GET `/local/v1/production/results/{id}` acrescenta autor, aparelho, operação,
motivo, data e movimento de entrada (omitido quando resultado zero). Consulta
exige permissão/aparelho atuais, mas continua disponível após expirar contrato.
Dados de outra empresa/loja não são expostos.

400: corpo inválido/ambíguo, campos extras/null, quantidade inválida ou acima do
plano, fração de peça. 401: sem sessão. 403: autorização/contrato. 404: referência
fora do escopo/inexistente. 409: estado/revisão/idempotência, unidade histórica,
consumo incompatível ou overflow. 413: corpo acima de 8192 bytes. Falhas internas
retornam erro e não publicam parcialmente.

OpenAPI completa: `docs/api/production-results.openapi.json`.

## Validação

As fixtures criam SQLite temporário e usam o harness HTTP em processo, sem
servidor de rede. Testes cobrem produto acabado exato, rendimento menor/zero,
preservação de ingredientes/plano/versão, replay, conflitos concorrentes,
quantidades/unidade/overflow, prova dos movimentos consumidos, autorização,
contrato, isolamento, rollback de INSERT/UPDATE/clock/outbox, estado e histórico,
backup/restauração de resultado, consumo e entrada. Testes históricos de backup
incluem schema 30, migrando a cópia restaurada sem alterar o original.

`aplicar.sh` exige base limpa `491a1f9`, testa backup/CLI primeiro, testa fluxos
P06/P04/P05 juntos, executa a suíte completa uma vez, vet, corrida e build sem
executar o binário. Remove a variável opcional de testes PostgreSQL no subshell.
Só então adiciona a lista explícita da P06 e registra o commit. Se falhar,
conserva os arquivos para diagnóstico, sem descarte nem commit parcial.

Validação executada e aprovada neste ambiente com Go 1.25.0: backup/CLI,
fluxos P06/P04/P05 em conjunto, uma execução da suíte completa, `go vet ./...`,
`go test -race` nos três fluxos (139,061 s) e build `CGO_ENABLED=0` de titan-local.
Também verificadas migrações SQLite 1–31, integridade/FKs, checksums históricos,
JSON/referências OpenAPI, aplicação do patch, gofmt e sintaxe Bash.
O script repete a validação na branch do usuário antes do commit.
