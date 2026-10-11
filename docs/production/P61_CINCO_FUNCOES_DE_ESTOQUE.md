# P61 — Cinco funções de estoque na interface

Base coordenada: `3157348`, branch `feat/producao-receitas`.

## Entrega

1. Entrada manual de estoque com revisão explícita, quantidade exata e motivo.
2. Transferência entre locais com origem/destino distintos e gravação atômica existente.
3. Perda manual de estoque com motivo e proteção do saldo reservado existente.
4. Consulta do recibo original de uma movimentação pelo identificador.
5. Histórico paginado das movimentações manuais por produto/local.

Acesse **Estoque e locais** (`/local/stock`). Produtos são os da página carregada; locais são os da loja. Revisão não grava estoque. Quantidade é convertida com BigInt para milésimos da unidade explícita do produto, sem conversão volume/massa. Produtos por unidade exigem quantidade inteira. A interface e os novos recibos limitam quantidades a 9007199254740991 milésimos; valores fora desse intervalo são recusados, sem arredondamento.

## Contratos compartilhados alterados

- `backend/internal/localapi/stock.go`: acrescenta duas consultas GET e permite o campo opcional `expected_unit` no POST existente. As regras de sessão/contrato existentes continuam sendo usadas.
- `backend/internal/localdb/stock/stock.go`: quando `expected_unit` é informado, confere a unidade atual do produto dentro da mesma transação autorizada antes de gravar/repetir. Unidade diferente gera 409; unidade inválida ou quantidade incompatível gera 400. Clientes antigos que omitem o campo mantêm o contrato anterior. A pré-condição `expected_unit` é exclusiva do pedido HTTP: ela é omitida do evento `stock.operation`, preservando seu formato e schema 1.
- `LocalCatalog.tsx`: substitui o formulário antigo de entrada pelas cinco funções e mantém cadastro de locais, catálogo, seus filtros e paginação. Cadastro de produto/local conserva seu fluxo anterior.
- `package.json`: acrescenta os testes das novas funções ao `test:local`.

Sem alteração de autenticação, infraestrutura, criptografia, formato de backup ou migrações. SQLite permanece **44**, última migração `0044_production_receipt_voids.sql`; nenhuma nova numeração utilizada.

## Recibos e história

`GET /local/v1/stock/operations/{id}/receipt` exige `manage_stock`, aparelho aprovado e operador/aparelho originais, na mesma empresa e loja. Ausência ou escopo diferente resulta 404. Retorna o pedido persistido, responsável, aparelho, data e unidade; não deriva quantidade do saldo atual. A unidade de produtos com movimentos não pode ser alterada pelo contrato de edição existente.

`GET /local/v1/stock/operations/history?product_id=...&location_id=...&offset=0` exige `manage_stock` e aparelho aprovado. Inclui operações manuais de todos os operadores/aparelhos da empresa nesta loja. Página de 50, ordem data descendente e ID descendente como desempate, total global e indicador de próxima página no mesmo snapshot. Transferência aparece uma vez no histórico de cada local envolvido. Não é um extrato completo: vendas, inventários, compras e produção têm registros próprios. Consultas não reservam nem baixam saldo; continuam sendo autorizadas pela API, independentemente da permissão exibida na tela.

## Idempotência e falhas

Uma pendência compartilhada entre os três formulários é persistida por empresa/loja/aparelho/operador antes do POST. Web Locks protegem abas, e falha de armazenamento impede gravação. Cada ação começa consultando o recibo; confirmação exige recibo correspondente aos dados preservados. Perda de resposta mantém o mesmo ID e pedido. Montar a tela não repete POST. Resultado incerto não pode ser descartado; uma repetição exige clique explícito.

Entradas do formato antigo em `titan-stock-entry` são reconhecidas como incertas e convertidas sob o mesmo bloqueio, preservando ID, produto, quantidade, destino e motivo. Primeiro consulta o recibo; repete somente a pedido do operador. Como pedidos antigos não preservavam a unidade, sua repetição mantém o contrato legado sem `expected_unit`. Não mantenha a versão antiga da interface aberta durante a atualização; a nova fila tem formato diferente e a versão antiga não sabe resolvê-la.

O servidor permanece responsável pelo contrato Inventory ativo, autorização, referências, atomicidade, overflow e saldo livre após reservas. Novas consultas não alteram as regras de reserva/consumo. Não automatiza recebimento de compra por uma entrada avulsa.

## Validação

Os testes novos cobrem os cinco fluxos, payloads e unidades exatas, limite inteiro, conflito de unidade, referências e filtros, isolamento, paginação 51/50, recibo original após movimentos posteriores, backup/restauração em bancos temporários, estoque reservado, idempotência e recuperação de resposta perdida. Verificam que consultas e recusas não alteram movimentos/eventos/reservas.

Aplicador: testes específicos, suíte completa, `go vet`, detector de corrida dos testes HTTP de estoque, testes locais da interface, compilação TypeScript/Vite, gofmt e `git diff --check`; só então adiciona os arquivos explícitos e cria o commit. Para em falha preservando as alterações. Não inicia servidor, não aponta banco real, não faz merge/push.

Validação visual no computador do usuário e integração revisada com main permanecem pendentes. Esta entrega não conclui todas as ações avançadas de compras/recebimento nem representa percentual global do projeto.
