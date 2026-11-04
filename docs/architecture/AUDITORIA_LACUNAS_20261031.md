# Auditoria de lacunas do TitanSystem

Documento solicitado: AUDITORIA_LACUNAS_20261031.md. Inspeção/testes: **03/11/2026**.
Repositório local: `/home/danillo-malta/projetos/zzeussss`.
Base: `feat/recebiveis`, **b16d22cc553a4b3bcb9b78ddeabe6310c80fe229**; auditoria em `feat/auditoria-lacunas`.
A branch local anterior de auditoria, em 6d99eb3, foi preservada como `archive/auditoria-lacunas-6d99eb3` antes de criar esta branch a partir de recebíveis.
Única alteração de conteúdo nesta tarefa: este documento. Nenhum código corrigido.

## Critério, método e limites

**REAL:** implementação e teste encontrados, apenas para a capacidade delimitada. **ESQUELETO:** estrutura/interface parcial, sem operação integral comprovada. **AUSENTE:** não foi encontrada evidência de implementação e teste para a capacidade especificada. Um módulo contém capacidades de estados diferentes; REAL não significa produto inteiro concluído.

Leitura dirigida de regras, nomes de arquivos/testes, roteadores, recibos, clientes HTTP e implementações relevantes. A matriz de APIs no anexo usa busca estática de caminhos literais/compostos: referência em cliente não prova navegação por uma tela, e ausência de correspondência não prova inexistência de consumidor dinâmico. Não houve suíte completa, build, instalação, alteração de banco instalado, acesso a segredos, emissão real, chamada à Focus, teste externo ou uso de hardware. Testes encontrados e testes executados estão separados. A solicitação autoriza este relatório e os testes pontuais, como exceção ao AGENTS.md.

| Item | Estado | Resultado delimitado |
|---|---|---|
| 1. Cupom NFC-e | REAL no percurso de homologação em código | Checkout ligado ao DANFE NFC-e HTML 58/80, com QR/chave; impressora física e aceitação Focus/SEFAZ não comprovadas. |
| 2. Impressão/hardware | ESQUELETO no conjunto | Impressão via SO e busca textual reais; drivers de gaveta/balança e validação física AUSENTES. |
| 3. TEF/cartão | AUSENTE como autorizador | Há representação e conferência manual de comprovantes; checkout oficial em dinheiro não confirma cartão. |
| 4. Mobile | ESQUELETO | Expo com mensagem de função não implementada; vendas/sincronização offline AUSENTES. |
| 5. Frontend-web | REAL em fluxos delimitados; ESQUELETO no conjunto | Seis módulos, testes de clientes/modelos; há adaptadores indisponíveis e cobertura visual incompleta. |
| 6. Rotas públicas | REAL nos controles/testes descritos | Sete entradas sem sessão nas APIs principais; limites IP e respostas uniformes locais comprovados. Sem prova de proteção distribuída. |
| 7. TitanSystem/ | ESQUELETO histórico | Cópia parcial divergente, com caminhos exclusivos; README mobile ainda aponta para ela. |
| 8. Varejo | REAL em operações delimitadas; ESQUELETO no conjunto | Recebíveis reais acrescentados; fidelidade transacional AUSENTE e operação multiloja integral não comprovada. |

## 1. Cupom NFC-e ao consumidor

**Receipt95 hoje.** `frontend-web/src/modules/pos/pages/Receipt95.tsx` mostra status da venda; nome do produto ou ordinal, quantidade, preço unitário e total por item; modalidades/valores das parcelas; total; recebido e troco quando compatíveis. O identificador interno da venda é omitido da lista visível, mas vai nas linhas da impressão comum. A impressão comum continua **não fiscal**: `src/i18n/desk95/pt-BR.json` diz “COMPROVANTE NÃO FISCAL. Não substitui documento fiscal.” O HTML desse comprovante não ganha chave/QR apenas por a venda ter status fiscal.

**Integração fiscal REAL, posterior à auditoria antiga.** Receipt95 incorpora `CheckoutFiscalStatus.tsx`. Para nota autorizada e fora de demonstração, esse componente oferece impressão fiscal e seleção 58/80 mm; `src/core/local/checkoutFiscal.mjs` chama `GET /local/v1/checkout/plans/:id/fiscal/danfe`. `src/core/local/fiscalPrint.mjs` abre o HTML protegido, aguarda decodificação das imagens QR antes de imprimir e fecha/recusa ao invalidar o contexto. `tests/fiscalPrint.test.mjs` cobre o caminho, a espera pelo QR, interrupção de sessão e textos nos cinco idiomas; executado nesta auditoria.

**Origem/formato:** `backend/internal/fiscalproduction/checkout_print.go::ProviderRegistry.CheckoutNFCeDANFE` exige sessão/contexto correspondentes, venda confirmada e nota autorizada em homologação. Usa o artefato nativo quando existente; no provedor externo, lê o XML autorizado arquivado e cifrado pelo cliente, verifica integridade/vínculo e renderiza localmente. **O cupom NFC-e Focus deste percurso não é um PDF baixado da Focus:** a origem fiscal é o XML arquivado, o HTML/QR é produzido pelo renderizador existente.

`backend/internal/fiscalcore/danfe.go::RenderHomologationNFCe`: HTML térmico **58/80 mm**, QR PNG embutido, chave de acesso de 44 dígitos, emitente, número/série, itens, valores, pagamentos, protocolo e endereço de consulta. Destaque: **“EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL”**. `src/core/local/print95.ts` imprime o comprovante comum em HTML 58/80/A4 por `window.print()`. Não há geração própria de PDF térmico nem comandos ESC/POS nesse caminho; salvar PDF depende do diálogo do SO.

Provas encontradas: `fiscalcore/danfe_test.go::TestFiscalDANFEExactSourceLayoutAndEscapedData`, `TestFiscalDANFEQRBindingAndFormatting`. Provas **executadas**: `localapi/checkout_fiscal_test.go::TestHTTPCheckoutNFCeHomologationIntegration` (58/80, QR, aviso, sessão e ausência de reemissão) e `fiscalproduction/checkout_print_test.go::TestCheckoutEncryptedNFCePrintArchiveIsolationIntegrityAndNoNetwork` (isolamento, integridade e nenhuma chamada de rede para imprimir). A autoridade de teste é descartável; isso não é autorização oficial. Nenhum cupom foi impresso fisicamente ou lido por consumidor nesta auditoria.

NF-e 55 é outra saída: `fiscalproduction/focus_nfe.go::DANFE`, teste `focus_nfe_test.go::TestFocusNFeDANFEDownloadRestricted`, e `tenant/pages/enterprise89/FiscalNFe55.tsx` fazem download restrito do PDF descrito pelo adapter. Não confundir com o cupom 65 acima.

## 2. Impressão e hardware

| Recurso | Estado e evidência | Falta comprovar/implementar |
|---|---|---|
| Impressão HTML pelo SO | REAL em software: `fiscalPrint.mjs`, `print95.ts`, `tests/fiscalPrint.test.mjs` | Impressora real, margem/corte/QR e instalação limpa; não certifica todas as marcas. |
| Driver ESC/POS, serial/USB e seleção de equipamento | AUSENTE como implementação com teste | `desktop/main.cjs`, `preload.cjs` e `desktop/src/pos/README.md` não oferecem driver correspondente comprovado. |
| Gaveta física | AUSENTE | Gaveta lógica/livro de caixa não são pulso elétrico de abertura; falta protocolo, implementação e teste. |
| Leitor de código de barras | REAL para entrada/busca textual; ESQUELETO para integração física | `LocalPointOfSale.tsx` mantém foco/atalhos; `core/local/cartEditing.mjs` seleciona SKU/barcode; `tests/localCartEditing.test.mjs` passou. Leitor HID configurado como teclado pode fornecer texto, mas não foi ensaiado. Sem driver dedicado comprovado. |
| Balança | AUSENTE como comunicação com equipamento | Quantidade fracionária manual não é captura de peso. Falta protocolo/driver/teste autorizado. |

`src/core/operations84/adapters/devices.ts::devicesAdapter84` devolve `unavailable` por padrão e exemplos apenas com demonstração explícita. A tela de dispositivos não comprova conexão. Não é possível prometer compatibilidade universal sem protocolos e equipamentos; nenhum periférico foi acessado nesta tarefa.

## 3. TEF e pagamento por cartão

**AUSENTE** integração autorizadora real com adquirente/TEF, consulta, desfazimento e conciliação. `backend/internal/localdb/sale/checkout_cash.go::cashOnly` e `FinalizeCashPlan` recusam pagamento que o fluxo não consegue confirmar (`ErrPaymentUnverified`). Teste encontrado: `localapi/checkout_cash_test.go::TestHTTPCheckoutCashFinalizationDoesNotApproveMixedPayments`.

**ESQUELETO/manual:** `pos/pages/PaymentParts94.tsx`, `ReceiptDesk108.tsx` e `core/local/paper106.mjs`, `receipt108.mjs` tratam modalidades e evidências/comprovantes. Testes paper106/receipt108 executados: ler/vincular comprovante não declara liquidação. OCR auxilia revisão e não confirma pagamento eletrônico. Falta contrato/SDK/ambiente da adquirente para comprovar integração real.

## 4. Mobile

`mobile/App.tsx` só renderiza View/Text: **“TitanSystem Mobile: vendas e sincronização offline ainda não implementadas”**. `mobile/package.json` define Expo/React Native e comandos start/android/ios/web; não há comando de testes. `mobile/src/` contém a estrutura documental, sem evidência de teste funcional do aplicativo.

**ESQUELETO** de aplicativo do produto planejado; **AUSENTES** venda operacional, autenticação funcional comprovada, persistência de comandos mobile, direitos exclusivos offline, recuperação e sincronização testada. Comentários sobre SQLite/WatermelonDB e aprovações são intenção, não código operacional. O transporte cifrado existente no backend não prova um app offline.

`mobile/README.md` ainda diz que o código está em `TitanSystem/mobile/` e deveria ser movido; isso é orientação histórica desatualizada frente ao App.tsx na raiz. A cópia histórica mobile contém READMEs, não um aplicativo operacional oculto. Nenhuma instalação de Expo foi feita.

## 5. Frontend-web e correspondência com API

Existem exatamente seis diretórios em `frontend-web/src/modules`. Caminhos da tabela relativos a `frontend-web/`. Testes citados estão em `tests/`; sua existência não significa execução nesta auditoria.

| Módulo | Função/estado | Arquivos e testes encontrados |
|---|---|---|
| auth | REAL em login local; ESQUELETO no conjunto de cadastros/recuperação visual | `src/modules/auth/pages/LocalLogin.tsx`, `Login.tsx`, `Registration.tsx`, `RegistrationProfile112.tsx`; `localClient.test.mjs`, `registration112.test.mjs`, `registrationModel.test.mjs`. |
| catalog | REAL em catálogo/estoque/compras/recebimento pelos contratos locais | `pages/LocalCatalog.tsx`, `StockWorkspace.tsx`, `ReviewedInventory.tsx`, `Orders115.tsx`, `SupplyChain.tsx`, `Receiving.tsx`, `ReceivingCases.tsx`, `DockAgenda.tsx`; `stockWorkspace`, `reviewedInventory`, `orders115`, `receiving`, `receivingCases`, `dockAgenda` `.test.mjs`. |
| financial | REAL em pagar/agenda/fornecedor e novos recebíveis; ESQUELETO no financeiro integral | `pages/Finance116.tsx`, `PayableDocuments.tsx`, `PayablesAgenda114.tsx`, `SupplierReport117.tsx`, `LocalReceivables.tsx`; `finance116`, `payableDocuments`, `agenda114`, `suppliers117`, `receivables` `.test.mjs`. Sem comprovação de conciliação bancária/faturamento integral de venda a prazo. |
| maintenance | REAL nos contratos de manutenção interna | `pages/LocalMaintenance.tsx`, `src/core/internalMaintenance/client.mjs`; `internalMaintenance.test.mjs`. Frota também tem telas em tenant e `fleetOfficial.test.mjs`; não é app motorista. |
| pos | REAL em PDV dinheiro, caixa, carrinhos, devolução e impressão fiscal de homologação em código | `pages/LocalPointOfSale.tsx`, `OfficialCarts.tsx`, `CustomerReturns65.tsx`, `OfficialCashJournal.tsx`, `Receipt95.tsx`, `CheckoutFiscalStatus.tsx`; `localPOS`, `checkout96`, `officialCarts`, `officialReturns65`, `cashJournal`, `checkoutFiscal`, `fiscalPrint` `.test.mjs`. TEF/hardware continuam fora dessa prova. |
| tenant | REAL em cadastro/contexto/fiscal e fontes específicas; ESQUELETO nos demais workspaces | `pages/LocalCompanyRegistration.tsx`, `LocalStaff.tsx`, `LocalOwnerDashboard.tsx`, `LocalSpreadsheet.tsx`, `enterprise89/FiscalNFe55.tsx`; `companyRegistration`, `localAccess`, `ownerDashboard62`, `sheetStockSnapshot`, `sheetFinanceSnapshot`, `nfe55` `.test.mjs`. `core/hr71/attendance.ts`, `schedules.ts`, `leave.ts`, `payroll.ts` têm adaptadores `missing`, não RH operacional. |

**Chamadas a rotas inexistentes confirmadas:** `src/core/api/dashboardService.ts` pede `/api/v1/metrics/overview` e `/api/v1/pos/sales/recent`; não constam dos roteadores/inventário. Busca por `dashboardService` encontrou apenas sua declaração, sem importador: **serviço órfão**, não tela ativa comprovadamente quebrada. Nenhuma outra tela ativa com 404 foi comprovada nesta inspeção. Não foi feito ensaio de todas as URLs dinâmicas.

**Backend sem tela identificada:** não há referência frontend/desktop encontrada para `/local/v1/account/recover`, `/local/v1/account/recovery/revoke`, `/api/v1/auth/recovery/key`, `/api/v1/auth/recovery/reset` e a família nativa `/local/v1/fiscal/intents`. São APIs/CLI sem tela comprovada, não funções automaticamente ausentes no backend. O novo `/checkout/plans/:id/fiscal/danfe` tem consumidor e não entra nessa lista. Rotas indisponíveis de indicação/SPED também não provam função por existirem.

**Mapa completo, com limite explícito:** o inventário atual contém 576 pares método/caminho. O anexo registra cada um, autenticação e arquivo de cliente candidato encontrado por busca estática. Há 142 rotas com correspondência estática de caminho e 434 sem evidência nessa busca. **Essas 434 não são uma afirmação de 434 telas faltantes:** wrappers, parâmetros de ação e composição entre arquivos podem ocultar a referência; métodos também não são inferidos pelo pareamento de caminho. Para essas entradas, ausência de tela permanece NÃO COMPROVADA, e não um 404 inventado. Testes de cliente sem página importadora também não comprovam tela navegável. Essa correspondência completa em execução é uma lacuna da auditoria, discriminada para evitar conclusão falsa.

## 6. Rotas públicas, enumeração e força bruta

Sete rotas públicas nas APIs principais, conforme `docs/api/route-inventory.json`, `backend/internal/localapi/server.go` e `backend/internal/core/routes/routes.go`:

| Método/caminho | Controle/effecto |
|---|---|
| GET /local/v1/health | Sem sessão; status local. |
| POST /local/v1/login | Senha; limitador de 5 requisições/minuto/IP. |
| POST /local/v1/account/recover | Chave privada de recuperação; limitador separado de 5/min/IP. |
| GET /api/v1/saude | Sem sessão; limitador geral online. |
| POST /api/v1/auth/login | Senha, RateLimitLogin e limitador progressivo instalado por `cmd/api/main.go`. |
| POST /api/v1/auth/refresh | Cookie/token de renovação e verificação de origem; não é acesso comercial anônimo. |
| POST /api/v1/auth/recovery/reset | Chave de recuperação, validação de origem e RateLimitLogin. |

As outras 569 entradas do inventário são marcadas como autenticadas. No servidor local o grupo posterior às três públicas usa `requireSession`; a API online usa seus guardas de autenticação/usuário. Isso não prova autorização adequada para todo papel em todas as rotas.

**Outro listener, separado:** `POST /sync/v1/events`, em `backend/internal/localdb/incoming/receiver_http.go::NewSealedReceiverHTTP`, usado pelo `titan-receive`, não requer sessão humana; exige envelope cifrado/assinado, peer autorizado e revogação/escopo. Testes encontrados `TestSealedHTTPRejectsAmbiguousAndPlaintextRequests`, `TestSealedHTTPRechecksGrantAndRevocation` em `receiver_http_test.go`. Não é uma oitava API de negócio anônima. Preflight CORS não é uma ação comercial; código histórico não montado não foi contado como listener ativo.

### Login local

`localapi/server.go` aplica limiter Fiber de **5 requisições/minuto/IP**, válidas ou inválidas. A sexta recebe 429 durante a janela; não há bloqueio durável por conta comprovado. O estado é em memória: reinício, distribuição entre processos ou outro IP não compartilham necessariamente o bloqueio; NAT pode reunir vários clientes no mesmo limite.

`localdb/localauth/auth.go::Login` agora chama `core/security/unknown_password.go::RejectUnknownPassword` para identidade desconhecida, com trabalho bcrypt. Portanto o retorno antes do bcrypt descrito na auditoria antiga **foi corrigido antes desta base**. `TestHTTPLoginUniformFailureAndBruteForceLimit` compara 401/corpo para usuário existente e inexistente e comprova a sexta tentativa bloqueada, inclusive com senha correta. `TestUnknownIdentityVerifierUsesNormalBcryptCost` verifica o custo normal do verificador de desconhecido. Ambos passaram aqui. Isso cobre corpo e trabalho criptográfico; não é medição estatística de tempos nem garantia contra enumeração por todo canal.

### Recuperação local

`localapi/account_access.go::recoverOwner` valida JSON estrito e limites de corpo; `localdb/localauth/recovery.go::RecoverOwner` exige identidade, chave privada de uso único e nova senha. A chave preparada pelo proprietário tem 32 bytes aleatórios e hash armazenado, vínculo a empresa/loja/aparelho/proprietário e validade de 365 dias. Recuperação válida muda a senha com hash, consome as chaves de recuperação e revoga sessões de forma transacional, com auditoria. **Não cria outro administrador, não fornece conta mestra ao provedor e não recupera a chave mestra do cofre.** Sem chave válida, não redefine a senha. Não é envio de e-mail ou descoberta de acesso.

Falhas de verificação retornam 401 `access_verification_failed` tanto para conta existente quanto desconhecida; corpo inválido pode ter erro sintático distinto antes da consulta. Cinco tentativas por minuto/IP; sexta 429. `TestHTTPRecoveryUniformExistingAndUnknownAccount`, `TestHTTPRecoveryRateLimitAndMalformedBodies` e `TestHTTPOwnerRecoveryRequiresKeyAndPreservesLoginWithoutVerifier` passaram nesta auditoria: resposta uniforme, limite, senha preservada após falha e recuperação autorizada. Testes apenas locais/descartáveis, nenhum ataque externo.

**Achados/limites de segurança:** não há comprovação de revogação das credenciais históricas (não foram lidas nem testadas). `core/api/dashboardService.ts` registra objetos de erro Axios com `console.error`; se esse serviço órfão voltar a ser usado, pode expor metadados/configuração de requisição. É risco de leitura, não vazamento reproduzido. Nenhum segredo foi incluído neste documento. A barreira de produção não foi alterada e estes testes não a habilitam.

## 7. Estrutura TitanSystem/

Comparação de caminhos rastreados relativos, sem ler .env/chaves/bancos. “Exclusivo” significa caminho inexistente na posição equivalente da raiz, não capacidade única: funcionalidades podem ter sido reorganizadas.

| Árvore antiga → raiz | Arquivos rastreados antigos | Caminhos exclusivos | Comuns iguais / diferentes |
|---|---:|---:|---:|
| TitanSystem/backend → backend | 25 | 21 | 2 / 2 |
| TitanSystem/frontend → frontend-web | 12 | 11 | 0 / 1 |
| TitanSystem/desktop → desktop | 3 | 0 | 0 / 3 |
| TitanSystem/mobile → mobile | 6 | 0 | 5 / 1 |

Exclusivos de `TitanSystem/backend/`:
- `internal/ai_insights/README.md`
- `internal/ai_insights/produtos_parados.go`
- `internal/auth/README.md`
- `internal/auth/middleware_tentativas_login.go`
- `internal/config/README.md`
- `internal/database/README.md`
- `internal/database/sqlite.go`
- `internal/handlers/README.md`
- `internal/handlers/ai_insights/README.md`
- `internal/models/auditoria.go`
- `internal/models/models.go`
- `internal/rewards/README.md`
- `internal/rewards/constantes.go`
- `internal/routes/routes.go`
- `internal/security/README.md`
- `internal/security/chaves.go`
- `internal/security/criptografia.go`
- `internal/sync/README.md`
- `internal/websocket/README.md`
- `internal/websocket/chat.go`
- `internal/websocket/rotas.go`

Exclusivos de `TitanSystem/frontend/` (todos documentação):
- `public/README.md`
- `src/README.md`
- `src/components/README.md`
- `src/components/datagrid/README.md`
- `src/components/kanban/README.md`
- `src/components/maps/README.md`
- `src/hooks/README.md`
- `src/pages/README.md`
- `src/pages/accountant/README.md`
- `src/pages/auth/README.md`
- `src/styles/README.md`

Não é uma duplicação integral do produto atual. Não foi encontrado instalador operacional exclusivo nessa árvore. `mobile/README.md` ainda aponta explicitamente para `TitanSystem/mobile/`. Menções em regras/READMEs do backend/desktop identificam a árvore histórica. Busca por `TitanSystem/` nos scripts/instaladores rastreados não encontrou referência ativa de execução; `backend/Makefile`, `docker-compose.yml` e `desktop/scripts/build.mjs` usam as pastas da raiz. Nenhuma árvore foi removida ou movida.

## 8. Capacidades de varejo, incluindo recebíveis

| Capacidade | Estado | Evidência de implementação e teste; limite |
|---|---|---|
| Estoque | REAL no núcleo local | `backend/internal/localdb/stock`, `stockreservation`, `stocktaking`; `localapi/stock_availability_test.go::TestHTTPStockAvailabilityReserveReleaseConsumeAndNoWrites` e `TestHTTPStockAvailabilityBackupRestoresPhysicalAndActiveReservations`; `frontend-web/tests/stockWorkspace.test.mjs`, `reviewedInventory.test.mjs`. Não certifica balança ou estoque global offline. |
| Compras/reposição/recebimento | REAL no circuito local contratado | `localdb/purchases`, `replenishment`, `supplycycle`; `localapi/supply_cycle_test.go::TestSupplyCycleVerticalReceivingFinancialApprovalPartialPaymentAndReturn`, `TestSupplyCycleActualLoopbackLostReplyRecoveryWithoutSecondPOST`; `orders115`, `receiving` `.test.mjs`. Decisão manual não prova contraparte externa integrada. |
| Financeiro | REAL em pagar e novos recebíveis; ESQUELETO na abrangência integral | `localdb/payables`, `receivables/store.go`, `reconciliation.go`, `customer_order.go`; `receivables_customer_order_test.go::TestHTTPReceivablesCustomerOrderRealFlowAndRestore`, `TestHTTPReceivablesCustomerOrderConcurrentBilling`, `TestHTTPReceivablesCustomerOrderRollback`; `receivables_reconciliation_test.go` contém `TestHTTPReceivablesReconcileAbsentDurableFence`. `LocalReceivables.tsx` + `tests/receivables.test.mjs`. Fluxo HTTP/restauração e fence de ausência passaram aqui. |
| Relatórios | REAL em consultas delimitadas | `localdb/management`, `localapi/management_reports_test.go::TestManagementPositionsValuationCSVAndPermissions`, `TestManagementReportQueriesBodiesAndSessionRequired`; frontend `sheetStockSnapshot.test.mjs`, `sheetFinanceSnapshot.test.mjs`, `ownerDashboard62.test.mjs`. Não equivale a DRE contábil auditada. |
| Devolução | REAL em cenários de dinheiro/estoque | `localdb/sale/customer_return_write.go`; `localapi/customer_returns_test.go::TestHTTPCustomerReturnPartialCashStockAndOfficialClosure`, `TestHTTPCustomerReturnDamagedGoodsNeverReenterStock`, `TestHTTPCustomerReturnFailureRollsBackCashStockHistoryAndOutbox`; `tests/officialReturns65.test.mjs`. Efeito fiscal e estorno eletrônico são operações distintas. |
| Fidelidade | AUSENTE como ledger transacional; ESQUELETO de adesão | `pos/pages/Checkout91.tsx::Club91` + `core/local/clubRequest91.ts::saveClubRequest91` guardam intenção `requested_local` em IndexedDB. Não há evidência de teste de crédito/resgate/estorno autoritativo concorrente no servidor. Cadastro/consentimento não é saldo de pontos. |
| Múltiplas lojas | ESQUELETO na operação integral; REAL no isolamento examinado | `localdb/identity`/sessões vinculam empresa/loja/aparelho; `localapi/receivables_customer_order_test.go::TestHTTPReceivablesCustomerOrderAuthorizedSecondStoreCannotAdoptOrigin` testa origem entre lojas, além dos testes de escopo de estoque. Falta prova integral das filas, consolidação, transferências e troca de contexto em todos os módulos. |

**Alcance dos recebíveis novos:** origem de pedido, parcelas/valores explícitos, recebimento em dinheiro, idempotência, reconciliação autorizada, preservação de resultado incerto, auditoria, rollback e restauração têm implementações/testes. A evidência executada usa API real local e SQLite descartável; não usa mock de persistência. Não declarar concluído faturamento parcial por itens de qualquer venda, estorno financeiro de todos os meios ou integração bancária. Ver também contratos de recebíveis e `CONTINUIDADE_TITANSYSTEM.md` da base, sem alterá-los nesta tarefa.

## 9. Testes efetivamente executados nesta auditoria

Todos existentes; nenhuma suíte completa. Logs de trabalho em `/tmp` não são arquivos de entrega versionados.

1. Em `backend/`:
   `go test ./internal/localapi -run '^(TestHTTPLoginUniformFailureAndBruteForceLimit|TestHTTPRecoveryUniformExistingAndUnknownAccount|TestHTTPRecoveryRateLimitAndMalformedBodies|TestHTTPOwnerRecoveryRequiresKeyAndPreservesLoginWithoutVerifier|TestHTTPCheckoutNFCeHomologationIntegration|TestHTTPReceivablesCustomerOrderRealFlowAndRestore|TestHTTPReceivablesReconcileAbsentDurableFence)$' -v -timeout 120s -count=1`
   **7 testes passaram; pacote 4,885 s**, zero falhas. Log `/tmp/titan-audit-localapi.log`.
2. `go test ./internal/fiscalproduction -run '^TestCheckoutEncryptedNFCePrintArchiveIsolationIntegrityAndNoNetwork$' -v -timeout 120s -count=1`
   **Passou; pacote 0,575 s**. Log `/tmp/titan-audit-archive-print.log`.
3. `go test ./internal/core/security -run '^TestUnknownIdentityVerifierUsesNormalBcryptCost$' -timeout 120s -count=1`
   **Passou; 0,115 s**. Log `/tmp/titan-audit-bcrypt.log`.
4. Em `frontend-web/`:
   `node --test tests/fiscalPrint.test.mjs tests/checkoutFiscal.test.mjs tests/localCartEditing.test.mjs tests/paper106.test.mjs tests/receipt108.test.mjs tests/receivables.test.mjs`
   **68 passaram, zero falhas, zero pulados; 563,217 ms**. Log `/tmp/titan-audit-front.log`.

Não foram repetidos TypeScript/build/race/CI nem testes físicos/Windows; não constituem evidência desta auditoria. Testes fiscais locais não comprovam aceitação pela Focus/SEFAZ. Nenhuma credencial histórica foi aberta ou revogada nesta tarefa.

## 10. Backlog priorizado (15 itens)

| # | Entrega pendente | Esforço | Quem executa / pré-requisito |
|---|---|---|---|
| 1 | Comprovar revogação de credenciais históricas e inventário seguro | P | Titulares dos serviços; Codex orienta sem ler valores. Saber quais serviços/instalações usam as chaves; preservar restauração de backups. |
| 2 | Ensaiar instalação, restauração e impressão fiscal em máquinas limpas | G | Codex + executor Windows/Linux e hardware do cliente. Definir versões e equipamento; Windows não comprovado. |
| 3 | Comprovar emissão/consulta/cancelamento homologados e cupom na Focus/SEFAZ | M | Cliente, Focus e contador; emitente, token/A1/CSC e autorização. Código com teste local não substitui ensaio oficial. |
| 4 | Completar faturamento de vendas a prazo e estornos financeiros | G | Codex + decisão comercial/contratos. Definir origem, faturamento parcial, crédito e efeito de cancelamentos; preservar novos recebíveis. |
| 5 | Fechar operação multiloja em filas, relatórios e transferências | G | Codex + dono/infraestrutura. Definir topologia, autorização e regras de estoque, sem prometer consistência offline universal. |
| 6 | Integrar TEF/cartão com recuperação e conciliação reais | G | Codex + adquirente/contrato/SDK e ambiente de teste. Escolher protocolo, modalidades e responsabilidades. |
| 7 | Implementar drivers de gaveta/balança e validar leitores/impressoras | G | Codex + protocolos e equipamentos. Matriz de suporte declarada; não promessa para qualquer hardware. |
| 8 | Entregar mobile operacional com persistência e sincronização segura | G | Codex + decisão de escopo/plataforma e dispositivos. Definir comandos permitidos offline e direitos exclusivos. |
| 9 | Implementar fidelidade transacional e estornos concorrentes | G | Codex + dono. Aprovar ganho, resgate, expiração e cancelamento; intenção local não é ledger. |
| 10 | Completar mapa executável tela/cliente/rota e eliminar serviços órfãos | M | Codex; rastrear composição de URLs/importadores e autorizações, sem contar ausência de literal como rota quebrada. |
| 11 | Tela de recuperação autorizada com chave do cliente | M | Codex; usar mecanismo existente, sem administrador paralelo nem recuperação do cofre pelo provedor. |
| 12 | Fortalecer limite de tentativas para implantação multiprocesso | M | Codex + infraestrutura. Definir proxy/IP confiável, armazenamento e política; preservar respostas uniformes. |
| 13 | Ampliar conciliação bancária e relatórios com fontes auditáveis | G | Codex + contratos bancários/contador. Separar previsão, parcial, comprovante e liquidação. |
| 14 | Trocar adaptadores RH indisponíveis por contratos reais aprovados | G | Codex + dono/assessoria. Definir escopo, privacidade e obrigações; não anunciar folha/eSocial implementados. |
| 15 | Corrigir documentação histórica e redigir logs seguros de serviços órfãos | P | Codex; tarefa futura de código/documentação, preservando árvore e arquivos atuais. |

Produção fiscal permanece bloqueada; homologação oficial, TEF, hardware e instalação limpa não foram declarados concluídos.

## Anexo — todos os pares método/caminho e referências frontend candidatas

Fonte: inventário de rotas da base. “Sem evidência estática” = consumidor/tela não resolvido por esta inspeção, não prova de ausência. “Candidato” = correspondência de caminho (parâmetros normalizados), sem inferência do método HTTP nem prova de página navegável. Para as ausências confirmadas por busca dirigida, ver seção 5.

| Método/caminho | Autenticação declarada | Cliente candidato / ausência de evidência |
|---|---|---|
| `GET /local/v1/access/audit` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/access/policy` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/access/policy` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/account/password` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/account/recover` | public | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/account/recovery/revoke` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/account/sessions` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/account/sessions/revoke-others` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/accounting/cash/journal` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/accounting/cash/report` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/accounting/cash/report.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/accounting/cash/sessions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/capabilities` | session | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/cash/bags` | session | `frontend-web/src/core/cashBags/source.mjs` |
| `POST /local/v1/cash/bags` | session | `frontend-web/src/core/cashBags/source.mjs` |
| `GET /local/v1/cash/bags/:id` | session | `frontend-web/src/core/cashBags/source.mjs`, `frontend-web/src/core/cashVaults/source.mjs` |
| `POST /local/v1/cash/bags/:id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/bags/:id/dispatch` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/bags/:id/receive` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/bags/:id/reject` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/bags/operations/:id` | session | `frontend-web/src/core/cashBags/source.mjs` |
| `POST /local/v1/cash/close` | session | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/cash/current` | session | `frontend-web/src/core/local/localClient.mjs` |
| `POST /local/v1/cash/movements` | session | `frontend-web/src/core/local/cashMovement.mjs` |
| `GET /local/v1/cash/movements/operations/:id` | session | `frontend-web/src/core/local/cashMovement.mjs` |
| `POST /local/v1/cash/open` | session | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/cash/sessions/:id/custody` | session | `frontend-web/src/core/cashBags/source.mjs` |
| `GET /local/v1/cash/sessions/:id/journal` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/sessions/:id/journal.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/sessions/:id/report` | session | `frontend-web/src/core/local/official101.mjs` |
| `GET /local/v1/cash/vault-counts` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vault-counts` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vault-counts/:id` | session | `frontend-web/src/core/cashVaults/source.mjs` |
| `POST /local/v1/cash/vault-counts/:id/accept` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vault-counts/:id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vault-counts/:id/reject` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vault-counts/:id/submit` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vault-counts/operations/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vaults` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vaults` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vaults/:id` | session | `frontend-web/src/core/cashVaults/source.mjs` |
| `POST /local/v1/cash/vaults/:id/archive` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vaults/:id/deposit` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vaults/:id/statement` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vaults/:id/supply` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/cash/vaults/:id/transfer` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vaults/operations/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vaults/pending-bags` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vaults/report` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/cash/vaults/report.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/catalog/operations/:kind/:id` | session | `frontend-web/src/core/catalogManagement/source.mjs` |
| `GET /local/v1/catalog/products/:id/edit-details` | session | `frontend-web/src/core/catalogManagement/source.mjs` |
| `GET /local/v1/catalog/products/:id/edit-state` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/catalog/products/:id/production-usage` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/catalog/products/:id/state` | session | `frontend-web/src/core/catalogManagement/source.mjs` |
| `POST /local/v1/catalog/products/:id/state` | session | `frontend-web/src/core/catalogManagement/source.mjs` |
| `GET /local/v1/catalog/products/:id/state-history` | session | `frontend-web/src/core/catalogManagement/source.mjs` |
| `POST /local/v1/catalog/products/:id/update` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/catalog/search` | session | `frontend-web/src/core/local/catalogSearch.mjs`, `frontend-web/src/core/productionRead/commands.mjs`, `frontend-web/src/core/stockWorkspace/source.mjs` |
| `POST /local/v1/checkout/plans` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/checkout/plans/:id` | session | `frontend-web/src/core/local/checkoutFiscal.mjs` |
| `POST /local/v1/checkout/plans/:id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/checkout/plans/:id/finalize-cash` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/checkout/plans/:id/fiscal/reprocess` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/checkout/plans/:id/parts/:part/proofs` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/checkout/proofs` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/checkout/proofs/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/checkout/proofs/:id/accept` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/checkout/proofs/:id/reject` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/checkout/proofs/:id/revoke` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/checkout/proofs/:id/withdraw` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/checkout/proofs/operations/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/checkout/proofs/report` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/checkout/proofs/report.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/comparison-site-operations/:id` | session | `frontend-web/src/core/local/comparisonClient.mjs` |
| `GET /local/v1/comparison-sites` | session | `frontend-web/src/core/local/comparisonClient.mjs` |
| `POST /local/v1/comparison-sites` | session | `frontend-web/src/core/local/comparisonClient.mjs` |
| `POST /local/v1/finance/payable-cash/:id/return` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/finance/payable-cash/:id/return-sources` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/finance/payable-cash/operations/:id` | session | `frontend-web/src/core/payables113/client.mjs` |
| `GET /local/v1/finance/payables` | session | `frontend-web/src/core/payableDocuments/source.mjs`, `frontend-web/src/core/payables113/client.mjs` |
| `POST /local/v1/finance/payables` | session | `frontend-web/src/core/payableDocuments/source.mjs`, `frontend-web/src/core/payables113/client.mjs` |
| `GET /local/v1/finance/payables/:id` | session | `frontend-web/src/core/payableDocuments/source.mjs` |
| `POST /local/v1/finance/payables/:id/approve` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/finance/payables/:id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/finance/payables/:id/cash-payments` | session | `frontend-web/src/core/payables113/client.mjs` |
| `POST /local/v1/finance/payables/:id/cash-payments` | session | `frontend-web/src/core/payables113/client.mjs` |
| `GET /local/v1/finance/payables/:id/cash-sources` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/finance/payables/:id/installment-balances` | session | `frontend-web/src/core/payables116/source.mjs` |
| `POST /local/v1/finance/payables/:id/reject` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/finance/payables/operations/:id` | session | `frontend-web/src/core/payableDocuments/source.mjs` |
| `GET /local/v1/finance/payables/report` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/finance/payables/report.csv` | session | `frontend-web/src/core/payables114/client.mjs` |
| `GET /local/v1/finance/receiving-origins/:id` | session | `frontend-web/src/core/deliveryCycle/source.mjs` |
| `GET /local/v1/finance/receiving-return-review/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/assets` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/assets` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/assets/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/assets/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/assets/:id/availability` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/assets/:id/costs` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/assets/:id/costs.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/assets/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/assets/:id/preventive` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/expenses` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/expenses` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/expenses/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/expenses/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/expenses/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/impacts` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/impacts` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/impacts/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/impacts/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/impacts/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/income` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/income` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/income/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/income/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/income/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/notices` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/notices/:id/ack` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/operations/:id` | session | `frontend-web/src/core/fleetOfficial/commands.mjs` |
| `GET /local/v1/fleet/plans` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/plans` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/plans/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/plans/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/plans/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/profiles` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/profiles` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/profiles/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/profiles/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/profiles/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/providers` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/providers` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/providers/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/providers/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/providers/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/quotes` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/quotes` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/quotes/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/quotes/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/quotes/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/requests` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/requests` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/requests/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/requests/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/requests/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/requests/:id/workshop` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/requests/:id/workshop.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/trips` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/trips` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/trips/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/trips/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/trips/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/inspections` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/inspections` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/inspections/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/inspections/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/inspections/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/labor` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/labor` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/labor/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/labor/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/labor/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/operations/:id` | session | `frontend-web/src/core/fleetOfficial/workshop.mjs` |
| `GET /local/v1/fleet/workshop/requirements` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/requirements` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/requirements/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/requirements/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/requirements/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/templates` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/templates` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/templates/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fleet/workshop/templates/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fleet/workshop/templates/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/health` | public | `frontend-web/src/core/context74/health.ts` |
| `GET /local/v1/locations` | session | `frontend-web/src/core/deliveryCycle/source.mjs`, `frontend-web/src/core/local/localClient.mjs`, `frontend-web/src/core/productionRead/source.mjs`, `frontend-web/src/core/sheets70/stockSnapshot.mjs`, `frontend-web/src/core/stockWorkspace/source.mjs` |
| `POST /local/v1/locations` | session | `frontend-web/src/core/deliveryCycle/source.mjs`, `frontend-web/src/core/local/localClient.mjs`, `frontend-web/src/core/productionRead/source.mjs`, `frontend-web/src/core/sheets70/stockSnapshot.mjs`, `frontend-web/src/core/stockWorkspace/source.mjs` |
| `POST /local/v1/login` | public | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/logistics/appointments` | session | `frontend-web/src/core/dockAgenda/source.mjs` |
| `POST /local/v1/logistics/appointments` | session | `frontend-web/src/core/dockAgenda/source.mjs` |
| `GET /local/v1/logistics/appointments.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/appointments/:id` | session | `frontend-web/src/core/dockAgenda/source.mjs` |
| `POST /local/v1/logistics/appointments/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/appointments/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/docks` | session | `frontend-web/src/core/dockAgenda/source.mjs` |
| `POST /local/v1/logistics/docks` | session | `frontend-web/src/core/dockAgenda/source.mjs` |
| `GET /local/v1/logistics/docks/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/insights/availability` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/insights/pending` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/insights/pending.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/insights/suppliers/performance` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/insights/suppliers/performance.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/insights/utilization` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/insights/utilization.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/logistics/occupancy` | session | `frontend-web/src/core/dockAgenda/source.mjs` |
| `GET /local/v1/logistics/operations/:id` | session | `frontend-web/src/core/dockAgenda/source.mjs` |
| `POST /local/v1/logout` | session | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/maintenance/assets` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/assets` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/assets/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/assets/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/assets/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/candidates` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/classifications` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/classifications` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/classifications/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/classifications/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/classifications/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/operations/:id` | session | `frontend-web/src/core/internalMaintenance/client.mjs` |
| `GET /local/v1/maintenance/queue` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/specialties` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/specialties` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/specialties/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/specialties/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/specialties/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/technicians` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/technicians` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/technicians/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/maintenance/technicians/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/maintenance/technicians/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/management/inventories/:id/variance` | session | `frontend-web/src/core/stockReports/source.mjs` |
| `POST /local/v1/management/production/recipe-versions/:id/cost-scenario` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `POST /local/v1/management/production/simultaneous-plan` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/management/purchases/receiving-board` | session | `frontend-web/src/core/receivingOverview/source.mjs` |
| `GET /local/v1/management/purchases/receiving-board.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/management/replenishment/preview` | session | `frontend-web/src/core/restockPreview/source.mjs` |
| `GET /local/v1/management/sales/products` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/management/sales/products.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/management/stock/positions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/management/stock/positions.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/management/stock/valuation` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/management/stock/valuation.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/me` | session | `frontend-web/src/core/local/localClient.mjs` |
| `POST /local/v1/module-contracts` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/pos/cart-operations/:id` | session | `frontend-web/src/core/officialCarts/source.mjs` |
| `GET /local/v1/pos/carts` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/pos/carts` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/pos/carts/:id` | session | `frontend-web/src/core/officialCarts/source.mjs` |
| `POST /local/v1/pos/carts/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/pos/carts/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/pos/carts/:id/reconcile` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/pos/carts/report` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/pos/carts/report.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/capacity` | session | `frontend-web/src/core/productionRead/source.mjs` |
| `POST /local/v1/production/capacity/alternatives` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/losses` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/losses/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/losses/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/losses/void` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/lot-search` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/lots` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/lots/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/lots/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/lots/:id/quality` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/lots/:id/quality/history` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `POST /local/v1/production/lots/void` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/material-reservations` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/material-reservations/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/material-reservations/state` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/operations/:kind/:id` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/order-search` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/orders` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `POST /local/v1/production/orders` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/orders/:id` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/orders/:id/history` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/orders/:id/material-cost` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/orders/:id/stages` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/orders/:id/stages/history` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/orders/:id/trace` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `POST /local/v1/production/orders/state` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/quality-reviews` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/recipe-state` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/recipe-versions` | session | `frontend-web/src/core/productionRead/source.mjs` |
| `POST /local/v1/production/recipe-versions` | session | `frontend-web/src/core/productionRead/source.mjs` |
| `GET /local/v1/production/recipe-versions/:id` | session | `frontend-web/src/core/productionRead/source.mjs` |
| `GET /local/v1/production/recipe-versions/:id/material-cost` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `GET /local/v1/production/recipes/:id/state` | session | `frontend-web/src/core/productionRead/commands.mjs` |
| `POST /local/v1/production/results` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/results/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/results/:id/losses` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/production/results/:id/lots` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/stage-plans` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/production/stages/state` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/products` | session | `frontend-web/src/core/local/localClient.mjs` |
| `POST /local/v1/products` | session | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/purchase-approvals` | session | `frontend-web/src/core/local/purchaseClient.mjs` |
| `GET /local/v1/purchase-cancellation-operations/:id/receipt` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-cycle-operations/:id` | session | `frontend-web/src/core/deliveryCycle/source.mjs`, `frontend-web/src/core/supplyDecisions/source.mjs` |
| `GET /local/v1/purchase-deliveries/:id` | session | `frontend-web/src/core/deliveryCycle/source.mjs` |
| `POST /local/v1/purchase-deliveries/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-deliveries/:id/history` | session | `frontend-web/src/core/deliveryCycle/source.mjs` |
| `POST /local/v1/purchase-deliveries/:id/payable` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-deliveries/:id/receive` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-deliveries/:id/reject` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-order-search` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-orders` | session | `frontend-web/src/core/local/purchaseClient.mjs` |
| `POST /local/v1/purchase-orders` | session | `frontend-web/src/core/local/purchaseClient.mjs` |
| `GET /local/v1/purchase-orders/:id` | session | `frontend-web/src/core/local/purchaseClient.mjs` |
| `POST /local/v1/purchase-orders/:id/authorize-receiving` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/deliveries` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/receipts` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/receipts/:receipt_id/void` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/receiving-cases` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-orders/:id/receiving-history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-orders/:id/receiving-overview` | session | `frontend-web/src/core/receivingOverview/source.mjs` |
| `GET /local/v1/purchase-orders/:id/receiving-overview.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-orders/:id/receiving-trace` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/rejections` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/supplier-decisions` | session | `frontend-web/src/core/supplyDecisions/source.mjs` |
| `GET /local/v1/purchase-orders/:id/supply-cycle` | session | `frontend-web/src/core/deliveryCycle/source.mjs`, `frontend-web/src/core/supplyDecisions/source.mjs` |
| `GET /local/v1/purchase-orders/:id/trace` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-receiving-case-operations/:id` | session | `frontend-web/src/core/receivingCases/source.mjs` |
| `GET /local/v1/purchase-receiving-cases` | session | `frontend-web/src/core/receivingCases/source.mjs` |
| `GET /local/v1/purchase-receiving-cases/:id` | session | `frontend-web/src/core/receivingCases/source.mjs` |
| `POST /local/v1/purchase-receiving-cases/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-receiving-cases/:id/history` | session | `frontend-web/src/core/receivingCases/source.mjs` |
| `POST /local/v1/purchase-receiving-cases/:id/returns` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-receiving/operations/:kind/:id/receipt` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-supplier-operations/:id/receipt` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-supplier-return-operations/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-supplier-returns` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-supplier-returns/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-supplier-returns/:id/actions` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-supplier-returns/:id/dispatch` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-supplier-returns/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-supplier-returns/:id/recover` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-suppliers` | session | `frontend-web/src/core/local/purchaseClient.mjs`, `frontend-web/src/core/payableDocuments/source.mjs` |
| `POST /local/v1/purchase-suppliers` | session | `frontend-web/src/core/local/purchaseClient.mjs`, `frontend-web/src/core/payableDocuments/source.mjs` |
| `GET /local/v1/purchase-suppliers/:id/edit-state` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/purchase-suppliers/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-suppliers/:id/update` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/replenishment/operations/:kind/:id` | session | `frontend-web/src/core/local/replenishmentClient.mjs` |
| `POST /local/v1/replenishment/policies` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/replenishment/products` | session | `frontend-web/src/core/local/replenishmentClient.mjs` |
| `POST /local/v1/replenishment/reviews` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/replenishment/suggestions` | session | `frontend-web/src/core/local/replenishmentClient.mjs` |
| `POST /local/v1/replenishment/suggestions` | session | `frontend-web/src/core/local/replenishmentClient.mjs` |
| `GET /local/v1/sales` | session | `frontend-web/src/core/local/localClient.mjs` |
| `POST /local/v1/sales` | session | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/sales/:id` | session | `frontend-web/src/core/local/localClient.mjs` |
| `POST /local/v1/sales/:id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/sales/quote` | session | `frontend-web/src/core/local/localClient.mjs` |
| `GET /local/v1/staff` | session | `frontend-web/src/core/local/localClient.mjs`, `frontend-web/src/core/productionRead/commands.mjs` |
| `POST /local/v1/staff` | session | `frontend-web/src/core/local/localClient.mjs`, `frontend-web/src/core/productionRead/commands.mjs` |
| `POST /local/v1/staff/:id/password-reset` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/staff/:id/sessions/revoke` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/stock/availability` | session | `frontend-web/src/core/stockWorkspace/source.mjs` |
| `GET /local/v1/stock/balance` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/stock/inventories` | session | `frontend-web/src/core/reviewedInventory/source.mjs` |
| `POST /local/v1/stock/inventories` | session | `frontend-web/src/core/reviewedInventory/source.mjs` |
| `GET /local/v1/stock/inventories/:id` | session | `frontend-web/src/core/reviewedInventory/source.mjs` |
| `POST /local/v1/stock/inventories/:id/counts` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/stock/inventories/:id/history` | session | `frontend-web/src/core/reviewedInventory/source.mjs` |
| `GET /local/v1/stock/inventories/:id/report.csv` | session | `frontend-web/src/core/reviewedInventory/source.mjs` |
| `POST /local/v1/stock/inventories/:id/state` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/stock/inventory-operations/:id` | session | `frontend-web/src/core/reviewedInventory/source.mjs` |
| `POST /local/v1/stock/operations` | session | `frontend-web/src/core/local/localClient.mjs`, `frontend-web/src/core/stockWorkspace/source.mjs` |
| `GET /local/v1/stock/operations/:id/receipt` | session | `frontend-web/src/core/stockWorkspace/source.mjs` |
| `GET /local/v1/stock/operations/history` | session | `frontend-web/src/core/stockWorkspace/source.mjs` |
| `GET /local/v1/stock/reservations` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/system/info` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/accounting/sped/request` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/accounting/sped/status/:job_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/analises/produtos-parados` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/login` | public | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/logout` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/password` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/recovery/key` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/recovery/reset` | public | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/refresh` | public | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/auth/sessions` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/sessions/:id/revoke` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/auth/sessions/revoke-others` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/adjustments` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/adjustments/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/adjustments/apply` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/adjustments/exports/csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/adjustments/exports/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/adjustments/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/barcodes/batches/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/barcodes/batches/apply` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/barcodes/batches/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/barcodes/import/apply` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/barcodes/import/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/barcodes/import/template` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/barcodes/lookup` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/barcodes/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/batches` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/batches/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/batches/:operation_id/import` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/batches/:operation_id/undo` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/batches/apply` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/batches/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/cost-history/entries` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/cost-history/entries/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/cost-history/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/cost-history/products/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/cost-history/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/costs/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/creations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/exports/csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/exports/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/imports/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/imports/apply` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/imports/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/margins/below-target` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/margins/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/margins/missing-cost` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/margins/negative` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/margins/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/quality/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/quality/no-active-gtin` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/quality/no-description` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/quality/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/quality/zero-stock` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/items` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/products/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/replenishment/products/:id/simulate` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/proposals` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/replenishment/proposals` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/proposals/:proposal_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/replenishment/proposals/:proposal_id/approve` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/replenishment/proposals/:proposal_id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/proposals/:proposal_id/check` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/proposals/:proposal_id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/proposals/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/proposals/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/replenishment/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/stock-settings/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/stock/below-minimum` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/stock/coverage` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/stock/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/stock/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/stock/zero` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/catalog/undos/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/undos/apply` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/catalog/undos/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/checks/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/checks/inactive` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/checks/price-changed` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/checks/suggestions/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/checks/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/report/approved` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/report/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/report/pending` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/report/rejected` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/report/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/reviews/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/discounts/suggest` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/suggestions` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/suggestions/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/discounts/suggestions/:id/approve` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/discounts/suggestions/:id/reject` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/discounts/suggestions/:id/review` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/discounts/suggestions/:id/reviews` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/active` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/barcodes` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/barcodes` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/barcodes/:barcode_id/active` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/barcodes/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/barcodes/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/cost` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/cost` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/cost/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/creation` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/details` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/margin` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/price` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/stock-settings` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/stock-settings` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/:id/stock-settings/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/produtos/:id/stock-settings/preview` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/produtos/search` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/purchase-documents` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchase-documents/:document_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/purchase-documents/:document_id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchase-documents/:document_id/download.json` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchase-documents/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchases` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/purchases` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchases/:purchase_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/purchases/:purchase_id/approve` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/purchases/:purchase_id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchases/:purchase_id/check` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchases/:purchase_id/history` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchases/export.csv` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchases/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/purchases/summary` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/recompensas/indicacoes` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/recompensas/indicacoes/recompensar` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/recompensas/indicacoes/saldo` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/saude` | public | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/supplier-offers` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/supplier-offers` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/supplier-offers/:offer_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `PUT /api/v1/supplier-offers/:offer_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/supplier-offers/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/suppliers` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /api/v1/suppliers` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/suppliers/:supplier_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `PUT /api/v1/suppliers/:supplier_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /api/v1/suppliers/operations/:operation_id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/company-trade/identity` | session | `frontend-web/src/core/companyTrade66/source.mjs` |
| `POST /local/v1/company-trade/peers` | session | `frontend-web/src/core/companyTrade66/source.mjs` |
| `GET /local/v1/company-trade/peers` | session | `frontend-web/src/core/companyTrade66/source.mjs` |
| `POST /local/v1/company-trade/peers/:id/revoke` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/company-trade/export` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/company-trade/orders/import` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/company-trade/orders/:id/decision` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/purchase-orders/:id/company-trade/decision` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/company-trade/messages` | session | `frontend-web/src/core/companyTrade66/source.mjs` |
| `GET /local/v1/company-trade/messages/:id` | session | `frontend-web/src/core/companyTrade66/source.mjs` |
| `GET /local/v1/company-trade/operations/:id` | session | `frontend-web/src/core/companyTrade66/source.mjs` |
| `GET /local/v1/company-trade/recovery` | session | `frontend-web/src/core/companyTrade66/source.mjs` |
| `POST /local/v1/company-trade/recovery/review` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/sale-return-operations/:id` | session | `frontend-web/src/core/customerReturns/source.mjs` |
| `GET /local/v1/sales/:id/returns` | session | `frontend-web/src/core/customerReturns/source.mjs` |
| `POST /local/v1/sales/:id/returns` | session | `frontend-web/src/core/customerReturns/source.mjs` |
| `POST /local/v1/sales/:id/returns/quote` | session | `frontend-web/src/core/customerReturns/source.mjs` |
| `POST /local/v1/sales/:id/returns/cash-session` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/sales/:id/returns/cash-session/quote` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/capabilities` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `GET /local/v1/fiscal/requirements/:uf` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `GET /local/v1/fiscal/profiles` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `POST /local/v1/fiscal/profiles` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `GET /local/v1/fiscal/profiles/:id` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `POST /local/v1/fiscal/intents` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/intents` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/intents/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/operations/:id/receipt` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/intents/:id/danfe` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/intents/:id/production` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/intents/:id/production/approve` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/intents/:id/production/emit` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/intents/:id/production/prepare` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/intents/:id/production/query` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/intents/:id/production/xml` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/intents/:id/production/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/intents/:id/production/cancellation` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/intents/:id/production/cancellation/xml` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/tax-policies` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/tax-policies/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/tax-quotes` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/tax-quotes/:id` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/onboarding` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/fiscal/onboarding/:id` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `GET /local/v1/fiscal/onboarding/:id/provider` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `POST /local/v1/fiscal/onboarding/:id/provider` | session | `frontend-web/src/core/fiscalCustomer/client.mjs` |
| `POST /local/v1/fiscal/nfe55/sales/:id/cancel` | session | `frontend-web/src/core/fiscalCustomer/nfe55.mjs` |
| `GET /local/v1/fiscal/nfe55/sales/:id/danfe` | session | `frontend-web/src/core/fiscalCustomer/nfe55.mjs` |
| `POST /local/v1/fiscal/nfe55/sales/:id/emit` | session | `frontend-web/src/core/fiscalCustomer/nfe55.mjs` |
| `POST /local/v1/fiscal/nfe55/sales/:id/preview` | session | `frontend-web/src/core/fiscalCustomer/nfe55.mjs` |
| `POST /local/v1/fiscal/nfe55/sales/:id/query` | session | `frontend-web/src/core/fiscalCustomer/nfe55.mjs` |
| `GET /local/v1/company/registration` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/company/registration` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/fiscal/nfe55/sales/:id/correction` | session | `frontend-web/src/core/fiscalCustomer/nfe55.mjs` |
| `GET /local/v1/fiscal/accounting-export` | session | `frontend-web/src/modules/tenant/pages/enterprise89/FiscalAccountingExport.tsx` |
| `POST /local/v1/fiscal/tax-tables/import` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/checkout/plans/:id/fiscal/danfe` | session | `frontend-web/src/core/local/checkoutFiscal.mjs` |
| `GET /local/v1/finance/receivables` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/finance/receivables` | session | Sem evidência estática; não concluir ausência de tela. |
| `GET /local/v1/finance/receivables/operations/:id` | session | `frontend-web/src/core/receivables/client.mjs` |
| `GET /local/v1/finance/receivables/:id` | session | `frontend-web/src/core/receivables/client.mjs` |
| `POST /local/v1/finance/receivables/:id/receive-cash` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/finance/receivables/:id/cancel` | session | Sem evidência estática; não concluir ausência de tela. |
| `POST /local/v1/finance/receivables/operations/:id/reconcile` | session | `frontend-web/src/core/receivables/client.mjs` |
| `GET /local/v1/finance/receivables/customer-orders/:id` | session | `frontend-web/src/core/receivables/client.mjs` |
