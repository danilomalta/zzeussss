# Fundação 03 — Permissões por ação, loja e departamento

Base do usuário: `ae98b4d`. Backend local apenas. Não muda telas, PostgreSQL,
instalação, planos ou bancos reais. A Parte 1 ainda não está encerrada.

## Modelo e limites

- Migração incremental `0027_access_policy.sql`: departamentos por empresa/loja,
  associação de cada pessoa a um departamento por loja, regras, delegações e
  eventos administrativos. Nenhum papel existente é convertido ou removido.
- Uma pessoa continua precisando de vínculo ativo e loja autorizada. Aparelho e
  sessão são revalidados dentro da transação da administração. IDs do corpo não
  escolhem empresa, loja ou ator.
- Departamento é um agrupamento de pessoas. Uma autorização de estoque/venda
  permite operar os dados comerciais da loja, não somente dados do departamento.
  Isolamento de prontuários, salários ou documentos departamentais depende dos
  futuros módulos de RH/produção e de seus filtros por registro.
- Sem regra específica, os papéis anteriores continuam funcionando. `allow`
  acrescenta a ação; `deny` vence papel e concessões; `inherit` retira o efeito
  daquela regra e volta a considerar as demais regras e o papel.
- O dono conserva as ações administrativas; esta API não modifica donos nem
  permite bloqueá-los. Recuperação do dono continua no mecanismo da entrega 02.
- As regras são consultadas por `identity.Can` e `CanOperateTx`; o contrato de
  módulos continua sendo uma verificação adicional. Conceder `sell` não instala
  POS nem concede chave verificadora. Capabilities devolve as ações efetivas
  usando os nomes anteriores, compatíveis com o cliente atual.
- Departamento inativo deixa de conceder ações e de autorizar delegados. Suas
  negações continuam valendo para evitar reativar privilégios do papel por
  acidente. O dono pode reativar e trocar uma negação por `inherit` explicitamente.
- Reassociar a pessoa a outro departamento troca as regras do grupo consideradas.
  Regras pessoais pertencem à pessoa/loja e continuam até alteração explícita.
  Remoção de vínculo da loja ou revogação da pessoa continua bloqueando tudo.

## Quem administra

| Ator | Permissão nesta entrega |
| --- | --- |
| Dono | Criar/editar/inativar departamentos; associar pessoas existentes; configurar regras e delegações da loja atual; consultar configuração e auditoria da loja |
| RH ou outra pessoa expressamente delegada | Associar pessoas elegíveis sem outro departamento ao departamento delegado; editar regras pessoais de terceiros desse departamento para ações que o delegado possui; consultar configuração e eventos de política desse departamento |
| Gerente sem delegação | Mantém suas ações comerciais anteriores; seu papel não autoriza administrar estas políticas |
| Demais pessoas | Usar somente suas ações efetivas; não consultar estas configurações/auditoria |

O delegado não pode mudar a si mesmo, criar outro delegado, mudar regras do
grupo, mover pessoas de outro departamento, alterar donos/gerentes/contadores ou
conceder `manage_staff`. Seus alvos são employee/cashier/stock/production.
Nenhuma concessão pode exceder as ações efetivas do delegado no momento da
alteração. Revogar sua delegação impede novas alterações e consultas; não apaga
automaticamente regras previamente aprovadas para terceiros. O dono precisa
revisar/revogar essas regras conforme a política da empresa.

Esta delegação não é uma conta global de RH nem implementa folha/ponto/salários.
Cadastro de novas pessoas e alterações de credenciais continuam nas APIs
anteriores, com seus limites e contrato de Staff. O helper legado de senha foi
restrito a dono/gerente, sem transformar uma concessão de ação em administrador
de credenciais. Aprovação de aparelho e emissão/consumo de convite preservam os
limites de papel e também respeitam bloqueios de `manage_staff`.

## APIs

| Método e rota | Uso |
| --- | --- |
| GET `/local/v1/access/policy?department_id=ID` | Configuração; delegado precisa informar seu departamento. Dono pode omitir e consultar a loja |
| POST `/local/v1/access/policy` | Mutação com senha atual, operation_id, revisão esperada e motivo |
| GET `/local/v1/access/audit?department_id=ID&limit=50&cursor=...` | Eventos, máximo 100 por página, cursor vinculado ao escopo |

Contrato: `docs/api/access-policy.openapi.json`. O corpo exato tem dez campos:
operation_id, current_password, kind, department_id, target_id, permission,
value, name, expected_revision, reason. Todos são obrigatórios; não utilizados
devem ser string vazia. A revisão inicial esperada é zero.

| kind | target_id | permission | value | name |
| --- | --- | --- | --- | --- |
| department | vazio | vazio | active/inactive | nome do departamento |
| member | identidade existente | vazio | assigned | vazio |
| rule | identidade associada ou vazio para grupo | ação conhecida | allow/deny/inherit | vazio |
| delegation | identidade existente da loja | vazio | active/revoked | vazio |

Corpo máximo 4096 bytes, application/json, sem campos extras/duplicados, null,
dados de escopo ou JSON adicional. Mutações limitadas a 20 por minuto/IP.
Senha nunca aparece na resposta nem no fingerprint. Motivo é texto administrativo:
não inserir credenciais ou dados pessoais desnecessários nele.

- Revisão divergente: 409, sem gravar. operation_id com payload diferente: 409.
- Após resposta perdida, repetir explicitamente o MESMO pedido. O servidor
  retorna a confirmação original com repeated=true e sua revisão original;
  não reaplica uma escrita antiga sobre política mais recente. Consultar a
  configuração para saber a revisão atual. Autorização e senha são rechecadas.
- Mudança e evento de auditoria confirmam juntos. Escrita ignorada por trigger,
  falha de auditoria ou falha no commit não gera sucesso. Nenhuma outbox comercial
  é usada: propagação de políticas a outros aparelhos é requisito futuro.
- Snapshot de configuração limitado a 500 registros por coleção. Acima disso,
  recusa com 400; filtrar por departamento. Não há truncamento silencioso.

## Auditoria

Dono, sem filtro, consulta quatro fontes desta loja: políticas de acesso,
administração/recuperação de conta, segurança da própria conta e cadastro de
funcionários. Com filtro de departamento, a consulta retorna somente eventos
de política daquele departamento. Delegado vê somente esta última visão.

Retorna referências, ação, ator, alvo, departamento, permissão, antes/depois,
motivo, revisão e horário. Não consulta passwords, password_hash, token_sha256,
chaves de recuperação ou request_hash. Ordem por horário e referência decrescentes;
cursor por chave evita repetir linhas com o mesmo horário. Cursor não é uma
autorização: cada página revalida sessão, aparelho e delegação atuais.

É uma consulta autorizada dos registros locais, não um log inviolável contra
administrador do sistema operacional. Não cobre ainda todos os eventos de
pareamento, convites, catálogo, caixa, estoque, vendas, fornecedores ou online.
Essas fontes permanecem nos módulos existentes e precisam de contrato de
consulta e cobertura administrativa completa nas próximas entregas.

## Compatibilidade, demonstração e verificação

Backup aceita snapshots 25, 26 e 27; restaura em NOVO arquivo e migra até 27,
revogando sessões/chaves de recuperação da cópia. Não modifica o original.
Versão futura ou checksum divergente continua recusado. Nenhuma migração antiga
foi reescrita. Bancos de teste são descartáveis.

Fluxo demonstrável por teste HTTP: dono cria departamento, associa funcionário,
concede leitura ao grupo, consulta catálogo com sessão do funcionário, aplica
bloqueio pessoal, verifica 403, volta a inherit e verifica 200. Outro fluxo delega
RH, concede teto de leitura, autoriza alteração de terceiro e recusa estoque,
outro departamento, autoregras e nova delegação; revogação bloqueia a sessão já
aberta. Testes incluem rollback, trigger IGNORE, revisão concorrente, resposta
repetida, reabertura do banco, paginação e fontes sanitizadas.

Verificações observadas no ambiente de desenvolvimento:

| Verificação | Resultado |
| --- | --- |
| go test -count=1 ./... | Passou em todos os pacotes |
| go vet ./... | Passou |
| go test -race das novas operações HTTP e concorrência | Passou |
| Backups históricos 25 e 26 restaurados até 27 | Passaram; originais preservados |
| CGO_ENABLED=0 build de titan-local, titan-access e titan-backup | Passou; ambiente usou -buildvcs=false |
| git diff --check e aplicação do patch na base limpa | Passaram |

A verificação de media type, limite de corpo e rate limit também passou em teste HTTP separado.
Interfaces de configuração, sincronização e PostgreSQL não são demonstrados
por estes testes e continuam pendentes.
