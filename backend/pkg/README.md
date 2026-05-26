# Camada Pública (`backend/pkg/`)

Esta pasta contém infraestrutura transversal genérica do **TitanSystem**. Aqui residem bibliotecas e utilitários exportáveis que **não possuem regras de negócio**.

## Diretrizes de Design

- **Sem Conhecimento de Domínio**: Nenhum arquivo dentro de `pkg/` deve importar pacotes de `internal/`. Eles devem ser agnósticos a entidades e regras específicas do TitanSystem.
- **Altamente Reutilizável**: Código aqui deve ser desenvolvido de forma modular, permitindo que outros serviços do ecossistema possam consumi-los (como microserviços futuros).

## Subpastas e Suas Responsabilidades

1.  **`middleware/`**: Middlewares utilitários para o framework Fiber (Rate limiting, CORS, Security headers, etc.).
2.  **`websocket/`**: Infraestrutura básica para conexões WebSocket bidirecionais de tempo real.
3.  **`monitoring/`**: Configurações de telemetria, trace e métricas expostas para Prometheus/Grafana.
4.  **`logger/`**: Configuração do logger estruturado do sistema (JSON format para centralizadores de log).
