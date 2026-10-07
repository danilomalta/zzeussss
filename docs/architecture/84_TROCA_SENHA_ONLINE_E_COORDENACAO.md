# Entrega 09 — Troca da própria senha online

Base confirmada no PC: `8fdf5fe`. Somente backend PostgreSQL. Não modifica
frontend, demonstração, contratos locais, banco SQLite nem módulo de produção.
SQLite 0028 foi reservado ao outro trabalho, sujeito à conferência da base.

## Contrato e comportamento

`POST /api/v1/auth/password`: access Bearer e sessão persistida ativa. Objeto
JSON exatamente com `current_password` e `new_password`; máximo 2048 bytes,
Content-Type application/json, valores string. Rejeita duplicatas, campos
extras, null, aninhamento, tipos errados, UTF-8 inválido e conteúdo posterior.
Não aceita usuário, empresa, papel ou ID de sessão enviados pelo navegador.

Nova senha de 12 a 72 bytes UTF-8, sem espaços nas extremidades e diferente
da atual. Limites são em bytes, não caracteres. Bcrypt DefaultCost para o
novo hash. Hash calculado antes dos bloqueios; senha atual comparada com o
valor corrente protegido na transação. Conta, papel, empresa ativa e sessão
do ator revalidados; horário do PostgreSQL consultado depois dos bloqueios e
depois da verificação bcrypt. Limite de cinco solicitações/minuto por IP,
além do limitador geral. Não é recuperação sem senha nem reset de funcionário.

Sucesso 204 confirma juntos hash, revogação de TODAS as sessões ainda não
revogadas dessa conta/empresa (incluindo a atual e sessões expiradas), trilha
de cada revogação e evento de troca com quantidade de sessões. Cookie refresh
expirado somente após confirmação. Sem token novo, senha ou hash na resposta.
Novo login obrigatório. Respostas do handler usam Cache-Control: no-store.

400 parser/política; 401 sessão/senha atual recusada; 413 corpo excessivo;
415 mídia; 429 limitador; 503 falha do serviço/transação. Erros antigos de
middleware mantêm seus contratos (inclusive 500 sem JWT_SECRET). Não imprimir
entrada, SQL com credenciais nem detalhes de banco em resposta.

Se a resposta/commit ficar incerto, não repetir automaticamente a troca.
Tentar novo login manualmente com a nova senha; se recusado, avaliar a senha
anterior sem loops automáticos. Só login confirmado demonstra a credencial
vigente. 503 ou perda de transporte não garantem rollback ocorrido no servidor.

## Coordenação de concorrência

Create, Rotate, Revoke e ChangePassword adquirem advisory lock transacional
por empresa/usuário ANTES de locks de sessão/identidade. A chave usa hash do
contexto com seed fixo. Colisão apenas serializa contas diferentes; autoridade
continua sendo verificada pelos IDs completos. Não é lock global de todos os
clientes. Uma criação de sessão que confirmou antes da troca fica revogada;
uma criação com hash antigo depois dela falha ao revalidar. Refresh/logout
obedecem ao mesmo bloqueio, sem ciclo entre user-row e session-row.

Rotate primeiro consulta o dono da sessão sem bloqueio para obter a chave;
depois do advisory lock, relê a sessão com FOR UPDATE e verifica identidade,
expiração e hash como antes. UUID sozinho não autoriza nenhuma renovação.
Rotação/replay permanecem com as regras da entrega 08.

Nenhuma mudança manual de users ou outro processo legado obedece automaticamente
a esse protocolo. Não rodar versões 08 e 09 simultaneamente para a mesma base.
Escritas comerciais antigas ainda precisam da autorização transacional própria;
trocar senha não cancela retroativamente uma venda já confirmada.

## Migração e ativação

Nova migração PostgreSQL `000006_online_password_changes.sql` acrescenta tabela
de auditoria, com FK composta à sessão do ator, quantidade positiva e índice
por conta. Não contém senha/hash/token. A migração 000005 fica byte a byte
inalterada. Nenhuma migração SQLite foi criada.

`titan-online migrate-sessions` agora valida o histórico COMPLETO 5/6 com
checksums e aplica apenas o sufixo ausente na mesma transação. Nova instalação
de segurança aplica 5+6; atualização de 5 aplica só 6; repetição de 5/6 é aceita.
Lacuna, versão desconhecida/futura ou alteração de qualquer checksum é recusada.
`check-sessions` e início da API exigem 5/6 e as colunas/tabelas esperadas.
Nenhuma migração histórica de inicialização é executada.

Primeiro ensaiar em PostgreSQL descartável, depois planejar backup verificado,
parada da API anterior, migração explícita e ativação. O bloco de aplicação do
patch NÃO migra o banco real. Não voltar a versão anterior como rollback
informal: ela não participa dos bloqueios nem aceita o novo histórico de esquema.

## Testes e limites

SQL mock: senha atual errada, usuário/contexto inexistente, sessão ausente,
revogada, expirada ou de papel divergente; política antes de banco; bcrypt do
novo valor, duas sessões revogadas, zero escrita, falha de revogação/auditoria,
commit incerto e falha de lock. Parser HTTP e cookie: duplicata/null/extra/tipos,
mídia/tamanho, resposta sem credencial, expiração apenas no sucesso. Rota real:
token ausente, sessão revogada e limite de tentativas. Migração: nova, upgrade,
repetida, lacuna, checksum adulterado antigo/novo, DDL/marker/commit falhos.

Teste PostgreSQL opt-in ampliado com trigger de rollback da troca e concorrência
entre troca/login/refresh; nenhuma sessão antiga pode sobreviver e novo login
usa o hash novo. Requer TITAN_SESSION_TEST_DATABASE_URL em loopback, banco
terminando em _test e schema isolado. NÃO executado nesta preparação por
ausência de PostgreSQL descartável configurado. Não inferir locks reais de mock.

Verificação de preparação: suíte Go completa, vet, race nos pacotes afetados,
build sem CGO dos executáveis api/titan-online/titan-local e diff check.
Nenhum teste de frontend necessário: código inalterado. Aceite PostgreSQL real,
cliente de senha, recuperação online, administração de terceiros, MFA,
auditoria consultável, retenção e rotação de chave seguem pendentes.
