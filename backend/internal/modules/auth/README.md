# Módulo Auth (`internal/modules/auth/`)

Responsável pela segurança de autenticação, sessões de usuário, emissão de tokens JWT e prevenção contra ataques brute-force através de limitador de tentativas de login.

## Arquitetura DDD

- **`domain/`**: Entidades de Usuário, Perfil e Credenciais.
- **`usecase/`**: Casos de uso de `Login` e `RefreshToken`.
- **`repository/`**: Busca e persistência de dados de usuários no PostgreSQL.
- **`delivery/`**: Endpoints HTTP (`/auth/login`, `/auth/refresh`) e middlewares de proteção contra força bruta.
