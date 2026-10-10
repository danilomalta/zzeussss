# P58 — edição do catálogo pela interface

Base P57 confirmada 02c87a8. Schema 44 preservado.

Contrato compartilhado mountCatalogEdit acrescenta GET /local/v1/catalog/products/{id}/edit-details e GET /local/v1/catalog/operations/{kind}/{id}, kind edit ou state. A primeira resposta reúne product_id, revisão de edição e ProductInput na mesma transação. Exige aparelho autorizado, ViewCatalog e ManageStock; custo permanece limitado aos gestores de estoque. O recibo é do operador/aparelho/loja originais dentro da empresa, valida payload canônico e devolve resultado da revisão original após mudanças posteriores. Nenhuma rota antiga ou migração é alterada.

POST /catalog/products/{id}/update conserva campos existentes: operation_id, expected_revision, sku, name, unit, barcode, price_cents e cost_cents. ProductID vai no caminho e no recibo, não no corpo HTTP. Nome/SKU/barcode têm limites UTF8 do backend; valores em centavos seguros, sem ponto flutuante. Revisão consultada é congelada. O servidor continua recusando troca de unidade quando houver estoque histórico, vendas, receitas, compras ou outros vínculos.

Fila própria titan-catalog vinculada à empresa/loja/aparelho/operador, Web Locks e payload persistido antes de POST. Recibo consultado primeiro; montagem/recarregamento não reenviam. Resposta incerta bloqueia descarte e novas edições; confirmação GET compara todos os campos. Catálogo e produção têm filas distintas. Os cadastros básicos legados de produto/local e entrada de estoque não são alterados neste lote.

Arquivos compartilhados: catalog_edit.go registra consultas; LocalCatalog inclui novos formulários sob CatalogPending; package.json registra testes. CatalogPending usa manage_stock e módulo inventory ativo para escrever, e permite consultar recibos sujeitos à autorização histórica da API. P59/P60 acrescentam estado/histórico à mesma área.

Testes: edições posteriores e reativação preservam recibos anteriores, backup/restauração em DB temporário, operador estrangeiro, payload adulterado e caixa sem acesso a custo/edição; frontend testa dinheiro no limite seguro, contexto, recusa, persistência e resposta perdida sem repetir edição. Suíte backend, vet, corrida específica e suíte frontend/build antes do commit. Nenhum servidor ou banco comercial/demonstração utilizado.
