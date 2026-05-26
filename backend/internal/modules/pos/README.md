# Módulo POS (`internal/modules/pos/`)

Representa o **Ponto de Venda** (PDV) e Frente de Caixa, sendo a engrenagem com maior concorrência do sistema. Gerencia vendas, itens vendidos, o motor de sugestão e aprovação de descontos por IA e os kanbans do funil logístico físico.

## Arquitetura DDD

- **`domain/`**: Entidades de `Sale` (vendas), `SaleItem`, `Order`, `OrderItem`, `DiscountSuggestion` e `StatusFunilLogistico`.
- **`usecase/`**: Mecanismo de sugestão automática de descontos e revisão.
- **`repository/`**: Busca e persistência de vendas e cupons.
- **`delivery/`**: Controladores de vendas e endpoints do motor de descontos `/discounts/*`.
