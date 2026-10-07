# Entrega 11 — Recuperação online com chave pessoal

Base no PC: `4b7351c`, branch `main`. Apenas autenticação online PostgreSQL.
Nenhuma migração SQLite, interface ou arquivo de produção foi alterado.

## Decisão de identidade

O cadastro online possui e-mail, mas não possui verificação comprovada desse
endereço. Esta entrega não usa o e-mail como prova de identidade e não envia
mensagens. Recuperação por e-mail e verificação do destinatário ficam pendentes.

A alternativa implementada é uma chave pessoal preparada pelo próprio usuário
enquanto ainda consegue entrar, mediante senha atual e sessão ativa. Quem não
preparou a chave, a perdeu ou deixou expirar ainda não tem recuperação automática.
Não existe acesso mestre da Titan, redefinição por gerente ou seleção de outra
conta nestas rotas. A recuperação SQLite do dono é outro serviço.

## Contratos

| Método e rota sob /api/v1/auth | Credenciais | Resultado |
| --- | --- | --- |
| POST /recovery/key | Bearer com sessão ativa + current_password | 201, recovery_key e expires_at |
| POST /recovery/reset | recovery_key + new_password, sem sessão | 204, exige novo login |

Os objetos JSON contêm exatamente os campos indicados. Limite de 2048 bytes,
Content-Type application/json, UTF-8 válido. Duplicatas, campos extras, null,
objetos aninhados, conteúdo posterior e query string são recusados. Respostas
dos handlers usam Cache-Control: no-store. A chave nunca é devolvida pela rota
pública de recuperação nem aparece em erros. Não usar chave em URL ou logs.

A emissão devolve a chave somente à conta autenticada após confirmar a senha
e o commit. Ela precisa ser guardada fora do aparelho, por exemplo em cofre de
senhas; não será consultável novamente. A interface para esse fluxo ainda não
foi implementada. Não colocar Bearer, chave ou senha em comandos de terminal,
histórico, capturas de tela ou arquivos versionados. Utilização comercial exige
HTTPS na implantação online; HTTP de desenvolvimento não protege os segredos.

## Política da chave

- Prefixo próprio rk1, UUID e 32 bytes aleatórios criptográficos. Refresh e JWT
  não são aceitos como chave de recuperação.
- Apenas SHA-256 da chave e do hash atual da credencial ficam no banco.
- Uma linha de chave por conta, vinculada à empresa e ao usuário por FK composta.
- Validade de 30 dias: trata-se de chave pessoal previamente guardada, não token
  de e-mail. A validade vem do relógio PostgreSQL e aparece na resposta.
- Emissão substitui a chave anterior; nunca coexistem duas chaves ativas.
- Intervalo mínimo persistido de um minuto por conta para emitir outra chave,
  além do limite HTTP de cinco tentativas por minuto por IP em cada rota.
- Uso único confirmado junto da troca de senha. Não cria sessão automaticamente.
- Qualquer alteração do hash da senha invalida a chave anterior. Papel atual
  divergente do papel na emissão também impede seu uso; empresa inativa bloqueia.
- Não invalida uma chave por causa de tentativa inválida. Não há bloqueio de conta
  provocado por solicitação anônima nem endpoint público que enumere e-mails.
- IP limiter é por processo. Limitação distribuída/global e retenção de auditoria
  ainda dependem da infraestrutura de implantação.

## Transações e concorrência

Emissão: bloqueio por conta → usuário → sessão ativa → confirmação da senha →
verificação do relógio/cooldown → substituição da chave → auditoria → commit.
A sessão é novamente exigida como ativa na gravação da chave.

Recuperação: consulta inicial somente por id+digest válidos → bloqueio por
conta → usuário/empresa → nova leitura bloqueada da chave → comparação da
credencial/papel → validação/hash da nova senha → consumo com expiração atual →
gravação da senha → revogação das sessões e auditorias → commit.

O mesmo bloqueio por conta já usado nas sessões coordena login, renovação,
logout, troca de senha, emissão e recuperação. Se login/refresh confirmar antes
da recuperação, suas sessões serão revogadas; depois, a credencial antiga será
recusada. Em duas recuperações concorrentes, somente uma consome a chave.

Falha de consumo, senha, revogação ou auditoria desfaz a transação inteira.
São verificados os números de linhas gravadas. Expiração durante bcrypt impede
o consumo. Resposta/commit incerto não autoriza repetição automática: tente novo
login com a senha escolhida. Na emissão incerta, a chave anterior pode ter sido
substituída; confirme login e gere outra após o cooldown, sem anunciar sucesso.

## Migração e implantação

PostgreSQL `000007_online_recovery_keys.sql` é aditiva. As migrações 5 e 6
permanecem byte a byte intactas. Histórico exige prefixo válido, checksums
corretos e esquema completo 5/6/7 antes de iniciar a API atualizada.

O pacote e seu script não migram o banco comercial. A ferramenta existente
`titan-online migrate-sessions` aplica as migrações somente quando invocada
explicitamente pelo administrador. Planejar parada dos processos antigos e
backup PostgreSQL antes de atualizar a implantação real; não misturar versões.
Backup comercial PostgreSQL continua pendente no roteiro de infraestrutura.

## Verificação

- Testes unitários: chave inválida, política de senha, senha atual incorreta,
  sessão expirada, limite persistido, falhas de emissão/auditoria/commit,
  credencial/papel divergentes, consumo zero, revogação e rollback.
- HTTP: JSON estrito, sem segredos/cookies em erros, autenticação obrigatória
  para emissão, limite da rota pública e recusa de origem não autorizada.
- Migrações: instalação nova, upgrade 5 e 6, histórico completo, lacunas,
  checksums modificados, versão futura e tabelas ausentes.
- O teste PostgreSQL isolado existente foi ampliado: substituição da chave,
  cooldown, rollback por trigger de auditoria, duas recuperações concorrentes
  com login/refresh, expiração, novo login e invalidação após troca de senha.

Testes PostgreSQL reais exigem executar `tools/test_online_postgres.py` no PC.
Esse teste preserva seu banco isolado e não usa dados comerciais. Não considerar
SKIP como aprovação. O teste real da entrega 10 não cobre o código novo até que
esta entrega seja executada novamente no PC.

O frontend, envio/verificação de e-mail, recuperação sem chave previamente
preparada e outros módulos não são anunciados como concluídos.
