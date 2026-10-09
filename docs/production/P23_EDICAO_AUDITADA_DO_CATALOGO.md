# P23 — Edicao auditada de produtos

Depende de P22. SQLite 0037_production_catalog_edits.sql: numero livre na branch.
Preserva migracoes antigas. Whitelist explicita do backup passa a 25-37, sem
mudar formato, criptografia, checksums ou validacoes de aparelho/esquema/integridade.

POST /local/v1/catalog/products/:id/update: operation_id, expected_revision,
sku, name, unit, barcode, price_cents, cost_cents. ID/empresa/loja nao vem do corpo.
GET /local/v1/catalog/products/:id/edit-state: product_id, revision, repeated=false.
Revisao inicial 0, incrementada a cada edicao; nao e a revisao da receita.
Exige manage_stock, como o escritor do catalogo existente; mutacao exige
Inventory ativo. Edicao vale para o produto global da empresa, nao somente loja.

SKU/nome/barcode aparados. SKU 1-100 bytes, nome 1-255, barcode ate 128, vazio
remove barcode. Preco/custo em centavos inteiros de 0 a 9007199254740991.
Unidades existentes apenas. Nesta etapa unit deve ser a mesma unidade atual;
P24 acrescenta a regra para mudar unidade de produto sem dados dependentes.
SKU/barcode duplicado, revisao obsoleta ou replay divergente retorna 409.
Produto ausente 404; entrada desconhecida, fracionaria ou invalida 400.

Edicao, revisao, auditoria before_json/request_json e outbox atomicos. Replay
identico retorna revisao original sem duplicar evento. GET edit-state informa
revisao atual, apos checar autorizacao; nao inclui precos/custos. Falha de trigger
que ignora auditoria deve reverter edicao inteira. Evento pendente da outbox nao
demonstra sincronizacao ou importacao entre aparelhos, que permanece pendente.

Produto atual muda, mas snapshots de receitas, ordens, compras e vendas nao sao
reescritos. Leitores antigos e ocultacao de custo para papeis nao autorizados
permanecem intactos. Dados da auditoria com custos nao recebem rota publica.

Compartilhados: server.go monta rotas; backup e testes de migracao recebem apenas
incrementos minimos. Novos arquivos catalog/edit.go e localapi/catalog_edit.go.
Testes: revisao/replay, duplicidade, rollback, autorizacao/escopo, centavos,
preservacao de item comprado e backup/restauracao de metadados/revisao/auditoria.
Nao inativa produto, nao recebe compra, nao move estoque. Sem merge/push.
