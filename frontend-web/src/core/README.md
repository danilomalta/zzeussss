# FSD Core Layer

Esta pasta (`src/core/`) contém as configurações globais de infraestrutura da aplicação:
- `api/`: Instância global do cliente de requisição HTTP (Axios) com tratamento de credenciais e interceptores seguros.
- `auth/`: Lógica de gerenciamento global de estado de autenticação (Zustand/RBAC).
- `offline/`: Mecanismos de sincronização e armazenamento local-first.
- `routes/`: Roteador central da aplicação.
