# Inutilização técnica de numeração em homologação — 28/10/2026

Base: `f532e39115e3a3c2113cfb08323f8057167cc9ce`, PR5.
Os sete checks dessa base passaram, inclusive o ensaio integrado:
https://github.com/danilomalta/zzeussss/actions/runs/40490651352/job/122143445013
Esse resultado certifica a base, não os fontes posteriores desta unidade.

Migração incremental72; migrações1–71, pastas originais, frontend/Electron,
serviços8182/8282 e banco comercial preservados. Nada instalado ou enviado à SEFAZ.

## Contrato e autoridade

`Invalidation` identifica UF, CNPJ, modelo55/65, série, ano e faixa inicial/final,
ambiente2 e justificativa. O ID contém exatamente esses identificadores. O limite
local10000números é proteção de recursos do componente, não limite legal nacional.
Séries0–889 seguem o recorte existente da autoridade local; ano futuro não recebe
permissão de envio pelo SQLite. Não há seleção automática de faixa ou motivo.

`HomologationInvalidationRecorder` exige sessão exata, contrato Fiscal e dono ativo
para envio. A série precisa ter origem explícita nesta empresa/loja e o perfil
precisa ser da mesma revisão, CNPJ, UF, modelo, série e homologação. Não toma a
série de outra loja, não usa outra empresa e não inutiliza números anteriores à
origem reconciliada. Contador/gerente autorizado pode recuperar histórico, sem
receber permissão de inutilizar ou retomar uma tentativa de outro ator/sessão.

O dono informa uma referência não secreta de sua decisão. Ela não certifica prazo,
obrigação ou elegibilidade jurídica da faixa. Não se aprovaram regras tributárias,
emitente, credenciamento ou uso comercial. A base deve ser a única autoridade da
combinação emitente/modelo/série; não é coordenação entre instalações independentes.

## Atomicidade e proteção da sequência

A mesma trava de escrita SQLite da numeração é obtida ANTES das leituras de
permissão e disponibilidade. Não se pode inutilizar faixa que contenha atribuição,
tentativa original de emissão ou outra reserva de inutilização. A guarda inclui
as tentativas incertas. Há índice de faixa e triggers defensivos nas três tabelas:
reserva de inutilização, atribuição de número e journal legado de emissão.

A reserva persiste os bytes SOAP assinados e hashes, faixa, perfil/revisão, operação,
ator/aparelho, hash da sessão e referência da decisão antes da rede. Não há
expiração ou liberação automática. Uma faixa pendente, recusada ou incerta fica
excluída da alocação até existir fluxo explícito de reconciliação; HTTP200 ou uma
recusa não apagam o registro.

O alocador pula apenas faixas que contêm o próximo número, inclusive faixas
adjacentes. Não salta números livres antes de uma faixa futura. Ao alcançar o
limite999999999, recusa nova atribuição sem voltar ao1. A sequência existente não
reinicia automaticamente por ano; a exclusão é conservadora entre anos na mesma
autoridade. A operação não significa que a SEFAZ já homologou os números.

## Transporte, resultado e recuperação

`InvalidateHomologation` exige recorder tipado. Confere o vínculo, gera XML,
assina o infInut, valida o XSD e passa pelo guard antes de um POST mTLS ao método
`NFeInutilizacao4/nfeInutilizacaoNF`. Endereço continua controlado pelo host confiável;
o renderer não fornece URL nem certificado. Não há repetição automática de POST.

A resposta é conferida por hash, SOAP, XSD, ambiente, UF e associação exata do ano,
CNPJ, modelo, série, início e fim. Somente cStat102 com protocolo estruturado e data
válida compõe `ProcInutNFe` validado.563 não é promovido a sucesso e não se extrai
protocolo de texto livre. Rejeição/duplicidade/Fault/HTTP diferente mantêm revisão
ou incerteza e conservam a faixa bloqueada.

A raiz `ProcInutNFe`, com P maiúsculo, é a declarada no XSD incorporado. O primeiro
teste positivo detectou a raiz minúscula e retornou `fiscal schema validation failed`;
foi corrigido o gerador, sem alterar/relaxar o schema. Outros erros iniciais foram
importações não usadas em testes e uma fixture legada com chave do número1 em vez
do número3 pretendido; a fixture foi corrigida e as validações preservadas.
O sandbox de rede também recusou o primeiro servidor de teste com
`listen tcp6 [::1]:0: socket: operation not permitted`; a rodada foi repetida com
permissão para loopback descartável, sem conexão operacional externa.

`RecoverHomologationInvalidation` usa o journal autenticado e conserva IDs/bytes
originais sem rede, nova assinatura ou chave privada atual. A assinatura arquivada
prova integridade do registro selecionado, não nova confiança/revogação ICP-Brasil.
Resposta recebida depois do logout ou da revogação pode ser preservada; isso não
reativa a autoridade anterior. Callback idêntico é idempotente, divergente é conflito.

## Schemas oficiais e CNPJ alfanumérico

O leiaute antigo exposto na SVRS tinha identificador exclusivamente numérico.
Não foi modificado para aceitar letras. Foram incorporados o leiaute e processo do
pacote oficial alfanumérico, mantendo as raízes simples oficiais de versão4.00 e
os tipos básicos já incorporados. Quatro arquivos novos; os38anteriores permanecem
com seus hashes originais. Compilação, assinatura e leitura fora do diretório do
projeto são verificadas, inclusive com CNPJ numérico e alfanumérico.

- [Pacote nacional atualizado](https://www.nfe.fazenda.gov.br/portal/exibirArquivo.aspx?conteudo=FOEnx2KAsCk%3D): SHA256 `cc50170b276c23bdab88650e1c68a46fdc707c8e810664452bba974cf744e7db`.
- ZIP interno `PL_010d_NT2026.004.zip`: SHA256 `c295706924ccba6044b53bca044c6069787db9a1b3bb8aa376266ef4fe4500d2`.
- [Raiz de entrada SVRS](https://dfe-portal.svrs.rs.gov.br/Schemas/PRNFE/inutNFe_v4.00.xsd).
- [Raiz de retorno SVRS](https://dfe-portal.svrs.rs.gov.br/Schemas/PRNFE/retInutNFe_v4.00.xsd).
- [NT2016.002: método e parâmetros SOAP](https://www.nfe.fazenda.gov.br/portal/exibirArquivo.aspx?conteudo=NsGi2n016p8%3D).
- [MOC7: serviço de inutilização](https://www.confaz.fazenda.gov.br/legislacao/arquivo-manuais/moc7-visao-geral.pdf).

Proveniência e hashes individuais estão no manifesto incorporado. Não foi
repetido download operacional com credencial; somente GETs de documentação e XSDs.

## Backup, evidências e limites

Backup aceita25–72; compatibilidade histórica inclui71. Reinício e Restore em
arquivo novo preservam faixa, assinatura, protocolo, IDs e processo. Restore
mantém a barreira de envio, inclusive para operação nova. A rodada nativa usa o
executável real titan-backup; a suíte geral usa biblioteca quando a flag não é dada.
O digest de venda/itens/pagamentos/caixa/estoque permanece igual após a operação.

Evidências e skips: `evidence/fiscal-invalidation-20261028/verification.json`.
Logs e builds em `work/integration-20261027/fiscal-invalidation-*`, fora do Git.
Nenhum binário, banco, log bruto, credencial ou ZIP de entrega incluído no commit.

Ainda não há homologação real, rota comercial/outbox de produção, reconciliação
que libere faixa recusada, autoridade global entre instalações ou catálogo jurídico
de elegibilidade. Permanecem também motor tributário nacional/RTC aprovado, demais
eventos/contingência, DANFE, SPED/PVA e NFS-e produção. Certificado/credenciamento
são necessários à homologação, mas não substituem essas implementações pendentes.
Capacidades oficiais continuam indisponíveis. Nenhum merge na main; credenciais
históricas ainda pendentes de troca pelo proprietário.


Resultado local final fiscal72: 58 pacotes, 1445 testes principais e 3139 resultados incluindo subtestes aprovados; zero falhas, 8 skips explícitos. Núcleo/CLI com -race: 36 principais/118 resultados em três pacotes. SQLite fiscal com -race: 62 principais/158 resultados e 77 comandos reais de backup/restauração, sem falhas ou skips nessas duas rodadas. Vet completo, quatro builds, 17 testes dos validadores e 537 rotas passaram. Os 25 fontes/configurações foram conferidos por hash antes e depois; as 71 migrações e os 38 XSDs anteriores permanecem idênticos. CI desta revisão deve rodar após publicação.

A conferência de whitespace reconhece CRLF dos XSDs oficiais (`cr-at-eol`), preservando os bytes e hashes publicados pela fonte. Nenhum schema foi normalizado para ocultar o aviso inicial.
