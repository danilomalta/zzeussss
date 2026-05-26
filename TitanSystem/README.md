# TitanSystem (Monorepo)

Este diretório agrupa todos os pilares do **TitanSystem**, um ecossistema de PDV (POS) multiplataforma.

## Estrutura

- **`backend/`**: API em Go (Fiber) + persistência (GORM + SQLite). Fornece autenticação, catálogo, pedidos e integrações.
- **`frontend/`**: Aplicação Web (React/TypeScript/Vite) que consome o backend e serve de base visual/funcional.
- **`desktop/`**: Aplicação Desktop (Electron) que empacota o frontend web para Windows/Linux/macOS.
- **`mobile/`**: Aplicação Mobile (React Native/Expo) para terminais/coletores e operação em campo.

## Como as partes se conectam

- O **frontend web**, o **desktop** e o **mobile** consomem a API do **backend** via HTTP.
- O **desktop** reutiliza a UI/fluxos do **frontend web** dentro do Electron.
- O **backend** é a fonte de verdade para dados de PDV (produtos, pedidos, itens, etc.).

