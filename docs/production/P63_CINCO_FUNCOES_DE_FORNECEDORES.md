# P63 — Cinco funções de fornecedores na interface local

Base de aplicação: `e1eccb70142308041d2648c836a8a9b4e0d25996` (`feat/producao-receitas`).

## Entrega

Na página `/local/orders`, abaixo da fila de criação já existente:

1. Consultar cadastro atual por ID: nome, estado e revisão.
2. Editar nome, preservando o estado consultado.
3. Ativar fornecedor, preservando o nome consultado.
4. Inativar fornecedor, preservando o nome consultado.
5. Consultar histórico auditado paginado: cadastro original, antes/depois, motivo,
   responsável, aparelho, operação e data de cada revisão.

As três decisões exigem motivo, revisão explícita e confirmação de uma prévia.
Editar campo ou consultar outro estado invalida a prévia. Token/contexto novo limpa
os resultados. Após confirmar, uma nova consulta é exigida para outra decisão.
Inativação usa a proteção existente para novos pedidos. Pedidos e recebimentos
históricos conservam os nomes e dados preservados originalmente.

## Contratos

Não há nova migração. SQLite permanece em **44**, com backup inalterado.
Nenhuma migração anterior, autenticação, infraestrutura, formato de backup ou
recuperação foi alterada.

- `GET /local/v1/purchase-suppliers/{id}/edit-state`: contrato P25 existente.
- `POST /local/v1/purchase-suppliers/{id}/update`: contrato existente, sem mudanças;
  `operation_id`, `expected_revision`, `name`, `status`, `reason`.
- `GET /local/v1/purchase-suppliers/{id}/history?offset=N`: contrato existente,
  página de 50 revisões em ordem crescente, totais globais.
- **Novo:** `GET /local/v1/purchase-supplier-operations/{operation_id}/receipt`.
  Sem query string; retorna `input`, `result`, `before_name`, `before_status`,
  `actor_id`, `device_id`, `created_at`. Resultado original com `repeated=false`,
  independente das alterações posteriores. Valida o JSON canônico preservado,
  revisão, estado anterior e data; inconsistência retorna 409.

As consultas usam a autorização existente `ViewOrders` e prova do aparelho,
empresa e loja. O recibo também exige o mesmo operador e aparelho da operação;
fora do escopo retorna 404 após autorização. Escritas continuam exigindo
`manage_replenishment` e contrato ativo do módulo `orders`, verificados pelo
servidor dentro da transação. A UI não amplia essas permissões.

Arquivos compartilhados: `server.go` recebe apenas a montagem do novo GET;
`LocalOrders.tsx` recebe o componente independente; `package.json` inclui os
novos testes. A criação de fornecedores/pedidos e sua fila antiga são preservadas.
Nenhum escritor de fornecedor foi modificado. A fila nova é exclusiva das
alterações de fornecedor e não migra a fila de criação anterior.

## Idempotência e resposta perdida

As três decisões usam o mesmo escritor auditado de revisão existente e a mesma
fila nova, isolada por empresa, loja, aparelho e operador. Web Lock e trava local
impedem duas submissões pela fila em abas concorrentes. O servidor mantém a
proteção de revisão para decisões em outras filas/aparelhos.

Antes de enviar, a operação é persistida e consultada por seu ID original.
Não há POST automático ao abrir a tela. Repetição é explícita, com os mesmos
campos/ID/revisão. POST bem-sucedido só libera a fila após GET do recibo original
compatível. Resposta perdida ou malformada conserva a incerteza. Uma primeira
recusa definitiva permite descarte explícito somente após GET 404; uma operação
já incerta nunca é descartada por uma recusa posterior. Falha de armazenamento
bloqueia novas escritas. O recibo não usa o cadastro atual como confirmação.

## Validação

- HTTP: recibo original após edição posterior/replay; ausência de mutação;
  backup/restauração; escopo de empresa/loja/aparelho/operador; papel negado;
  query indevida; JSON, antes/depois e data adulterados.
- Testes existentes de fornecedor: revisão obsoleta, concorrência, rollback,
  contrato/autorização, origem e paginação de 51 mudanças.
- Frontend: nome/estado preservados, limites UTF-8, corpo/rota exatos,
  recibos incompatíveis, resposta perdida, repetição explícita, descarte seguro,
  falha de armazenamento e validação das páginas/histórico.
- Aplicador: base e árvore limpa, checksum, patch verificável, gofmt, testes
  específicos, suíte completa, vet, corrida, testes frontend e build antes do commit.

Testes locais usam bancos temporários e aplicação Fiber em memória. Não abrem
servidor nem usam bancos reais ou da demonstração. Não houve aceitação visual
em navegador nesta entrega; testes automatizados e build não substituem essa
verificação. Não faz merge nem push.
