# Entrega 27 — Desfazer lotes por operação compensatória

Base main dccc8b0: entrega 26 validada em PostgreSQL real isolado no PC.
O lote original permanece no banco. Desfazer cria nova operação que restaura
valores anteriores e incrementa versões. Não reescreve auditoria ou vendas.

| Função | Resultado | Verificação |
|---|---|---|
| Prévia de reversão | Antes/depois, motivo, original e hash | Não grava produto, recibo ou outbox |
| Aplicação | Todos os itens ou nenhum, motivo e vínculo único | Falha tardia do evento reverte produtos, vínculos e recibos |
| Recuperação | Recibo pelo ID da reversão e operador original | Replay não altera produtos novamente |
| Consulta do original | Informa qual reversão foi gravada para o lote | Isolamento por empresa e autorização administrativa |

## Escopo e autorização

Somente owner/admin/manager. Esses perfis podem reverter lote da sua empresa
mesmo que outro operador o tenha aplicado; essa decisão é administrativa e
fica auditada com o ator da reversão e motivo. Não basta conhecer um UUID.
Stock/cashier não podem reverter. Empresa, ator, papel e sessão vêm da sessão
autenticada. A transação verifica novamente a sessão válida/não revogada,
associação atual, papel e empresa ativa antes de qualquer escrita.

GET do recibo por ID é restrito ao operador que aplicou a reversão. GET da
situação do original é administrativo e pode mostrar a reversão de outro
operador da empresa, assim como o histórico de lotes já permite auditoria
administrativa. Um ID de outra empresa não expõe dados.

Não altera SQLite, produção, catálogo local, frontend, estoque ou migrações
8–10 já publicadas. Não habilita fiscal, importação, reajuste percentual,
pagamento ou sincronização. Corrigir cadastro no online não atualiza o caixa
local por si só. Outbox é intenção durável sem trabalhador/ack implementado.

## Fluxo para cliente/API

1. Gere e salve novo `operation_id` antes de solicitar a reversão. Preserve
   o `source_operation_id` do lote aplicado e um motivo não vazio.
2. POST `/api/v1/catalog/undos/preview` com esses três campos. Exemplo:

```json
{"operation_id":"11111111-1111-4111-8111-111111111111","source_operation_id":"55555555-5555-4555-8555-555555555555","reason":"Corrige reajuste lançado por engano"}
```

3. Confira `items` com `before` e `after`. Preço e ativação anteriores são
   obtidos do recibo original; o cliente não fornece itens nem valores novos.
4. Após decisão humana, POST `/api/v1/catalog/undos/apply` com os mesmos
   campos e o `preview_hash` retornado. O motivo faz parte do hash da prévia.
5. Em perda de resposta/503, preserve o pedido e consulte GET
   `/api/v1/catalog/undos/{operation_id}`. Consulta indisponível não autoriza
   repetir automaticamente. Repetição explícita usa ID e conteúdo originais.
6. GET `/api/v1/catalog/batches/{operation_id}/undo` usa o ID do lote original
   e retorna sua reversão gravada. Se não há vínculo no contexto, retorna 404.

200: prévia ou resultado persistido/replay; 400: entrada inválida;
403: sem autorização; 404: original/recibo ausente no contexto;
409: conflito de versão, snapshot, hash, motivo/ID, original já revertido
ou tentativa de reverter outra reversão; 503: indisponibilidade/commit incerto.
401 e 429 continuam sob os guardas existentes. Cache-Control no-store.
O contrato negociado de erros v1 permanece disponível. Erros não exibem SQL,
tokens, credenciais, mensagens internas do banco ou conexões.

Parser estrito: JSON application/json, até 8 KiB, campos exatos, sem campos
duplicados/desconhecidos/nulos. UUIDs canônicos não nulos e diferentes.
Motivo obrigatório com até 500 caracteres, sem controles; espaços externos
são normalizados. Não recebe empresa, ator, itens, preço ou permissão no body.
POST e GET por ID não aceitam query extra. A quantidade continua limitada
pelo lote original (1–100 produtos). Versões não podem exceder a faixa segura.

## Condições para desfazer

Todos os produtos precisam ainda corresponder ao snapshot `after` do original:
versão, nome, descrição, SKU, preço e ativação. Uma edição intermediária,
mesmo que posteriormente restaure os mesmos valores, aumenta versão e
impede a reversão. Também impede uma alteração externa sem incremento de
versão que tenha modificado algum desses campos.

Não há sucesso parcial nem substituição automática das versões esperadas.
Uma falha em qualquer item impede tudo. Se houve mudança posterior, faça
nova alteração autorizada com prévia e versões atuais para corrigir os
valores desejados; não force a reversão e não edite o histórico.

Valores anteriores são restaurados, mas versões crescem: produto versão 7
alterado para versão 8 pelo lote passa à versão 9 quando revertido. A versão
não volta para 7. Outras rotinas continuam identificando a mudança.

Cada original admite uma reversão gravada. Novos IDs tentando desfazer o
mesmo original recebem 409. Repetição do ID original da reversão com mesmo
ator/conteúdo recupera seu recibo, inclusive após futuras edições. Conteúdo,
motivo ou ator diferentes com mesmo ID recebem 409. Não são permitidas
cadeias de desfazer uma reversão; uma nova correção deve ser lote independente.

## Transação e auditoria

Bloqueios: sessão → identidade da reversão → original → identidade do lote
compensatório → identidades dos itens → produtos em ordem crescente.
Essas identidades usam namespaces separados e compartilham a ordem dos
bloqueios das edições individuais e dos lotes normais.

O backend recalcula a prévia sob bloqueios, confere o hash e reaproveita
a rotina atômica de lote/edição. Os novos produtos, recibos individuais,
outboxes individuais, lote compensatório, vínculo ao original, motivo e
evento de reversão commitam juntos. Falha de metadado/evento reverte tudo.
Evento suprimido por trigger (zero linhas) não pode produzir sucesso.
Commit com resposta incerta retorna 503 sem inventar recibo de sucesso.

Recibos original e compensatório continuam no histórico de lotes. A situação
de compensação é consultada na rota específica do original; não é inferida
somente pelo valor atual do produto. O evento de reversão registra vínculo
e motivo, sem comprovar entrega a outro sistema.

## Migração e ativação explícitas

PostgreSQL `000011_online_catalog_undo.sql`: tabela de reversões com FK
composta para original/compensação/ator, unicidade por empresa+original,
e outbox da reversão. Histórico próprio `online_catalog_undo_migrations`
verifica versão 11 e checksum; exige extensão 10. Não reescreve históricos
anteriores. Sem extensão 11, a reversão falha antes de alterar os produtos.

Instalar o pacote não migra banco comercial. Ativação posterior explícita,
no ambiente autorizado, continua em backend:

```bash
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online migrate-catalog
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online check-catalog
```

Agora confere/aplica 8 → 9 → 10 → 11. Nunca executar 000001_init.sql.
Desativação do recurso preserva as tabelas e recibos; não remove auditoria
ou dados comerciais para tentar reverter o código.

## Verificação

Testes direcionados: parser/permissão, prévia sem escrita, valores exatos e
versões crescentes, edição posterior, original já revertido/cadeia, motivo
divergente, replay sem consultar produto atual, consulta por ator, migração
nova/repetida/futura/adulterada, falhas no metadado/evento e commit incerto.

O teste opt-in PostgreSQL exige banco novo isolado no PC e acrescenta ciclo
HTTP real, reversão feita por outro administrador da empresa, preservação do
original, isolamento, falha tardia após alterações e disputa entre dois IDs
de reversão do mesmo original. O instalador não cria commit se falhar. A
preparação sem servidor PostgreSQL não equivale à validação real desse ciclo.
