# Numeração durável do emissor em homologação — 27/10/2026

Base: `fc0d0f0f9fae54eee57334f52e9220dd07b8856c`, na branch fiscal do PR5.
Nenhum banco existente, serviço8182/8282, frontend ou Electron foi alterado.
A migração69 é incremental; as migrações anteriores permanecem inalteradas.

## Contrato implementado

`ConfigureHomologationSeries` registra a origem da série por emitente/modelo/
série. Exige sessão atual, contrato Fiscal e papel owner. Recebe perfil e revisão,
primeiro número e referência não secreta da conferência do responsável. Não
presume que uma empresa existente possa começar no número1. A referência é
rastreabilidade da decisão humana, não certificado de liberação pela SEFAZ.
Repetição idêntica é aceita; alteração ou redução da origem é recusada.

`AllocateHomologationNumber` liga o próximo número à intenção e sessão originais.
Persiste atomicamente número, instante de emissão, cNF aleatório, chave completa,
aparelho, ator e operação. Não há rede, assinatura ou reserva apenas em memória.
A numeração é derivada do maior número já reservado ou registrado no journal,
respeitando o início explicitamente configurado. Repetição idêntica recupera
chave/instante originais; divergência de operação ou instante é conflito.

A trava SQLite é obtida antes de ler permissões e a sequência, inclusive entre
conexões independentes. As restrições únicas e os registros imutáveis impedem
reutilização. Uma recusa de autorização não avança a sequência. As consultas do
último número usam os índices de emitente/modelo/série existentes, sem transferir
o histórico inteiro ao código Go.

O orquestrador exige essa atribuição durável antes de aceitar o formulário para
assinatura. A chave e o instante precisam ser os originais. O transporte técnico
anterior permanece compatível com tentativas legadas, mas não pode usar número
atribuído a outra intenção nem mudar a chave de uma intenção já numerada.
Uma intenção legada já registrada no journal não recebe outro número.

`ReadHomologationNumber` exige sessão e permissão atuais, respeita empresa/loja,
e permite recuperação autorizada após logout, perda do contrato, reinício ou
restauração. Não faz POST nem dá nova autorização de envio à sessão atual.
A restauração preserva o número/chave e mantém a barreira de reconciliação,
inclusive para configurar séries e numerar intenções novas.

## Regras de segurança e limites

- Homologação apenas. Produção e novas rotas HTTP continuam indisponíveis.
- Número reservado nunca expira ou é liberado automaticamente. Falha de XML,
  certificado, rede ou encerramento não autoriza reutilização.
- Ao atingir999999999 o componente recusa nova reserva; não volta sozinho ao1.
  Regularização, inutilização e mudança de série precisam de fluxo oficial.
- Esta base deve ser a única autoridade daquela combinação emitente/modelo/
  série. Não é coordenação entre bancos/PCs independentes, clonagem de disco,
  outro ERP ou todas as instalações de uma empresa. Uma origem já ocupada por
  outra loja/empresa nesta base não pode ser tomada pelo chamador.
- A chave calculada não é autorização fiscal. Não se aprovaram alíquotas, CRT,
  CFOP, NCM, classificação, DANFE, SPED/PVA ou NFS-e de produção.
- Certificado, credenciamento e emitente autorizado continuam necessários para
  homologação oficial; há também implementação independente ainda pendente.

## Backup e compatibilidade

O executável passa a aceitar snapshots25–69. O teste histórico cria cada banco
anterior a partir das respectivas migrações originais e inclui agora o schema68.
Os dados originais não são migrados para inspecionar/exportar o backup.
O teste de numeração confere a chave original após restaurar em outro arquivo,
e demonstra que nem série nem intenção nova eliminam a barreira de Restore.
O relatório genérico `pilot_cash_v1` continua restrito às tabelas comerciais
que já declara; não se transformou em certificado de igualdade de todas as
novas tabelas fiscais. O teste fiscal confere sua própria atribuição restaurada.

## Evidências

Resultados e hashes finais: `evidence/fiscal-numbering-20261027/verification.json`.
Logs e executáveis de ensaio ficam fora da árvore Git, em
`work/integration-20261027/fiscal-numbering-*`. Nenhum executável foi instalado.

A primeira rodada de teste recebeu `venda: recebido=400 esperado=201 corpo=Bad Request`
porque a fixture vendia fração de produto cadastrado como unidade. Os cenários
foram corrigidos para unidades inteiras, com entrada/transferência pelas APIs
reais da fixture. Nenhuma validação comercial foi relaxada para passar testes.
Não confundir esses erros iniciais com o resultado das rodadas finais.

## Referência oficial consultada

O Portal Nacional descreve a sequência por estabelecimento e série, limitada a
999999999. A aplicação conserva o bloqueio conservador ao esgotamento, sem
presumir reconciliação oficial ou autorização de reuso:
https://www.nfe.fazenda.gov.br/Portal/perguntasFrequentes.aspx?tipoConteudo=auR4yGlWmRY%3D

Este incremento não conclui o fiscal inteiro nem substitui os demais requisitos.

## Resultado local final

Suíte geral:58pacotes aprovados, 1400 funções principais e
2976 resultados incluindo subtestes aprovados, zero falhas. Oito skips
explícitos: dois PostgreSQL, três pontes frontend, dois auxiliares de processo
e um ensaio visual opt-in. Não declarar essas integrações executadas localmente.
Rodada fiscal com -race e CLI real:31 funções/80 resultados, zero falhas/zero
skips,44 comandos do titan-backup. Todas as compatibilidades históricas25–68
restauraram para69. Vet completo, quatro builds,17 testes dos validadores
e mapa de537rotas passaram. Binários e logs permanecem no laboratório.
O CI deve validar o commit novo; o resultado de revisão anterior não o substitui.
