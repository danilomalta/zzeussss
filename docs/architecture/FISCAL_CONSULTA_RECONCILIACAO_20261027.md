# Consulta e reconciliação técnica da NF-e — 27/10/2026

Base: `09104000bc9794019e4396ded37817b88528630a`, branch fiscal do PR5.
Migração incremental70. As migrações1–69, pastas originais, serviços8182/8282,
frontend, Electron e bancos comerciais foram preservados. Nada foi instalado.

## Comportamento entregue

`Client.ConsultNFe` constrói `consSitNFe`4.00 e usa SOAP1.2 no método
`NFeConsultaProtocolo4/nfeConsultaNF`. É uma consulta explícita por chave em
homologação, com certificado correspondente ao CNPJ, endpoint escolhido pelo
host confiável, journal antes da rede e conferência de autoridade antes do envio.
O POST de consulta não contém outra NFe, evento, assinatura ou número novo.
Não existe repetição automática da emissão original.

`HomologationQueryRecorder` mantém pedidos e respostas imutáveis em tabelas
separadas. Vincula empresa, loja, intenção, chave e hash da emissão original,
identificador próprio, aparelho, ator e hash da sessão da consulta. A identidade
referencia o cadastro central `memberships`, inclusive para novos funcionários;
a tabela legada `users` não é fonte de autorização.

Leitura exige sessão atual e permissão contábil. Envio exige também contrato
Fiscal atual e a mesma sessão/ator/aparelho que reservou aquela consulta. Outro
contador autorizado pode ler o histórico e iniciar uma consulta NOVA, mas não
retomar a tentativa alheia. Logout/revogação bloqueiam o envio antigo; resposta
que já estava em trânsito pode ser preservada depois do logout. Callback idêntico
é idempotente; divergente é conflito. Falhas não apagam a tentativa.

`Reconcile` confere os bytes e assinatura arquivados, chave, CNPJ, ambiente,
hashes, identificadores, XML de resposta e vínculo do protocolo ao digest da NFe.
Cada resultado pertence à sua consulta: resposta antiga que chega depois não
sobrescreve outra consulta nem promove um estado global silenciosamente.

Autorização indicada100/150, sem eventos adicionais, pode compor `nfeProc`
com a NFe assinada original e o protocolo correspondente, validado pelo XSD.
Cancelamento indicado101, denegação110/301/302, não encontrado217 e demais
situações têm estados distintos. Eventos/cancelamento embutidos no retorno de
autorização exigem revisão, sem gerar processo tratado como autorizado atual.
217 nunca libera número, apaga incerteza ou provoca nova emissão.

A recuperação do orquestrador anterior também passa a produzir `nfeProc`
validado quando há protocolo correspondente. A verificação da assinatura
arquivada comprova integridade dos bytes em journal confiável; não revalida
cadeia/revogação ICP-Brasil nem concede autoridade para emitir novamente.

## Reinício e restauração

Backup aceita schemas25–70, incluindo compatibilidade histórica69. O teste
restaura por biblioteca e, na rodada nativa, pelo executável real `titan-backup`.
Chave, IDs e processo permanecem iguais após reinício/restauração.
Consultas novas são permitidas após Restore com sessão/permissão atuais, para
investigar documentos; a barreira durável continua bloqueando novas emissões,
origens de série e numeração. Nenhuma reconciliação libera essa barreira aqui.

## Correção de execução instalada

Foi reproduzido um defeito real de integração: `CompileFile` do Helium0.11.0
abria o XSD inicial pelo diretório de trabalho, mesmo com FS configurado.
Os testes do pacote fiscal passavam porque o fonte existia naquele diretório;
na API chamada de outra pasta a mesma validação falhava com erro de schema.

A compilação agora lê o XSD inicial do filesystem incorporado e fornece suas
dependências pelo mesmo FS, com diretório-base explícito. Não busca schemas
na rede ou no disco do cliente. Regressões mudam para pasta temporária e
validam compilação/assinatura e o comando real do CLI com cache frio.

## Schemas e fontes oficiais

Quatro arquivos oficiais foram acrescentados, sem alterar os XSDs anteriores:

- [consSitNFe_v4.00.xsd](https://dfe-portal.svrs.rs.gov.br/Schemas/PRNFE/consSitNFe_v4.00.xsd)
- [retConsSitNFe_v4.00.xsd](https://dfe-portal.svrs.rs.gov.br/Schemas/PRNFE/retConsSitNFe_v4.00.xsd)
- [leiauteConsSitNFe_v4.00.xsd](https://dfe-portal.svrs.rs.gov.br/Schemas/PRNFE/leiauteConsSitNFe_v4.00.xsd)
- [procNFe_v4.00.xsd](https://dfe-portal.svrs.rs.gov.br/Schemas/PRNFE/procNFe_v4.00.xsd)

Origem, SHA256 e revisão estão no manifesto incorporado; o teste confere os38
arquivos. O primeiro download do pacote Nacional retornou literalmente:
`urllib.error.HTTPError: HTTP Error 302: The HTTP server returned a redirect error that would lead to an infinite loop. The last 30x error message was: Found`.
Foi usada a distribuição oficial SVRS acima, com hashes individuais. Não se
contornou a validação nem se substituiu schema por arquivo não oficial.
Método SOAP conferido na [NT2016.002](https://www.nfe.fazenda.gov.br/Portal/exibirArquivo.aspx?AspxAutoDetectCookieSupport=1&conteudo=u%2F8gEs0hzEM%3D).

## Escopo comprovável e pendências

Os testes de transporte usam mTLS loopback e certificado sintético explicitamente
confiado. Os testes SQLite verificam o journal/permissões/reinício/Restore com
APIs comerciais reais da fixture, mas a emissão e resposta fiscal são sintéticas.
Não houve chamada operacional a SEFAZ, emissão de documento real ou homologação
com emitente credenciado. Os dois conjuntos não substituem essa homologação.

Ainda não há rota HTTP comercial para estes componentes, provisionamento de
certificado ou outbox fiscal de produção. Continuam pendentes autoridade entre
instalações, catálogo tributário nacional aprovado/RTC, eventos oficiais completos,
inutilização/contingência, DANFE, SPED/PVA e NFS-e de produção. A falta de um
emitente/certificado impede homologação real; as demais pendências também exigem
implementação, não apenas um certificado. Todas as capacidades oficiais de
produção permanecem explicitamente indisponíveis. Não foi feita cobrança fiscal,
transmissão real nem consulta real a órgão público.

Resultados finais e hashes: `evidence/fiscal-query-20261027/verification.json`.
Logs e executáveis ficam em `work/integration-20261027/fiscal-query-*`, fora do
Git. O CI precisa validar o novo commit. Credenciais antigas ainda não trocadas
mantêm o merge bloqueado, independentemente do resultado dos checks.


Resultado local final fiscal70:58pacotes/1416 funções principais/
3026 resultados aprovados, zero falhas, oito skips listados na evidência.
Núcleo/CLI -race:29 funções/64 resultados em três pacotes, zero falhas/skips.
Fiscal SQLite -race:41 funções/102 resultados,55 comandos de backup real,
zero falhas/skips. Vet completo, quatro builds,17 testes dos validadores e
537rotas aprovados. Nenhum binário instalado. Os sete checks da base0910400
passaram; esta revisão precisa de CI próprio. Nenhum merge realizado.
