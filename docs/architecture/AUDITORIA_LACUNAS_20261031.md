# Auditoria de lacunas do TitanSystem

Arquivo solicitado: 20261031; inspeção e testes realizados em **03/11/2026**.
Base: `feat/fase8-fechamento`, `fbbf2bc74006b33053e0e05d73cfdc9bdf8cb73a`.
Branch: `feat/auditoria-lacunas`. Única alteração: este documento.

## Critério e limites

REAL = implementação e teste encontrados, somente no escopo descrito. ESQUELETO = interface/adaptador parcial, sem a operação completa. AUSENTE = nenhuma evidência de implementação e teste para a capacidade especificada. Um módulo pode conter capacidades dos três estados. Teste encontrado e teste executado são discriminados abaixo. Não houve emissão fiscal, acesso a banco instalado, leitura de .env, teste externo, instalação ou suíte completa. Classificação de hardware não certifica equipamento físico. Classificação fiscal não comprova autorização Focus/SEFAZ.

| Item | Estado | Conclusão delimitada |
|---|---|---|
| 1. Cupom NFC-e | ESQUELETO no percurso checkout → impressão fiscal; REAL no gerador nativo isolado | Receipt95 imprime comprovante não fiscal; DANFE nativo HTML tem QR/chave, sem ligação de impressão encontrada nessa tela. |
| 2. Hardware | ESQUELETO | Impressão HTML e entrada por teclado existem; drivers de gaveta/balança e ensaio físico não comprovados. |
| 3. TEF/cartão | AUSENTE como integração autorizadora; ESQUELETO manual | Checkout oficial confirma dinheiro; representação visual/comprovante de cartão não liquida pagamento. |
| 4. Mobile | ESQUELETO; vendas offline AUSENTES | App Expo mostra texto de função ainda não implementada. |
| 5. Frontend | REAL em fluxos delimitados; ESQUELETO no conjunto | Seis módulos; interfaces locais reais coexistem com workspaces locais e adaptadores indisponíveis. |
| 6. Rotas públicas | REAL para sessão, recuperação e limite IP; ESQUELETO para cobertura antienumeração | Sete rotas públicas nas APIs principais; receptor de sincronização separado usa prova criptográfica. |
| 7. TitanSystem/ | ESQUELETO histórico | Cópia parcial, não equivalente à raiz; documentação mobile ainda aponta para ela. |
| 8. Varejo | REAL em dinheiro/estoque/compras/devolução; ESQUELETO no conjunto | Fidelidade transacional não comprovada; múltiplas lojas têm isolamento, não prova de operação consolidada completa. |

## 1. Cupom NFC-e ao consumidor

**Receipt95 hoje:** `frontend-web/src/modules/pos/pages/Receipt95.tsx` mostra status da venda; produto (nome conhecido ou ordinal), quantidade, preço unitário e total por item; parcelas com modalidade/valor; total; recebido e troco quando fornecidos e compatíveis com o total. A referência interna da venda é omitida da lista visível, mas incluída nas linhas enviadas à impressão. Não há nesse recibo emitente fiscal, chave de acesso de 44 dígitos, número/série, protocolo ou QR Code.

O aviso vem de `frontend-web/src/i18n/desk95/pt-BR.json`: **“COMPROVANTE NÃO FISCAL. Não substitui documento fiscal.”** `CheckoutFiscalStatus.tsx` exibe o estado fiscal separadamente; esse componente não é acrescentado ao HTML impresso por `print95`. Portanto o status “autorizada” não transforma o comprovante em DANFE NFC-e.

`frontend-web/src/core/local/print95.ts`: HTML local escapado, papel **58 mm, 80 mm ou A4**, prévia em popup e `window.print()`. Não gera PDF diretamente nem comandos ESC/POS; salvar PDF depende do diálogo do sistema. Não encontrei teste que exerça `printDocument96`/`print95` diretamente: AUSENTE a prova automatizada desse layout. `tests/receipt108.test.mjs` cobre livro de comprovantes, não impressão física nem DANFE.

**Gerador fiscal separado REAL:** `backend/internal/fiscalcore/danfe.go::RenderHomologationNFCe` gera HTML local, QR PNG embutido, chave, dados do emitente/consumidor, itens, valores, número/série, protocolo e consulta. Valida vínculo ao XML assinado/protocolo. Aviso explícito **“EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL”**, repetido no documento. NFC-e aceita 58/80 mm; o validador comum também aceita 210 mm para NF-e 55 (não NFC-e); `backend/internal/localapi/fiscal_danfe.go::readFiscalDANFE` entrega HTML protegido em `GET /local/v1/fiscal/intents/:id/danfe`, exigindo largura e fuso. Testes: `fiscalcore/danfe_test.go::TestFiscalDANFEExactSourceLayoutAndEscapedData` e `TestFiscalDANFEQRBindingAndFormatting`; `localapi/fiscal_danfe_test.go::TestHTTPFiscalDANFERealHTTPExactArtifactAndReadonly` (este executado nesta auditoria, passou).

**Focus:** `backend/internal/fiscalproduction/focus_nfe.go::DANFE` é download restrito do PDF de **NF-e 55** retornado na consulta, não gerador de NFC-e. Teste `focus_nfe_test.go::TestFocusNFeDANFEDownloadRestricted`. A tela `tenant/pages/enterprise89/FiscalNFe55.tsx` usa `core/fiscalCustomer/nfe55.mjs::danfe`. Não encontrei consumidor frontend da rota nativa `fiscal/intents/:id/danfe` nem download de DANFE NFC-e Focus ligado ao Receipt95. Falta escolher e ligar a origem correta do documento para a nota de cada provedor, mantendo SEM VALOR FISCAL em homologação; não imprimir comprovante não fiscal como se fosse NFC-e.

## 2. Impressão e hardware

| Recurso | Estado | Evidência e limite |
|---|---|---|
| Impressora térmica pelo SO | ESQUELETO | `print95.ts` entrega HTML 58/80/A4 ao navegador; sem teste de layout direto, spooler ou impressora real. DANFE nativo tem teste de artefato, não de dispositivo. |
| Driver térmico/ESC-POS e seleção persistente | AUSENTE | `desktop/src/pos/README.md` não contém driver; busca por ESC/POS/serial/USB não encontrou implementação e teste do periférico. |
| Gaveta física | AUSENTE | Livro de caixa/gaveta lógica não é pulso elétrico de abertura. `Desk95.tsx` tem UI devices; `core/local/service95.ts::disconnected95` recusa comandos indisponíveis. Sem driver/teste físico. |
| Leitor HID/teclado | ESQUELETO de hardware; REAL busca textual | `pos/pages/LocalPointOfSale.tsx` tem foco, busca por nome/SKU/barcode e Enter. `tests/localPOS.test.mjs` testa regras do PDV, não leitor físico. Leitor configurado como teclado pode fornecer texto; isso não comprova compatibilidade nem captura dedicada. |
| Balança | AUSENTE como integração de equipamento | Quantidade fracionária em milésimos existe, mas nenhum protocolo de leitura de peso/driver/teste foi comprovado. Entrada manual de peso não é leitura de balança. |

Não houve uso de periféricos. `operations84/Devices84.tsx` e Desk95 não provam comunicação com hardware. `desktop/preload.cjs`/`main.cjs` não fornecem um driver fiscal/TEF/balança completo.

## 3. TEF e cartão

**AUSENTE** integração com adquirente/TEF que autorize, consulte, desfaça e reconcilie transação real. `backend/internal/localdb/sale/checkout_cash.go::cashOnly` e `FinalizeCashPlan` recusam alocação não confirmável com `ErrPaymentUnverified`. Teste `backend/internal/localapi/checkout_cash_test.go::TestHTTPCheckoutCashFinalizationDoesNotApproveMixedPayments` (encontrado; não executado nesta auditoria).

**ESQUELETO/registro manual:** `pos/pages/PaymentParts94.tsx`, `OCR96.tsx`, `ReceiptDesk108.tsx`; `core/local/paper106.mjs`, `receipt108.mjs` registram/leem/revisam comprovantes e vínculos. Testes `paper106.test.mjs` (“reads essential fields without claiming bank approval”) e `receipt108.test.mjs` (“does not claim settlement”) executados e aprovados. Não há autorização eletrônica decorrente de OCR, seleção “crédito/débito” ou vínculo de notinha.

## 4. Mobile

`mobile/App.tsx`: somente View/Text **“TitanSystem Mobile: vendas e sincronização offline ainda não implementadas”**. `mobile/package.json`: Expo/React Native e comandos start/android/ios/web, sem comando de testes. Não encontrei teste funcional mobile. `mobile/src/` contém a estrutura documental planejada; não há prova de venda, autenticação operacional, banco próprio, ponto, sincronização ou recuperação offline no aplicativo. **ESQUELETO** de app, **AUSENTES** essas capacidades.

O mobile é parte do produto **planejado**, conforme `PROJECT_RULES.md`; não há evidência de app operacional antigo que deva ser instalado. `mobile/README.md` afirma que o código está em TitanSystem/mobile e deve ser movido: texto desatualizado diante do App.tsx já na raiz. Não inferir que iniciar Expo entrega as funções.

## 5. Frontend-web, módulos e correspondência de API

Todos os caminhos abaixo partem de `frontend-web/`. Testes citados estão em `tests/`; salvo os explicitamente executados na seção 9, apenas sua existência/conteúdo foi inspecionada.

| Módulo | Função e estado | Fontes e testes encontrados |
|---|---|---|
| auth | REAL login local; ESQUELETO cadastro online | `src/modules/auth/pages/LocalLogin.tsx`, `Login.tsx`, `Registration.tsx`, `RegistrationProfile112.tsx`, `RegistrationSummary112.tsx`; `tests/localClient.test.mjs`, `registration112.test.mjs`, `registrationModel.test.mjs`. Cadastro online não deve ser confundido com cadastro local de empresa. |
| catalog | REAL catálogo, estoque, reposição, compras, docas e recebimento nos contratos locais | `pages/LocalCatalog.tsx`, `StockWorkspace.tsx`, `ReviewedInventory.tsx`, `Orders115.tsx`, `SupplyChain.tsx`, `Receiving.tsx`, `ReceivingCases.tsx`, `DockAgenda.tsx`; `stockWorkspace`, `reviewedInventory`, `orders115`, `receiving`, `receivingCases`, `dockAgenda` `.test.mjs`. Não prova todos os equipamentos e integrações externas. |
| financial | REAL contas a pagar/agenda/fornecedor; ESQUELETO financeiro integral | `pages/Finance116.tsx`, `PayableDocuments.tsx`, `PayablesAgenda114.tsx`, `SupplierReport117.tsx`; `finance116`, `payableDocuments`, `agenda114`, `suppliers117` `.test.mjs`. Contas a receber e conciliação bancária oficiais completas não comprovadas. |
| maintenance | REAL manutenção interna contratada; demais fluxos devem respeitar disponibilidade | `pages/LocalMaintenance.tsx`, `src/core/internalMaintenance/client.mjs`; `tests/internalMaintenance.test.mjs`. Frota também tem telas em tenant e testes `fleetOfficial.test.mjs`, não constitui módulo mobile. |
| pos | REAL PDV oficial em dinheiro, caixa/carrinhos/devolução; ESQUELETO hardware/TEF/cupom fiscal integrado | `pages/LocalPointOfSale.tsx`, `OfficialCarts.tsx`, `CustomerReturns65.tsx`, `OfficialCashJournal.tsx`, `Receipt95.tsx`, `CheckoutFiscalStatus.tsx`; `localPOS`, `checkout96`, `officialCarts`, `officialReturns65`, `cashJournal`, `checkoutFiscal` `.test.mjs`. |
| tenant | REAL cadastro/contexto, fiscal homologação e algumas fontes; ESQUELETO demais workspaces | `pages/LocalCompanyRegistration.tsx`, `LocalStaff.tsx`, `LocalOwnerDashboard.tsx`, `LocalSpreadsheet.tsx`, `enterprise89/FiscalNFe55.tsx`; `companyRegistration`, `localAccess`, `ownerDashboard62`, `sheetStockSnapshot`, `sheetFinanceSnapshot`, `nfe55` `.test.mjs`. RH `src/core/hr71/attendance.ts`, `schedules.ts`, `leave.ts`, `payroll.ts` chamam `missing`: indisponíveis fora de demonstração. |

**Chamadas sem rota correspondente identificadas:** `src/core/api/dashboardService.ts` chama `/api/v1/metrics/overview` e `/api/v1/pos/sales/recent`; ambas ausentes em `backend/internal/core/routes/routes.go` e no inventário. Contudo a busca de importações encontrou apenas a própria declaração: **serviço órfão, não tela ativa comprovadamente quebrada**. Não atribuir essas chamadas ao dashboard atual sem importação. `LocalOwnerDashboard`/fontes locais usam contratos próprios. Não foi comprovada outra tela ativa chamando rota inexistente. Isso não equivale a provar todas as combinações de URLs dinâmicas em execução.

**Rotas backend sem tela/consumidor frontend comprovado:** busca em `frontend-web/src` e desktop não encontrou `/account/recover`, `/account/recovery/revoke`, `/auth/recovery/key`, `/auth/recovery/reset` nem `/fiscal/intents`. Logo recuperação existe na API/CLI, mas a tela de recuperação correspondente é AUSENTE por evidência. No fiscal nativo, criação/listagem/consulta de intents, DANFE e operações production abaixo não têm tela encontrada. Isso não torna as rotas production habilitadas: continuam sujeitas às travas existentes. Os endpoints online de indicações/SPED retornam indisponível e não provam uma função transacional só por existirem.

O inventário tem **567 pares método/caminho**. Não foram executadas todas as telas nem todos os caminhos dinâmicos. O anexo lista todas as rotas e referências literais encontradas no frontend, para tornar explícito o que a busca comprova e o que ainda depende de rastreamento de composição de URLs. Referência em cliente não prova botão navegável; ausência de literal não prova ausência de consumidor dinâmico. Não transformar essa limitação em uma lista falsa de 404.

## 6. Rotas públicas e segurança de autenticação

Todas as rotas marcadas públicas no inventário, conferidas nos roteadores `localapi/server.go` e `core/routes/routes.go`:

| Método/caminho | Proteção e efeito |
|---|---|
| GET /local/v1/health | Sem sessão; status local, sem dado comercial. |
| POST /local/v1/login | Credenciais humanas, aparelho fixado pelo processo; 5 requisições/minuto por IP. |
| POST /local/v1/account/recover | Chave de recuperação privada, de uso único, vinculada ao proprietário/empresa/loja/aparelho; 5/minuto por IP. |
| GET /api/v1/saude | Sem sessão; sujeito ao limitador geral online. |
| POST /api/v1/auth/login | Senha; RateLimitLogin 5/min/IP e limitador progressivo instalado por `cmd/api/main.go`. |
| POST /api/v1/auth/refresh | Cookie/token de renovação e validação de origem; sem sessão de acesso prévia, não anônimo sem prova. |
| POST /api/v1/auth/recovery/reset | Chave de recuperação e validação de origem, RateLimitLogin. |

As demais **560** entradas do inventário exigem sessão por seus grupos. Esse levantamento não substitui prova de autorização de papel em todas as rotas. Serviço separado: `POST /sync/v1/events`, em `localdb/incoming/receiver_http.go`, não usa sessão humana; exige envelope cifrado, assinatura, aparelho/peer autorizado e validação de escopo. Testes `receiver_http_test.go::TestSealedHTTPRejectsAmbiguousAndPlaintextRequests` e `TestSealedHTTPRechecksGrantAndRevocation` encontrados, não executados. Não confundir com rota de negócio pública. Árvores históricas não montadas não foram contabilizadas como listeners ativos.

### Login local

`server.go::login` devolve **401 com corpo genérico** quando `localauth.Login` retorna ErrDenied: identidade inexistente, senha errada e vínculo negado. Corpo inválido/ausente gera 400 e erro de infraestrutura pode gerar 500. `localauth/auth.go::Login` retorna antes do bcrypt quando a identidade não existe; senha errada de conta existente executa bcrypt. **Achado de leitura: possível enumeração por diferença de tempo**, não reproduzida por teste de distribuição temporal. Não encontrei teste existente que compare tempo/corpo entre as duas situações nem teste dedicado de 6 tentativas no login local.

Limiter Fiber em memória: **5 requisições por minuto/IP**, inclusive tentativas válidas; sexta recebe 429 até a janela liberar. Não encontrei bloqueio durável por conta ou escalonamento persistente nesse login. Reinício/outro IP pode contornar o estado em memória; clientes atrás do mesmo proxy/IP podem compartilhar o limite. Não anunciar proteção absoluta contra força bruta.

### Recuperação local

`localapi/account_access.go::recoverOwner` exige JSON estrito (limite 4096 bytes) com identity_id, recovery_key e new_password. `localdb/localauth/recovery.go::RecoverOwner`: chave aleatória 32 bytes, codificação validada, SHA-256 armazenado, comparação constante, validade e uso único; exige owner no mesmo contexto. Atualiza senha, revoga sessões e consome chaves com auditoria na mesma transação. **Não cria outro proprietário, não usa e-mail automático, não oferece acesso ao provedor e não recupera chave mestra do cofre.** A chave precisa ter sido preparada antes pelo proprietário autenticado.

Identidade inexistente/vínculo inválido e chave errada convergem para **401 `access_verification_failed`**, pelo tratamento de ErrDenied. Forma inválida gera 400/415/413 conforme entrada; senha inválida pode gerar 400 antes de consultar conta. A rotina calcula bcrypt da senha nova antes de consultar owner, mas isso não é prova de equalização temporal completa. Sem bloqueio por conta além do limitador IP em memória.

**Testes existentes executados:** `TestHTTPRecoveryRateLimitAndMalformedBodies` comprova cinco 401 e sexta 429, além de JSON duplicado recusado; não compara identidades existentes/inexistentes. `TestHTTPOwnerRecoveryRequiresKeyAndPreservesLoginWithoutVerifier` comprova uso único, não vazamento da chave e login após recuperação. `localauth/recovery_test.go` cobre escopo, expiração, revogação, perfil, auditoria e revogação de sessões. **AUSENTE teste específico de enumeração entre contas**: não inventado nem acrescentado nesta tarefa somente leitura.

### Outros achados de segurança (não corrigidos)

- `core/api/dashboardService.ts` imprime o erro Axios bruto em console.error; potencial de expor configuração/headers se voltar a ser usado. Sem consumidor ativo encontrado; não houve reprodução com token real.
- `core/auth/useAuthStore.ts::logout` limpa estado online; não prova revogação no servidor. O logout **local** usa seu cliente e não deve ser confundido com esse caminho.
- Existem arquivos .env no histórico conhecido; nenhum conteúdo foi aberto e nenhuma revogação de credenciais foi comprovada nesta auditoria. Ausência de arquivo rastreado atual não resolve segredo histórico.
- Não foi encontrada conta mestra do provedor nos caminhos inspecionados. Não houve auditoria dinâmica de toda dependência, logs instalados ou infraestrutura do cliente; não afirmar ausência universal de vazamento.

## 7. TitanSystem/ e referências antigas

Comparação de caminhos rastreados relativos: `TitanSystem/backend` tem 2 arquivos iguais, 2 diferentes e **21 caminhos exclusivos** em relação a backend/; frontend tem README diferente e **11 caminhos exclusivos**; desktop tem 3 READMEs diferentes e nenhum caminho exclusivo; mobile tem 5 READMEs iguais, README diferente e nenhum caminho exclusivo. “Exclusivo por caminho” não significa função exclusiva: arquivos Go relocados podem ter equivalentes na raiz.

Exclusivos backend: `internal/ai_insights/{README.md,produtos_parados.go}`, `internal/auth/{README.md,middleware_tentativas_login.go}`, `internal/config/README.md`, `internal/database/{README.md,sqlite.go}`, `internal/handlers/README.md`, `internal/handlers/ai_insights/README.md`, `internal/models/{auditoria.go,models.go}`, `internal/rewards/{README.md,constantes.go}`, `internal/routes/routes.go`, `internal/security/{README.md,chaves.go,criptografia.go}`, `internal/sync/README.md`, `internal/websocket/{README.md,chat.go,rotas.go}`.

Há equivalentes relocados na raiz para segurança e websocket; portanto não são novas funções automaticamente. A rota antiga não protegida na cópia não está montada pelo cmd/api atual e não foi comprovada compilável/operacional. Não executar essa árvore como alternativa.

Exclusivos frontend: somente READMEs em `public`, `src`, `src/components`, `src/components/{datagrid,kanban,maps}`, `src/hooks`, `src/pages`, `src/pages/{accountant,auth}`, `src/styles`. Nenhuma tela funcional exclusiva comprovada. Nenhum instalador exclusivo encontrado nesta árvore.

Referências ainda encontradas: `mobile/README.md` orienta usar/mover `TitanSystem/mobile/` (desatualizado); `TitanSystem/frontend/src/README.md` aponta para `TitanSystem/backend` (histórico); `desktop/README.md`, `backend/README.md` e `PROJECT_RULES.md` mencionam as cópias explicitamente como históricas. Busca de referências de caminho não encontrou instalador/script ativo direcionando build a TitanSystem/backend/frontend/mobile. Makefile/Compose e `desktop/scripts/build.mjs` usam a raiz e frontend-web. Não houve remoção nem correção documental fora deste arquivo.

## 8. Varejo por capacidade

| Capacidade | Estado e escopo | Arquivo e teste que comprovam; lacuna |
|---|---|---|
| Estoque | REAL local | `backend/internal/localdb/stock`, `stockreservation`, `stocktaking`; `localapi/stock_availability_test.go::TestHTTPStockAvailabilityReserveReleaseConsumeAndNoWrites`; `StockWorkspace.tsx`, `tests/stockWorkspace.test.mjs`. Hardware e sincronização comercial completa entre lojas não comprovados por isso. |
| Compras/reposição/recebimento | REAL em fluxo delimitado | `localdb/purchases`, `replenishment`, `supplycycle`; `localapi/supply_cycle_test.go::TestSupplyCycleVerticalReceivingFinancialApprovalPartialPaymentAndReturn`; `pilot_cash_acceptance_test.go::TestPilotCashAuthenticatedSupplierRestartAndRestore` e `TestPilotCashPartialReceiptDiscrepancyRestartAndRestore`. Há evidência de parcial/divergência e aceite autenticado: não listá-los como ausentes. |
| Financeiro | REAL contas a pagar em dinheiro; ESQUELETO financeiro integral | `localdb/payables`; `localapi/payable_cash_test.go::TestHTTPPayableCashPartialExactBalanceAndNoDuplicateMovement`; Finance116/Agenda114/Fornecedor117 com testes. Contas a receber, banco/TEF e DRE oficial completa não comprovados. |
| Relatórios | REAL para fontes delimitadas | `localdb/management/sales.go`, `localapi/management_reports_test.go::TestManagementPositionsValuationCSVAndPermissions`; `localdb/management/sales_test.go::TestManagementSalesABCComparisonScopeAndPageTotals`; ownerDashboard62/stockReports testes. Não equivalem a contabilidade fiscal ou indicadores de toda a rede. |
| Devolução | REAL física/caixa em dinheiro no contrato | `localdb/sale/customer_return*`; `localapi/customer_returns_test.go::TestHTTPCustomerReturnPartialCashStockAndOfficialClosure`; `supplier_returns_test.go::TestSupplierReturnPartialReceiptRecoveryReplayAndVoidGuard`. Fiscal, estorno eletrônico e cancelamento da venda são efeitos separados. |
| Fidelidade | ESQUELETO visual; AUSENTE operação transacional | `core/enterprise89/engine.mjs` tem coleção loyalty e validação local; `tests/enterprise89.test.mjs` cobre workspace, não livro de pontos durável no backend. Rotas online rewards/indicações em `core/routes/routes.go` retornam 501. Sem emissão/resgate/estorno real de pontos comprovados. |
| Múltiplas lojas | REAL isolamento; ESQUELETO operação distribuída completa | `localdb/identity/authorize.go`, `authorize_test.go::TestTenantStoreAndRoleIsolation` e `TestStoreLinkCannotCrossTenant`; fontes fixam tenant/store/device. `enterprise89` branches não prova administração/saldo/consolidação distribuída completa. |

## 9. Testes realmente executados nesta auditoria

Somente seleção pontual; nenhum teste novo, sem suíte completa.

- `go test ./internal/localapi -run '^(TestHTTPRecoveryRateLimitAndMalformedBodies|TestHTTPOwnerRecoveryRequiresKeyAndPreservesLoginWithoutVerifier|TestHTTPFiscalDANFERealHTTPExactArtifactAndReadonly)$' -timeout 120s -count=1 -v`: **3 testes passaram, 2,592 s**. HTTP/DB de ensaio, não Focus real.
- `go test ./internal/localdb/localauth -run '^(TestOfflineLoginRevocationAndLogout|TestRecoveryIsSingleUseAuditedAndRevokesEveryOwnerSession|TestRecoveryRejectsWrongKeyScopeExpiryRevocationAndManager)$' -timeout 120s -count=1 -v`: **3 testes passaram, 3,951 s**.
- `go test ./internal/modules/auth/usecase ./internal/modules/auth/delivery -run '^(TestLoginRealScenarios|TestLoginDoesNotReturnInternalErrorsOrSubmittedSecrets|TestRecoveryStrictInputNoSecretsOrCookiesOnFailure)$' -timeout 120s -count=1 -v`: **3 testes passaram; pacotes 0,207 s e 0,006 s**. O login online usa sqlmock, não PostgreSQL real. Não há ensaio de força bruta temporal aí.
- `node --test tests/paper106.test.mjs tests/receipt108.test.mjs tests/localPOS.test.mjs tests/checkout96.test.mjs`: **47 passaram, 0 falhas, 0 pulados; 106,44 ms**. Não testa periférico físico ou impressão de QR no Receipt95.

Logs locais: `/tmp/titan-lacunas-localapi.log`, `/tmp/titan-lacunas-auth.log`, `/tmp/titan-lacunas-online-auth.log`, `/tmp/titan-lacunas-frontend.log`. Não são adicionados ao repositório. Não foi feito build/CI completo nem inferido resultado de CI.

## 10. Backlog (15 itens, por impacto no cliente)

| Ordem | Entrega | Esforço | Quem e decisão prévia |
|---|---|---|---|
| 1 | Ligar documento NFC-e correto ao checkout e impressão, QR/chave/SEM VALOR FISCAL | M | Codex para código; cliente/Focus para ensaio oficial. Decidir origem por provedor e retenção. |
| 2 | Ensaiar instalação, abertura, backup/restauração e impressora em máquina limpa Linux/Windows | G | Hardware/executor Windows e cliente; escolher equipamentos/perfil de implantação. |
| 3 | Cobrir enumeração temporal e força bruta do login/recuperação; definir limite por conta/proxy | M | Codex testes/código; dono decide política de bloqueio sem negar serviço a loja inteira. |
| 4 | Credenciais históricas: inventário privado e revogação comprovada | M | Titulares dos serviços; depende de identificar credenciais sem publicá-las. Codex não revoga conta externa sozinho. |
| 5 | Cartão/TEF real, consulta, desfazimento, idempotência e reconciliação | G | Contrato/adquirente/hardware; escolher integrador e escopo. Codex implementa contrato documentado. |
| 6 | Balança, gaveta e seleção de impressora suportadas | G | Hardware/protocolos e dono; escolher modelos, SO e responsabilidade de instalação. |
| 7 | Fechar contas a receber/faturamento e separar realizado/projeção | G | Codex com contratos; dono/contador definem competência, origem e permissões. |
| 8 | Mobile operacional com identidade, dados próprios e limites offline | G | Codex código; dono/hardware definem primeiro fluxo e direitos offline; validar aparelho real. |
| 9 | Operação multiloja com administração e consolidação autorizada | G | Codex com contratos; dono define topologia e autoridade de estoque/venda. |
| 10 | Fidelidade durável, adesão explícita, extrato/resgate/estorno | G | Dono define regras e privacidade; Codex implementa contrato após decisão. |
| 11 | Tela de recuperação de conta sem acesso privilegiado do provedor | M | Codex; dono define procedimento do cliente e custódia da chave. |
| 12 | Resolver serviços órfãos/dashboard e rastrear telas ↔ URLs dinâmicas em ensaio navegável | M | Codex; escolher contratos oficiais, sem inventar métricas. |
| 13 | RH além de credenciais (ponto, escala, férias/documentos) | G | Dono/regras trabalhistas; Codex após contratos e escopo. |
| 14 | Revisar documentos TitanSystem/mobile e neutralizar riscos do logging Axios online | P | Codex; decidir conservação histórica, sem apagar árvore por conveniência. |
| 15 | Homologação fiscal por cliente, dados do contador e capacidades ainda indisponíveis | G | Cliente/contador/Focus/prefeitura; certificados, contrato, regras e município. Produção não é habilitada por esta auditoria. |

## Anexo A — inventário de páginas por módulo

Listagem de arquivos, não declaração de que todos sejam rotas navegáveis independentes:

### auth

- `auth/pages/LocalLogin.tsx`
- `auth/pages/Login.tsx`
- `auth/pages/Registration.tsx`
- `auth/pages/RegistrationProfile112.tsx`
- `auth/pages/RegistrationSummary112.tsx`

### catalog

- `catalog/pages/CatalogManagement.tsx`
- `catalog/pages/CompanyTrade66.tsx`
- `catalog/pages/DeliveryCycle.tsx`
- `catalog/pages/DockAgenda.tsx`
- `catalog/pages/DockInsights.tsx`
- `catalog/pages/LocalCatalog.tsx`
- `catalog/pages/LocalComparison.tsx`
- `catalog/pages/LocalOrders.tsx`
- `catalog/pages/LocalStock.tsx`
- `catalog/pages/Orders115.tsx`
- `catalog/pages/Receiving.tsx`
- `catalog/pages/ReceivingCases.tsx`
- `catalog/pages/ReceivingOverview.tsx`
- `catalog/pages/ReplenishmentPanel.tsx`
- `catalog/pages/RestockPreview.tsx`
- `catalog/pages/ReviewedInventory.tsx`
- `catalog/pages/StockReports.tsx`
- `catalog/pages/StockWorkspace.tsx`
- `catalog/pages/SupplierDecisions.tsx`
- `catalog/pages/SupplyChain.tsx`

### financial

- `financial/pages/CashSources116.tsx`
- `financial/pages/Checkout.tsx`
- `financial/pages/Finance116.tsx`
- `financial/pages/LocalPayables113.tsx`
- `financial/pages/LocalPayablesAgenda114.tsx`
- `financial/pages/PayableDocuments.tsx`
- `financial/pages/Payables113.tsx`
- `financial/pages/PayablesAgenda114.tsx`
- `financial/pages/SupplierReport117.tsx`

### maintenance

- `maintenance/pages/LocalMaintenance.tsx`

### pos

- `pos/pages/Bag99.tsx`
- `pos/pages/CashClosing92.tsx`
- `pos/pages/CashCount.tsx`
- `pos/pages/CashCustodyGuard.tsx`
- `pos/pages/CashDesk109.tsx`
- `pos/pages/Checkout91.tsx`
- `pos/pages/Checkout96.tsx`
- `pos/pages/CheckoutFiscalStatus.tsx`
- `pos/pages/ClosingReview97.tsx`
- `pos/pages/Count107.tsx`
- `pos/pages/CustomerReturns65.tsx`
- `pos/pages/Desk95.tsx`
- `pos/pages/FinalClose108.tsx`
- `pos/pages/Ledger99.tsx`
- `pos/pages/LocalPointOfSale.tsx`
- `pos/pages/OCR96.tsx`
- `pos/pages/OfficialCarts.tsx`
- `pos/pages/OfficialCashBags.tsx`
- `pos/pages/OfficialCashJournal.tsx`
- `pos/pages/OfficialCashMovement.tsx`
- `pos/pages/OfficialCashVaults.tsx`
- `pos/pages/OfficialReport101.tsx`
- `pos/pages/PaymentParts94.tsx`
- `pos/pages/Phone95.tsx`
- `pos/pages/PointOfSale.tsx`
- `pos/pages/Receipt95.tsx`
- `pos/pages/ReceiptDesk108.tsx`
- `pos/pages/Reconciliation93.tsx`
- `pos/pages/Report95.tsx`
- `pos/pages/Report98.tsx`
- `pos/pages/ReturnQueue110.tsx`
- `pos/pages/ReturnReview110.tsx`
- `pos/pages/Returns100.tsx`
- `pos/pages/SaleCustomer93.tsx`
- `pos/pages/SalesHistory.tsx`
- `pos/pages/Suspended104.tsx`
- `pos/pages/Suspended111.tsx`
- `pos/pages/Till94.tsx`
- `pos/pages/requests105/Operations105.tsx`
- `pos/pages/requests105/RequestForm105.tsx`
- `pos/pages/requests105/RequestReview105.tsx`
- `pos/pages/workspace103/Archives103.tsx`
- `pos/pages/workspace103/Queue103.tsx`
- `pos/pages/workspace103/Sales103.tsx`
- `pos/pages/workspace103/UI103.tsx`

### tenant

- `tenant/pages/AdminDashboard.tsx`
- `tenant/pages/CompanySetupSteps.tsx`
- `tenant/pages/CustomerOperations.tsx`
- `tenant/pages/DeliveryTracking78.tsx`
- `tenant/pages/LocalBusiness85.tsx`
- `tenant/pages/LocalCompanyRegistration.tsx`
- `tenant/pages/LocalDistributionMap.tsx`
- `tenant/pages/LocalEnterprise89.tsx`
- `tenant/pages/LocalHome.tsx`
- `tenant/pages/LocalHumanResources.tsx`
- `tenant/pages/LocalIntegrations74.tsx`
- `tenant/pages/LocalLossPrevention76.tsx`
- `tenant/pages/LocalMainMenu.tsx`
- `tenant/pages/LocalOperation90.tsx`
- `tenant/pages/LocalOperations83.tsx`
- `tenant/pages/LocalOperations84.tsx`
- `tenant/pages/LocalOwnerDashboard.tsx`
- `tenant/pages/LocalSalesAnalysis.tsx`
- `tenant/pages/LocalSectors86.tsx`
- `tenant/pages/LocalSettings80.tsx`
- `tenant/pages/LocalSpreadsheet.tsx`
- `tenant/pages/LocalStaff.tsx`
- `tenant/pages/LocalSubscription.tsx`
- `tenant/pages/LocalWorkbench73.tsx`
- `tenant/pages/Sheets70.tsx`
- `tenant/pages/business85/Commercial85.tsx`
- `tenant/pages/business85/Documents85.tsx`
- `tenant/pages/business85/Fleet85.tsx`
- `tenant/pages/business85/Food85.tsx`
- `tenant/pages/business85/Lines85.tsx`
- `tenant/pages/business85/Purchasing85.tsx`
- `tenant/pages/business85/UI.tsx`
- `tenant/pages/enterprise89/FinanceFiscal.tsx`
- `tenant/pages/enterprise89/FiscalAccountingExport.tsx`
- `tenant/pages/enterprise89/FiscalCorrection.tsx`
- `tenant/pages/enterprise89/FiscalCustomer.tsx`
- `tenant/pages/enterprise89/FiscalNFe55.tsx`
- `tenant/pages/enterprise89/FiscalProfilePicker.tsx`
- `tenant/pages/enterprise89/FiscalProviderCredentials.tsx`
- `tenant/pages/enterprise89/Privacy.tsx`
- `tenant/pages/enterprise89/UI.tsx`
- `tenant/pages/enterprise89/WorkAreas.tsx`
- `tenant/pages/fleetMaintenance/FleetMaintenance.tsx`
- `tenant/pages/fleetOfficial/FleetCommandControls.tsx`
- `tenant/pages/fleetOfficial/FleetOfficial.tsx`
- `tenant/pages/fleetOfficial/ProviderQuoteControls.tsx`
- `tenant/pages/fleetOfficial/WorkshopControls.tsx`
- `tenant/pages/operation90/Closing.tsx`
- `tenant/pages/operation90/FiscalLosses.tsx`
- `tenant/pages/operation90/InventoryProducts.tsx`
- `tenant/pages/operation90/POS.tsx`
- `tenant/pages/operation90/Purchases.tsx`
- `tenant/pages/operation90/UI.tsx`
- `tenant/pages/operations84/Cash84.tsx`
- `tenant/pages/operations84/Devices84.tsx`
- `tenant/pages/operations84/Inventory84.tsx`
- `tenant/pages/operations84/Journal84.tsx`
- `tenant/pages/operations84/Market84.tsx`
- `tenant/pages/operations84/Performance84.tsx`
- `tenant/pages/operations84/Quality84.tsx`
- `tenant/pages/operations84/Returns84.tsx`
- `tenant/pages/operations84/Supply84.tsx`
- `tenant/pages/operations84/UI.tsx`
- `tenant/pages/sectors86/Delivery86.tsx`
- `tenant/pages/sectors86/Food86.tsx`
- `tenant/pages/sectors86/OfficialRecipes.tsx`
- `tenant/pages/sectors86/ProductionAftermath.tsx`
- `tenant/pages/sectors86/ProductionCommands.tsx`
- `tenant/pages/sectors86/ProductionExecution.tsx`
- `tenant/pages/sectors86/ProductionPeople.tsx`
- `tenant/pages/sectors86/ProductionPlanning.tsx`
- `tenant/pages/sectors86/ProductionStageHistory.tsx`
- `tenant/pages/sectors86/Profile86.tsx`
- `tenant/pages/sectors86/RecipeState.tsx`
- `tenant/pages/sectors86/Recipes86.tsx`
- `tenant/pages/sectors86/UI.tsx`

## Anexo B — inventário de API e referência textual no frontend

Extraído de `docs/api/route-inventory.json`. “Sem literal completo” significa apenas ausência de correspondência textual normalizada; chamadas que compõem prefixos/ações separadamente precisam rastreamento, não são marcadas como rota inexistente. Método não é inferido pela referência textual. A identificação de rotas efetivamente sem tela está na seção 5. A lista abaixo não mede cobertura de testes nem acessibilidade de UI.

| Método | Caminho | Referência literal no frontend (ou composição a rastrear) |
|---|---|---|
| GET | `/local/v1/access/audit` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/access/policy` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/access/policy` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/account/password` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/account/recover` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/account/recovery/revoke` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/account/sessions` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/account/sessions/revoke-others` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/accounting/cash/journal` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/accounting/cash/report` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/accounting/cash/report.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/accounting/cash/sessions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/capabilities` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/cash/bags` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/bags` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/bags/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/bags/:id/cancel` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/bags/:id/dispatch` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/bags/:id/receive` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/bags/:id/reject` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/bags/operations/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/close` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/cash/current` | `frontend-web/src/core/local/localClient.mjs` |
| POST | `/local/v1/cash/movements` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/movements/operations/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/open` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/cash/sessions/:id/custody` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/sessions/:id/journal` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/sessions/:id/journal.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/sessions/:id/report` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vault-counts` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vault-counts` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vault-counts/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vault-counts/:id/accept` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vault-counts/:id/cancel` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vault-counts/:id/reject` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vault-counts/:id/submit` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vault-counts/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vaults` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vaults` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vaults/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vaults/:id/archive` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vaults/:id/deposit` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vaults/:id/statement` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vaults/:id/supply` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/cash/vaults/:id/transfer` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vaults/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vaults/pending-bags` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vaults/report` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/cash/vaults/report.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/catalog/operations/:kind/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/catalog/products/:id/edit-details` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/catalog/products/:id/edit-state` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/catalog/products/:id/production-usage` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/catalog/products/:id/state` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/catalog/products/:id/state` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/catalog/products/:id/state-history` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/catalog/products/:id/update` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/catalog/search` | `frontend-web/src/core/local/catalogSearch.mjs` |
| POST | `/local/v1/checkout/plans` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/checkout/plans/:id` | `frontend-web/src/core/local/checkoutFiscal.mjs` |
| POST | `/local/v1/checkout/plans/:id/cancel` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/checkout/plans/:id/finalize-cash` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/checkout/plans/:id/fiscal/reprocess` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/checkout/plans/:id/parts/:part/proofs` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/checkout/proofs` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/checkout/proofs/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/checkout/proofs/:id/accept` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/checkout/proofs/:id/reject` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/checkout/proofs/:id/revoke` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/checkout/proofs/:id/withdraw` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/checkout/proofs/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/checkout/proofs/report` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/checkout/proofs/report.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/comparison-site-operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/comparison-sites` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/comparison-sites` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/finance/payable-cash/:id/return` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payable-cash/:id/return-sources` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payable-cash/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/finance/payables` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables/:id` | `frontend-web/src/core/payables114/client.mjs` |
| POST | `/local/v1/finance/payables/:id/approve` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/finance/payables/:id/cancel` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables/:id/cash-payments` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/finance/payables/:id/cash-payments` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables/:id/cash-sources` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables/:id/installment-balances` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/finance/payables/:id/reject` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables/report` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/payables/report.csv` | `frontend-web/src/core/payables114/client.mjs` |
| GET | `/local/v1/finance/receiving-origins/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/finance/receiving-return-review/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/assets` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/assets` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/assets/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/assets/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/assets/:id/availability` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/assets/:id/costs` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/assets/:id/costs.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/assets/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/assets/:id/preventive` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/expenses` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/expenses` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/expenses/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/expenses/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/expenses/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/impacts` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/impacts` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/impacts/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/impacts/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/impacts/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/income` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/income` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/income/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/income/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/income/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/notices` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/notices/:id/ack` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/plans` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/plans` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/plans/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/plans/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/plans/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/profiles` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/profiles` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/profiles/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/profiles/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/profiles/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/providers` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/providers` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/providers/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/providers/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/providers/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/quotes` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/quotes` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/quotes/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/quotes/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/quotes/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/requests` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/requests` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/requests/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/requests/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/requests/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/requests/:id/workshop` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/requests/:id/workshop.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/trips` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/trips` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/trips/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/trips/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/trips/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/inspections` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/inspections` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/inspections/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/inspections/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/inspections/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/labor` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/labor` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/labor/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/labor/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/labor/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/requirements` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/requirements` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/requirements/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/requirements/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/requirements/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/templates` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/templates` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/templates/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fleet/workshop/templates/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fleet/workshop/templates/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/health` | `frontend-web/src/core/context74/health.ts` |
| GET | `/local/v1/locations` | `frontend-web/src/core/local/localClient.mjs`; `frontend-web/src/core/sheets70/stockSnapshot.mjs` |
| POST | `/local/v1/locations` | `frontend-web/src/core/local/localClient.mjs`; `frontend-web/src/core/sheets70/stockSnapshot.mjs` |
| POST | `/local/v1/login` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/logistics/appointments` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/logistics/appointments` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/appointments.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/appointments/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/logistics/appointments/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/appointments/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/docks` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/logistics/docks` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/docks/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/insights/availability` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/insights/pending` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/insights/pending.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/insights/suppliers/performance` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/insights/suppliers/performance.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/insights/utilization` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/insights/utilization.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/occupancy` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/logistics/operations/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/logout` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/maintenance/assets` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/assets` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/assets/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/assets/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/assets/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/candidates` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/classifications` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/classifications` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/classifications/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/classifications/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/classifications/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/queue` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/specialties` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/specialties` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/specialties/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/specialties/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/specialties/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/technicians` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/technicians` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/technicians/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/maintenance/technicians/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/maintenance/technicians/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/management/inventories/:id/variance` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/management/production/recipe-versions/:id/cost-scenario` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/management/production/simultaneous-plan` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/management/purchases/receiving-board` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/management/purchases/receiving-board.csv` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/management/replenishment/preview` | `frontend-web/src/core/restockPreview/source.mjs` |
| GET | `/local/v1/management/sales/products` | `frontend-web/src/core/salesAnalysis/source.mjs` |
| GET | `/local/v1/management/sales/products.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/management/stock/positions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/management/stock/positions.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/management/stock/valuation` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/management/stock/valuation.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/me` | `frontend-web/src/core/local/localClient.mjs` |
| POST | `/local/v1/module-contracts` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/pos/cart-operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/pos/carts` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/pos/carts` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/pos/carts/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/pos/carts/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/pos/carts/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/pos/carts/:id/reconcile` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/pos/carts/report` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/pos/carts/report.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/capacity` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/capacity/alternatives` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/losses` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/losses/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/losses/:id/history` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/losses/void` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/lot-search` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/lots` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/lots/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/lots/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/lots/:id/quality` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/lots/:id/quality/history` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/lots/void` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/material-reservations` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/material-reservations/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/material-reservations/state` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/operations/:kind/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/order-search` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/orders` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/orders` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/orders/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/orders/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/orders/:id/material-cost` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/orders/:id/stages` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/orders/:id/stages/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/orders/:id/trace` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/orders/state` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/quality-reviews` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/recipe-state` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/recipe-versions` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/recipe-versions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/recipe-versions/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/recipe-versions/:id/material-cost` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/recipes/:id/state` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/results` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/results/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/results/:id/losses` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/production/results/:id/lots` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/stage-plans` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/production/stages/state` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/products` | `frontend-web/src/core/local/localClient.mjs` |
| POST | `/local/v1/products` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/purchase-approvals` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-cancellation-operations/:id/receipt` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-cycle-operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-deliveries/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-deliveries/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-deliveries/:id/history` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-deliveries/:id/payable` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-deliveries/:id/receive` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-deliveries/:id/reject` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-order-search` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/authorize-receiving` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/cancel` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/deliveries` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/receipts` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/receipts/:receipt_id/void` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/receiving-cases` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders/:id/receiving-history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders/:id/receiving-overview` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders/:id/receiving-overview.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders/:id/receiving-trace` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/rejections` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/supplier-decisions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders/:id/supply-cycle` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-orders/:id/trace` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-receiving-case-operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-receiving-cases` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-receiving-cases/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-receiving-cases/:id/actions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-receiving-cases/:id/history` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-receiving-cases/:id/returns` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-receiving/operations/:kind/:id/receipt` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-supplier-operations/:id/receipt` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-supplier-return-operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-supplier-returns` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-supplier-returns/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-supplier-returns/:id/actions` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-supplier-returns/:id/dispatch` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-supplier-returns/:id/history` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-supplier-returns/:id/recover` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-suppliers` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-suppliers` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-suppliers/:id/edit-state` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/purchase-suppliers/:id/history` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-suppliers/:id/update` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/replenishment/operations/:kind/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/replenishment/policies` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/replenishment/products` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/replenishment/reviews` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/replenishment/suggestions` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/replenishment/suggestions` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/sales` | `frontend-web/src/core/local/localClient.mjs` |
| POST | `/local/v1/sales` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/sales/:id` | `frontend-web/src/core/local/localClient.mjs` |
| POST | `/local/v1/sales/:id/cancel` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/sales/quote` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/staff` | `frontend-web/src/core/local/localClient.mjs` |
| POST | `/local/v1/staff` | `frontend-web/src/core/local/localClient.mjs` |
| POST | `/local/v1/staff/:id/password-reset` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/staff/:id/sessions/revoke` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/availability` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/balance` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/inventories` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/stock/inventories` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/inventories/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/stock/inventories/:id/counts` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/inventories/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/inventories/:id/report.csv` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/stock/inventories/:id/state` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/inventory-operations/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/stock/operations` | `frontend-web/src/core/local/localClient.mjs` |
| GET | `/local/v1/stock/operations/:id/receipt` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/operations/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/stock/reservations` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/system/info` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/accounting/sped/request` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/accounting/sped/status/:job_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/analises/produtos-parados` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/login` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/logout` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/password` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/recovery/key` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/recovery/reset` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/refresh` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/auth/sessions` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/sessions/:id/revoke` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/auth/sessions/revoke-others` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/adjustments` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/adjustments/:operation_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/adjustments/apply` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/adjustments/exports/csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/adjustments/exports/preview` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/adjustments/preview` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/barcodes/batches/:operation_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/barcodes/batches/apply` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/barcodes/batches/preview` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/barcodes/import/apply` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/barcodes/import/preview` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/barcodes/import/template` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/barcodes/lookup` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/barcodes/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/batches` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/batches/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/batches/:operation_id/import` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/batches/:operation_id/undo` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/batches/apply` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/batches/preview` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/cost-history/entries` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/cost-history/entries/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/cost-history/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/cost-history/products/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/cost-history/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/costs/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/creations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/exports/csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/exports/preview` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/imports/:operation_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/imports/apply` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/imports/preview` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/margins/below-target` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/margins/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/margins/missing-cost` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/margins/negative` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/margins/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/quality/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/quality/no-active-gtin` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/quality/no-description` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/quality/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/quality/zero-stock` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/items` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/products/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/replenishment/products/:id/simulate` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/proposals` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/replenishment/proposals` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/proposals/:proposal_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/replenishment/proposals/:proposal_id/approve` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/replenishment/proposals/:proposal_id/cancel` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/proposals/:proposal_id/check` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/proposals/:proposal_id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/proposals/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/proposals/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/replenishment/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/stock-settings/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/stock/below-minimum` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/stock/coverage` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/stock/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/stock/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/stock/zero` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/catalog/undos/:operation_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/undos/apply` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/catalog/undos/preview` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/checks/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/checks/inactive` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/checks/price-changed` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/checks/suggestions/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/checks/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/report/approved` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/report/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/report/pending` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/report/rejected` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/report/summary` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/reviews/:operation_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/discounts/suggest` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/suggestions` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/suggestions/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/discounts/suggestions/:id/approve` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/discounts/suggestions/:id/reject` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/discounts/suggestions/:id/review` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/discounts/suggestions/:id/reviews` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/active` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/barcodes` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/barcodes` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/barcodes/:barcode_id/active` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/barcodes/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/barcodes/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/cost` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/cost` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/cost/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/creation` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/details` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/margin` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/price` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/stock-settings` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/stock-settings` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/:id/stock-settings/history` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/produtos/:id/stock-settings/preview` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/produtos/search` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/purchase-documents` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchase-documents/:document_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/purchase-documents/:document_id/cancel` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchase-documents/:document_id/download.json` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchase-documents/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchases` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/purchases` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchases/:purchase_id` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/purchases/:purchase_id/approve` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/purchases/:purchase_id/cancel` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchases/:purchase_id/check` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchases/:purchase_id/history` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchases/export.csv` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchases/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/purchases/summary` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/recompensas/indicacoes` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/recompensas/indicacoes/recompensar` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/recompensas/indicacoes/saldo` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/saude` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/supplier-offers` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/supplier-offers` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/supplier-offers/:offer_id` | Sem literal completo; verificar composição/consumidor |
| PUT | `/api/v1/supplier-offers/:offer_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/supplier-offers/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/suppliers` | Sem literal completo; verificar composição/consumidor |
| POST | `/api/v1/suppliers` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/suppliers/:supplier_id` | Sem literal completo; verificar composição/consumidor |
| PUT | `/api/v1/suppliers/:supplier_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/api/v1/suppliers/operations/:operation_id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/company-trade/identity` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/company-trade/peers` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/company-trade/peers` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/company-trade/peers/:id/revoke` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/company-trade/export` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/company-trade/orders/import` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/company-trade/orders/:id/decision` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/purchase-orders/:id/company-trade/decision` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/company-trade/messages` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/company-trade/messages/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/company-trade/operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/company-trade/recovery` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/company-trade/recovery/review` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/sale-return-operations/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/sales/:id/returns` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/sales/:id/returns` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/sales/:id/returns/quote` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/sales/:id/returns/cash-session` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/sales/:id/returns/cash-session/quote` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/capabilities` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/requirements/:uf` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/profiles` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/profiles` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/profiles/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/intents` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/intents` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/intents/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/operations/:id/receipt` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/intents/:id/danfe` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/intents/:id/production` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/intents/:id/production/approve` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/intents/:id/production/emit` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/intents/:id/production/prepare` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/intents/:id/production/query` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/intents/:id/production/xml` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/intents/:id/production/cancel` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/intents/:id/production/cancellation` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/intents/:id/production/cancellation/xml` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/tax-policies` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/tax-policies/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/tax-quotes` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/tax-quotes/:id` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/onboarding` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/onboarding/:id` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/onboarding/:id/provider` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/onboarding/:id/provider` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/nfe55/sales/:id/cancel` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/nfe55/sales/:id/danfe` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/nfe55/sales/:id/emit` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/nfe55/sales/:id/preview` | Sem literal completo; verificar composição/consumidor |
| POST | `/local/v1/fiscal/nfe55/sales/:id/query` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/company/registration` | `frontend-web/src/core/companyRegistration/client.mjs` |
| POST | `/local/v1/company/registration` | `frontend-web/src/core/companyRegistration/client.mjs` |
| POST | `/local/v1/fiscal/nfe55/sales/:id/correction` | Sem literal completo; verificar composição/consumidor |
| GET | `/local/v1/fiscal/accounting-export` | `frontend-web/src/modules/tenant/pages/enterprise89/FiscalAccountingExport.tsx` |
| POST | `/local/v1/fiscal/tax-tables/import` | Sem literal completo; verificar composição/consumidor |
