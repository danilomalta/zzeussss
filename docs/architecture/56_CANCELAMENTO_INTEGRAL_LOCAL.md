# Cancelamento integral local em dinheiro

Entrega técnica: Backend_Cancelamento_01, baseada no commit f626ac8.
Relaciona-se ao aceite do PDV na fase original 5. Não conclui a fase inteira.

## Comportamento

POST /local/v1/sales/:id/cancel, com a sessão humana Bearer já validada.
Corpo exato: {"operation_id":"UUID NOVO","reason":"Motivo e confirmação da devolução"}.
Empresa, loja, aparelho, usuário e valor não são aceitos no corpo.

Dono ou gerente com vínculo ativo e acesso à loja pode cancelar integralmente
uma venda em dinheiro do mesmo aparelho. O caixa original deve continuar aberto,
mesmo que outro turno já exista. A autorização exige permissão cancel_sale e
contrato POS válido na mesma transação. Caixa/vendedor não recebe essa permissão.
Não se exige que o gerente seja o vendedor original. IDs de operação são
restritos a 128 bytes; motivo obrigatório com até 500 caracteres.

A operação registra responsável, motivo, horário, hash do pedido, valor e links
aos movimentos compensatórios. Devolve a quantidade original ao local original,
registra saída de dinheiro, marca pagamentos como reversed e venda como cancelled,
e insere sale.cancelled na outbox. Todos os efeitos compartilham a transação.
A venda, seus itens, preços, pagamentos, movimento original e evento sale.committed
não são apagados; valores históricos permanecem disponíveis.

Resposta 200 inclui sale_id, operation_id, refunded_cents, cancelled_at e repeated.
Repetição idêntica pelo mesmo autorizador retorna o resultado original, inclusive
após fechamento ou reabertura do banco. Toda repetição revalida autorização e
contrato atual. Outro pedido, outra identidade ou motivo diferente gera conflito.
Repetir POST de conclusão após cancelamento retorna 409; a consulta da venda
mostra cancelled e os metadados de cancellation, sem simular nova venda.

400: pedido inválido/ambíguo; 401: sessão inválida/revogada; 403: permissão ou
contrato insuficiente; 404: venda fora do contexto; 409: estado incompatível,
saldo insuficiente, overflow ou conflito; 503: emissor ausente. Falha de escrita
retorna erro e faz rollback. Escrita ignorada por trigger também causa rollback.

## Persistência e comunicação

Migração nova 0020_sale_cancellations.sql, sem alterar migrações aplicadas.
Tabelas sale_cancellations e sale_cancellation_stock, com FKs e unicidade para
impedir estorno duplicado e vincular empresa, loja, aparelho e venda.
O teste de quantidade de migrações é atualizado de 19 para 20.

O protocolo reconhece sale.cancelled e o comando de aprovação aceita esse tipo.
Parceiros existentes NÃO ganham a permissão automaticamente. O dono deve aprovar
sale.cancelled explicitamente; uma permissão de sale.committed não é suficiente.
O limite anterior de quatro tipos por pedido administrativo permanece; é possível
conceder a nova permissão em um pedido separado. Sem a aprovação o evento permanece
pendente; nunca é confirmado por simulação. Recebimento confirma armazenamento
durável do evento, não aplicação automática de um estorno no banco remoto.

## Limites

Somente estorno operacional integral em dinheiro, com mercadoria devolvida ao
local original e caixa original aberto. A requisição representa uma confirmação
humana dessa devolução; o software não verifica fisicamente a mercadoria.
Caixa fechado, devolução parcial, produto danificado sem retorno ao estoque,
cartão, PIX e cancelamento fiscal exigem fluxos próprios e não são implementados.
Não há mudança de frontend, pagamentos externos, emissão fiscal ou banco móvel.
A consulta da venda mantém o isolamento anterior ao vendedor; o gerente não ganha
uma consulta geral de vendas nesta entrega.

## Verificação

Novos testes Go cobrem estorno integral, fechamento, repetição após fechamento,
gerente distinto do vendedor, papéis e revogações, contrato e sessão obrigatórios,
caixa original fechado, IDs conflitantes, JSON ambíguo, rollback de cada efeito e
da observação do relógio, triggers que ignoram escrita, concorrência, reabertura,
saldo de caixa, overflow de estoque, linhas repetidas, isolamento, expiração,
aprovação administrativa explícita e recepção sem aplicação de negócio.

No ambiente de preparação foi verificada a migração SQLite de 19 para 20 com
preservação de dados e foreign_key_check, além da aplicação e whitespace do patch.
O ambiente de preparação não possui Go instalado: a suíte Go, go vet e build
precisam passar no PC antes de registrar o commit. Os testes usam bancos temporários,
sem abrir a instalação de demonstração. A API em execução continua com seu binário
anterior até ser reiniciada deliberadamente depois da validação.
