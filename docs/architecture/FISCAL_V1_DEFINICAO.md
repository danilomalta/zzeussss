# Fiscal v1 — definição de pronto

Data: 05/11/2026. Base consolidada: `feat/backend-p16b`, originada de
`origin/feat/backend-p16` (`45ad65a`). Este checklist é o recorte de aceite;
não é uma declaração de conclusão do fiscal brasileiro inteiro.

## Checklist obrigatório

FEITO abaixo significa apenas o alcance expressamente indicado. Teste com
autoridade descartável ou `httptest` nunca prova aceitação Focus/SEFAZ.
O v1 só pode ser anunciado entregue quando TODOS os critérios estiverem FEITO,
com evidência do mesmo candidato consolidado. Produção continua proibida.

- **V1-01 — FEITO, implementação e testes locais:** cadastro fiscal manual por
  empresa, proprietário, revisão, emitente, regime, ambiente, A1/CSC/token no
  mecanismo privado existente; diagnóstico sem segredos. Evidências:
  `backend/internal/fiscalproduction/customer_credentials.go`,
  `backend/internal/fiscalproduction/customer_credentials_test.go` e
  `backend/internal/localapi/fiscal_customer_credentials_test.go`, teste
  `TestHTTPFiscalCustomerCredentialsOwnerRecoveryAndNoDisclosure`.
- **V1-02 — PENDENTE, aceite integrado da instalação:** tela de venda chama a
  API real e emite NFC-e65 somente em homologação, no provedor configurado
  pelo servidor, com dinheiro, pagamento/venda/documento fiscal separados.
  `backend/internal/localapi/checkout_fiscal_test.go`,
  `TestHTTPCheckoutNFCeHomologationIntegration`, prova o circuito técnico com
  autoridade descartável; não substitui o ensaio da tela instalada com a Focus.
  Implementação da primeira emissão no checkout existente: `frontend-web/src/core/local/checkoutNFCe.mjs`,
  `Checkout96.tsx` e `CheckoutNFCeSetup.tsx`. Cliente consulta perfil/regras do cliente,
  solicita cotações tributária e comercial, cria plano e finaliza em dinheiro com
  Environment=2; configuração de provedor continua resolvida pelo servidor.
  `checkoutNFCe.test.mjs` cobre recusas, perda de resposta, privacidade, cinco idiomas
  e venda preservada após falha fiscal. O bridge `checkoutNFCe.bridge.mjs`, ativado
  por `TITAN_CHECKOUT_NFCE_BRIDGE` no teste HTTP acima, atravessa JavaScript, API Go,
  SQLite e autoridade mTLS descartáveis, incluindo consulta após recriar o cliente
  e cupom 80mm. Isso é evidência técnica local, não aceite Focus/SEFAZ ou instalação.
- **V1-03 — FEITO, geração e autorização de acesso em código:** cupom com QR,
  chave e destaque SEM VALOR FISCAL, obtidos da evidência autorizada. Fontes:
  `backend/internal/fiscalproduction/checkout_print.go` e `backend/internal/fiscalproduction/checkout_print_test.go`,
  `TestCheckoutEncryptedNFCePrintArchiveIsolationIntegrityAndNoNetwork`.
  Aprovação de impressão física não decorre de HTML: o equipamento declarado
  no piloto exige ensaio próprio no gate V1-08.
- **V1-04 — FEITO, novos arquivos e recuperação em código:** XML e eventos
  disponíveis arquivados cifrados por chave do cliente, vinculados à nota;
  falha de obtenção/gravação não perde venda nem autoriza nova emissão.
  Evidências: `backend/internal/fiscalproduction/focus_archive.go`,
  `backend/internal/fiscalproduction/focus_archive_test.go`, `backend/internal/fiscalproduction/focus_archive_integrity_test.go`,
  `backend/internal/localdb/fiscal/journal_crypto.go` e `backend/internal/localdb/fiscal/journal_crypto_test.go`.
  Backup, restauração e exportação devem preservar os mesmos bytes/IDs;
  `backend/internal/localapi/fiscal_transmission_test.go` e
  `backend/internal/localapi/fiscal_accounting_export_test.go` cobrem o circuito local.
- **V1-05 — FEITO, código e ensaios locais:** recuperação consulta a mesma
  referência, inclusive perda de resposta/reinício, sem duplicar nota,
  venda, caixa ou estoque. Evidências: `backend/internal/fiscalproduction/focus_test.go`,
  `TestFocusFaultTimeoutThenConsultationNeverDuplicates`,
  `backend/internal/localapi/fiscal_native_journal_integrity_test.go` e
  `backend/internal/localapi/fiscal_transmission_test.go`. O ensaio real é V1-09.
- **V1-06 — FEITO, código e ensaios locais:** cancelamento fiscal explícito,
  autorizado ao proprietário, separado de venda/estorno/estoque/caixa;
  cancelada e erro de cancelamento visíveis, motivo oficial preservado,
  sem prazo presumido. Evidências: `backend/internal/fiscalproduction/focus_test.go`,
  `TestFocusFaultRejectedMalformedAndCancellationReasons`,
  `backend/internal/localapi/fiscal_cancellation_test.go`. Recusa real do provedor deve ser
  mantida; o estado local não transforma uma recusa em autorização.
- **V1-07 — PENDENTE, segurança e implantação aceitas pelo cliente:** código
  tem isolamento, sessão, permissões, cofre, chaves próprias, backup e
  exposição protegida (`backend/internal/fiscalproduction/vault_concurrency_test.go`,
  `backend/internal/fiscalproduction/customer_credentials_test.go`, `backend/internal/deployment/*_test.go`,
  `backend/internal/localdb/backup/backup_test.go`). Falta aceite da instalação escolhida
  pelo cliente: local, servidor próprio ou nuvem dele, destino e chaves sob
  seu controle, backup/restauração provados e instrução de perda de chave.
  Revogação de credenciais históricas depende de evidência do titular;
  Git não a comprova. Nenhum acesso de provedor ou segredo compartilhado.
- **V1-08 — PENDENTE, gate P35:** executar os comandos reais dos workflows no
  hash candidato: PostgreSQL descartável, auxiliares, bridges, regressão
  completa e verificador, contratos, frontend/TypeScript/desktop e instalação
  limpa/restauração. Zero integrações obrigatórias puladas; evidências de UI,
  plataformas e impressora do recorte. Windows sem executor real permanece
  NÃO VALIDADO. Os gates focados de P16b NÃO substituem P35 nem CI remoto.
  Procedimento: `PROMPTS_BACKEND_TITANSYSTEM_20261104.md`, seção P35.
- **V1-09 — PENDENTE, aceitação real:** ao menos UMA NFC-e aceita pela Focus
  em HOMOLOGAÇÃO, com emitente autorizado/configurado pelo cliente, A1, CSC
  e token privados, resposta conferida, cupom e XML arquivado recuperáveis.
  `backend/internal/fiscalproduction/focus_sandbox_test.go`, `TestFocusSandboxRealNFCe`, é
  opt-in; sem as variáveis privadas do cliente é pulado e não satisfaz este
  critério. Não enviar segredos em conversa, log ou relatório. Responsáveis:
  dono/cliente configura; Focus responde pelo contrato e aceite do serviço.

## Evidência local do candidato — 06/11/2026

**FEITO no alcance técnico local:** candidato `0bc771ebf0fc772ac8be2183d9048080aad209cb`,
clone só commitado, suíte completa/25 integrações obrigatórias aprovadas,
regressões fiscais e pilotos com race detector, contratos, frontend,
builds, instalação backend em diretório novo e abertura do pacote Linux
com perfil descartável. Contagens, árvores Git, limites e logs estruturais
sanitizados: `docs/evidence/fiscal-v1-20261106/summary.json`.
Esse registro e seus artefatos não alteram código operacional.
**PENDENTES V1-02/07/08/09:** ensaio da tela instalada com Focus,
aceite da infraestrutura/chaves do cliente, revogações históricas,
checks remotos e instalação em OS limpo/Windows/impressora real.
A abertura da configuração desktop em 1.028 ms não mede login ou emissão.
Nenhuma emissão real executada; produção permanece bloqueada.

**FEITO no alcance da recuperação, após esse gate:** ação explícita na tela
para `recovery_required`, pelo ID e corpo original do servidor; sem retry
automático ou armazenamento da solicitação no navegador. Gravação do status
exige evento efetivamente persistido. Evidência focada e falha anterior
reproduzida: `docs/evidence/fiscal-recovery-20261106/summary.json`.
O gate integral acima pertence ao candidato anterior; não é apresentado
como reexecução integral dessa correção. Os aceites pendentes não mudam.

## Registros antigos de homologação

Enquanto NÃO houver produção, o cliente pode optar por descartar seus
registros antigos de homologação em vez de migrá-los. Isso é uma escolha
explícita do cliente sobre a instalação/dados corretos, nunca exclusão
automática, `clean`, reset, remoção de WIP ou autorização para apagar dados
de outra instalação. Esta consolidação não apaga nem migra nada.
Migração dos registros antigos não é requisito do v1 neste recorte.
Se o cliente decidir conservá-los, a compatibilidade de leitura e suas
chaves históricas seguem o contrato existente; JSON/base64 legado não é
apresentado como cifrado. Não descartar chaves necessárias a backups que
o cliente quer preservar sem antes provar sua restauração.

## Backlog fora do v1

- NF-e55 além da venda interna — **responsável: contador**; definir cenário,
  origem, referência, tributos e efeitos, com decisão comercial do dono e
  campos documentados pela Focus. Código existente não amplia o aceite v1.
- NFS-e municipal — **responsável: prefeitura**; requisitos do município/emitente,
  cobertura da Focus e revisão do contador antes de habilitar o cenário.
- SPED/PVA — **responsável: contador**; obrigação, leiaute, período, registros e
  validação oficial, sem chamar rascunho de arquivo aprovado.
- A3 — **responsável: Focus**; suporte do provedor atual ou decisão do dono por
  outro caminho documentado. Continua recusado no adapter atual.
- IBS/CBS — **responsável: contador**; fontes oficiais, vigência e enquadramento
  revisados do cliente antes de ampliar cobertura. Não inventar alíquotas.
- Contingência própria — **responsável: dono**; decidir operação/consistência e
  escopo. Estado de contingência informado pela Focus não é motor offline
  próprio nem prova de disponibilidade irrestrita.
- Inutilização — **responsável: Focus**; reconciliar ano/série/faixa/numeração
  remota com segurança e obter decisão explícita do cliente/contador.
  Teste técnico do journal não resolve esse contrato.
- TEF — **responsável: dono**; contratar adquirente/provedor, definir terminais e
  obter ensaios reais de pagamento e estorno, sem sucesso simulado.
- Mobile operacional — **responsável: dono**; definir operações, topologia,
  autorização e direitos offline antes de prometer venda independente.
- RH — **responsável: advogado**; requisitos legais/privacidade/retenção com o
  cliente; o dono define o escopo comercial e o contador as obrigações.

Nenhum item deste backlog habilita produção por inferência. Mudança de
recorte exige decisão explícita registrada e seus próprios critérios.
