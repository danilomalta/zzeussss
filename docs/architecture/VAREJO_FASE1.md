# Varejo fase 1 — registro de implementação

Data: 03/11/2026. Branch: `feat/varejo-fase1`. Base limpa: `5833f2822212a5bf0d53202eb67ce78840d634b6` (`feat/auditoria-lacunas`). `feat/tela-api` não existia em origin na consulta. Não houve instalação, emissão real, consulta de CNPJ, mudança de cofre/rotação, workflow ou ferramentas.

## A — PARCIAL: crédito e compensação financeira

- Política e cadastro de cliente por empresa, todos os recursos desligados por padrão. Limites, parcelas, vencimentos e papéis são explícitos. Nenhum juro, multa, prazo, ponto ou validade comercial foi criado como padrão.
- `/local/retail`: configuração, consentimento com data do servidor, consulta, exportação por pedido confirmado e anonimização dos contatos; mantém identificador opaco e razão financeiro. Dados pessoais ficam no servidor do cliente e na memória da tela; sem localStorage, telemetria ou dados pessoais nos eventos de transporte. Auditoria guarda somente identificadores e hash do comando; dados pessoais não ficam em snapshots de resultados da auditoria de clientes. A revogação de consentimento impede novas concessões de crédito/fidelidade.
- `/local/receivables`: crédito opcional com item/quantidade e parcelas explícitas. `receivables.Create` continua sendo o caminho único para o recebível. `sale.CompleteCreditTx` reaproveita precificação, estoque, impostos e autorização da venda. Venda, baixa de estoque e recebível são atômicos, sem pagamento fictício ou movimento de caixa. Limite de crédito é calculado para o cliente na empresa inteira; inadimplência considera o calendário IANA configurado explicitamente pela empresa; crédito habilitado sem calendário é recusado. Não usa o fuso implícito da máquina. O formulário inicial aceita um item; o contrato aceita a lista existente de itens.
- Baixas parciais/totais preservam o razão existente. Compensação de recebimento gera retirada vinculada e registro imutável, sem apagar o recebimento original. GET/reconciliação tratam o resultado incerto, inclusive cerca durável para estorno não aplicado. O registro durável no aparelho contém só IDs, nunca nome/CPF/telefone/comando.
- Estorno financeiro por item/quantidade de venda em dinheiro usa alocação cumulativa inteira, limita quantidade/valor ao vendido/pago e preserva venda, estoque e nota. Impede reembolso duplicado pelo cancelamento/devolução física preexistentes, em ambos os sentidos. Não confundir estorno financeiro com retorno físico. Tela mantém aviso nos cinco idiomas.
- Padrão das ações sensíveis: proprietário. Concessão/estorno também aceitam os papéis explicitamente configurados, em interseção com as permissões de loja e os contratos Finance/POS existentes; configurar um papel não contorna os controles anteriores. Configuração e dados pessoais são exclusivos do proprietário.
- Toda nova escrita usa `sqlitewrite.Run`: BEGIN IMMEDIATE antes de ler, busy_timeout e até quatro tentativas. Operação idempotente e revisão/valor anterior impedem duplicação. Nenhum esquema corrente foi escrito à mão; a nova migração entra na lista embutida usada por LatestSchemaVersion().

### Limites recusados e retomada A

1. Estorno de principal ainda não recebido e acerto de dívida de venda a prazo por item não foram implementados; cancelamento simples de recebível originado de crédito é recusado para não apagar a dívida deixando a venda válida.
2. Estorno financeiro por item com vínculo tributário é recusado até reutilizar o rateio tributário autoritativo da origem. Nenhum imposto é estimado.
3. Na base desta fase, a emissão fiscal do novo fluxo a prazo não foi integrada. Atualização posterior em `feat/fiscal-prazo-homologacao`: ação dedicada nas contas a receber para NFC-e Focus 65, com escolha explícita 21/91, sem recebimento fictício; detalhes e limites em `FISCAL_PENDENCIAS_20261031.md`. As rotas de finalização/reprocesso recusam a origem de crédito com `credit_fiscal_payment_not_mapped`; nenhuma emissão é disparada. A documentação oficial distingue `indicador_pagamento=1` e códigos distintos de forma de pagamento, como Crédito em Loja e Pagamento Posterior. Não converter automaticamente parcelas em uma dessas modalidades. Falta interface/contrato explícito de modalidade e sua integração ao emissor existente. Não é ausência desses campos na documentação.
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

## C — multiloja em uma instalação compartilhada

Commit B: `a094935`.

- Cadastro de loja e transferências entre lojas da mesma empresa, desligados por padrão. Cada transferência registra origem, operador, aparelho de origem, custódio aprovado no destino e dois movimentos atômicos; o aparelho destino não é apresentado como autor da solicitação. Exige autorização nas duas lojas, saldos anteriores explícitos, unidade compatível e estoque livre suficiente, preservando reservas. Sem criar aparelho, chave ou protocolo de sincronização automaticamente.
- Relatório por loja/consolidado filtra lojas autorizadas; saldo físico, reservado e livre separados por produto/local/unidade. Vendas brutas por período UTC explícito incluem crédito e não representam recebimento nem caixa. Estoque é posição atual com horário, não saldo histórico do período. Recusa acima de 1.000 lojas/5.000 posições, sem apresentar recorte como total. Conta de pontos, direitos e razão não expõem vendas de outra loja.
- Operações novas usam o mesmo escritor imediato, revisão, idempotência e auditoria imutável. Recuperação explícita no servidor verifica o resultado original ou cria cerca durável para comando ausente; nunca repete a operação. A cerca é isolada por empresa/loja/aparelho/operador. Outro usuário não pode bloquear previamente o ID do proprietário.
- A interface guarda somente ID de operação e contexto no diário IndexedDB. Após falha/reinício, a resolução é explícita antes de nova escrita; troca de sessão/contexto invalida consultas anteriores. Cadastro, CPF, telefone e comando completo não são persistidos no cliente. Download de perfil é ação explícita, local. A exportação histórica completa permanece pendente em A.
- `/retail/access` retorna apenas contexto e capacidades do próprio operador; são dicas de interface, não autorização. Configuração exige proprietário e Core; transferência exige Inventory e permissões de estoque. Instalação sem Finance pode configurar/transferir estoque; relatório financeiro exige também autorização contábil nas lojas consultadas. Demais comandos mantêm contratos/permissões anteriores.
- Não há evento remoto inventado para venda a prazo nem transferência: esta entrega é restrita ao banco compartilhado da instalação. A venda em dinheiro conserva seu evento preexistente. Sale rejeita um movimento de caixa órfão com a mesma origem, em vez de incorporá-lo à nova venda.
- Testes: `retail_stores_test.go` cobre concorrência, rollback, isolamento, reservas, backup/restauração, sessões revogadas e contrato HTTP. `retail_bridge_test.go` executa o cliente Node contra API real/SQLite, perde a resposta após commit, recria o cliente e resolve pelo ID sem segundo POST. Não comprova navegador físico ou sincronização entre servidores. `calendar_test.go` prova limite de dia no fuso declarado. Nenhum teste existente perdeu asserções; o teste de renderização de recebíveis recebeu o formatador real no carregador de módulos, preservando os mesmos valores esperados.

### BLOQUEIO — depende de decisão de topologia do dono

1. Banco único local: lojas usam uma instalação; depende de rede local e disponibilidade do servidor.
2. Banco único em servidor próprio: filiais acessam serviço autorizado por TLS; depende da conexão.
3. Banco único na nuvem do cliente: mesmas transações; cliente controla contas, TLS, chaves e backup.
4. Servidor separado por loja: requer protocolo durável; não basta copiar SQLite.
5. Estrela com central do cliente: exige filas idempotentes e definição de autoridade de cada registro.
6. Vendas offline entre instalações: exigem direitos exclusivos de estoque/limite antes da desconexão.
7. Transferência entre bancos: origem deve bloquear duravelmente antes de creditar o destino.
8. Conflitos cadastrais: dono deve definir autoridade e revisão; timestamps não resolvem dinheiro.
9. Restauração/revogação: protocolo deve distinguir backup antigo de participante autorizado atual.
10. Escolher disponibilidade, autonomia e responsabilidade operacional antes de implementar sincronização.

### Validação final

- Nove testes novos de concorrência, cada um `-count=50 -timeout=120s`, sem falha: Configuration, CustomerConsent, CreditSale, ReceiptReversal, SaleFinancialRefund, LoyaltyRedemption, StoreTransfer, StoreCreation, RetirementAndCommand (`TestRetailConcurrent*`). Logs `/tmp/retail-final50-*.log`; exatamente um vencedor ou resultado consistente entre comando e aposentadoria.
- Cofre/rotação, incluindo testes de órfãos, isolamento e rotação original: `-count=20 -timeout=120s`, passou, 61,356 s. Sem alterações nesses componentes.
- API varejo (`^TestRetail|^TestHTTPRetail`), SQLite e ponte cliente/API reais: passou, 30,124 s. Domínios identity, stock, sale, retail e receivables passaram; receivables é coberto pela integração, sem arquivo de teste próprio.
- Frontend completo: 2.595 casos, todos passaram, nenhum pulado. TypeScript passou. Build Vite passou (25,21 s), mantendo aviso de chunks acima de 500 kB. Contratos JSON/inventário: 591 rotas; 10 testes de contratos passaram.
- `-race`: os mesmos 30 testes de varejo passaram em 10 grupos de três, cada comando com `-count=1 -timeout=120s`, sem diagnóstico de corrida. A primeira execução agregada atingiu 120 s de duração acumulada; nenhum teste/asserção foi alterado e o limite não foi aumentado. Logs `/tmp/retail-race-group-*.log`.
- UMA suíte completa `go test ./... -count=1 -timeout=30m -json`: exit 0, 3.843 testes/subtestes aprovados, nenhuma falha. Executáveis auxiliares e `go build ./...` passaram. PostgreSQL 17 foi iniciado exclusivamente em diretório/porta loopback descartáveis; nenhum serviço/banco preexistente foi usado.
- Verificador oficial INALTERADO: exit 1. Todas as 24 integrações obrigatórias presentes; recusou os pulos de `TestReceivablesCustomerOrderFrontendBridge`, `TestReceivablesFrontendRealHTTPBridge` e `TestReceivablesBrowserRestartRecovery`. O workflow da base não define `TITAN_RECEIVABLES_BRIDGE` e `TITAN_RECEIVABLES_BROWSER_BRIDGE`. Comparação exata `git diff --exit-code 5833f28 --` dos dois testes, workflow e verificador retornou zero: omissão já presente na base. Não executei a suíte completa da base, não alterei o relatório bruto e não alego aceite completo do verificador/CI.
- Complemento focado, sem repetir a suíte completa: os três testes acima passaram com as duas variáveis apontando para os scripts existentes, `-count=1 -timeout=120s -json`, pacote 3,241 s. O último reinicia processos Chromium com perfil/IndexedDB descartáveis e API/SQLite reais. Log `/tmp/retail-receivables-bridges.jsonl`.
- Outros pulos: `TestFocusSandboxRealNFCe` (sem token/emitente, nenhuma emissão real), `TestMigrationCrashChild`, `TestPilotCashProcess` e `TestPilotCashScreenFlow`, permitidos pelo verificador. Nenhum teste PostgreSQL foi pulado. Não houve falha nova de teste nesta execução; a recusa de configuração do verificador permanece como pendência local/CI.
- Evidências brutas: `/tmp/titan-retail-ci-jaz90v8h/backend-full.jsonl`, `backend-integrations.json`, `/tmp/retail-c-*`, `/tmp/retail-final50-*`, `/tmp/retail-race-group-*`. Arquivos temporários não são publicados. Workflows e ferramentas não foram alterados.
- Próximo passo de configuração: habilitar explicitamente os dois bridges no ambiente de aceitação/workflow, preservando o verificador e os testes. Sem enfraquecer os pulos permitidos ou substituir teste real por mock.

## Produção

Continua bloqueada. Sem validação Focus/SEFAZ, TEF, hardware, Windows ou dados reais nesta tarefa. Aceitação real Focus/SEFAZ, TEF, hardware, Windows e sincronização entre instalações não estão comprovados. As lacunas de A e B impedem declarar a etapa inteira concluída.
