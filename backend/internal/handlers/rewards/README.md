# `internal/handlers/rewards/` (Endpoints de recompensas)

Esta pasta contém os handlers HTTP do sistema de recompensas, especialmente indicações.

## Objetivo

- Registrar indicações de forma idempotente.
- Consultar saldo e histórico.
- Conceder créditos conforme eventos do sistema (ex.: conta aprovada, primeira compra).

## Regras de segurança

- Prevenir abuso (indicador tentando gerar múltiplos créditos para o mesmo evento).
- Auditar todas as concessões e tentativas suspeitas.
- Validar permissões por perfil (Dono/Contador podem ter visão ampliada).

