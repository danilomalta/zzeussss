# Entrega 26 — Lotes de preço e ativação no catálogo online

Base: main 6fb33cd, entrega 25 validada em PostgreSQL isolado no PC.
Esta entrega mantém a frente de produção separada e não altera SQLite,
frontends, estoque, vendas históricas ou migrações já publicadas.

## Funcionalidades e responsabilidades

| Função | Comportamento implementado | Como verificar |
|---|---|---|
| Prévia | Retorna antes/depois de 1 a 100 produtos, na mesma visão consistente | Produtos e recibos permanecem iguais após prévia |
| Aplicação | Preço em centavos ou ativação, controle de versão, todos os itens na mesma transação | Falha no segundo outbox reverte ambos os produtos |
| Recuperação | Recibo estável por empresa, ator, ID e conteúdo | Consultar após perda de resposta; replay não aumenta versões |
| Histórico | Consulta paginada dos lotes da empresa, com ator e recibos dos itens | Limite/offset, vazio real, isolamento e filtros rejeitados |

Somente owner/admin/manager podem usar todas estas rotas. Stock e cashier
não recebem permissão por acessar o catálogo. O backend deriva empresa,
usuário, papel e sessão do acesso autenticado. Na transação de escrita,
confere novamente associação, papel, empresa ativa, validade e revogação.
Estas são permissões online; não implementam delegações por departamento
nem contratos de módulos do SQLite.

## Fluxo e contrato

1. Cliente salva um `operation_id` UUID canônico antes da operação.
2. Envia POST `/api/v1/catalog/batches/preview` com `operation_id` e `items`.
   Cada item tem `product_id`, `expected_version`, `action` e somente
   `price_cents` (action price) ou `ativo` (action active).
3. Confere a lista retornada: `before`, `after`, ação e `preview_hash`.
4. Após decisão humana, envia POST `/api/v1/catalog/batches/apply` com os
   mesmos itens, o mesmo ID e o `preview_hash` retornado. O servidor
   normaliza a ordem por ID; mudar a ordem não cria outra operação.
5. Em resposta perdida/503, conserva os dados e consulta GET
   `/api/v1/catalog/batches/{operation_id}`. Não reenvia automaticamente.
   Só uma consulta confiável permite decidir por repetição explícita com
   conteúdo e ID originais. Uma consulta falha não comprova ausência.
6. GET `/api/v1/catalog/batches?limit=10&offset=0` consulta histórico da
   empresa (limite padrão e máximo de 10 lotes, pois cada recibo inclui até 100 produtos). A consulta por ID é restrita ao operador original; o histórico
   administrativo pode mostrar operações de outros operadores da empresa.

HTTP 200 representa prévia ou resultado gravado/replay, conforme a rota.
400: entrada inválida ou sem mudança; 403: sem permissão;
404: produto/recibo ausente no contexto; 409: conflito de versão/hash/ID;
503: indisponível ou resultado de commit incerto. Erros não exibem SQL,
tokens, valores de conexão ou dados internos. Todas as consultas enviam
Cache-Control no-store. Erros negociados usam o contrato v1 existente.

Exemplo de prévia (IDs e versões fictícios):

```json
{"operation_id":"11111111-1111-4111-8111-111111111111","items":[{"product_id":1,"expected_version":3,"action":"price","price_cents":475},{"product_id":2,"expected_version":2,"action":"active","ativo":false}]}
```

São recusados: campos duplicados/desconhecidos/nulos, IDs estrangeiros,
produto repetido mesmo com outra ação, vazio, mais de 100 itens, dinheiro
fracionário/negativo/acima de 999999999999, versões fora da faixa segura,
query em POST/consulta por ID e filtros extras no histórico. Corpo máximo
64 KiB. Não há sucesso parcial nem itens silenciosamente ignorados.
Um produto que já tem o valor solicitado impede o lote; corrija a prévia.

## Concorrência, auditoria e publicação

A prévia usa repeatable read e bloqueio da sessão, mas não altera produtos,
auditoria ou eventos. Não é uma reserva e não promete duração de validade.
O hash SHA-256 inclui ator, empresa, operação e snapshots. Ele identifica
o conteúdo da prévia, não é segredo, assinatura ou aprovação de supervisor.
Não substitui a decisão humana e a autorização no servidor.

Aplicação: bloqueio da sessão → ID de lote → IDs determinísticos dos itens
→ produtos em ordem crescente. A prévia é recalculada sob bloqueios de
produto. Qualquer versão ou hash diferente recusa tudo antes das mudanças.
Cada item incrementa a versão uma vez e reaproveita a mesma rotina de
edição individual, auditoria `online_catalog_operations` e outbox existente.
O recibo de lote e todos os produtos/auditorias/outboxes commitam juntos.
Um trigger que suprima/mude gravação também não pode produzir sucesso
inventado. A perda da resposta de commit retorna incerteza recuperável.

As operações dos itens recebem UUIDs determinísticos por lote/produto.
Uma repetição do lote compara ator e hash do pedido antes de verificar
versões atuais: recupera o resultado original mesmo após edição posterior.
Mesmo ID com outro ator/conteúdo retorna 409. Outro ator não consulta o
recibo por ID (404); um administrador autorizado pode consultar o histórico.
As alterações aparecem também no histórico individual de cada produto.

Outbox é intenção durável: não implementa trabalhador, envio, confirmação,
reconciliação ou atualização de caixas. Alterações não reescrevem preços
gravados em vendas anteriores. Não se pode chamar esse fluxo de sincronizado.

## Migração e ativação explícitas

PostgreSQL `000010_online_catalog_batches.sql` cria recibos e índice do lote.
Histórico próprio `online_catalog_batch_migrations` valida versão 10 e checksum,
exige cadastro 9 previamente instalado e rejeita versões futuras/adulteradas.
Não reescreve os históricos 8/9. Todos permanecem consultáveis pela versão
anterior que conhece suas respectivas estruturas. Não representa garantia de
compatibilidade de novos recursos com binários antigos.

O instalador do pacote somente modifica código e testa em banco novo
isolado. Nunca migra banco comercial. Para ativação posteriormente no
ambiente autorizado, em backend, a ferramenta explícita permanece:

```bash
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online migrate-catalog
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online check-catalog
```

Agora migrate-catalog confere/aplica 8 → 9 → 10; check-catalog verifica
todos. Antes da extensão 10, a aplicação do lote falha indisponível antes
de mudar produtos. Nunca executar 000001_init.sql. Para desativar o recurso,
retire acesso às novas rotas; não apague recibos ou auditoria para fazer rollback.

## Verificações e limites

Testes: parser estrito, limite/perfil, prévia sem gravação, snapshots exatos,
versão desatualizada, hash divergente, falha do segundo evento, falha do
recibo de lote, commit incerto, replay sem acessar versão atual, escopo de
consulta, migração repetida/futura/adulterada e rotas HTTP montadas.
O teste PostgreSQL opt-in acrescenta ciclo real, concorrência, isolamento,
rollback após primeira alteração e preserva as verificações das entregas
anteriores. O pacote exige esse teste no PC antes de criar commit.
Preparação sem servidor PostgreSQL não equivale à validação real.

Esta entrega não inclui desfazer lote, importação CSV/XML, reajuste percentual,
custo/margem, promoção por período, fiscal, embalagens, barcodes, pesquisa
por fornecedor ou interface. Para corrigir um lote aplicado, faça nova prévia
com versões atuais e valores desejados; não edite o histórico e não repita
uma operação antiga com conteúdo diferente. Desfazer automático necessita
operação compensatória própria que respeite alterações intermediárias.
