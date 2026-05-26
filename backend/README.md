# `backend/` (Raiz do Backend)

Esta pasta representa a **raiz do backend** no monorepo.

## Atenção (estado atual do repositório)

No estado atual, o código do backend está em `TitanSystem/backend/`.

Esta pasta existe para cumprir a separação **na raiz** (`backend/`, `frontend/`, `desktop/`, `mobile/`) e deixar a arquitetura óbvia para qualquer pessoa que abrir o monorepo.

## Como isso se conecta ao sistema

- O backend expõe a API HTTP e é consumido pelo Web, Computador e Mobile.
- A implementação atual (Go + Fiber + GORM + SQLite) está dentro de `TitanSystem/backend/`.

## O que deve morar aqui (próximo passo de organização)

- Em uma limpeza de repositório, o conteúdo de `TitanSystem/backend/` deve ser movido para `backend/` preservando histórico de versão.

