# P64 — Cinco funções de pedidos locais

Base: `0b03236`, branch `feat/producao-receitas`, após P63 aprovada.

## Cinco funções na página `/local/orders`

1. Consultar pedido preservado: fornecedor, produto, unidade e quantidade exata,
   situação comercial e situação de recebimento separadas.
2. Consultar a aprovação humana que originou o pedido: sugestão, operação,
   responsável, aparelho, motivo e data.
3. Consultar a auditoria original de criação: operação, responsável, aparelho e data.
4. Cancelar pedido local ainda não autorizado para recebimento: motivo obrigatório,
   prévia dos itens e confirmação explícita, com estado esperado preservado.
5. Consultar o recibo original do cancelamento pelo ID da operação, sem repetir POST.

As três primeiras funções aparecem em seções distintas do mesmo snapshot autorizado
obtido por `GET /purchase-orders/{id}/trace`. Não são consultas comerciais remotas.
As quantidades são milésimos da unidade explícita, formatadas sem ponto flutuante.
Não há preço, frete, total negociado, envio ao fornecedor ou confirmação remota inferidos.

## Cancelamento e integridade

Usa o escritor existente da P26, sem modificá-lo. O estado esperado é
`local_not_sent`, e autorização de recebimento já registrada impede cancelamento.
A autorização concorrente também é conferida pelo servidor dentro da transação.
A decisão conserva documento, itens, fornecedor original e aprovação de reposição;
a mesma aprovação não fica disponível para outro pedido. Não movimenta estoque,
reservas, caixa ou pagamentos. Não é anulação de recebimento nem devolução comercial.

A nova fila de cancelamento tem chave exclusiva por empresa, loja, aparelho e operador.
Web Lock e trava local bloqueiam decisões simultâneas pela fila. Persistência é
conferida antes do envio. Ao abrir a tela, nenhuma escrita é feita automaticamente.
Antes de POST, consulta-se o recibo pelo ID original. Uma resposta perdida mantém
incerteza e é resolvida por GET ou repetição explícita com os mesmos dados/ID.
POST bem-sucedido só limpa a fila após um GET original compatível.

Recusa inicial definitiva permite descarte explícito somente após GET 404.
Uma operação já incerta não é descartada por recusa posterior. Falha ou conteúdo
inválido do armazenamento bloqueia novos cancelamentos. Alterar campo, consultar
outro pedido ou trocar token invalida a prévia. Após sucesso, consulte novamente;
listas e detalhes previamente carregados são snapshots, não atualização automática.

## Contratos e arquivos compartilhados

SQLite permanece em **44**, sem migração e sem alteração da compatibilidade do backup.
Autenticação, infraestrutura, serviços online, formato de backup e recuperação intactos.

- `GET /local/v1/purchase-orders/{id}/trace`: existente, somente leitura.
- `POST /local/v1/purchase-orders/{id}/cancel`: existente, com
  `operation_id`, `expected_status`, `reason`, sem campos de escopo no corpo.
- **Novo:** `GET /local/v1/purchase-cancellation-operations/{operation_id}/receipt`.
  Sem query string; devolve `input` e `result`, original com `repeated=false`.
  Valida JSON canônico, correspondência com a auditoria do cancelamento, motivo,
  data, documento original e ausência de autorização de recebimento contraditória.

As consultas usam `ViewOrders` e prova do aparelho/escopo. O recibo exige também
operador e aparelho originais, e responde 404 fora desse contexto autorizado.
Escrita exige `manage_replenishment` e contrato ativo `orders`, pelo servidor.
Não há ampliação de permissões nem escritor novo.

- `server.go`: monta apenas o GET de recibo.
- `LocalOrders.tsx`: integra componente independente e exibe `cancelled` na lista/detalhe.
- `purchaseClient.mjs`: corrige validação antiga que aceitava apenas `local_not_sent`;
  passa a aceitar também o estado `cancelled`, já existente na API. Estados fictícios
  como `sent` continuam recusados. A fila anterior de criação mantém seus contratos,
  campos e IDs, e consegue confirmar a criação preservada de um pedido depois cancelado.
- `package.json`: inclui testes novos.

## Verificação

HTTP novo: recibo original, replay, ausência de movimento de estoque/dinheiro,
backup/restauração, queries indevidas, empresa/loja/aparelho/operador, papel negado,
JSON/motivo/data adulterados. Testes existentes de cancelamento, rastreabilidade
 e autorização cobrem documentos preservados, rollback, revisão de decisões e corrida.

Frontend: consistência entre pedido/aprovação/criação, quantidades exatas seguras,
recusa de cancelamento após autorização, prévia e campos congelados, corpo/rota exatos,
recibo incompatível, resposta perdida, repetição, descarte, falha de armazenamento,
lista/detalhe cancelados e fila antiga de criação após cancelamento posterior.

Aplicador confere base limpa, checksum, patch, gofmt, testes específicos, suíte completa,
vet, corrida, testes frontend e build, e registra somente a P64 após sucesso.
Bancos de teste temporários; Fiber em memória, sem servidor ou banco real/demonstração.
Sem merge ou push. Aceitação visual em navegador não foi realizada nesta entrega.
