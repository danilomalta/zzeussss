# P07 — perdas declaradas por resultado de produção

Base confirmada no PC: `5fe7dcc`, branch `feat/producao-receitas`, árvore limpa.
Pasta de aplicação: `~/Downloads/zzeus-frontend-producao`. A P06 passou nos testes,
vet, corrida e build antes do commit mostrado pelo proprietário.

## Contrato de negócio

A P06 preserva o plano, a produção boa e a diferença de rendimento. Essa diferença
não prova automaticamente que houve perda física. A P07 permite a uma pessoa
com `manage_production` registrar explicitamente a quantidade classificada como
perda, com motivo, ligada a um resultado concluído.

Exemplo: plano de 30 unidades, resultado bom de 27, diferença de 3. Registrar uma
perda de 2 mantém 27 unidades boas no estoque, informa 2 de perda declarada e
1 de diferença ainda não classificada. Uma segunda declaração só pode usar esse
1 restante. A soma das declarações ativas nunca pode exceder a diferença.

Não há nova baixa de ingredientes nem de produto acabado: o consumo já foi
registrado na P05 e a P06 só deu entrada na produção boa. Perda posterior de um
produto bom que já entrou no estoque é uma operação física de estoque, distinta
desta classificação de rendimento. Custos, retrabalho, lotes, qualidade por etapa,
sobreprodução e execução por etapas continuam pendentes de entregas próprias.

A quantidade usa milésimos da **unidade histórica do resultado**. `quantity_milli=1`
em `g` significa 0,001 g; em `kg`, 0,001 kg. Não há float nem conversão implícita.
Produtos `unit` exigem múltiplos de 1000. Mudança posterior no catálogo não muda a
unidade histórica desta declaração, porque ela não movimenta estoque atual.

## Registro e correção auditada

Estados da declaração: `recorded` (revisão 1) e `voided` (revisão 2, terminal).
Anular exige operação própria, revisão esperada e novo motivo. Conserva o registro
original e acrescenta evento com autor, aparelho, motivo e data. Não devolve
material, não muda produção boa, receita, revisão ou conclusão da ordem.
A quantidade anulada volta a ficar não classificada; corrigir exige registrar
uma nova declaração com novo ID. Um ID anulado não pode ser reaproveitado.

A criação e a anulação exigem permissão atual do autor/aparelho e contrato de
produção válido. Trata-se de anotação pós-conclusão por operador autorizado;
não exige reativar o responsável original para corrigir a classificação.
Cada operação grava declaração/estado, auditoria e outbox atomicamente.
RowsAffected deve ser 1. INSERT/UPDATE ignorado ou erro desfaz tudo, inclusive
avanço do relógio de licença. Concorrência não duplica declaração nem anulação.

`operation_id` é idempotente por empresa/aparelho neste serviço. Replay exige
mesmo autor, loja, ação e corpo normalizado; retorna o resultado original com
`repeated=true`. Mesmo após anulação, replay da criação retorna seu resultado
histórico `recorded`; GET informa o estado atual `voided`. Reutilizar operação
com outros dados/ação retorna 409. Autorização é revalidada antes do replay.

## Migração e integração compartilhada

Nova migração SQLite **0032_production_losses.sql**: tabelas `production_losses`
e `production_loss_events`, com chaves de empresa/loja, referências ao resultado,
produto, membro/aparelho, unicidade de operação e de revisão por declaração.
0032 estava livre na cópia P06; o script também verifica todos os `0032_*.sql`
na branch antes de aplicar. Nenhuma migração 0001–0031 foi alterada.

Arquivos compartilhados:
- `localapi/server.go`: registra as cinco rotas P07 no router protegido existente.
- `backup/backup.go`: acrescenta somente 32 à lista explícita; aceita
  **25, 26, 27, 28, 29, 30, 31, 32**. Conserva ValidateSchema, checksums,
  integrity_check, foreign_key_check, validação do aparelho, formato e criptografia.
  A autorização continuada do proprietário cobre esse ajuste; versões futuras
  continuam recusadas.
- Testes de compatibilidade incluem restauração histórica de schema 31.
  Testes locais esperam 32 migrações; sondas fictícias de falha usam 33.

Não altera contratos de leitura/escrita P02–P06: o resumo de perdas usa endpoint
novo. Autenticação, sessões, infraestrutura operacional, migrações PostgreSQL,
catálogo e escritores de estoque não foram editados. `production.loss.changed`
(schema de evento 1) vai para a outbox com autor, ação, requisição e snapshot da
declaração; aplicação remota desse evento não é implementada nesta entrega.

## API

POST `/local/v1/production/losses`, objeto exato, sem query:
```json
{"operation_id":"loss-op-1","loss_id":"loss-1","result_id":"result-1","quantity_milli":2000,"reason":"Duas unidades rejeitadas na conferencia"}
```

Criação: 201. Replay: 200. Resposta contém `loss_id`, `result_id`, `status`,
`revision`, `repeated`.

POST `/local/v1/production/losses/void`:
```json
{"operation_id":"void-op-1","loss_id":"loss-1","expected_revision":1,"reason":"Corrigir classificacao apos reconferencia"}
```

Anulação/replay: 200. Sem edição destrutiva, reativação ou DELETE.

Consultas:
- GET `/local/v1/production/losses/{id}`: declaração atual, quantidade/unidade,
  motivo original, estado, revisão, autor e datas.
- GET `/local/v1/production/losses/{id}/history`: `{items:[...]}`, até dois eventos,
  ordenados por revisão, com motivos da criação e da anulação, autores/aparelhos.
- GET `/local/v1/production/results/{id}/losses?offset=0`: resumo e até 50 registros
  incluindo anulados, ordenados por `created_at,id`. Somente `offset` inteiro não
  negativo é aceito. Os totais são do resultado inteiro, mesmo numa página vazia.

Resumo: `planned_milli`, `produced_milli`, `shortfall_milli`,
`recorded_loss_milli` (soma de `recorded`) e `unclassified_shortfall_milli`
(diferença menos essa soma), com resultado/produto/unidade e `items`.
Todas as leituras usam uma transação, sessão/permissão/aparelho atuais e escopo
empresa/loja. Contrato expirado não bloqueia a consulta histórica autorizada.

400: JSON incompleto, duplicado, extra/null, quantidade/revisão inválida, fração
de peça, query inválida. 401: sem sessão. 403: permissão/contrato. 404: resultado
ou declaração não encontrado no escopo. 409: idempotência, estado/revisão,
quantidade acima da diferença restante ou total armazenado inconsistente.
413: corpo acima de 8192 bytes. 500: falha interna com rollback da escrita.
Limites de ID/motivo seguem os serviços de produção existentes (128/255 bytes).

OpenAPI: `docs/api/production-losses.openapi.json`.

## Verificação e execução

Testes com fixtures SQLite temporárias e HTTP em processo verificam criação,
anulação, replay histórico, estoque/plano/conclusão preservados, saldo de
classificação, precisão em gramas, paginação com totais globais, excesso e
aggregate overflow, concorrência de criação/anulação, autorização/expiração,
isolamento, falhas ignoradas/abortadas com rollback, e backup/restauração de
declaração ativa, anulada e seus eventos junto da produção original.

`aplicar.sh` exige base limpa `5fe7dcc`, confere migração e checksum, aplica,
testa backup/CLI primeiro, testes P07, suíte completa uma vez, vet, corrida e
build temporário. Não executa o binário. Remove a variável opcional de testes
PostgreSQL no subshell. Somente depois registra os arquivos explícitos da P07.
Se parar, conserva alterações para diagnóstico sem commit parcial/descarte.
Sem merge, push ou execução contra bancos comerciais/demonstração.

Validação executada e aprovada aqui com Go 1.25.0: backup/CLI, testes específicos
P07 (5,416 s), uma execução da suíte completa, `go vet ./...`, testes P07 com
`-race` (64,968 s) e build `CGO_ENABLED=0` de titan-local sem executar o binário.
Também conferidos SQLite 1–32 em memória, integrity_check/foreign_key_check,
checksums de migrações antigas, JSON/referências OpenAPI, aplicação do patch,
formatação Go e sintaxe Bash. O script repete os testes na branch do usuário.
