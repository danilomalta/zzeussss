# Configurações Globais Blindadas
**Linguagem**: Go
**Papel**: Carrega strings sensíveis do `.env` ou do ambiente da Nuvem.
**Segurança Militar**: Pattern Singleton para encapsular os secrets. Impede que a chave vazada da API seja printada no terminal por acidente.
