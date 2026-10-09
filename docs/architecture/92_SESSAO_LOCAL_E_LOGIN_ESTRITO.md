# Sessão local e login estrito — entrega 17

Base de aplicação: `911b997`. Altera apenas o parser do login local e os headers de cache de sucesso de login/me, acrescentando documentação e testes. Não modifica banco, migrações, senhas armazenadas, TTL, frontend ou produção.

## Problema e comportamento

O login usava BodyParser genérico, enquanto as escritas críticas usam JSON exato. Agora aceita somente objeto com identity_id e password, ambos strings não vazias, até 4096 bytes. Campos extras, duplicados, null, ausentes, tipos incompatíveis e segundo JSON são recusados antes da autenticação. Corpo maior retorna 413; JSON inválido retorna 400; credencial inválida continua 401.

Clientes existentes que enviam os dois campos JSON preservam o fluxo. Clientes que enviavam formulário ou campos adicionais precisam ajustar suas requisições. Não há alteração silenciosa de senha ou identidade com trim. Login e consulta de contexto retornam Cache-Control: no-store; tokens e respostas não devem ser registrados.

## Contrato de sessão

- GET health é sinal de processo, não prova de prontidão de banco, licença ou integrações.
- POST login emite token opaco com validade de oito horas; não é JWT e não tem refresh local.
- GET me retorna empresa, loja, identidade, aparelho e expiração derivados da sessão/estação. Resolve revalida revogação e escopo.
- POST logout revoga a sessão atual e retorna 204 vazio. Nova tentativa com token revogado retorna 401.

Limitação de cinco tentativas por minuto é a do middleware atual por IP; não constitui proteção DDoS distribuída. O perfil local usa loopback; acesso de rede depende da configuração HTTPS do servidor de loja. Não se deve expor login HTTP publicamente.

## Aceite e limites

Teste HTTP cobre login, headers no-store, contexto exato, validade, logout vazio e recusa do token após revogação. Casos negativos verificam parser, limite e senha incorreta sem criação de sessão ou vazamento dos dados enviados.

Contrato em docs/api/local-session.openapi.json. O mapa mantém as 69 rotas existentes; encerra as marcações pending das quatro rotas locais restantes. Cobertura documental não significa conclusão de todos os recursos do produto. Rotas online pendentes ou bloqueadas permanecem assim identificadas.

Verificações: JSON/referências, suíte da API local e localauth, vet, race dos testes novos e testes do cliente frontend. Os testes usam bancos temporários. Nenhum push, servidor, migração real ou alteração na branch de produção faz parte da aplicação.
