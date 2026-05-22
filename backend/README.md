# Backend (Go + Fiber + GORM + PostgreSQL)

Este diretório contém a **API do TitanSystem**.

## Objetivo

- Expor endpoints HTTP (Fiber) para operações de PDV: catálogo, pedidos, status do sistema, etc.
- Persistir dados relacionalmente em **PostgreSQL** via **GORM** (Banco de dados oficial e único homologado).

## Pastas principais

- **`cmd/`**: pontos de entrada (executáveis). Cada subpasta representa um binário.
- **`internal/`**: implementação interna (não deve ser importada por outros módulos).

## O que deve ser programado aqui

- **Modelos** de domínio (ex.: `Product`, `Order`) em `internal/models`.
- **Conexão e migrações** do banco em `internal/database`.
- **Rotas e handlers** HTTP em `internal/routes` e `internal/handlers`.
- **Configuração** (env, portas, caminhos) em `internal/config`.

