# `internal/routes/` (Registro de rotas)

Esta pasta concentra a **definição e o registro** das rotas HTTP do backend.

## Por que existe

Separar rotas de handlers melhora a legibilidade:

- `routes` define **quais endpoints existem** e como são agrupados (ex.: `/api/v1`)
- `handlers` implementa **o que acontece** em cada endpoint

## O que deve ser programado aqui

- Funções de registro como `Register(app *fiber.App)` e agrupamentos por versão.
- Middlewares globais (CORS, logger, recover) podem ser aplicados no `main.go` ou aqui, desde que exista um lugar único e claro.

