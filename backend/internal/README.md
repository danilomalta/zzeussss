# `internal/` (Implementação interna)

Esta pasta contém toda a implementação interna do backend.

## Por que existe

Em Go, o diretório `internal/` impede importações externas acidentais, forçando um limite claro: **o que está aqui é privado do módulo**.

## O que deve ser programado aqui

- **Configuração** (`config/`): leitura de variáveis de ambiente, defaults e validações.
- **Banco de dados** (`database/`): inicialização do GORM, migrações e helpers.
- **Modelos** (`models/`): estruturas GORM/JSON do domínio do PDV.
- **Handlers** (`handlers/`): lógica HTTP (parse/validação, chamadas ao DB, respostas).
- **Rotas** (`routes/`): definição/registro de endpoints e agrupamento por versão.

