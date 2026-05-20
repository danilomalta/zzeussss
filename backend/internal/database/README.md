# TitanSystem: Database Layer (PostgreSQL & SQLite)

**O que você vai codar aqui**: 
Conexões GORM, Migrations e Controladores dos 3 Servidores principais. Você terá 3 instâncias de banco (Cloud, On-Premise, Embutido).

**Linguagem**: Go (Golang) + SQL.

**Processo e Regras**:
1. O banco deve suportar 3 tipos de infra:
   * **Nuvem do Cliente**: Conecta-se via URL PostgreSQL externa.
   * **Servidor Próprio**: Conecta-se no PostgreSQL rodando na infra da TitanSystem.
   * **Embutido no Sistema (Modo Apagão / Local)**: Conecta-se via SQLite localmente quando cair a internet.
2. Cada transação no banco tem que ser encapsulada com proteção contra Race Conditions (travas de linha).

**Barreira de Segurança Contras Vazamentos**:
* NENHUM arquivo `.sqlite` pode ser adicionado ao GitHub (verifique `.gitignore`).
* As strings de conexão (URI do Postgres) JAMAIS estarão hardcoded. Elas vêm 100% de `os.Getenv()`.
* DADOS SENSÍVEIS (Cartão de crédito, Identidade de pessoas expostas publicamente) DEVEM usar criptografia AES-256 no repouso dentro do PostgreSQL.
