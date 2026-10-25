# TitanSystem (Monorepo)

O **TitanSystem** é um software ERP/PDV para computador Windows/Linux, organizado como monorepo. O cliente desktop usa as telas React existentes e se conecta ao serviço autorizado da empresa, localmente ou por HTTPS conforme a configuração. Não é o site institucional.

## Pilares

- **Backend (Go)**: API Fiber com PostgreSQL via pgx e GORM.
- **Interface do aplicativo (React/TypeScript, pasta `frontend-web/`)**: telas internas do ERP compiladas para o desktop. O nome histórico da pasta não a transforma em site institucional.
- **Desktop (Electron, pasta `desktop/`)**: janela nativa, arquivos compilados sem Vite em produção e configuração explícita de conexão. Integração comercial e validação Windows têm aceites próprios.
- **Mobile (React Native + Expo)**: estrutura existente; vendas offline ainda não implementadas.

## Onde está o código

O código ativo fica na raiz do repositório:

- `backend/`
- `frontend-web/`
- `desktop/`
- `mobile/`

## Documentação por pasta

Cada pasta do monorepo possui um `README.md` explicando:

- o propósito da pasta
- como ela se conecta ao restante do sistema
- o que deve ser implementado ali


A árvore `TitanSystem/` está preservada para inventário e comparação; Compose e CI usam as pastas da raiz. O modo offline e a venda pelo smartphone ainda são planejados.

## Desktop, serviços e site

- `frontend-web/`: fonte única das telas internas do software. Vite é ferramenta de desenvolvimento/compilação.
- `desktop/`: executa e distribui essas telas como aplicativo; não duplica os módulos.
- `backend/`: serviços Go, autorização e persistência. O servidor pode ser separado do PC cliente; não é conteúdo de página web.
- `website/`: área reservada ao site público futuro, sem copiar telas, dados ou permissões do ERP.

Não mover as pastas históricas apenas para mudar nomes: contratos, scripts, CI e alterações locais dependem dos caminhos atuais. Consulte [a execução desktop](desktop/README.md) e o [registro único de continuidade](docs/architecture/CONTINUIDADE_TITANSYSTEM.md).
