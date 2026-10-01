# Sangria e suprimento local

Base prevista: commit 9c11c74, com cancelamento integral validado.

POST /local/v1/cash/movements, sessão Bearer humana, aparelho ativo e contrato POS vigente.
Corpo exato: session_id, operation_id, kind, amount_cents, reason.
kind: withdrawal (sangria) ou supply (suprimento). Valor positivo em centavos inteiros.
Permissão manage_cash: dono ou gerente com vínculo ativo e acesso à loja.
O gerente pode registrar movimentação no turno de outro operador, apenas na mesma loja e aparelho.

## Garantias

- Caixa existente e aberto para nova movimentação.
- Sangria não pode ultrapassar saldo; suprimento não pode ultrapassar int64.
- Saldo esperado não aparece no retorno nem na consulta cega do turno.
- Movimento, autoria, motivo e outbox são gravados na mesma transação.
- Falha ou escrita silenciosamente ignorada desfaz todas as alterações, inclusive observação de horário do contrato.
- Repetição exata preserva identificador e horário; exige autorização e licença atuais mesmo após fechamento.
- Reutilização da chave com valor, motivo, tipo, turno ou autor diferente resulta em conflito.
- Chave idempotente tem escopo próprio do caso de uso: empresa/aparelho/operation_id.
  Use UUID novo para cada operação de negócio. Não reutilize IDs de venda/fechamento como prática de cliente.
- Fechamento considera valores assinados de cash_movements, mantendo seu comportamento anterior.
- Migração 0021 adiciona auditoria com FKs compostas para turno e movimento; não altera migrações anteriores.

## Transporte

cash.movement exige concessão explícita por parceiro. Confiança ou aprovação de cash.open/close não concede o novo evento.
Recepção durável não aplica dinheiro a outro caixa: transporte e aplicação de negócio permanecem distintos.

## Limites

API e backend; interface de sangria/suprimento ainda não criada.
Registro de retirada/entrada física: não é integração bancária nem comprovante fiscal.
A operação não transfere valores entre caixas e não permite alterar turno fechado.
Não altera a senha, o banco real da demonstração ou o processo atualmente em execução.

## Validação

Testes HTTP de fechamento, repetição, conflitos, limites monetários, JSON exato, autenticação,
permissões, revogação, contrato, falhas, concorrência, persistência e rollback do relógio.
Testes de aprovação explícita do novo evento e recepção idempotente.
Go deve ser compilado/testado na máquina de aplicação; ambiente de preparação não possui Go.
