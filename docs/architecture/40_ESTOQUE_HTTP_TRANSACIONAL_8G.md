# Fase 8G — Estoque pela API local

## Rotas

- `POST /local/v1/stock/operations`: registra `entry`, `transfer` ou `loss`.
  Recebe `operation_id`, `kind`, `product_id`, `from_location_id` e/ou
  `to_location_id`, `quantity_milli` e `reason`.
  Empresa, loja, operador e aparelho são derivados da sessão protegida.
- `GET /local/v1/stock/balance?product_id=ID&location_id=ID`: consulta saldo
  da empresa e loja da sessão. Exige permissão `ManageStock` e aparelho ativo,
  mas não contrato vigente. Não publica estoque para fornecedor ou consumidor.

Quantidades são inteiros em milésimos: 1000 representa uma unidade da medida
do produto. Não usar float para interpretar a entrada. O backend trabalha
com int64; clientes JavaScript precisarão tratar a representação exata de
valores grandes, sem pressupor que Number cobre toda a faixa int64.

Novos movimentos exigem contrato `inventory`, permissão e aparelho ativo,
validados dentro da mesma transação de operação, movimentos e outbox.
Sem emissor: 503; sessão inválida: 401; sem módulo/permissão/vigência: 403.
Corpo inválido ou referência fora da empresa/loja: 400. Saldo insuficiente,
overflow ou reutilização conflitante de operação: 409. Falha de persistência:
500, sem divulgar SQL ou dados internos.

Operação nova retorna 201 e `repeated: false`. Repetição idêntica e autorizada
retorna 200 e `repeated: true`; não duplica saldo, movimentos ou evento.
Repetições também consultam licença e permissões atuais. Consulta histórica
de recibo de operação, distinta da consulta de saldo, ainda não foi entregue.

## Domínio e dados

`stock.RecordWithContract` compartilha o núcleo de gravação com o caminho
legado. O chamador resolve sessão/prova do aparelho; db e Store usam o mesmo
SQLite. Não existe preflight de licença em uma transação separada.

Produto e destino são validados na empresa/loja antes da gravação; não depende
de uma falha de FK para descobrir uma entrada de produto estrangeiro.
Saídas não superam saldo local. Acréscimos não podem superar MaxInt64.
IDs de operação, produto e locais têm limite de 128 bytes.

Outbox falhando desfaz toda a operação e a observação de horário do contrato.
Nenhuma migração nova, alteração de schema ou banco real nesta etapa.

`stock.Record` permanece legado sem licença para migração de chamadores
internos; NÃO é usado pela nova rota. Isto não afirma que estoque de todas as
outras funções ou a API PostgreSQL antiga foram licenciados nesta entrega.

## Limites e verificação

Doze testes HTTP novos em SQLite descartável cobrem entradas/transferências/
perdas, idempotência/conflito, sessão/papel/aparelho, módulo/configuração,
rollback da outbox e relógio observado, saldo/referências inválidas,
empresa/loja estrangeira, JSON e quantidade inválidos, overflow, reabertura,
expiração com consulta preservada e duas requisições idênticas concorrentes.

Não há transmissão real de outbox, recebimento de pedido B2B, baixa fiscal,
rede LAN, autonomia móvel ou reserva de doca nesta etapa. Registro manual
de entrada não representa automaticamente confirmação de compra/financeiro.
Rotas de caixa/venda serão integradas em seguida com seus próprios requisitos.

O ambiente de preparação não possui Go: patch pode ser conferido, mas testes,
vet e build devem ser executados no PC antes de declarar a etapa pronta.
