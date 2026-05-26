# Módulo Agenda (`internal/modules/agenda/`)

Gerencia agendamentos de horários de atendimento, reservas de serviços e controle de filas, ideal para comércios locais que operam sob agendamento integrado com o PDV.

## Arquitetura DDD

- **`domain/`**: Entidades de agendamento, clientes agendados e serviços disponíveis.
- **`usecase/`**: Casos de uso de agendamento de horários e alertas automáticos de atrasos.
- **`repository/`**: Persistência de horários e calendários.
- **`delivery/`**: Endpoints HTTP e canais WebSocket para atualizações em tempo real da agenda.
