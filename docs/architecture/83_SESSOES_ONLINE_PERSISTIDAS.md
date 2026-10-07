# Entrega 08 — Sessões online persistidas

Base no PC: `b0bb9c1`. Apenas backend PostgreSQL. Não muda o frontend, a
demonstração, a instalação SQLite nem as migrações locais.

## Operações entregues

| Operação | Comportamento |
| --- | --- |
| POST /api/v1/auth/login | Verifica senha/vínculo; revalida hash sob bloqueio; grava sessão, hash de refresh e auditoria antes de responder |
| POST /api/v1/auth/refresh | Consome segredo opaco uma vez, grava substituto e auditoria na mesma transação |
| POST /api/v1/auth/logout | Revoga a sessão do Bearer e expira o cookie; sem corpo |
| GET /api/v1/auth/sessions | Até 100 sessões do próprio usuário/empresa, mais recentes; identifica a atual; sem tokens/hashes |
| POST /api/v1/auth/sessions/:id/revoke | Revoga somente uma sessão desse usuário/empresa; sem corpo |
| POST /api/v1/auth/sessions/revoke-others | Mantém a sessão atual e revoga as demais desse usuário/empresa; sem corpo |

As quatro operações de consulta/revogação exigem access token e sessão ativa no
banco. Nenhuma permite selecionar outra empresa/usuário no corpo. Administrar
sessões de outras pessoas, trocar senha e recuperar acesso online ficam para a
próxima entrega, com autenticação e permissões próprias.

## Renovação e revogação

- Access JWT HS256 tem `sid` e dura até 15 minutos, limitado pela expiração da
  sessão. As rotas comerciais implementadas revalidam sessão, usuário, papel e
  empresa ativa no PostgreSQL antes do handler. Logout/revogação bloqueiam esse
  acesso sem aguardar a expiração do JWT.
- Refresh NÃO é JWT: versão, UUID da sessão e segredo aleatório de 32 bytes.
  O banco guarda apenas SHA-256 do token completo. UUID público não autoriza
  renovar: segredo desconhecido é recusado e não revoga a sessão.
- Cada família de sessão dura sete dias desde o login. Renovações não prolongam
  indefinidamente esse prazo. Cookie HttpOnly, Secure/Strict por padrão, mantém
  `Path=/api/v1/auth/refresh`. Desenvolvimento HTTP usa configuração explícita
  anterior; a entrega não instala HTTPS externo.
- Bloqueio `FOR UPDATE` da sessão serializa renovações. Consumo, substituição e
  auditoria precisam confirmar juntos. Falha/zero escrita não emite token.
- Reutilização de um hash consumido revoga a família e grava auditoria; só então
  retorna 401. Não revoga outras famílias independentes da conta.
- Se a resposta de renovação se perder, o segredo antigo já pode estar consumido.
  Repeti-lo revoga essa sessão. Fazer novo login; não repetir automaticamente.
  O frontend ainda precisa coordenar refresh entre abas/solicitações.
- Alteração/revogação de papel ou suspensão da empresa bloqueiam o uso. Alterar
  manualmente `users.password_hash` não revoga sessões automaticamente nesta
  entrega; a futura API de senha deverá revogá-las na mesma transação.
- `Origin` não autorizado ou `Sec-Fetch-Site: cross-site` bloqueia refresh antes
  de qualquer operação. Lista segue `ALLOWED_ORIGINS`, padrão localhost:3000.
  Clientes nativos sem Origin ainda precisam possuir o segredo. CORS sozinho
  não é considerado proteção de mutações por cookie.
- Respostas de login/refresh/gestão não devem ser armazenadas em cache.

## Migração e ativação

Arquivo novo: `backend/db/migrations/000005_online_sessions.sql`. Acrescenta
índice composto de usuários, sessões, hashes de refresh e auditoria. Foreign keys
compostas impedem vínculo entre empresas diferentes. O novo registro
`online_security_migrations` guarda versão/checksum; advisory lock serializa
aplicações concorrentes. DDL e registro confirmam juntos.

Executável novo: `cmd/titan-online`.

```text
titan-online migrate-sessions
titan-online check-sessions
```

O comando lê a configuração privada PostgreSQL existente, inclusive `.env`
quando executado do diretório do backend. Não imprime credenciais. Não executa
`000001_init.sql`, nenhuma migração histórica, nem migração SQLite. Repetição com
checksum igual é aceita; versão futura/adulterada é recusada.

Aplicar a migração é manutenção explícita, não ação automática ao iniciar API.
Parar a API online antiga antes de ativar a versão nova. Aplicar com backup
verificado e primeiro ensaiar em banco descartável. Não alterar a base da
demonstração para testar. A API nova recusa iniciar sem esquema/checksum esperado.
Sessões/tokens anteriores a esta entrega exigem novo login, sem fallback que
aceite JWT antigo sem `sid` ou refresh JWT.

Não há migração de rollback destrutiva. Não voltar ao executável online antigo
após ativação: ele não conhece as novas regras de revogação. O instalador Linux
da entrega05 gerencia executáveis locais, não a implantação da API PostgreSQL.

## Testes e limites observados

Testes SQL mock: criação, hash em vez de segredo, falha de sessão/hash/auditoria/
commit, consumo condicional, replay, segredo inválido/desconhecido, recusa de
sessão revogada, revogação de outra empresa, ator expirado, revogar demais,
rollback e migração nova/repetida/futura/adulterada/interrompida. Guardas de
rotas foram adaptadas para exigir sessão persistida; requisições antigas sem
`sid` são recusadas. Testes de origem cobrem CSRF antes da consulta.

O teste opt-in `TestPostgresDurableRotationConcurrencyAndRollback` precisa de
`TITAN_SESSION_TEST_DATABASE_URL`: PostgreSQL em loopback, banco terminando em
`_test`. Não lê `.env`; cria schema isolado e fixtures mínimas, não usa tabelas da
empresa. O schema fica preservado para inspeção. Verifica migração repetida,
duas renovações concorrentes, replay, rollback real com trigger e logout.
Sem variável, é explicitamente pulado. O ambiente desta entrega não permitiu
iniciar PostgreSQL nativo; esse teste NÃO foi executado. Mock não demonstra
comportamento real de locks, commit interrompido, carga ou migração PostgreSQL.

Verificação: suíte Go completa, `go vet`, detector de corrida dos pacotes online,
compilação sem CGO de API, `titan-online` e `titan-local`, e `git diff --check`.
Não há teste novo de frontend: ele permaneceu inalterado.

Pendências: ensaio PostgreSQL real/piloto, autorização protegida durante as
escritas comerciais antigas, departamentos/lojas/aparelhos online, auditoria
consultável, retenção dos hashes consumidos e sessões, recuperação/troca de
senha, segredo forte/rotação/issuer/audience, proteção externa e implantação.
Esta entrega não encerra a matriz F01–F11 ou habilita nuvem/híbrido comercial.
