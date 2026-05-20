# Backend (Go + Fiber + GORM + SQLite)

Este diretório contém a **API do TitanSystem**.

## Objetivo

- Expor endpoints HTTP (Fiber) para operações de PDV: catálogo, pedidos, status do sistema, etc.
- Persistir dados localmente em **SQLite** via **GORM** (e, no futuro, suportar outro banco se necessário).

## Pastas principais

- **`cmd/`**: pontos de entrada (executáveis). Cada subpasta representa um binário.
- **`internal/`**: implementação interna (não deve ser importada por outros módulos).
- **`titan_pos.db`**: banco SQLite local do ambiente de desenvolvimento (pode ser sobrescrito via variável de ambiente).

## O que deve ser programado aqui

- **Modelos** de domínio (ex.: `Product`, `Order`) em `internal/models`.
- **Conexão e migrações** do banco em `internal/database`.
- **Rotas e handlers** HTTP em `internal/routes` e `internal/handlers`.
- **Configuração** (env, portas, caminhos) em `internal/config`.

