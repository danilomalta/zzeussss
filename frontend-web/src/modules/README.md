# FSD Modules Layer

Esta pasta (`src/modules/`) contém os módulos de negócio isolados do TitanSystem, seguindo o padrão de arquitetura modular:
- `auth/`: Processo de login e autenticação multifator.
- `catalog/`: Catálogo de produtos, supply chain e logística B2B.
- `pos/`: Frente de caixa rápido (PDV).
- `tenant/`: Administração de empresas (multi-tenant) e monitoramento do sistema ("Olho de Deus").
- `financial/`: Fechamento de caixa, pagamentos e faturamento fiscal (Checkout/NFe).

Cada módulo contém suas próprias pastas internas para manter a autonomia:
- `components/`: Componentes específicos do módulo.
- `pages/`: Telas e visões completas expostas pelo módulo.
- `hooks/`: React hooks contendo lógica de estado local ao módulo.
- `services/`: Chamadas à API específicas do módulo.
- `store/`: Estado específico do módulo.
- `__tests__/`: Cobertura de testes unitários e de integração locais.
