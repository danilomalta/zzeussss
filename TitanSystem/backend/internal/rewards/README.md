# `internal/rewards/` (Recompensas e gamificação)

Esta pasta contém o sistema de **recompensas** do TitanSystem, incluindo indicações.

## Objetivo

- Incentivar indicações com regras claras:
  - **R$ 50** para o responsável que indicou um novo negócio/conta aprovada
  - **R$ 10** para ações menores (ex.: primeira compra do indicado, dependendo da regra final)

## Princípios

- Regras de recompensa devem ser **idempotentes** (o mesmo evento não pode gerar crédito duas vezes).
- Toda concessão deve ser **auditável** (quem, quando, por qual motivo e qual valor).
- O crédito deve ter estado (ex.: pendente, aprovado, estornado) e trilha de evento.

## O que deve morar aqui

- Modelos e serviços de:
  - indicações (quem indicou quem)
  - eventos de recompensa
  - cálculo e consulta de saldo

## Como se conecta ao resto do sistema

- Endpoints HTTP expõem criação/consulta de recompensas.
- `AuditLog` registra concessões e tentativas suspeitas.

