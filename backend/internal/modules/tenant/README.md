# Módulo Tenant (`internal/modules/tenant/`)

Responsável pela gestão de multi-tenancy SaaS no **TitanSystem**, incluindo o ciclo de vida do cliente (tenant), isolamento de dados e o sistema de gamificação e recompensas por indicações.

## Arquitetura DDD

- **`domain/`**: Entidades que representam o Tenant e as Indicações/Recompensas. Regras puras sobre o valor de recompensa (R$ 50 para novas contas, R$ 10 para ações menores).
- **`usecase/`**: Regras de negócio de ativação de novos inquilinos e concessão de indicações de forma idempotente e auditada.
- **`repository/`**: Persistência de dados cadastrais dos tenants e trilhas de indicação.
- **`delivery/`**: Endpoints HTTP para criação e consulta de saldo de indicações.
