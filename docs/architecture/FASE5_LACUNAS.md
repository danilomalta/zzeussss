# Fase 5 — fluxo do cliente em homologação

Base: `feat/ci-green`, `1bf33ba1fb9628c198750fed708c2d710d0d4bed`. Reauditoria dirigida das decisões do AGENTS e da auditoria de 30/10, após fases 1–4. PRONTO significa capacidade no recorte indicado com fonte e teste, não aceitação oficial nem implantação já executada para um cliente.

| Área | Estado na base | Evidência atual |
|---|---|---|
| Pré-cadastro manual | PRONTO | `backend/internal/localdb/companyregistration/store.go`; `TestRegistrationBeforeFiscalOrContractAndAfterReopen`, `TestHTTPCompanyRegistrationManualStagesRecoveryAndIsolation`. Inicialização privada: `cmd/titan-local/main.go`, `TestInitCreatesPrivateKeyAndRejectsSecondRun`. |
| Cadastro completo central | PARCIAL | `LocalCompanyRegistration.tsx`, `companyregistration/store.go`; mesmos testes de persistência/retomada. Campos centrais manuais existem; caminhos para os demais cadastros estão dispersos e o atalho de funcionários não usa sua rota operacional. |
| Funcionários e permissões fiscais | PRONTO | `localapi/staff.go`, `TestHTTPStaffCreationRepeatsWithoutLeakingCredentials`; `fiscal_nfe55_test.go/TestHTTPNFe55OwnerContractPrivacyAndStates`: emissão 55 somente proprietário; manager/accountant/cashier recusados. Não equivale a RH completo. |
| Meios fiscais | PARCIAL | `fiscalproduction/customer_credentials.go`, `customer_nfe55.go`, `localapi/fiscal_nfe55.go`, `FiscalProviderCredentials.tsx`; `TestHTTPFiscalCustomerCredentialsOwnerRecoveryAndNoDisclosure`, `TestHTTPNFe55OwnerContractPrivacyAndStates`, `TestVaultOwnA1AndMissingKey`. Perfil/emitente, token/CSC cifrados e diagnóstico existem. A1 do cliente na Focus, credenciamento e aceite real são externos. Retomada pela tela depende de conhecer o ID do perfil. |
| Cofre e chaves do cliente | PRONTO | `fiscalproduction/vault_lifecycle.go`, `cmd/titan-fiscal/vault.go`; `TestCustomerMasterKeyGenerationBackupRotationAndStaleWriter`, `TestIndependentCustomerKeysCannotOpenOtherVault`, `TestVaultCommandsKeepMasterKeyOutOfStandardOutput`. Geração, exportação privada, rotação, recuperação com backup/chave correspondente e recusa sem chave. Aviso de perda em cinco idiomas no CLI e cadastro. Sem recuperação pelo provedor. |
| Local, servidor e nuvem | PARCIAL | `deployment/profile.go`, `cmd/titan-local/serve.go`, `desktop/setup/`; `TestCustomerCloudVersion2ExplicitDestinationAndNoPublicDefault`, `TestCustomerCloudVersion2RequiresRealTLSCertificate`, `TestServerHTTPSLoginReadRevocationAndNetworkAdmission`. Destino SQLite explícito por `--db`; servidor/cliente-nuvem exigem configuração e TLS do cliente. Instalador Linux descartável passou na base; Windows limpo não comprovado aqui. Não existe provisionamento automático de infraestrutura. |
| Isolamento de clientes | PRONTO | `customer_credentials.go`, `companyregistration/store.go`; `TestCustomerCredentialsTenantIsolationAndConcurrentRotation`, `TestIndependentCustomerKeysCannotOpenOtherVault`, `TestHTTPCompanyRegistrationManualStagesRecoveryAndIsolation`. Sem conta mestra do fornecedor. Administrador do host do cliente mantém controle do próprio processo. |

## Lacunas CÓDIGO desta entrega (duas, por impacto)

1. **CÓDIGO C1 — localizar/retomar perfis fiscais:** lista paginada da revisão corrente por empresa/loja, somente proprietário com sessão e contrato fiscal válidos; seleção pela tela e recusa de respostas de sessão antiga. Não retorna segredos e não transmite à Focus.
2. **CÓDIGO C2 — primeiros passos do proprietário:** painel dentro do cadastro central com etapas mínimas/completas derivadas do registro persistido, acessos corretos ao fiscal e funcionários, e instruções privadas de instalação/chaves/backup. Etapas manuais e validações oficiais nunca aparecem como concluídas automaticamente. Cinco idiomas; sem consulta automática ou armazenamento de dados pessoais/segredos no navegador.

## Lacunas EXTERNAS e limites

- **EXTERNA:** consulta automática de CNPJ/fonte e consentimento para enviar dados a terceiro: decisão do dono; somente digitação manual nesta entrega.
- **EXTERNA:** contratação/conta Focus, emitente habilitado, A1 e CSC do próprio cliente, UF/credenciamento, token privado e emissão real de homologação. Nenhum ensaio real será executado aqui.
- **EXTERNA:** classificação tributária e revisão do contador; dados fiscais corretos fornecidos pelo cliente. Nada de imposto inventado.
- **EXTERNA:** escolha do destino, host/servidor/nuvem do cliente, TLS, redes autorizadas, configuração privada do processo e contrato/licença de módulos. Sem endereço central ou provisionamento do provedor.
- **EXTERNA:** custódia e cópia externa das próprias chaves e backups; perda de todas as cópias não tem recuperação pelo fornecedor. Testar a restauração da instalação concreta é responsabilidade operacional do cliente.
- **EXTERNA:** executor Windows/máquina limpa para comprovar o instalador nessa plataforma; não confundir build com instalação observada.
- Produção fiscal, A3, SPED/PVA e NFS-e continuam fora do recorte; nenhuma trava será alterada. Uma configuração válida e testes locais não são autorização oficial.

## Entregas de código

- C1 implementada: `localdb/fiscal/profiles.go`, `localapi/fiscal_profiles.go`, `FiscalProfilePicker.tsx`; `TestHTTPFiscalProfilesLatestPaginationIsolationAndNoAuthority`, `TestHTTPFiscalProfilesRejectOverridesRolesExpiredContractAndSession`, `tests/fiscalProfiles.test.mjs`. Paginação por ID e revisão corrente; sem total fictício, sem armazenamento no navegador. O alias OpenAPI de onboarding continua exclusivamente POST.

## Validação

Pendente nesta branch: testes novos; após cada lacuna, cofre/rotação `-count=20`, contratos e suíte frontend. Ao final, uma execução completa de aceitação com PostgreSQL/CLIs/bridges e verificador, e frontend completo; comparação com a base verde `feat/ci-green`. Nenhum teste existente será enfraquecido.
