# Varejo fase 1 — registro de implementação

Data: 03/11/2026. Branch: `feat/varejo-fase1`. Base limpa: `5833f2822212a5bf0d53202eb67ce78840d634b6` (`feat/auditoria-lacunas`). `feat/tela-api` não existia em origin na consulta. Não houve instalação, emissão real, consulta de CNPJ, mudança de cofre/rotação, workflow ou ferramentas.

## A — PARCIAL: crédito e compensação financeira

- Política e cadastro de cliente por empresa, todos os recursos desligados por padrão. Limites, parcelas, vencimentos e papéis são explícitos. Nenhum juro, multa, prazo, ponto ou validade comercial foi criado como padrão.
- `/local/retail`: configuração, consentimento com data do servidor, consulta, exportação por pedido confirmado e anonimização dos contatos; mantém identificador opaco e razão financeiro. Dados pessoais ficam no servidor do cliente e na memória da tela; sem localStorage, telemetria ou dados pessoais nos eventos de transporte. Auditoria guarda somente identificadores e hash do comando; dados pessoais não ficam em snapshots de resultados da auditoria de clientes. A revogação de consentimento impede novas concessões de crédito/fidelidade.
- `/local/receivables`: crédito opcional com item/quantidade e parcelas explícitas. `receivables.Create` continua sendo o caminho único para o recebível. `sale.CompleteCreditTx` reaproveita precificação, estoque, impostos e autorização da venda. Venda, baixa de estoque e recebível são atômicos, sem pagamento fictício ou movimento de caixa. Limite de crédito é calculado para o cliente na empresa inteira; inadimplência considera a data UTC do servidor. O formulário inicial aceita um item; o contrato aceita a lista existente de itens.
- Baixas parciais/totais preservam o razão existente. Compensação de recebimento gera retirada vinculada e registro imutável, sem apagar o recebimento original. GET/reconciliação tratam o resultado incerto, inclusive cerca durável para estorno não aplicado. O registro durável no aparelho contém só IDs, nunca nome/CPF/telefone/comando.
- Estorno financeiro por item/quantidade de venda em dinheiro usa alocação cumulativa inteira, limita quantidade/valor ao vendido/pago e preserva venda, estoque e nota. Impede reembolso duplicado pelo cancelamento/devolução física preexistentes, em ambos os sentidos. Não confundir estorno financeiro com retorno físico. Tela mantém aviso nos cinco idiomas.
- Padrão das ações sensíveis: proprietário. Concessão/estorno também aceitam os papéis explicitamente configurados, em interseção com as permissões de loja e os contratos Finance/POS existentes; configurar um papel não contorna os controles anteriores. Configuração e dados pessoais são exclusivos do proprietário.
- Toda nova escrita usa `sqlitewrite.Run`: BEGIN IMMEDIATE antes de ler, busy_timeout e até quatro tentativas. Operação idempotente e revisão/valor anterior impedem duplicação. Nenhum esquema corrente foi escrito à mão; a nova migração entra na lista embutida usada por LatestSchemaVersion().

### Limites recusados e retomada A

1. Estorno de principal ainda não recebido e acerto de dívida de venda a prazo por item não foram implementados; cancelamento simples de recebível originado de crédito é recusado para não apagar a dívida deixando a venda válida.
2. Estorno financeiro por item com vínculo tributário é recusado até reutilizar o rateio tributário autoritativo da origem. Nenhum imposto é estimado.
3. A emissão fiscal deste novo fluxo a prazo não foi integrada. As rotas de finalização/reprocesso recusam a origem de crédito com `credit_fiscal_payment_not_mapped`; nenhuma emissão é disparada. A documentação oficial distingue `indicador_pagamento=1` e códigos distintos de forma de pagamento, como Crédito em Loja e Pagamento Posterior. Não converter automaticamente parcelas em uma dessas modalidades. Falta interface/contrato explícito de modalidade e sua integração ao emissor existente. Não é ausência desses campos na documentação.
   Fontes somente de leitura: https://doc.focusnfe.com.br/reference/emitir_nfce e https://campos.focusnfe.com.br/nfe/NotaFiscalXML.html (formas_pagamento / indicador_pagamento / forma_pagamento), consultadas em 03/11/2026. Não houve envio de dados à Focus.
4. A exportação de privacidade desta unidade entrega o cadastro atual; exportação consolidada de todo o histórico pessoal/financeiro ainda precisa integrar as fontes existentes. Anonimização não reescreve o razão financeiro nem remove dados inseridos livremente em documentos antigos. O operador não deve incluir nome/CPF/telefone em referências, motivos e descrições comerciais livres.

### Evidências A

- `TestRetailConcurrentConfiguration`, `TestRetailConcurrentReceiptReversal`, `TestRetailConcurrentSaleFinancialRefund`: cada teste com 50 repetições, zero falhas; exatamente um vencedor em cada disputa. Log conjunto `/tmp/retail-a-all50.log`, 74,679 s.
- `TestRetailCreditSaleAtomicWithoutCash`, `TestRetailPrivacyKeepsFinancialLedger`, `TestRetailReversalRollback`, `TestRetailAbsentReversalFence`, `TestRetailDefaultRoleAndSessionIsolation`: testes novos de integração SQLite. A fixture de manager inicialmente não tinha vínculo de loja; o teste passou após criar o vínculo explícito e obter nova sessão, sem afrouxar autenticação/autorização.
- Regressões de recebíveis e finalização em dinheiro: `/tmp/retail-a-integration-permitted.log`, passou, 31,066 s. Primeira execução bloqueada por sandbox para sockets locais; repetição autorizada usou apenas loopback descartável.
- Cofre/rotação: `TestVaultConcurrent*` e `TestCustomerCredentialsTenantIsolationAndConcurrentRotation`, count=20, passaram, 22,147 s (`/tmp/retail-a-vault-final.log`). Sem edição nesses componentes.
- Pacotes alterados de domínio: passaram (`/tmp/retail-a-packages.log`); recebíveis/retail têm integração em localapi, não testes unitários locais fictícios.
- Frontend completo: `/tmp/retail-a-front-accepted.log`, 2.593 testes, zero falhas/pulados, 2,944 s. TypeScript passou. Os relatórios intermediários que contavam apenas arquivos não são a contagem de casos usada aqui.
- Contratos: verificador e 10 testes de contratos passaram. Novas rotas em `route-inventory.json`, `routes.openapi.json`, `retail.openapi.json`; contrato de recebíveis ampliado. Logs de execução em `/tmp`, sem dados comerciais, não são incluídos na distribuição.

## B — PARCIAL: razão de fidelidade e direito ao benefício

Commit A: `94ffc59`.

- Programa por empresa, desligado por padrão. A configuração exige bases inteiras de acúmulo/resgate e validade explícitas, com consentimento específico. Não há pontos por padrão.
- Acúmulo autorizado após associação explícita de cliente e venda confirmada; venda pendente/cancelada, outra loja/empresa, cliente sem consentimento e repetição com novo ID não criam um segundo crédito. A política usada e a validade ficam imutáveis por venda.
- Saldo deriva do razão imutável. Antes de consultar/resgatar, a transação reconcilia cancelamentos e reembolsos financeiros/físicos com a origem. Recalcula pela regra original, mesmo após alteração do programa. Não inventa proporcionalidade tributária. Reversão é materializada antes do resultado/resgate, não dentro da transação antiga de cancelamento da venda. Se não puder reconciliar, recusa a operação.
- Resgate exige saldo anterior explícito, quantum configurado e confirmação. Retira pontos dos lotes dentro da validade, registra direito ao benefício em centavos e é idempotente. Não pode gerar saldo negativo. Uma reversão posterior a pontos já consumidos pode revelar dívida de pontos; novos resgates são recusados enquanto não houver saldo suficiente. Não há ajuste silencioso ou apagamento do razão.
- A tela mostra conta, razão, direitos ao benefício e aviso nos cinco idiomas: o registro não aplica desconto, não paga a venda e não movimenta dinheiro. Nenhum cadastro pessoal é colocado no armazenamento do navegador ou em eventos de transporte.
- `TestRetailConcurrentLoyaltyRedemption`: 50 repetições sem falhas, vencedor único e replay sem nova retirada. `TestRetailLoyaltyRollbackAndCancellation`, `TestRetailLoyaltyPartialRefundReconcilesOriginalPolicy`, `TestRetailLoyaltyConsentAndUnknownCustomer`, `TestRetailLoyaltyExpiryIsExplicit`, `TestHTTPRetailLoyaltyRealAPI` passaram em SQLite descartável. Logs `/tmp/retail-b-*`.
- Gate: cofre/rotação count=20 passou (21,023 s); frontend completo 2.593 testes, zero falhas/pulados; TypeScript e contratos passaram (586 rotas). Nenhum teste existente foi removido ou afrouxado.

**Retomada B:** aplicação do direito ao benefício no checkout ainda não foi implementada. É necessário definir seu uso (desconto, vale ou produto) e como desfazer benefício já utilizado; não assumir pagamento ou entrega. O acúmulo de venda em dinheiro demanda associação explícita pós-confirmação, não leitura automática de CPF/adesão. Exportação consolidada por titular permanece parcial como registrado em A.

## C — a executar após o gate B

Operação multiloja restrita a uma instalação. Topologia entre instalações permanece fora do escopo: depende de decisão de topologia do dono.

## Produção

Continua bloqueada. Sem validação Focus/SEFAZ, TEF, hardware, Windows ou dados reais nesta tarefa. Validação completa final e PostgreSQL descartável serão registrados ao final das etapas.
