# `internal/ai_insights/` (Análises automáticas)

Esta pasta contém serviços de **análises automáticas** sobre os dados do PDV.

## Objetivo

- Encontrar padrões operacionais e oportunidades de melhoria (ex.: produtos parados).
- Gerar indicadores para decisões automáticas (ex.: sugestão de desconto, reposição, alertas).

## Princípios

- As análises devem ser **determinísticas e auditáveis**: dado o mesmo banco e parâmetros, o resultado deve ser reproduzível.
- Toda ação automatizada sugerida deve produzir evidências (por que foi sugerida) e ser rastreável.

## O que deve morar aqui

- Serviços que varrem o banco e retornam resultados prontos para uso por:
  - endpoints HTTP
  - rotinas agendadas
  - painéis no frontend

## Como se conecta ao resto do sistema

- Usa `internal/database` para consultas e `internal/models` para estruturar resultados.
- Pode registrar eventos em `AuditLog` quando análises forem executadas em contexto crítico.

