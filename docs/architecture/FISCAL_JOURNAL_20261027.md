# Journal fiscal transacional de homologação — 27/10/2026

Base exata: f492db7b9d46c8f6830bf72a83fd22525537f18c. Branch separada
`feat/fiscal-homolog-journal-20261027`, baseada no PR4. As pastas originais,
os serviços8182/8282, frontend e Electron não foram alterados.

## Componente entregue e seu alcance

A migração68 acrescenta o vínculo imutável à sessão que preparou a intenção,
o journal de requisições e respostas e a barreira de restauração. As migrações
anteriores permanecem inalteradas. O backup aceita25–68 e, ao restaurar em
caminho novo, revoga sessões e acrescenta a barreira fiscal persistente.

`fiscal.NewHomologationRecorder` implementa o contrato Recorder do cliente TLS
existente, no mesmo SQLite da venda/perfil/intenção. Não há novo banco paralelo.
O host confiável fornece intenção, endpoint e chave; não existe rota que aceite
esses parâmetros do renderer. Não recebe nem persiste chave privada, CSC ou
certificado. O token humano permanece somente em memória; seu vínculo durável
é SHA-256, nunca o token em texto.

A reserva confere sessão exata, empresa/loja/aparelho/ator, contrato Fiscal,
perfil original ainda vigente e emitente/modelo/série/UF da chave. Só aceita
ambiente2 e intenção originalmente de homologação. Produção é recusada.
A mesma intenção/operação não pode ser reservada duas vezes. Há também índice
único para emitente/modelo/série/número: outra venda não pode reutilizar esse
número mudando apenas cNF ou a chave completa. Isso impede reutilizar números
registrados neste banco; NÃO é alocação automática ou autoridade nacional de
numeração entre instalações independentes.

O commit da requisição antecede qualquer rede. O cliente chama o novo SendGuard
após esse commit e imediatamente antes do POST para conferir novamente sessão,
contrato, perfil e barreira. Recusa preserva a tentativa original e envia zero
requisições. Não há promessa de revogação retroativa de chamada já entregue à
rede; o resultado dessa chamada pode ser gravado mesmo após logout/cancelamento.

Resposta recebida não significa autorização fiscal. Complete aceita somente
`uncertain` ou `response_received`, confere hashes e identidade e preserva o
primeiro resultado. Callback idêntico é idempotente; divergente é recusado.
Recuperação confere sessão/permissão atuais e devolve bytes/IDs originais sem
POST. Nova sessão não assume o direito de enviar a intenção da sessão anterior.
Intenções anteriores à migração68 não ganham vínculo novo automaticamente.

Backup restaurado recebe barreira por empresa mesmo se o snapshot anteceder a
reserva. Até uma venda/intenção nova com sessão nova é impedida de transmitir.
A barreira não possui comando de liberação: reconciliação externa e mecanismo de
numeração precisam ser definidos antes disso. A prova vale para o Restore
suportado; copiar discos/bancos por fora desse fluxo não oferece essa garantia.

## Limites que continuam efetivos

Não é o emissor completo. O journal não constrói/valida o enquadramento tributário
nem verifica a correspondência econômica integral do XML com todos os itens
originais. O cliente técnico continua validando XML/assinatura no seu próprio
fluxo; falta o orquestrador que ligue cálculo, reserva de número, geração,
certificado, envio, consulta, protocolo e eventos à intenção comercial.
Nenhuma capacidade HTTP de produção foi habilitada. Os estados de prontidão
continuam falsos, inclusive emissão oficial, DANFE, NFS-e e SPED.

Continuam sem entrega integral:

- Regras nacionais/estaduais/municipais completas, classificação e benefícios
  por produto/atividade/CRT/vigência e reforma tributária, com aprovação contábil.
- DANFE de conformidade oficial e o ciclo completo de contingência/eventos.
- Escrituração EFD/SPED completa e execução/aceite pelo PVA correspondente.
- NFS-e de produção: permanece a incompatibilidade já reproduzida do XSD fixado.
  O perfil de homologação separado não foi promovido para produção.
- Homologação real: não há emitente piloto, certificado ou credenciamento
  disponibilizado. Nenhum endpoint fiscal oficial foi chamado nesta entrega.

Certificado sintético, servidor TLS descartável, PASS de XSD e resposta HTTP200
não são homologação oficial. Nenhuma alíquota foi escolhida automaticamente.

## Verificação e rastreabilidade

Os testes novos usam SQLite real e as rotas de preparação em harness Fiber;
seus payloads técnicos não são notas fiscais. O teste do SendGuard usa HTTP/TLS
real em loopback com certificados sintéticos e comprova zero POST após recusa.
São testes distintos; não afirmar um E2E SQLite+SEFAZ que não foi executado.

Há regressões de oito reservas concorrentes, conflito entre vendas/cNF, logout,
mesmo usuário com nova sessão, quatro dimensões de contexto, revogação de
contrato, revisão do perfil, respostas inválidas, reinício do banco e backup
cifrado/restauração antes da reserva, depois dela e após resposta recebida.
A rodada com o executável titan-backup real executa também verificações negativas
de chave/contexto e recusa de sobrescrita; isso não valida transmissão fiscal. Evidências exatas ficam em
`docs/architecture/evidence/fiscal-journal-20261027/verification.json`.
Logs completos e binários técnicos ficam no laboratório externo à árvore Git;
não entram no pacote ou no produto instalado. Builds locais usam
`-buildvcs=false` somente para omitir carimbo VCS nesta worktree.

O CI foi estendido ao PR empilhado para a branch de integração e acrescenta
regressões fiscais com `-race`. Cada revisão exige seu próprio resultado remoto;
os sete checks verdes do PR4 não certificam esta alteração. O bloqueio de merge
por credenciais historicamente expostas continua em vigor.

## Referências oficiais consultadas

- NFS-e produção: https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/documentacao-atual
- Guia EFD3.2.4: https://www.gov.br/sped/pt-br/assuntos/escrituracoes-digitais/efd-icms-ipi/manuais-e-documentos-tecnicos/guia-pratico-efd-versao-3-2-4.pdf
- Serviço EFD/PVA: https://www.gov.br/pt-br/servicos/entregar-escrituracao-fiscal-digital-efd-icms-ipi

Essas referências não são uma certificação tributária nem provam validação PVA.

## Resultado local final

A suíte geral local passou:58pacotes com testes aprovados,2911 resultados
de teste/subteste aprovados, zero falhas. São1384 funções principais
aprovadas; oito testes foram explicitamente ignorados. Dois exigem PostgreSQL,
três exigem as pontes frontend configuradas, dois são auxiliares de processos
e um é o ensaio de tela com ativação explícita.
Essas integrações ausentes não são declaradas validadas localmente.

A primeira rodada teve oito falhas ao compilar o CLI de transporte por
`error obtaining VCS status: exit status128`. A repetição completa omitiu
somente o carimbo VCS e forneceu titan-trade real com fila habilitada. Nenhum
teste foi removido ou tolerado para obter PASS. A rodada final usa os fontes
finais Go desta unidade.

A rodada final fiscal com -race e titan-backup real passou:12 funções,27
resultados incluindo subtestes, zero falhas e33 chamadas do executável de
backup. A fronteira mTLS/SendGuard também passou com -race, em dois testes.
Vet final, dez testes de contratos e537rotas passaram. API, backup, transporte
e CLI fiscal foram compilados em pasta de laboratório, sem instalação.
O CI remoto desta branch ainda precisa de sua própria execução e não é
substituído por estes resultados. Fiscal inteiro continua não concluído.
