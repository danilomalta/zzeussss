# Módulo HR (`internal/modules/hr/`)

Responsável pela gestão de recursos humanos, controle de ponto de funcionários, folhas de pagamento e escala de turnos dentro de cada empresa do ecossistema TitanSystem.

## Arquitetura DDD

- **`domain/`**: Entidades de colaboradores, registro de ponto e contratos de trabalho.
- **`usecase/`**: Casos de uso de fechamento de folha de ponto e escalas automáticas.
- **`repository/`**: Persistência de registros de ponto e contratos.
- **`delivery/`**: Endpoints HTTP e integração de relatórios.
