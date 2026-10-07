# Entrega 10 — Validação das sessões em PostgreSQL real

Base de aplicação: `120efb9`, entrega 09. Esta entrega adiciona uma ferramenta
de teste; não altera APIs, migrações, frontend ou o módulo de produção.

## Por que esta entrega existe

`go test ./...` permite que o teste PostgreSQL seja ignorado quando não existe
`TITAN_SESSION_TEST_DATABASE_URL`. Testes unitários, race e compilação não
demonstram execução das transações no PostgreSQL real.

`tools/test_online_postgres.py` prepara um banco novo para executar exatamente
`TestPostgresDurableRotationConcurrencyAndRollback`. Exige eventos JSON de
execução e aprovação do teste e do pacote. Saída vazia, teste ignorado,
nome diferente, erro e timeout não contam como sucesso.

## Execução no PC Linux

Requisitos: Go compatível com o backend, `psql`, `sudo`, servidor PostgreSQL
local em execução, usuário de sistema `postgres` com acesso administrativo
por autenticação peer no socket `/var/run/postgresql`.

Execute como seu usuário habitual, sem `sudo` antes do Python:

```bash
cd "$HOME/Downloads/zzeus-frontend"
GOTOOLCHAIN=go1.25.0 python3 -B tools/test_online_postgres.py --pg-port 5432
```

O `sudo -v` pode solicitar sua senha do Linux para criar **somente** o usuário
e o banco novos de teste. A ferramenta não solicita a senha do Titan. Se seu
PostgreSQL usa outro socket, autenticação ou administração remota, este fluxo
não é compatível; não altere credenciais reais para contornar a recusa.

## Isolamento e limites

- Antes de criar recursos, verifica ferramentas, Go e conexão administrativa.
- Gera nomes aleatórios: usuário `titan_online_<hex>` e banco correspondente
  com sufixo `_test`. Não reutiliza nomes nem bancos existentes.
- Cria usuário sem superusuário, criação de banco/usuário, replicação ou bypass
  de RLS. Ele é dono somente do banco novo e tem limite de 12 conexões.
- Gera senha temporária em memória, transmitida por stdin na criação do usuário
  e pelo ambiente do subprocesso Go. Não aparece em argumentos ou no relatório.
  A validade da senha termina em uma hora; isso não remove o banco nem encerra
  conexões já abertas. Não há promessa de proteção contra administrador do SO.
- Remove variáveis de conexão PostgreSQL/Titan herdadas e não lê `.env`.
- A conexão do teste é exclusivamente TCP `127.0.0.1`, na porta escolhida.
  `sslmode=disable` vale somente para esta conexão local de teste.
- A conexão administrativa usa o banco de manutenção `postgres` exclusivamente
  para `SELECT 1`, `CREATE ROLE` e `CREATE DATABASE` de recursos novos.
- O teste existente cria esquema próprio e tabelas de fixture. Exercita as
  migrações incrementais de segurança 5 e 6, sem executar `000001_init.sql`.
- Nenhum processo comercial é encerrado; portas de API/Vite não são utilizadas.
- Banco, usuário e esquema de teste ficam preservados. Não há DROP ou limpeza
  automática. Cada execução cria recursos adicionais para inspeção posterior.
- Falha ou interrupção pode deixar recursos parciais. A etapa e confirmações
  recebidas constam do relatório; timeout não prova que o comando não executou.
- Relatório JSON em diretório temporário privado (0700), arquivo 0600, sem logs
  brutos, DSN ou senha. Preserve o relatório se precisar de evidência durável.

## O que o teste real cobre

| Operação | Evidência exigida |
| --- | --- |
| Migrações 5/6 | Aplicação, repetição e verificação do esquema |
| Renovação concorrente | Uma renovação aceita; reutilização revoga a família |
| Auditoria de renovação | Falha injetada desfaz a transação |
| Troca de senha | Hash novo, auditoria e revogação dos acessos anteriores |
| Auditoria de senha | Falha injetada preserva hash anterior e sessão |
| Senha, login e renovação concorrentes | Sem sessão antiga ativa após troca |
| Login posterior | Hash antigo recusado; hash novo aceito |

Não comprova todos os serviços online, envio de e-mail, recuperação online,
produção comercial, nuvem/híbrido, carga, failover ou funcionamento em aparelho.

## Verificação da ferramenta

```bash
python3 -B -m unittest discover -s tools/tests -p test_online_postgres.py -v
git diff --check
```

Os dez testes Python usam processos simulados para verificar preflight, isolamento,
restrições do usuário, erros, timeout, ausência de segredos e recusa de SKIP.
Eles **não** substituem a execução PostgreSQL acima. O PostgreSQL real não foi
executado no ambiente de preparação desta entrega. Seu resultado deve ser
registrado separadamente após execução no PC.

## Coordenação

Chat A: autenticação, infraestrutura e esta validação. Chat B: produção e
demais módulos autorizados em `feat/producao-receitas`, worktree próprio.
Nenhum número de migração SQLite é consumido por esta entrega. Aplicar esta
ferramenta na `main` não exige integrar o trabalho do Chat B.
