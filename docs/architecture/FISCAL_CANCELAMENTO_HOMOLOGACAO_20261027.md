# Cancelamento fiscal recuperável em homologação — 27/10/2026

Base: `4d99c54c7b0fc00d774aa0709b43320e6869fac0`, branch fiscal do PR5.
Migração incremental71; migrações1–70 e os38XSDs anteriores não foram alterados.
Pastas originais, serviços8182/8282, frontend, Electron e banco comercial foram
preservados. Executáveis compilados ficam no laboratório; nada foi instalado.

## Contrato implementado

`CancelHomologationNFe` liga o evento110111/sequência1 ao registro original da
NF-e. O host fornece `CancellationRecorder`; um Recorder genérico não habilita
esta orquestração. Ambiente2 obrigatório. O fluxo valida o vínculo antes de
assinar, constrói/confere o evento e guarda o SOAP original antes da rede.
O SendGuard confere a autoridade imediatamente antes do único POST mTLS.

O recorder SQLite exige intenção original desta empresa/loja, consulta arquivada
correspondente com protocolo de autorização e `nfeProc` validado, contrato Fiscal
atual e permissão de cancelamento (dono/gerente). A consulta escolhida precisa
ser a mais recente reservada para essa intenção; uma consulta mais nova, mesmo
pendente, impede usar aquela autorização de envio. Há índice específico para
essa verificação sem varrer o histórico inteiro. Isso é o último estado conhecido
na base; não prova ausência de eventos externos posteriores ainda não consultados.

Chave, CNPJ e protocolo no cancelamento precisam ser os da evidência original.
O instante do evento não pode anteceder o protocolo nem estar mais de cinco
minutos à frente do relógio local. A justificativa passa pelo schema específico.
O responsável fornece referência explícita, não secreta, de sua decisão humana.
Essa referência NÃO certifica prazo legal, circulação da mercadoria ou elegibilidade
tributária; não foi implementado catálogo nacional de prazos por UF.

A reserva persiste operação, ator, aparelho, hash da sessão, consulta de origem,
hash da autorização, referência da decisão e bytes exatos assinados. A intenção
aceita uma única reserva desse evento/sequência. Nem outra sessão, identificador
ou justificativa libera uma segunda emissão. A resposta original é imutável;
callback idêntico é idempotente, divergente é conflito. Callback em trânsito após
logout/revogação pode ser preservado, sem conceder nova autoridade de envio.

## Resposta e recuperação

A interpretação confere hash, HTTP, SOAP1.2, versão1.00, XSD, lote, ambiente,
órgão, chave, tipo de evento e sequência. Somente128 no lote e135/155 no evento,
com protocolo/instante correspondentes, compõem `procEventoNFe` validado pelo XSD.
Se houver Id no retorno, ele deve corresponder ao protocolo. HTTP200 sozinho,
evento136 não vinculado, duplicidade573, rejeição, erro ou Fault não indicam
cancelamento concluído. Estados de revisão e incerteza permanecem explícitos.

`RecoverHomologationCancellation` lê somente o journal autenticado. Confere o XML
e a assinatura arquivados com o certificado público original, sem exigir chave
privada ou validade atual do certificado antigo. Isso é integridade de arquivo,
não nova confiança ICP-Brasil ou permissão de envio. Recuperação usa IDs/bytes
originais até quando o solicitante tentou outro identificador para a mesma intenção.

Cancelamento fiscal não altera venda, dinheiro recebido, estorno, recebíveis,
movimentos de caixa ou estoque. A regressão calcula um digest das tabelas de venda,
itens, pagamentos, caixa e estoque antes/depois para provar essa separação.
Não se devolve dinheiro nem mercadoria implicitamente ao receber um protocolo.

## Reinício, backup e restauração

O backup aceita schemas25–71; compatibilidade histórica inclui70. Reinício conserva
pedido, resposta, IDs e processo de evento. Restore em arquivo novo conserva os
mesmos bytes e mantém a barreira contra qualquer envio fiscal, inclusive outro
cancelamento com identificador diferente. Consulta e recuperação autorizadas
continuam possíveis. Não há liberação automática de barreiras ou de números.
Na rodada nativa, o cenário usa o executável real `titan-backup` recém-compilado;
na suíte geral sem essa flag, usa a biblioteca em banco descartável.

## Falha de CI investigada e corrigida

O CI da base4d99c54 terminou com seis checks aprovados e uma falha em
[TestSupplyCycleFinancialReadRestrictionAndOriginalActor](https://github.com/danilomalta/zzeussss/actions/runs/40486719972/job/122130227766).
O teste procurava `31500` em qualquer parte do JSON. A data sintética
`2026-10-27T10:20:01.315004798Z` continha essa sequência e causou falso positivo;
não havia campo financeiro no retorno exibido. O mesmo padrão existia em outra
verificação do histórico e foi corrigido também.

As duas verificações agora percorrem campos JSON e recusam os nomes financeiros,
inclusive em objetos aninhados e chaves escapadas. A regressão distingue datas/IDs
com os mesmos dígitos de vazamento real de `total_cents`, origem, documento e
parcelas. As verificações403/404 por usuário e as permissões do produto não foram
alteradas. Os dois testes pertinentes foram repetidos20vezes, sem falhas.

Na primeira rodada nova de testes negativos de cancelamento, os journals já
recusaram os hashes adulterados com `invalid fiscal input`, antes da leitura que
o teste pretendia verificar. O teste foi corrigido para exercitar diretamente a
fronteira de recuperação com o registro substituído. Não se relaxou a validação.
Um comando inicial foi disparado fora de backend e retornou literalmente
`go: go.mod file not found in current directory or any parent directory; see 'go help modules'`;
a repetição ocorreu no módulo correto. Resultados finais não incluem esse erro
como aprovação nem omitem os limites das verificações locais.

## Fontes oficiais e limites

- [NT2013.005 — cancelamento por evento e status135/155](https://hom.nfe.fazenda.gov.br/Portal/exibirArquivo.aspx?AspxAutoDetectCookieSupport=1&conteudo=ee%2FBnNosTWA%3D)
- [NT2018.004 — processamento de evento e status155](https://www.nfe.fazenda.gov.br/portal/exibirArquivo.aspx?conteudo=DEisbx2XCIs%3D)
- [NT2016.002 — serviçoNFeRecepcaoEvento4](https://www.nfe.fazenda.gov.br/Portal/exibirArquivo.aspx?AspxAutoDetectCookieSupport=1&conteudo=u%2F8gEs0hzEM%3D)
- Schemas de eventos já incorporados, com hashes preservados no manifesto.

Os testes usam certificado sintético e mTLS loopback; os testes SQL persistem
respostas sintéticas. Não houve POST a SEFAZ nem documento comercial cancelado.
Não há nova rota comercial, provedor de certificado, autoridade global de status
ou elegibilidade jurídica automática. Uma reserva recusada/incerta não é apagada;
correção e novo envio após rejeição exigem evolução explícita do fluxo, sem override.

Continuam pendentes emissão/outbox de produção, motor tributário nacional/RTC
aprovado, demais eventos, inutilização/contingência, DANFE, SPED/PVA, NFS-e de
produção e homologação real com emitente/certificado/credenciamento. O recurso
implementado aqui é técnico e restrito à homologação; não conclui o fiscal nacional.

Resultados finais, skips e hashes estão em
`evidence/fiscal-cancellation-20261027/verification.json`. Logs e executáveis de
ensaio ficam fora do Git em `work/integration-20261027/fiscal-cancellation-*`.
O novo commit precisa do próprio CI. Sem merge na main; credenciais históricas
continuam pendentes de troca pelo proprietário.


Resultado local final fiscal71:58pacotes/1430 funções principais/
3084 resultados incluindo subtestes aprovados, zero falhas; oito skips
explicitados na evidência. Núcleo/CLI -race:32 funções/89 resultados
em três pacotes; SQLite fiscal -race:51 funções/133 resultados e
66comandos de backup real, ambos sem falhas/skips. Regressão de CI:dois testes
repetidos20vezes,40aprovações. Vet completo, quatro builds,17 testes dos
validadores e537rotas passaram. Fontes conferidos por hash antes/depois dos
testes. Nenhum binário instalado. Novo CI obrigatório; nenhum merge.
