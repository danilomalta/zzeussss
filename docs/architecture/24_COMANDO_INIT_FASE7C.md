# Fase 7C — Comando de instalação local

`go run ./cmd/titan-local init` exige caminhos novos para o banco SQLite e o arquivo privado do aparelho. Recebe a senha por stdin, sem argumento de linha de comando, e nunca a imprime. Guarda chave privada Ed25519 em arquivo exclusivo 0600, fora do repositório. Imprime apenas IDs para o primeiro login. Nenhum PostgreSQL é acessado.

O comando não reutiliza arquivos existentes. Uma falha entre a criação da chave e o fim da instalação pode deixar arquivos incompletos; inspecione antes de qualquer nova tentativa. O executável ainda não inicia servidor nem tela.
