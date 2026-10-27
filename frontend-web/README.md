# Interface do aplicativo TitanSystem

Fonte única das telas internas React/TypeScript do ERP desktop: PDV, estoque, compras, recebimento, produção, financeiro, RH e demais áreas. O nome histórico frontend-web não indica o site institucional.

- Desenvolvimento: Vite serve a interface para testes e faz o proxy local. Abrir no navegador é um instrumento de desenvolvimento.
- Aplicativo: ../desktop/ compila estas mesmas fontes e abre os arquivos em uma janela Electron, sem Vite em produção.
- Serviços: o backend Go executa separadamente, no computador ou servidor autorizado. As APIs operacionais /local/v1 e online /api/v1 não são intercambiáveis.
- Site público futuro: ../website/, sem copiar o ERP.

Não mover ou duplicar as telas para desktop/src. Preserve cinco idiomas, temas, permissões, contratos e tentativas pendentes. Alterar o modo de conexão não autoriza migração de banco nem sincronização automática.

As instruções do aplicativo ficam em ../desktop/README.md. O registro único de continuidade fica em ../docs/architecture/CONTINUIDADE_TITANSYSTEM.md.
