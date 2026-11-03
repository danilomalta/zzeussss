# Fiscal: componentes locais e limites de entrega — 26/10/2026

## Base, destino e decisão do proprietário

Destino: `/home/danillo-malta/Downloads/zzeus-frontend-backend-b02a`.
Branch `feat/backend-b02a-compat`, HEAD
`327002939a517a314864cb2d42cfb984ca09f79e`, com preparação fiscal67 e demais
alterações locais anteriores preservadas. Frontend, Electron, bancos existentes
e serviços8182/8282 não foram alterados.

O proprietário informou que não possui certificado nem emitente piloto e exige
preparação para clientes de qualquer UF. A identificação das27UFs foi coberta,
sem escolher empresa, certificado, banco, regime ou alíquotas em seu lugar.
Configuração multijurisdição não constitui homologação em todos os estados.

## Componentes acrescentados

- `backend/internal/fiscalcore/tax.go`: cálculo com `big.Rat`, entrada decimal
  textual e arredondamento monetário explícito. Regra deve corresponder a
  produto, origem/destino, CRT, modelo e vigência inclusiva, com referências
  legal e de aprovação. Percentual, valor por unidade, isenção explícita,
  redução de base, MVA, inclusão/exclusão de tributos anteriores e dedução
  de imposto próprio. Ausência/ambiguidade da regra causa recusa, nunca zero.
  Isso é um executor matemático de políticas, não um classificador fiscal nem
  uma biblioteca nacional de regras e benefícios pronta.
- `nfe.go`, `totals.go`: chave de acesso com DV e CNPJ alfanumérico, modelos55/65,
  dados serializados como texto escapado, centavos exatos e troco. Conferência
  das somas legadas ICMS/PIS/COFINS/IPI/II contra os itens; grupos não mapeados
  são recusados, inclusive RTC/IBS/CBS, ISSQN, partilha e devolução tributária.
  Não há reserva de numeração, integração automática do cálculo ao cadastro
  ou suporte integral a CST/CSOSN/operações especiais. Frete/transporte está
  limitado ao modo9 do construtor; não representa logística fiscal completa.
- `xml.go`: pré-validação limitada de tamanho/profundidade/atributos/IDs,
  rejeição de DTD/entidades externas, compilação XSD de arquivos embutidos sem
  busca pela localização sugerida no XML. XMLDSig RSA/SHA-1 conforme perfil
  legado fiscal, C14N1.0, URI exata e vínculo do emitente ao SAN e-CNPJ.
  Verifica certificado/chave, cadeia explicitamente confiada, validade,
  conteúdo assinado, totais e consistência da chave com os campos internos.
  Intermediárias são separadas das raízes; não são promovidas a âncoras.
  Não fornece PFX/A3/PKCS11, renovação, provisionamento ICP-Brasil ou revogação.
- `nfse.go`: serialização candidata de DPS nacional1.01, envelope JSON
  gzip/base64 e descompressão limitada de NFS-e. **DPS permanece bloqueada na
  validação**, conforme incompatibilidade abaixo. Não é cobertura de todos os
  provedores municipais. CNPJ alfanumérico é recusado pelo construtor DPS de produção
  porque o identificador do esquema fixado é numérico; letras não são removidas.
  Evento de cancelamento NF-e110111 possui construtor e validação do detalhe
  oficial, assinatura e vínculo da chave ao emitente. Outros eventos oficiais
  e assinatura de eventos NFS-e permanecem indisponíveis.
- Perfil **separado de homologação** nacional20/10/2026: `BuildDPSHomolog`,
  `SignXML("nfse_dps_homolog_20261020", ...)` e `PackDPSHomolog`. Usa o novo
  pacote oficial restrito, sem modificar os arquivos do pacote de produção.
  Inclui `finNFSe` explícito, aceita CNPJ numérico/alfanumérico e exige ambiente2;
  séries90000–99999 são recusadas. Construção, assinatura e envelope passaram
  com certificados sintéticos. O perfil não é selecionado automaticamente pela
  API, não é transmitido pelo CLI e não habilita emissão de produção.
- `qr.go`: parâmetros NFC-e QR2 e3, online e offline, assinatura do perfil3
  offline, hash com CSC no perfil2 e PNG com margem de quatro módulos. A URL
  deve ser fornecida pela configuração confiável de UF/ambiente. CSC não aparece
  no resultado. Isso não implementa o ciclo de contingência: autorização de
  entrada, fila, prazos, retransmissão/reconciliação e saída ainda faltam.
  O suplemento/QR ainda não é incorporado automaticamente ao XML transmitido.
- `transport.go`, `journal.go`: cliente técnico TLS1.2+ com certificado mútuo,
  validação do servidor, sem proxy herdado, redirecionamento ou reenvio automático.
  SOAP de autorização NF-e e cancelamento; envelope de DPS disponível somente
  quando a DPS passar a validação. Destino HTTPS governamental é configuração
  confiável, não entrada do navegador. Não existe catálogo nacional de endpoints
  versionado/homologado neste incremento. Pedidos e respostas ficam em arquivos
  privados, exclusivos e sincronizados em disco. Uma repetição de ID é recusada;
  resultado incerto preserva a requisição original. Resposta HTTP recebida não
  significa autorização. A leitura de protocolo exige associação exata de
  chave/ambiente/digest, formato previsto e status100/150; rejeita SOAP Fault,
  status pendente/duplicidade e conteúdo escondido em envelopes arbitrários.
  A origem autenticada deve ser garantida pelo chamador do parser.
- `json.go`: rejeição de campos duplicados/colidentes por caixa, excesso de
  profundidade e valores JSON adicionais; CLI/journal rejeitam campos desconhecidos.
- `cmd/titan-fiscal`: ferramenta **offline**, com comandos `capabilities`,
  `tax-quote`, `build-nfe`, `build-dps`, `build-dps-homolog` e `validate`. Não recebe certificados ou
  segredos e não transmite. Saída de arquivo novo0600 com `O_EXCL` e fsync;
  não sobrescreve. Entrada deve ser arquivo regular limitado; link é recusado.
  `validate` verifica XSD, sem afirmar assinatura/autorização/homologação.
  `build-nfe` produz XML ainda não assinado: o XSD completo requer assinatura,
  cuja validação só ocorre por `SignXML`; não chamar esse artefato de nota emitida.

## Incompatibilidade DPS/NFS-e reproduzida

No XSD nacional1.01 baixado do portal oficial,
`tiposSimples_v1.01.xsd`, `TSSerieDPS` declara:

```
<xs:pattern value="^0{0,4}\d{1,5}$"/>
```

A combinação fixada de esquema e validador recusa a série sintética `1` com:

```
Element '{http://www.sped.fazenda.gov.br/nfse}serie': [facet 'pattern']
The value '1' is not accepted by the pattern '^0{0,4}\d{1,5}$'.
```

O teste `TestFiscalDPSKnownSchemaCompatibilityGapFailsClosed` prova essa recusa;
seu PASS não comprova emissão de DPS no perfil de produção fixado. Não alteramos o XSD oficial, não removemos
restrições e não assinamos/transmitimos o documento rejeitado. A correção exige
resolver a interoperabilidade e comparar com a validação nacional. Este registro
não atribui o defeito a uma parte sem a prova correspondente. O Swagger de
produção restrita retornou `HTTP Error 403: Forbidden` sem certificado; não houve
ensaio da API oficial nem comprovação do envelope atual contra aquele Swagger.

## Perfil de homologação atualizado, sem alterar produção

A página oficial de homologação disponibiliza um pacote diferente, publicado em
20/10/2026. Seus XSDs corrigem o padrão de série e acrescentam campos obrigatórios
como `finNFSe`; não basta trocar a expressão regular do pacote antigo. Esse
pacote foi baixado, validado e embutido em pasta própria, com hashes no manifesto.
O XML gerado passou no XSD oficial inalterado, na assinatura e no envelope gzip
para o perfil de homologação. Os testes recusam troca para ambiente1, série
reservada, adulteração de valor e uso silencioso desse documento pelo perfil
antigo. Isso resolve a geração local de DPS para o perfil de homologação; não
é aceite do serviço nacional nem solução do pacote de produção antigo.

O primeiro download direto recebeu `HTTP Error 403: Forbidden`; abrir a página
pública e usar sua sessão normal de cookies resultou em HTTP200 para o ZIP.
Não foram usados login, certificado, senha ou contorno de autenticação.

## Dependências e CI

`go.mod` agora usa mínimoGo1.26.2 e toolchainGo1.26.10 porque o validador puro Go
adicionado exige essa geração. Bibliotecas: helium0.11.0, etree1.7.0,
goxmldsig1.6.1 e barcode1.1.0; dependências transitivas e checksums registrados.
Não foi substituído o Go do sistema ou qualquer executável dos serviços.
Os dois workflows usam `go-version-file: backend/go.mod`. Foi removida apenas
a instalação não utilizada de golangci-lint1.55.2 por curl; `go vet` permanece.
Sem push/publicação, nenhuma execução remota nova é alegada.

Durante a seleção do validador, versões de jacoelho/xsd foram descartadas por
incompatibilidades de compilação dos esquemas fiscais; não integram a entrega.
O helium compilou os esquemas e validou os XMLs sintéticos NF-e/NFC-e/evento.
Compilar esquemas não prova todas as regras de negócio nem todas as notas técnicas.

## Proveniência dos esquemas

`schemas/manifest.json` registra URLs públicas, SHA-256 dos ZIPs oficiais e
hashes de cada XSD copiado. Há34XSDs embutidos; somente `evento_root.xsd` é um
vínculo de raiz gerado para o tipo oficial `TEvento`, declarado separadamente
no manifesto. Nenhuma restrição dos XSDs oficiais foi editada.

- [Pacotes NF-e](https://www.nfe.fazenda.gov.br/portal/listaConteudo.aspx?tipoConteudo=BMPFMBoln3w%3D): PL_010g_v1.03,
  SHA-256 `b329ecbe7e4c05a41bfbc1527f105defb7047328acfeada7c77e1666da485eaf`.
  O portal exigiu sessão de cookies; o acesso simples retornou ciclo de redirecionamento.
- [Documentação nacional NFS-e](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/documentacao-atual/documentacao-atual):
  XSD1.01-20260209, SHA-256 `e7935cbd9470527c6cc32984c1b2263e614183bf0139ce2733eaaed2de9a8072`.
- [Homologação nacional NFS-e](https://www.gov.br/nfse/pt-br/biblioteca/documentacao-tecnica/producao-restrita):
  `esquemas_prod_restrita_20261020.zip`, SHA-256
  `25fdcecc651a67a216496ab10d7566631e6ed10f721e37d62c7845dadb0ea267`.
  Arquivos em `schemas/nfseHomolog20261020`, com produção preservada em `nfse101`.
- [Manual QR NFC-e6.0](https://www.nfe.fazenda.gov.br/portal/exibirArquivo.aspx?conteudo=k%2FIuuaW4YiY%3D):
  parâmetros QR2/3; o teste verifica a assinatura offline gerada sinteticamente.

Os testes de chave cobrem27UFs, inclusive cálculo de DV para CNPJ alfanumérico.
Isso não significa regras tributárias ou credenciamento homologados em27UFs.

## Integração ainda não entregue

A API de preparação fiscal67 permanece com `transmission_available=false`,
`nfse_available=false` e demais capacidades oficiais negativas. Ela não carrega
certificados, reserva números ou chama o cliente técnico. Não foi colocada uma
biblioteca não homologada para emitir no banco de uma empresa.

O journal em arquivo NÃO integra o backup SQLite. Antes de habilitar qualquer
emissor, mover/integrar pedidos e respostas à fila fiscal transacional com
isolamento tenant/loja/aparelho, reserva de numeração, auditoria, consulta e
recuperação, e testar backup/restauração do conjunto completo. Não usar o
journal isolado como solução de produção. O estado incerto não deve ser limpo
ou reenviado por logout, troca de contexto, reinício ou recuperação de backup.

Faltam também classificação/políticas legais completas e RTC, mapa de endpoints
por UF/modelo/ambiente/vigência, consulta oficial e reconciliação de protocolo,
recibos/denegação/rejeição, eventos adicionais, ciclo de contingência, DANFE/QR
incorporado e impressão oficial, escrituração SPED com livros/PVA, NFS-e e
provedores municipais e homologação real. Essas pendências não são resolvidas
somente apresentando um certificado. Certificado/credenciamento são necessários
para a etapa oficial, mas há implementação independente ainda a concluir.

## Aplicação, reversibilidade e resultados

Aplicação protegida por HEAD/branch, hashes de cada arquivo preexistente,
recusa de novos caminhos já existentes/links e backup anterior. Plano e cópias
originais ficam no diretório de trabalho `work/fiscal-runtime-20261026`.
Não houve commit, push, novo ZIP, instalação de aplicativo, alteração dos bancos
ou reinício dos serviços8182/8282. Os artefatos compilados são ensaios técnicos.

## Resultados observados na árvore aplicada

- `go test -count=1 -timeout=25m ./...`:58pacotes com testes aprovados,
  16pacotes sem testes, zero pacotes falhos. `internal/localapi`:490,196s.
- Depois da adição do perfil separado de homologação, `go test -race -count=1 -v
  ./internal/fiscalcore ./internal/localdb/fiscal ./cmd/titan-fiscal`:19funções
  aprovadas, sendo15do componente/CLI e4da preparação local anterior.
- `go vet ./...` e `go build ./...` aprovados novamente após esse incremento.
- Conferência do mapa de537rotas e10testes do verificador OpenAPI aprovados.
- Builds técnicos do CLI para Linux/amd64 e Windows/amd64 com CGO desativado
  aprovados. O executável Windows não foi executado em Windows nem instalado.
- `go mod verify`: todos os módulos verificados. `git diff --check` sem erros.

A suíte integral começou antes da adição do perfil de homologação. Essa adição
se limita ao componente/CLI, sem mudança em API/persistência/serviços; seus testes
com race, vet e build completos foram repetidos depois. Os logs preservam essa
sequência, sem atribuir o teste integral anterior ao novo perfil por inferência.

Uma tentativa anterior falhou por checksum transitivo ausente de
`go.yaml.in/yaml/v3` na atualização de testify. A dependência foi registrada e a
suíte integral reexecutada com sucesso. O verificador OpenAPI detectou referências
fiscais ausentes em `routes.openapi.json`; sete caminhos/oito operações foram
ligados ao contrato fiscal existente e o verificador passou. Não foram suprimidos
os testes ou as verificações que apontaram essas falhas.

Evidências: [evidence/fiscal-componentes-20261026](evidence/fiscal-componentes-20261026).
O manifesto registra os hashes dos59arquivos aplicados antes das evidências.
Os27arquivos da preparação anterior que não eram documentos deliberadamente
atualizados foram reconferidos e preservados. Não houve homologação, emissão,
SEFAZ real, provedor municipal, PVA/SPED, DANFE oficial ou uso de certificado real.
