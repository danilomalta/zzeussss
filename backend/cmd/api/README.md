# TitanSystem API Entrypoint
**Linguagem**: Go
**Papel**: Ponto de entrada (main.go) para inicializar o servidor Fiber e registrar as rotas.
**Segurança Militar**: Não ler `.env` diretamente aqui; instancie o pacote de config que blinda o acesso em memória.
