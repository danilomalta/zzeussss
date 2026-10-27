# B02B7 — cofres locais e suprimento financiado

## Base e escopo

Base de instalação: B02B6 `8ac7f0c`, branch `feat/backend-b02a-compat`, na
worktree `zzeus-frontend-backend-b02a`. A saída do PC confirma testes, build e
commit da B02B6. Esta entrega acrescenta dois fluxos completos de backend:
cofre com entradas por malotes recebidos e transferências; suprimento do caixa
financiado pelo saldo desse cofre na mesma transação.

São controles de dinheiro físico no aparelho e loja atuais. Não representam
conta bancária, aprovação de adquirente, razão contábil ou prova física de entrega.
Não editam autenticação, sessões, frontend ou protocolo de sincronização legado.

## Valores e origem

Todo cofre começa em zero, sem campo de saldo inicial ou crédito livre. Uma
entrada exige malote `received`, validado com sua sangria, histórico e contagem.
Somente o recebedor registrado pode creditá-lo. Usa o valor contado, não o
declarado: sangria300 e contagem290 geram crédito290 e mantêm diferença−10 no
malote. A divergência não é perdoada nem encerrada pelo crédito.

O vínculo único entre malote e lançamento impede novo crédito em outro cofre,
mesmo com outro identificador de operação. Contagem zero gera lançamento zero
e consome o vínculo, preservando a rastreabilidade. Malotes antigos recebidos
ficam pendentes até ação explícita do recebedor; a migração não credita saldos.

Transferência exige cofres ativos distintos, valor positivo, saldo suficiente e
dois lançamentos opostos. O saldo total dos cofres permanece igual. Arquivamento
exige saldo zero; o cofre continua consultável e não pode ser reativado ou apagado.
Valores são centavos inteiros entre zero e9007199254740991; débitos/transferências
têm valor mínimo1. Overflow, saldo insuficiente e operações incompatíveis são
recusados sem saldo ou histórico parcial. Somas de conferência usam inteiros exatos.

## Suprimento do caixa

O suprimento exige caixa aberto e autorização pelas regras existentes de
`register.MoveTxWithContract`. Debita o cofre e grava o movimento real do caixa,
auditoria, journal e evento legado `cash.movement` dentro de uma única transação.
Não modifica estoque. O movimento permanece disponível nas consultas e fechamento
oficiais existentes. Não pode ser aplicado depois do encerramento do turno.

Uma operação de suprimento legado já existente não pode ser adotada como débito
novo no cofre: o identificador é recusado. Não há conciliação automática de
suprimentos antigos. A API legada de suprimento permanece com seu contrato;
somente a nova rota estabelece financiamento obrigatório pelo cofre.

## APIs e autorização

Prefixo `/local/v1`. Contrato detalhado em
`docs/api/cash-vaults.openapi.json`, inventário e OpenAPI principal atualizados.

| Método | Caminho após o prefixo | Função |
| --- | --- | --- |
| POST | `/cash/vaults` | Criar cofre com saldo zero |
| GET | `/cash/vaults` | Listar cofres locais paginados |
| GET | `/cash/vaults/pending-bags` | Malotes recebidos pelo usuário ainda sem crédito |
| GET | `/cash/vaults/operations/{id}` | Recuperar recibo original do autor |
| GET | `/cash/vaults/{id}` | Saldo e estado atuais conferidos |
| GET | `/cash/vaults/{id}/statement` | Extrato paginado e saldo total conferido |
| POST | `/cash/vaults/{id}/deposit` | Creditar malote recebido |
| POST | `/cash/vaults/{id}/transfer` | Transferir para outro cofre |
| POST | `/cash/vaults/{id}/supply` | Debitar cofre e suprir caixa atomicamente |
| POST | `/cash/vaults/{id}/archive` | Arquivar cofre zerado com motivo |

Todas exigem sessão válida e `ManageCash`, com isolamento de tenant/loja/aparelho.
Escritas exigem contrato POS; leituras históricas não dependem de POS ativo.
Recebimento e crédito não concedem permissões novas. Transferência, suprimento e
arquivamento registram motivo. Criação, entradas e recibos registram autor/data.

Paginação: `limit` padrão50, intervalo1–100; `offset` padrão0, máximo1000000000.
Parâmetros repetidos, desconhecidos ou inválidos são recusados; POST não aceita
query. JSON estrito, corpo até8192bytes, sem campos de saldo ou autor enviados
pelo cliente. IDs novos têm até128 caracteres ASCII seguros para caminhos.
Respostas usam `Cache-Control: no-store`. Erros:400 entrada,401 sessão,403 acesso,
404 recurso invisível/inexistente,409 conflito/inconsistência/saldo,413 corpo,
422 limite monetário,503 contratos indisponíveis; erros internos continuam500.

## Idempotência, histórico e integridade

`operation_id` é estável no tenant/aparelho. Reenvio do mesmo autor e conteúdo
retorna o recibo gravado com `repeated=true`, sem novo efeito; conteúdo diferente
gera409 e outro autor não obtém esse recibo. Reenvio recupera o saldo da operação
original, não o saldo atual. Consultar GET do cofre para o saldo de agora.
Após reinício, restauração ou perda de resposta, o mesmo ID permite recuperação.
Escritas continuam exigindo autorização e contrato vigentes, inclusive reenvios.

Histórico de operações, lançamentos, vínculo de malote e eventos privados são
imutáveis por triggers. O saldo materializado deve conferir com os lançamentos,
o recibo de criação e o último recibo. Depósitos conferem a contagem e o vínculo
do malote; suprimentos conferem movimento, auditoria e journal. Transferências
conferem os dois lançamentos do recibo. Uma leitura de extrato valida o cofre
inteiro antes de paginar, mesmo quando a página solicitada está vazia. Não há
reparo silencioso, remoção do passado ou resposta parcial com saldo utilizável
quando uma dessas conferências falha.

Essas verificações detectam inconsistências cobertas; não são proteção contra
administrador que reescreva deliberadamente banco, triggers e todas as evidências.
A conferência completa cresce com o histórico do cofre. Não houve benchmark de
carga comercial; o pacote não anuncia capacidade irrestrita de volume.

## Dependências e coordenação

Nova migração `0049_cash_vaults.sql`, reservada após a0048 da B02B6. Nenhum script
0001–0048 foi alterado. Backup aceita25–49 e restaura bases antigas migrando até49.
Fixtures de esquema futuro passaram para50, sem mudar o executor de migrações.

Arquivos compartilhados modificados: `register/movement.go`,
`cashcustody/read.go`, `localapi/server.go`, `backup/backup.go`, testes de backup
e de migração, inventário/contratos HTTP. Os helpers `MoveTxWithContract` e
`cashcustody.ReadTx` recebem transação do chamador e não fazem commit; o chamador
deve fazer rollback em qualquer erro. Os wrappers legados mantêm sua transação.
Isso evita transações aninhadas e permite rollback conjunto do cofre e caixa.

Eventos `cash.vault.*` ficam em `cash_vault_events`, fila privada imutável local.
O protocolo antigo não os recebe e eles não entram na outbox compartilhada.
Não há transporte entre aparelhos nem confirmação de entrega remota. O evento
legado do movimento real de caixa mantém seu formato e transporte existentes.

O aplicador exige a base/branch exatas e hashes dos arquivos, verifica aplicação
e retomada com `git apply --check`, preserva alterações externas e recusa avanço
da produção nas migrações acima44 ou scripts pendentes naquela worktree.
Reconfere essa coordenação e os hashes antes do commit. Não faz merge ou push.

## Validação e pendências

Evidências executadas e comandos ficam em `EVIDENCIAS.txt` no pacote. Testes usam
SQLite temporário, incluindo rollback com triggers de falha/IGNORE, duplicidade,
concorrência, saldo insuficiente, overflow, fonte alterada, reinício, restauração,
isolamento e contratos HTTP. Não acessam banco da demonstração.

O frontend completo desta base não está disponível no ambiente de implementação.
O aplicador executa `npm ci`, `test:local` e `build` no PC antes de registrar commit;
a integração visual com esses endpoints continua pendente. Também permanecem
pendentes banco/adquirente, aprovação de cartão/Pix, venda mista dosR$315,
financeiro completo, despesas/pagamentos externos, remessas remotas e aceite de
volume real. As etapas de produção P01–P12 não são marcadas concluídas por este pacote.
