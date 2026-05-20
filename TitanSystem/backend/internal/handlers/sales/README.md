# `internal/handlers/sales/` (Vendas)

Esta pasta contém os handlers HTTP relacionados ao fluxo de **vendas** do PDV.

## Responsabilidades

- Receber requisições de criação/consulta/ajuste de vendas.
- Validar entradas (itens, preços, quantidades, totais).
- Gravar dados de venda no banco (SQLite via GORM).
- Registrar auditoria (`AuditLog`) para ações críticas.
- Integrar com o Modo Apagão:
  - aceitar lotes de operações pendentes
  - garantir idempotência (não duplicar venda se o cliente reenviar)

## O que deve ser programado aqui

- Endpoints para:
  - criar venda
  - listar vendas
  - detalhar venda
  - cancelar/estornar (com política de negócio)
- Endpoints de sincronização (quando definidos) podem ficar aqui ou em `internal/sync`, mas o ponto é existir um único lugar claro para o “fluxo de venda”.

