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

- C2 implementada: `CompanySetupSteps.tsx`, `companyRegistration/journey.mjs`, `tests/companySetup.test.mjs`, contrato de navegação `docs/api/customer-installation-flow.md`. Progresso apenas do registro persistido; roteiros de infraestrutura/chaves não são execução nem prova de backup. Acesso a funcionários corrigido para `/local/staff`; nenhuma permissão ampliada e nenhuma rota HTTP nova.

## Regressão e reversão delimitada

O commit inicial C1 `f6ee2b5` passou cofre/rotação 20 vezes e contratos, mas o teste existente `enterprise89.test.mjs` identificou namespace de traduções incorreto no novo picker. Foi revertido sozinho em `2dba972`. A reapresentação usa o namespace fiscal `fw`, preservando o teste, os cinco idiomas e as demais mudanças da unidade.

## Estado após as duas entregas

- Pré-cadastro e cadastro central manual: PRONTO no recorte dos campos centrais e da retomada por API; agora o roteiro reúne as etapas e os acessos diretos. `CompanySetupSteps.tsx`, `LocalCompanyRegistration.tsx`, `TestHTTPCompanyRegistrationManualStagesRecoveryAndIsolation`, `tests/companySetup.test.mjs`. Não significa RH completo nem preenchimento automático de todos os módulos.
- Funcionários/permissões, cofre por cliente e isolamento: permanecem PRONTO nos recortes e evidências da tabela. O ciclo de chaves é operacional pelo CLI existente, não por recebimento de chave no navegador.
- Meios fiscais e implantação permanecem PARCIAL pela comprovação externa delimitada acima. A descoberta de perfis deixou de depender de um ID anotado, mas não autoriza uma nota nem provisiona infraestrutura.

## Validação executada — 02/11/2026

- C1 corrigida `e83adfc`: cofre/rotação `-count=20 -timeout 120s` passaram (fiscalproduction 61,733s; CLI 0,424s), testes de contratos e mapa de 564 rotas passaram; frontend 2.543/2.543, TypeScript, testes Go novos e pacote localdb/fiscal passaram.
- C2 `3366b9a`: cofre/rotação com as mesmas 20 repetições passaram (58,999s; CLI 0,613s), contratos passaram e frontend pós-commit 2.547/2.547. Testes existentes preservados; nenhuma regressão em C2.
- Final: uma execução de `go test ./... -count=1 -timeout=30m -json`, com PostgreSQL 17 descartável, os oito executáveis compilados e as variáveis/bridges do workflow; seguida de `tools/verify_backend_acceptance.py` sem alterações. PASS em 721,11s: 1.619 testes principais, 3.741 incluindo subtestes, 25 integrações obrigatórias aprovadas.
- Os quatro pulos opcionais do workflow são `TestFocusSandboxRealNFCe` (sem token/CNPJ), `TestMigrationCrashChild` e `TestPilotCashProcess` (auxiliares chamados pelos respectivos pais), `TestPilotCashScreenFlow` (ensaio visual opt-in). Nenhuma integração obrigatória pulada. Nenhuma emissão real executada.
- Frontend final: 2.547 testes aprovados, zero falhas/pulos, TypeScript e `npm run build` passaram. O estágio compilado do desktop também passou (37,25s); não é prova de instalação em máquina limpa nem atualização automática da instalação do cliente.
- Comparação com `feat/ci-green` / `1bf33ba`: ZERO falhas novas na validação final. A regressão intermediária foi revertida e corrigida sem enfraquecer testes. Não houve mudança em cofre, rotação, travas de produção, `.github/` ou `tools/`, nem migração nova ou esquema numérico fixo.
- Logs locais preservados em `/tmp/titan-fase5-final-wrzuh2sa/` e `/tmp/titan-fase5-c*-*.log`; não publicados com dados de ambiente. CI remoto e aceite oficial da Focus não foram aferidos.
