# Impressão de NFC-e a partir do journal — 28/10/2026

Base: `bc21e6e972b190f081683fc17a58756fe0037301`. Os sete checks dessa base
passaram, inclusive o integrado [40640900803](https://github.com/danilomalta/zzeussss/actions/runs/40640900803/job/122658539869).
Isso não certifica a revisão posterior de impressão descrita aqui.

## Operação disponível

`GET /local/v1/fiscal/intents/{id}/danfe?width_mm=80&timezone=America%2FSao_Paulo`
retorna HTML estático de uma NFC-e modelo65, emissão normal, ambiente2. Exige
protocolo correspondente ao XML arquivado. Não recebe XML, valores, certificado,
CSC ou endereço fiscal do navegador. `58` e `80` são as larguras aceitas; a zona
IANA brasileira é explícita, pois a UF sozinha não identifica todo fuso municipal.
O executável incorpora tzdata para não depender da instalação do Windows/Linux.

O dono/contador autorizado pode ler os documentos da loja. Vendedor lê somente
sua tentativa e aparelho. A sessão atual é conferida; logout revoga leitura.
Uma nova sessão autorizada pode reimprimir os mesmos bytes. Perda do contrato
Fiscal não apaga o acesso histórico. Restore revoga as sessões, conserva o
documento e mantém a barreira de emissão; impressão não a remove.

O processo é reconstruído dos journals imutáveis: emissão síncrona válida ou
consulta mais recente autorizada. Consulta pendente/negativa não cai no resultado
antigo. Qualquer reserva de cancelamento bloqueia a cópia com aparência de ativa,
mesmo sem resposta. Cópia cancelada e reconciliação dessa situação ficam fora
deste leiaute, em vez de serem apresentados como documento ativo.

## Vínculo e conteúdo

O núcleo confere XSD, assinatura do emitente, chave, ambiente, digest, protocolo
e data. Preserva itens, quantidades, unidades, valores, descontos, acréscimos,
pagamentos, identificação, mensagens e tributos aproximados quando constam no
XML. Não usa float nem consulta novamente cadastro/preço. Campo de troco ausente
é indicado como ausente, sem inventar zero. Texto é escapado, nunca HTML ativo.

`CompleteNFCeXML` insere o suplemento de consulta derivado do documento assinado
e da configuração confiável do host. Reconfere a assinatura e o XSD. Não substitui
suplemento existente nem ativa contingência. Corrige os separadores `%7C` que
impediam inserir o QR gerado no XSD oficial: os `|` são preservados, sem alterar
schema. Testes cobrem versões2/3 e digest inalterado.

A resposta usa `no-store`, CSP sem scripts/conexões/fontes remotas, imagens PNG
embutidas, `nosniff` e `no-referrer`. `X-Titan-Source-SHA256` identifica o processo
validado reconstruído; não é o hash bruto do SOAP original. O outro cabeçalho
identifica o HTML retornado. Nenhum token, chave privada ou CSC entra no artefato.

## Leiaute e ensaio visual

Referência: [manual nacional DANFE NFC-e6.0/QR2–3](https://www.nfe.fazenda.gov.br/portal/exibirArquivo.aspx?conteudo=k%2FIuuaW4YiY%3D),
SHA256 `bf906cc212f1edd19b1df7d1cdf4fbf5e73c3fec77567b6e69fa74453c95db5e`.
O leiaute inclui identificação/endereço do emitente, itens, totais e pagamentos,
consulta/chave agrupada, consumidor, QR, número/série, horários locais, protocolo,
mensagens e destaque de homologação. Usa margens laterais2mm e imagem QR30mm.
O papel deve ser selecionado no driver: HTML não configura sozinho uma bobina.

Três documentos sintéticos são renderizados no Chromium em perfil descartável:
58mm,80mm e100itens. PDFs de QA ficam fora do Git. No modo headless padrão, a
folha é Letter com a coluna na largura selecionada; isso não prova impressora
térmica física. Poppler renderiza as páginas; extração confere os100códigos e
inspeção visual verifica cabeçalho, quebras e rodapé. Não foi testada leitura do
QR por câmera física. Os PDFs são ensaio de impressão, não saída PDF do backend.

O primeiro ensaio revelou endereço omitido por seleção incorreta de caminho XML;
foi corrigido com regressão de conteúdo. O cupom longo revelou uma regra CSS que
empurrava a lista inteira para outra página; foi removida sem permitir cortar
um item entre páginas. Uma asserção de logout esperava200, mas o contrato correto
é204; foi corrigido o teste. A fixture de protocolo usava `Z`, rejeitado pelo XSD
incorporado; passou a usar `+00:00`, preservando o schema e a conversão de horário.

## Alcance e continuidade

Sem migração: schema72 e os42XSDs anteriores permanecem intactos. Nenhuma mudança
de frontend/Electron, pastas originais, serviços8182/8282 ou banco comercial.
Testes HTTP usam socket loopback real e SQLite descartável. A resposta fiscal de
teste continua sintética; não houve POST operacional à SEFAZ nem homologação real.

Ainda faltam DANFE NF-e55, produção/contingência, cópia cancelada, leiautes especiais,
PDF nativo e ensaio em impressora física. Também permanecem integração de produção
do emissor, regras tributárias nacionais/RTC aprovadas, demais eventos, SPED/PVA,
NFS-e produção e homologação com emitente/certificado/credenciamento. Capacidades
oficiais continuam falsas. Isso não é conclusão do módulo fiscal completo.

Evidências: `evidence/fiscal-danfe-20261028/verification.json`.
Logs, executáveis e prévias: `work/integration-20261027/fiscal-danfe-*`, fora do Git.
Sem merge na main; credenciais históricas ainda pendentes de troca.


Resultado local final: 58 pacotes, 1457 testes principais e 3173 resultados incluindo subtestes aprovados; zero falhas, 8 skips explícitos. Núcleo/CLI -race: 42/139 em três pacotes. SQLite/API fiscal -race: 68/171, com 88 comandos reais de backup/restauração; ambas as rodadas sem falhas/skips. Vet, quatro builds,17 testes dos validadores,538 rotas e QA visual passaram.15 fontes/configurações conferidos por hash;72 migrações e42XSDs anteriores intactos. Nenhum executável instalado. CI do novo commit deve rodar após publicação.
