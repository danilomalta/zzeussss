# Motor Compartilhado (`backend/internal/core/`)

Esta pasta contém o motor compartilhado e as engrenagens transversais de baixo nível de apoio ao ecossistema do **TitanSystem**.

## Diretrizes de Design

- **Tecnologias de Suporte**: Esta camada integra e configura bancos de dados, chaves de criptografia e sincronização.
- **Apoio aos Módulos**: Os módulos de negócio em `internal/modules/` dependem de `core/` para acessar infraestrutura básica (ex: inicializar o DB).

## Subpastas e Suas Responsabilidades

1.  **`database/`**: Inicialização de pooling de alta concorrência (`pgxpool`) e interface GORM do PostgreSQL.
2.  **`config/`**: Carregamento e validação de configurações globais e variáveis de ambiente.
3.  **`security/`**: Algoritmos de segurança militar (Argon2id para senhas e criptografia simétrica AES-256-GCM para dados sensíveis).
4.  **`sync/`**: Protocolo e motores de sincronização local-first e offline-first do ERP.
5.  **`routes/`**: Roteamento principal unificado que liga os endpoints de entrega de todos os módulos.
