# `cmd/` (Executáveis)

Esta pasta contém os **pontos de entrada** do backend.

## Por que existe

Em projetos Go, é comum separar cada binário em `cmd/<nome>/main.go`. Isso facilita:

- ter múltiplos serviços no futuro (API, workers, jobs, etc.)
- build/deploy separados
- organização clara do que é “executável” vs. “biblioteca interna”

## O que deve ser programado aqui

- Apenas `main.go` e código mínimo de bootstrap (carregar config, iniciar DB, iniciar servidor).
- Regras de negócio, rotas e acesso a dados devem ficar em `internal/`.

