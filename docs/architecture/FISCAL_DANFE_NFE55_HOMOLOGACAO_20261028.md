# DANFE NF-e 55: cópia técnica A4 de homologação

Base: `a790b660d2a7bc2c27c0ae707c73ca65c462c4f8`, branch
`feat/fiscal-homolog-journal-20261027`. Esta unidade não declara o fiscal
completo nem promove o emissor a produção.

## Comportamento implementado

A mesma rota autenticada `GET /local/v1/fiscal/intents/{id}/danfe` aceita
`width_mm=210&timezone=America%2FSao_Paulo` para NF-e modelo55, emissão normal,
ambiente2 e orientação retrato. NFC-e65 conserva58/80mm. Uma escolha
incompatível é recusada, sem adaptar silenciosamente o modelo.

O arquivo é selecionado do journal, nunca recebido como XML pelo navegador.
Valida schema oficial, assinatura do emitente e identidade de chave/ambiente/
digest/protocolo. A consulta mais recente controla a leitura; incerteza,
negativa ou cancelamento iniciado não podem ressuscitar uma autorização antiga.
Permissões, sessão atual, empresa, loja, operador e aparelho continuam na API.
Vendedor não ganha leitura das tentativas de outro vendedor. Acesso contábil
não concede poder de emitir/cancelar.

A4 tem folhas numeradas, identificação e chave repetidas, identificação da
operação, destinatário, datas no fuso brasileiro explícito, retirada/entrega,
fatura/duplicatas quando presentes, totais do XML sem recálculo, transporte,
volumes, produtos com ICMS/IPI e informações adicionais. Quantidades e valores
decimais são preservados. Ausência de valor não vira zero. Descrições longas
e informações complementares continuam em outras linhas/folhas. Cabeçalhos
das colunas se repetem na continuação dos produtos. Nenhum conteúdo vira HTML
executável; sem JavaScript, fontes ou imagens remotas.

O código128 usa conjuntoC para pares numéricos e conjuntoA para letras
maiúsculas, com checksum103, margens claras de10módulos e raster em escala
inteira. Largura115mm e altura10mm no leiaute. Chaves numéricas são comparadas
nos testes com o codificador independente já dependência do projeto; chaves
alfanuméricas têm decodificação de retorno e conferência de checksum. Isso não
substitui leitura por equipamento físico.

Backup/restauração conserva XML/IDs e a cópia HTML determinística após novo
login. A sessão antiga restaurada é recusada; imprimir não remove a barreira
de restauração, não transmite e não altera venda/caixa/estoque/financeiro.
Schema72 e migrações/XSDs anteriores permanecem intactos.

## Fontes oficiais consultadas

- MOC7.0, AnexoII, DANFE e código de barras:
  https://www.confaz.fazenda.gov.br/legislacao/arquivo-manuais/moc7-anexo-ii-manual-especificacoes-tecnicas-danfe-codigo-barras.pdf
- NTC2025.001 v1.00, CNPJ alfanumérico, seção6 e tabela de símbolos:
  https://www.cte.fazenda.gov.br/portal/exibirArquivo.aspx?conteudo=cxpT6%2F6wST8%3D

A nota conjunta amplia o código para combinação C/A e aumenta a largura
mínima. Há uma inconsistência na frase que menciona código100 para alternar
A/C: os símbolos padronizados são101 para entrar emA e99 para voltar aC;
100 selecionaB. A implementação usa os símbolos da tabela e documenta essa
diferença, sem alegar validação oficial do leiaute.

## Limites e instalação

Saída HTML estática de homologação, com marca sem valor fiscal e hashes.
Não é DANFE55 de produção homologado, PDF nativo, cópia cancelada, paisagem,
etiqueta, EPEC, FS-DA ou contingência. A diagramação técnica precisa de aceite
formal completo e ensaio em impressora/leitor físicos antes de uso operacional.
Os grupos tributários provêm do XML e não demonstram motor nacional completo.

Emissor de produção, regras tributárias nacionais e RTC aprovadas, contingência,
SPED/PVA, NFS-e de produção e homologação oficial continuam pendentes. Nenhum
certificado real, banco comercial ou credencial foi aberto. Não há mudança de
frontend/Electron, instalação de executáveis, reinício dos serviços8182/8282 ou
merge na main. Publicação somente na branch fiscal de trabalho. O código está na worktree isolada, não no
atalho instalado.

Resultados finais e hashes: `evidence/fiscal-danfe55-20261028/verification.json`.
