# Fase 7E — Início da API local

`titan-local serve` recusa caminhos não existentes e arquivo de chave que não seja regular com modo 0600. Antes de abrir a porta, prova a posse da chave privada num desafio de uso único do SQLite. A API escuta somente `127.0.0.1:8181`; nenhuma porta LAN é aberta. O processo não inicializa PostgreSQL.

A base SQLite é particular do aparelho. O processo precisa ser encerrado normalmente antes de backup e não deve ser compartilhado em diretório de rede. Rotas de catálogo, caixa e venda serão adicionadas em seguida.
