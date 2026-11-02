# Tributação aplicada ao XML — 29/10/2026

Base `8a98d2a158d0c3cd7f1cdfb78231cdd2dc40d55a`, mesma branch fiscal/PR5.
Esta unidade amplia o backend SQLite/API local. Não altera frontend, Electron,
instalações, serviços existentes ou bancos comerciais. Não há nova migração;
as75migrações e os schemas oficiais anteriores permanecem intactos.

O código anterior persistia regras e cálculos, mas ainda exigia copiar seus
valores para o XML. Agora uma revisão pode citar a memória original em cada linha
por `TaxQuoteOperationID`. O backend verifica a fonte e monta os grupos, NCM/CFOP,
CEST, cBenef e FCI suportados. Conteúdo conflitante fornecido pelo cliente é erro;
não é sobrescrito silenciosamente. Isso não constitui cobertura nacional completa.

## Enquadramentos representados

Para venda doméstica, há ICMS CST00/10/20/40/41/50/70 em CRT2/3 e
CSOSN102/103/300/400 em CRT1. O código confere regime, método, redução de base,
ST e indicadores de soma ao total. Não converte código do Simples em CST normal.
CRT4, crédito do Simples, diferimento e retenção anterior exigem escopos próprios.

PIS/COFINS: CST01/02 por percentual,03 por quantidade,04–09 sem destaque
matemático e49/99 por percentual, quantidade ou valor zero aprovado. O método
matemático `exempt` não define sozinho isenção legal: a natureza é a do CST
declarado. Não há classificação automática de incidência monofásica por NCM.

IPI: CST50/99 tributados por percentual ou quantidade;51–55 sem destaque
matemático. `cEnq` deve vir da aprovação. O sistema não preenche999 por padrão.
O escopo IPI deve ser explícito: `classified` ou `not_applicable`. FCP também
exige escopo explícito, distinguindo próprio, ST, ambos e não aplicável.

São representados FCP próprio, FCP-ST e ST por MVA, com bases e somas separadas.
O FCP-ST deve usar a mesma base ST representada; desconhecimento de retenção
anterior não produz valores presumidos de CST60/CSOSN500.

## Referência interestadual e MVA ajustada

`ordinary_goods_2026` é uma referência **nominal** de ICMS interestadual para
mercadorias comuns, datada e limitada ao período2026. Contempla os27estados/DF,
com7% nas saídas deMG/PR/RJ/RS/SC/SP para os demais,12% nas outras rotas e4%
nas origens1/2/3/8 previamente classificadas. O algoritmo não determina conteúdo
de importação, exceção CAMEX, PPB ou incidência de um benefício. Origens4/6/7
precisam representar exceções corretamente classificadas pelo responsável.
Operação interna, combustível/energia (capítulo27), modelo65 interestadual,
alíquota divergente e período fora da referência são recusados nesse modo.

Para origens3/5/8 nesse modo, exige FCI em formato UUID ou referência explícita
de dispensa revisada; não cria números nem verifica registro da FCI no fisco.
Os códigos cBenef, NCM/CEST e municípios têm validação estrutural, não consulta
automática aos cadastros/tabelas de vigência de todas as UFs.

A MVA ajustada é opcional e depende de escolha expressa da regra aplicável:
`[(1 + MVA original/100) × (1 − alíquota interestadual/100) /
(1 − alíquota interna/100) − 1] ×100`.
O registro conserva entrada, fração exata e percentual aplicado em quatro casas
antes da base. Não busca MVA original por NCM nem presume acordo de ST.
Alíquotas do ajuste devem corresponder às da regra de ICMS próprio/ST; não se
aplica MVA original e ajustada simultaneamente. Resultado negativo, divisão por
zero, referência/precisão ausente e contexto interno são recusados.

## DIFAL e FCP de destino

`cv236_21_single_base_2026` representa o caso restrito de venda interestadual
NF-e55 a consumidor final não contribuinte, CRT2/3, CST00, sem benefício,
IPI/ST ou mudança especial de base. A política declara base única com preço
da operação já incluindo o imposto do destino. Não é uma função de formação
automática de preço nem um cadastro de alíquotas internas.

O componente `DIFAL` aplica a alíquota interna **explicitamente aprovada** e
deduz o ICMS próprio calculado à alíquota nominal interestadual. FCP de destino
fica no componente separado `FCPUFDEST`; seu escopo precisa declarar incidência
ou não aplicação. Este modo admite o adicional de até2pontos do convênio.
A parte do destino é100% no período suportado. Os valores não são somados outra
vez a `vNF`, pois integram o preço declarado. Totais e detalhes são reconciliados.

O XML contém `ICMSUFDest` e os três totais correspondentes. Não mistura o FCP
com a diferença de ICMS. Base dupla, benefícios do Convênio52/91 e demais
exceções não são convertidos para esse método. Venda interestadual a consumidor
final sem a classificação de destino compatível não recebe XML parcial nesse
fluxo. Não cria/paga GNRE, não liquida tributo e não movimenta banco.

## Vínculo ao emissor e preservação

Em `POST /fiscal/intents/:id/production/prepare`, envie
`Invoice.Lines[].TaxQuoteOperationID` em todas as linhas. O cliente pode omitir
ou deixar nulos `Taxes`/`TaxTotals` para preenchimento. Valores já informados
precisam corresponder exatamente à fonte. Descrição, unidade, quantidade, preço,
emissor e destinatário continuam sujeitos às verificações econômicas/contratuais.

O servidor exige mesma empresa, loja, aparelho, produto, perfil/revisão,
classificação do destinatário, origem/destino e data atribuída à nota. Dinheiro,
quantidade e preço devem coincidir com a venda persistida. Regras inativas ou
superadas bloqueiam novas revisões, aprovação e envio; essa verificação é refeita
imediatamente antes do POST, pelas guardas existentes. Não libera número reservado
nem apaga uma operação incerta. Revisões e recibos antigos permanecem consultáveis.

O modo manual anterior continua distinto e não adquire prova de política aprovada.
Referenciar uma memória sem mapeamento suportado não aciona fallback manual.
Retomar sessão não reautoriza revisão antiga. Reinício e restauração conservam
os hashes e referências; restauração mantém as barreiras contra novo envio.

O contrato comercial existente ainda admite venda em dinheiro sem encargos.
Consequentemente, ter cálculo/XML unitário de ST/IPI não habilita emissão de uma
venda cujo preço/pagamento não represente esses acréscimos. O vínculo econômico
recusa essa combinação, em vez de alterar uma venda já concluída para fazê-la caber.

Valores monetários usam decimais/frações exatos. O mapeador respeita os limites e
escalas do XSD; não trunca alíquota, quantidade ou base para produzir XML diferente
da memória. A referência interestadual, o ajuste MVA e o mapeamento têm versões
datadas: novas versões precisam preservar a interpretação dos recibos antigos.

## Validação e estado real

Os testes específicos verificam combinações de CST/CSOSN, tributação por unidade,
IPI, FCP/ST, DIFAL e MVA ajustada, XML assinado nos XSDs oficiais, recusa de
alterações, regra superada, reinício e backup criptografado. Há6318seleções nominais
interestaduais (27×26×9origens); esse número **não** equivale a6318enquadramentos
legais ou a uma cobertura completa das legislações estaduais.

O ensaio nativo faz10requisições TCP à API real, persiste política/memória/revisão/
aprovação, envia um POST mTLS assinado e recupera o XML sem reenviar. Certificados,
autoridade e dados são descartáveis e declarados como teste. Não houve homologação
ou transmissão operacional à SEFAZ. O CI exige esse ensaio, o circuito tributário
e DIFAL/FCP como integrações adicionais obrigatórias.

A compatibilidade foi comparada com o CLI da unidade anterior: a cotação genérica
de referência produz os mesmos bytes e hash. Campos novos omitidos preservam a
serialização histórica. Resultados finais, skips, hashes e limites constam em
[verification.json](evidence/fiscal-tax-classification-20261029/verification.json).

Continuam sem cobertura integral: catálogo/versionamento nacional de NCM/CEST/
cBenef/cClassTrib e enquadramentos, RTC IBS/CBS/IS, ICMS monofásico, créditos,
desoneração/diferimento, retenção anterior, todos os acordos de ST e DIFAL especial,
Simples/DAS/apuração, importação/exportação e ISS/NFS-e. Tampouco conclui DANFE,
contingência, SPED/PVA ou homologação oficial. Alíquotas internas, MVAs originais
e benefícios não são deduzidos só pela UF ou pelo NCM. Não marcar “fiscal completo”.

## Fontes oficiais usadas

- [Resolução22/1989 — Câmara](https://www2.camara.leg.br/legin/fed/ressen/1989/resolucao-22-19-maio-1989-481183-publicacaooriginal-1-pl.html).
- [NT2013.006 — NF-e, origens e FCI](https://www.nfe.fazenda.gov.br/Portal/exibirArquivo.aspx?conteudo=slmjSi5sbdk%3D).
- [Referência de alíquota nominal — EconomiaGO](https://orientacaotributaria.economia.go.gov.br/spo-web/perguntasfrequentes/perguntafrequente/21629).
- [ConvênioICMS236/21 — CONFAZ](https://www.confaz.fazenda.gov.br/legislacao/convenios/2021/CV236_21).
- [DIFAL e benefícios — EconomiaGO](https://orientacaotributaria.economia.go.gov.br/spo-web/perguntasfrequentes/perguntafrequente/20468).
- [Resolução Normativa72 — SEF/SC, fórmula MVA](https://legislacao.sef.sc.gov.br/consulta/views/Publico/DocumentoLegalViewer.ashx?id=44C2D0ED-75AE-4366-83FA-F7FAD9C265F7).
- [Árvore oficial NF-e — SEF/MG](https://portalsped.fazenda.mg.gov.br/spedmg/nfe/download/EstruturaNFe.html) e XSDs oficiais já versionados.

Fontes de políticas passam a aceitar HTTPS de órgãos `*.leg.br`, além de
`*.gov.br`, sem aceitar credenciais na URL, porta explícita ou hosts disfarçados.
Isso valida procedência declarada, não a interpretação jurídica de cada política.
