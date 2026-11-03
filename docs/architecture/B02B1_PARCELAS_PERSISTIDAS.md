# B02B1 — plano de checkout e parcelas persistidas

## Base e coordenação

Aplicação: B02A `b1e3d80`, na worktree `zzeus-frontend-backend-b02a`, branch
`feat/backend-b02a-compat`. Referência da produção: `ba29cbd`, esquema 44,
árvore limpa segundo a saída fornecida em 17/10/2026.

Esta entrega introduz **0045_checkout_plans.sql** somente nesta linha de backend.
As migrações 0001–0044 não são reescritas. A produção deve considerar este nome
e número na próxima integração; não criar outra 0045 nem aplicar este patch
diretamente na branch de produção. O aplicador interrompe se a referência de
produção mudar ou surgirem alterações nas migrações/backup dessa worktree.

Arquivos compartilhados afetados: montagem de rotas em `localapi/server.go`,
inventário de rotas, limite explícito de versões do backup e fixtures de
migração. Autenticação, sessões e migrações históricas permanecem inalteradas.
O backup passa a aceitar 45, preservando versões 25–44, ValidateSchema,
checksums, integrity_check, foreign_key_check, verificação do aparelho e
revogação de sessões na restauração. Não há downgrade automático de banco 45.

## Comportamento implementado

- POST `/local/v1/checkout/plans`: deriva empresa, loja, aparelho e operador da
  sessão. Exige permissão de venda, contrato POS e caixa aberto desse operador.
- Calcula itens com a mesma regra da cotação B01/B02A: preços do catálogo,
  produto ativo, local de venda, frações exatas, arredondamento e saldo livre
  considerando reservas de produção. A soma cumulativa de linhas é verificada.
- Exige `expected_total_cents` igual ao total atual; a soma das parcelas deve
  ser exatamente esse valor. Não recebe descontos, preços ou aprovações.
- Persiste checkout, operação, IDs das parcelas, ordem, meios e centavos, mais
  o snapshot dos itens cotados. Até 20 parcelas e 500 linhas. A unidade monetária
  é centavo inteiro, limitada ao inteiro seguro do cliente JavaScript.
- Recupera o checkout pelo seu ID, inclusive após fechar/reabrir SQLite e servidor.
  Repetir a criação exata devolve o snapshot histórico, sem recalcular preços ou
  estoque. Outro conteúdo para a mesma operação ou checkout retorna conflito.
- POST `/local/v1/checkout/plans/{id}/cancel`: exige operação e motivo, grava
  cancelamento e histórico sem apagar parcelas. Repetição exata é segura;
  mudar o motivo depois ou tentar reabrir um plano cancelado gera conflito.
- Leitura e desistência de um plano não submetido continuam disponíveis sem
  contrato POS vigente; continuam exigindo identidade/aparelho autorizados e
  o mesmo operador, empresa e loja. Outro caixa não pode ler ou cancelar.
- Plano, parcelas, operação e evento de outbox compartilham uma transação.
  Falha ao inserir evento desfaz toda criação ou cancelamento.

## Estados reais e limites

O plano é `prepared` ou `cancelled`. Todas as parcelas ficam `not_requested` e
`provider_confirmed=false`, inclusive dinheiro. `can_finalize=false` em toda
resposta desta versão. Esses valores expressam a ausência de uma solicitação de
cobrança, e não pagamento pendente no banco ou aprovação de operadora.

`cash`, `debit`, `credit`, `pix` e `creditAccount` são meios da distribuição
planejada. A presença desses nomes não habilita cobrança nem concessão de
crédito. Parcelas aqui são partes de pagamento; não são agenda de vencimentos
ou recebíveis de adquirente. A intenção de um cartão de crédito não cria
parcelamento financeiro nem cronograma de liquidação.

Não há reserva de estoque, preço travado, entrada no caixa, baixa de estoque,
venda, título financeiro ou recibo fiscal. Não vincular a resposta desta API a
`Checkout96.finalize` nem marcar venda concluída. A rota antiga de venda em
dinheiro continua com seu contrato anterior; não está ligada a estes planos.

Uma nova tentativa com outro ID é um novo plano, sem reservar o saldo. Planos
alternativos podem cotar o mesmo saldo; nenhum compromete os ingredientes.
O saldo disponível do snapshot é histórico após a criação. A futura conclusão
deve revalidar preço/estoque, confirmar parcelas e registrar os efeitos comerciais
de forma atômica; não pode tratar este snapshot como autorização de venda.

O cancelamento é apenas desistência de uma alocação não submetida. Não cancela
cobrança externa, não estorna e não exige fotografia ou comprovante local.

Eventos `checkout.plan.created` e `checkout.plan.cancelled` são auditados na
outbox local. O protocolo de transporte não os aplica comercialmente; sua
presença não demonstra sincronização. Nenhum conector é ativado.

Frontend97/98, seus comprovantes, adapter Checkout96, financeiro e fechamento
por meio não são alterados. O cenário R$315 valida somente a persistência de
R$100 dinheiro + R$96 débito + R$119 crédito. Não é aceite da venda mista.

## Verificação

Os testes HTTP exercitam respostas reais do servidor contra SQLite descartável:
criação/recuperação do R$315, reinício, resposta perdida e repetição, IDs
conflitantes, entrada ambígua/aprovação forjada, soma e limites exatos,
arredondamento de peso, saldo cumulativo, produto inativo, cancelamento
preservando histórico, rollback induzido na outbox e concorrência no mesmo
processo. Backup/restauração conferem as parcelas, cancelamento, operações e
eventos. As respostas são confrontadas com o novo OpenAPI estrito.

A matriz geral de isolamento percorre também as três novas rotas com sessões
anônimas, estrangeiras ou revogadas. O teste de concorrência HTTP usa o pool
serializado SQLite do processo; não demonstra múltiplos aparelhos vendendo.

Comandos executados e resultados constam de EVIDENCIAS.txt do pacote.
O aplicador executa novamente Go, vet, race e build antes do commit. Não inicia
servidor, não acessa o banco da demonstração e não faz merge ou push.

## Próximos trabalhos

| Etapa | Estado após B02B1 | Critério pendente |
|---|---|---|
| B02A — produção e saldo livre | Concluída em b1e3d80 | Aceite da interface de produção separado |
| B02B1 — distribuição persistida | Código e testes nesta entrega | Aplicação e testes no PC |
| Confirmação e recuperação de cobranças | Pendente | Fonte autorizada, estados reais, perda de resposta, cancelamento |
| Venda mista e financeiro | Pendente | Parcelas vinculadas, transação, caixa só em espécie, títulos e reversões |
| Fechamento por meio e integração 97/98 | Pendente | Totais oficiais, permissões e conferência sem duplicação |
| Aceite R$315 completo | Pendente | Reinício, concorrência, resposta perdida e fechamento conciliado |
