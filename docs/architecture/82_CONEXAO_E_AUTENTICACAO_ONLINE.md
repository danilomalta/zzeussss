# Entrega 07 — Conexão e autenticação online

Base no PC: `ee18b3f`. Backend oficial `backend/`; frontend e banco da demonstração
inalterados. Sem migração nova, sem execução das migrações PostgreSQL antigas.

## O que foi corrigido

- PostgreSQL deixou de usar uma senha padrão. Exige usuário, senha e nome do
  banco explícitos. Credenciais com caracteres especiais são codificadas como URL.
- Conexão remota exige `sslmode=verify-full`: certificado confiável e nome do
  servidor conferido, TLS mínimo 1.2, sem fallback para texto aberto. Vale também
  para endereços da LAN. `sslmode=disable` só é aceito explicitamente para
  `localhost`, IPv4/IPv6 loopback.
- Erros de configuração/conexão são genéricos. A biblioteca devolve erro em vez
  de encerrar o processo. GORM não imprime SQL, parâmetros, hashes ou erros crus
  na inicialização de produção. O executável encerra se o banco não inicializar.
- Validação compartilhada de access/refresh: apenas HS256, expiração obrigatória,
  `iat` futuro recusado quando presente, tipo correto e empresa/operador/papel
  como strings não vazias, sem espaços nas extremidades. Token máximo 8192 bytes.
- Login não devolve erros internos nem senha submetida. Banco indisponível gera
  503; credencial recusada continua com resposta genérica 401. Banco não
  inicializado não causa panic no caso de uso de login.
- Testes das quatro rotas comerciais online implementadas verificam barreira de
  sessão/papel e consultas com empresa autenticada. Cadastro ignora uma empresa
  fornecida no corpo e persiste a empresa da sessão. Geração de descontos consulta
  somente produtos ativos dessa empresa, em transação.

## Configuração PostgreSQL

Escolher `DATABASE_URL` ou as variáveis `DB_HOST`, `DB_PORT`, `DB_USER`,
`DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` e opcionalmente `DB_SSLROOTCERT`.
Credenciais devem permanecer no ambiente privado; não colar valores em logs ou
na conversa. Se URL for informada, ela tem precedência.

- Host padrão: `localhost`; porta padrão: 5432; TLS padrão para variáveis
  separadas: `verify-full`. Usuário, senha e banco não possuem padrão.
- URL deve usar `postgres://` ou `postgresql://`, credenciais explícitas, banco
  e `sslmode` explícito. DSN com pares `host=...` não é aceito nesta configuração.
- Opções URL permitidas: `sslmode`, `sslrootcert`, `sslcert`, `sslkey`,
  `connect_timeout`, `application_name`. Duplicatas, overrides de host, múltiplos
  hosts, `options` e modos `allow/prefer/require/verify-ca` são recusados.
- Timeout de conexão limitado a 10 segundos, pool máximo 50, mínimo zero.
  Não abre dez conexões automaticamente ao iniciar.
- Uma configuração antiga com senha implícita, TLS não verificado ou endpoint
  remoto em texto aberto deixa de iniciar. Corrigir a configuração privada;
  não enfraquecer o código para contornar o certificado.

## Limites do aceite

Testes usam SQL mock para observar consultas, parâmetros, transações e recusas.
Não demonstram execução em PostgreSQL real, migrações, desempenho, backups nem
certificado aceito/recusado em conexão real. Esse ensaio continua pendente em
banco descartável. Nenhum servidor de banco real foi iniciado ou acessado.

Revalidação de vínculo ocorre antes do handler. Ainda falta transação com
revalidação protegida contra revogação concorrente nas escritas online. Não
equivale à autorização local por ação/loja/departamento/aparelho.

Refresh continua sendo JWT com validade de sete dias e consulta de vínculo ativo.
Emitir outro token não invalida o anterior: rotação persistida, replay, logout,
troca/recuperação de senha e administração de sessões online são a próxima
entrega. Não apresentar isso como revogação individual já implementada.

Outras pendências online: segredo forte/rotação e issuer/audience explícitos,
parser estrito de JSON, padronização de erros e limites, paginação de catálogo,
dinheiro exato no domínio legado (usa float64), auditoria/idempotência de criação,
contratos de módulos e isolamento por loja/aparelho. HTTPS externo e operação
comercial em nuvem/híbrida continuam sem aceite. Rotas 501 seguem indisponíveis.

## Verificação

Em `backend/`: `go test -count=1 ./...`, `go vet ./...`, teste de corrida dos
pacotes online alterados e compilação de `cmd/api` e `cmd/titan-local` sem CGO.
Configuração rejeita credenciais ausentes, URL inválida, duplicatas, endpoint
sobrescrito e TLS inseguro; preserva caracteres especiais da senha. Tokens
inválidos são recusados antes de consulta ao banco; renovação revogada não emite
cookie. Testes de isolamento não substituem um piloto de duas empresas reais.

Contrato incremental: `docs/api/online-auth.openapi.json`. Cobre login/refresh
online; não descreve o ERP inteiro nem acrescenta APIs planejadas.
