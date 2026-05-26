# Módulo Catalog (`internal/modules/catalog/`)

Gerencia o catálogo de produtos do **TitanSystem**, controle inteligente de estoque, previsões de demanda e a análise inteligente de "produtos parados" em estoque via IA.

## Arquitetura DDD

- **`domain/`**: Entidade `Product` contendo lógica de variabilidade de demanda, ponto de reposição e alertas de estoque mínimo.
- **`usecase/`**: Análises inteligentes de insights (ex: produtos sem giro e previsões).
- **`repository/`**: Persistência de produtos e consultas avançadas de estoque.
- **`delivery/`**: Endpoints HTTP de CRUD de produtos e rota de análise `/analises/produtos-parados`.
