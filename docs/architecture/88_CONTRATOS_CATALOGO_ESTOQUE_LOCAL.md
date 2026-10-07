# Entrega 13 — Contratos detalhados de catálogo e estoque locais

Base: `36da957`. Escopo: sete operações HTTP existentes; documentação e testes. Não altera handlers, migrações, dinheiro, autenticação, frontend nem a branch de produção.

`docs/api/catalog-stock.openapi.json` descreve as consultas e criações de produtos/locais, busca no catálogo, saldo e movimentos de estoque. Inclui campos, tipos, permissões, limites, variantes de resposta e repetição de operação. O mapa da entrega 12 aponta para o contrato detalhado dessas sete operações.

O teste HTTP lê os schemas do documento e confere respostas reais de bancos temporários: listas cheias/vazias, busca cheia/vazia, saldo, criação de produto/local, entrada nova e repetida. Reuso conflitante do ID deve manter um único movimento/outbox e o saldo original. O checker cobre o subconjunto de schemas utilizado, não toda a especificação OpenAPI. Testes anteriores continuam cobrindo autorização, transferência, perda, rollback e concorrência.

Compatibilidade verificada no código:

- Local listado tem `ID` maiúsculo, diferente de `id` dos produtos e respostas de criação. O contrato respeita isso para não quebrar clientes.
- Preço/custo local são centavos int64; busca impõe adicionalmente o limite seguro de JavaScript. Lista simples/cadastro não possuem esse mesmo limite explícito. Clientes devem manter suas verificações de dinheiro exato.
- `quantity_milli` significa milésimos da unidade cadastrada. Para `kg`, 1000 equivale a 1 kg; para `g`, 1000 equivale a 1 g. A API de estoque atual permite frações de `unit`; eventual bloqueio requer mudança funcional separada.
- Movimentos aceitam `operation_id` e replay; criação de produto/local não tem esse protocolo. Não anunciar idempotência nesses cadastros.
- JSON de estoque rejeita campos desconhecidos, duplicados e conteúdo extra. Cadastro de produto/local usa parser legado; não prometer a mesma rigidez sem implementação.
- SKU/barcode duplicados podem retornar 500 pelo tratamento legado. O contrato não inventa correção para 409.
- Saldo exige `manage_stock`; leitura de catálogo exige `view_catalog`. Leituras não instalam contrato e não concedem módulos.
- Busca não pesquisa marca/fornecedor porque esses campos ainda não estão no modelo desse catálogo.

Validação: checker JSON/referências, teste de payload HTTP, testes de domínio/API existentes, vet e race direcionado. Nenhum processo ou banco real é iniciado. Catálogo online, contratos de caixa/vendas e demais domínios seguem pendentes; não são substituídos pelo documento local.
