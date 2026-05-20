# `internal/handlers/ai_insights/` (Endpoints de análises)

Esta pasta contém handlers HTTP para expor análises automáticas do sistema.

## Objetivo

- Transformar análises internas (serviços) em respostas HTTP consumíveis pelo frontend.
- Padronizar entradas (parâmetros) e saídas (JSON) com mensagens seguras.

## Regras

- Não expor dados sensíveis desnecessários.
- Definir limites de paginação/tamanho de resposta para evitar abuso.
- Registrar auditoria quando uma análise gerar ações automáticas (futuro).

