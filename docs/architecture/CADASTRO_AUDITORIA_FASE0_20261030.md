# Auditoria única — cadastro e fiscal — 30/10/2026

Base conferida: `feat/fiscal-customer-credentials`, `53d5968fb6ff540ec0ef04d470db278a369823d2`. Árvore limpa. Inspeção por ls, rg, nomes de testes e trechos necessários; não é certificação legal nem execução dos testes. PRONTO abaixo tem escopo estrito; PARCIAL tem evidência de componentes mas não da área inteira; AUSENTE significa falta de evidência da capacidade solicitada.

| Área | Estado | Arquivo e evidência localizada |
|---|---|---|
| Pré-cadastro | PARCIAL | `localdb/localsetup/setup.go`, `TestInitializeEmptyDatabaseAndLogin`; `Registration.tsx` apenas calcula proposta, sem POST de criação. |
| Cadastro completo | PARCIAL | `enterprise89/WorkAreas.tsx`, `fiscalproduction/customer_credentials.go`; falta cadastro central completo persistido com retomada. |
| Funcionários (cadastro/acesso) | PRONTO | `localapi/staff.go`, `TestHTTPStaffCreationRepeatsWithoutLeakingCredentials`, `TestHTTPStaffMissingModuleAndFailedAuditCannotCreateMember`; não significa RH completo. |
| Meios fiscais | PARCIAL | `localdb/fiscal/store.go`, `fiscalproduction/customer_credentials.go`; `TestHTTPFiscalCustomerCredentialsOwnerRecoveryAndNoDisclosure`; Focus token/CSC e perfil, sem certificado autoatendido Own. |
| Cofre/chaves por cliente | PARCIAL | `fiscalproduction/secret_vault.go`, `TestVaultEncryptedIsolationAndNoDisclosure`; AES-GCM/AAD por tenant, porém raiz FISCAL_MASTER_KEY compartilhada pelo processo; sem ciclo completo geração/backup/rotação. |
| Local/servidor/nuvem | PARCIAL | `deployment/profile.go`, `TestModesAndOfflineResponsibilities`, `TestTLSRejectsExpiredWrongSANKeyAndPermissions`; local/servidor disponíveis, cloud/hybrid recusados no núcleo. Desktop configura conexão HTTPS remota. |
| NFC-e | PARCIAL | `fiscalproduction/focus.go`, `TestHTTPCheckoutNFCeHomologationIntegration`, `TestFocusSandboxRealNFCe` opt-in; teste real não executado. |
| NF-e 55 | PARCIAL | `fiscalproduction/runtime.go`, `fiscalcore/issuance.go`; transporte próprio existe, adapter Focus restrito a 65. |
| Cancelamento fiscal | PARCIAL | `fiscalproduction/focus.go`, `TestFocusFaultRejectedMalformedAndCancellationReasons`; Own tem `TestFiscalCancellationMTLSOriginalJournalAndRecovery`; não comprova aceite real. |
| Inutilização | PARCIAL | `fiscalcore/invalidation.go`, `TestFiscalInvalidationMTLSJournalProcessAndOfflineRecovery`; falta no adapter Focus. |
| Contingência | AUSENTE | Nenhuma evidência de fluxo operacional completo encontrada; não contar modos XML rejeitados como entrega. |
| DANFE | PARCIAL | `fiscalcore/danfe55.go`, `TestFiscalDANFE55ExactArchiveAndBarcode`; sem recuperação de documento pelo adapter Focus. |
| Tributação por regime/UF | PARCIAL | `fiscalcore/tax_policy.go`, `TestTaxPolicyValidationAndAllUFContexts`, `TestGoodsTaxClassificationsOfficialXSDAndSignature`; não comprova cobertura legal de todas as combinações. |
| SPED/PVA real | AUSENTE | `modules/financial/usecase/sped_service.go`, `TestSpedNeverFabricatesCompletedJobWithoutImplementation`: recusa, não SPED operacional. |
| NFS-e produção | AUSENTE | Componentes DPS/XSD em fiscalcore não comprovam serviço de produção; `TestFiscalDPSOfficialHomologProfileBuildSignAndPack` é componente de homologação. |

## Segurança — achados e limites

- S1: `secret_vault.go/OpenFiscalSecretVault` usa uma chave mestra do ambiente para todo o arquivo; AAD impede troca de linhas, mas conhecer a chave raiz e os tenants permite descriptografar seus registros. Falta isolamento de chaves por cliente, não apenas de consultas.
- S2: quem controla o processo/host e a chave mestra pode ler memória/cofre. A implantação deve ficar sob controle do cliente; criptografia não torna um administrador de infraestrutura incapaz de ler um segredo em uso. Nenhuma conta mestra de suporte/backdoor localizada pelas buscas.
- S3: `git log --all --full-history --name-only -- '*.env'` confirma `backend/.env` e `frontend-web/.env` no histórico em commits antigos. Valores não foram lidos. Revogação anterior não comprovada; não reescrever histórico nem afirmar rotação de credenciais reais.
- Endereços encontrados: Focus homologação (transporte fiscal explícito); IBGE em CSP (permissão, não prova de envio), consultas externas opcionais de comparação e contato institucional `titantecnology.com.br`/WhatsApp (navegação, não prova de acesso a dados). Não localizado servidor central de custódia de chaves/dados nas buscas realizadas. Dependências, arquivos de segredos e históricos de valores não foram abertos.
- Telemetria: referências em `localapi/server.go` e `cmd/titan-receive/main.go` são comentários; não encontrada integração ativa Sentry/Datadog/OpenTelemetry/PostHog nas fontes consultadas. Nenhum log de segredo fiscal comprovado; isto não certifica ausência universal.

## Continuação autorizada

Pré-cadastro mínimo proposto: nome da empresa e responsável, nome da estação e credencial inicial definida privadamente pelo cliente (mecanismo localsetup existente). CNPJ/IE/UF/CRT/provedor/ambiente entram no cadastro completo; não são necessários para entrar e conhecer o ERP. Sem consulta externa automática nesta etapa. Preservar licenciamento real e permissões: cadastro não contrata módulos nem concede acesso fiscal automaticamente.

Bloqueios externos delimitados pelo solicitante: SPED/PVA, NFS-e, A3, validação tributária por contador, produção fiscal e ensaio Focus real sem token do cliente. Não impedem desenvolver as etapas independentes. Não houve execução de testes nem leitura de valores .env nesta auditoria.
