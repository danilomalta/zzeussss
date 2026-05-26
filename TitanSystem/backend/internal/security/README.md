# TitanSystem: Security & Envs Layer

**O que você vai codar aqui**: 
Middlewares de JWT, validadores de rotas, política CORS Strict, bloqueio de IP.

**Linguagem**: Go (Golang) + Fiber Middleware.

**Processo e Regras**:
1. Criar middleware limitador de taxa (Rate Limiting) para impedir que robôs derrubem ou "adivinhem" senhas do PDV e Retaguarda via *Brute Force*.

**Barreira de Segurança Contras Vazamentos Nível Militar**:
* **ENV FILES**: NENHUM `.env` sobe para nuvem. Você usará pacotes como `godotenv` em ambiente dev e variáveis de máquina no Docker localmente.
* **INJECTION**: Usar parâmetros `?` no GORM SEMPRE, evitando injeções de SQL cruas.
* As chaves JWT devem ser renovadas a cada 15 minutos (Short-lived) com emissão de um *Refresh Token* gravado apenas num Cookie HTTPOnly Seguro (jamais no localStorage).
* Exigir IP whitelistng nas rotas de admin que acessam relatórios críticos.
