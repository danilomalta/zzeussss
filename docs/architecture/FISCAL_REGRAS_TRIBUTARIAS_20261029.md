# Políticas tributárias persistidas — 29/10/2026

Base: `d2773f3f12cc1e659d0bc13f2f753e41257523bb`, branch
`feat/fiscal-homolog-journal-20261027`, PR5. Backend SQLite/API local.

Esta entrega conecta a calculadora decimal existente a políticas versionadas,
aprovadas pelo proprietário e isoladas por empresa/loja. Não contém um catálogo
completo da legislação tributária nacional. Não se deve interpretar aprovação
do proprietário como validação jurídica automática, nem cálculo como emissão.

## Operação entregue

- `POST /local/v1/fiscal/tax-policies`: publica uma revisão com expectativa da
  revisão anterior, identificador idempotente, perfil fiscal exato, produto real,
  vigência inclusiva, referência da aprovação e fontes oficiais declaradas.
  Exige proprietário ativo, sessão atual, dispositivo autorizado, permissão
  contábil e contrato fiscal. Não amplia permissões de caixa.
- `GET /local/v1/fiscal/tax-policies/:id[?revision=N]`: revisão explícita ou mais
  recente, com verificação dos hashes e metadados. Sem enumeração de outras lojas.
- `POST /local/v1/fiscal/tax-quotes`: calcula um item com uma revisão ativa e
  atual. Grava entrada, resultado, hash da política e ator autenticado na mesma
  transação. Recusa divergência de produto, UF, CRT, modelo, data ou classificação.
- `GET /local/v1/fiscal/tax-quotes/:operation_id`: recupera a memória original
  do aparelho na loja atual. Exige permissão contábil. Não aplica regra mais nova.

A publicação exige decisões booleanas explícitas para ativação e soma de cada
tributo ao total; valores omitidos/nulos não viram `false` silenciosamente.
Reenvio exato retorna o mesmo registro; reutilização do identificador com outra
entrada/ator/loja é conflito. Revisão inativa bloqueia novos cálculos e preserva
recuperação e reenvio idempotente dos anteriores. Alteração do perfil exige nova
aprovação da política. Regras ativas com a mesma chave/classificação e vigências
sobrepostas são recusadas, inclusive quando possuem IDs diferentes.

A classificação exige operação, finalidade, municípios de origem/destino, situação
do destinatário, consumidor final, origem do produto e CEST opcional. Município é
validado por formato/prefixo de UF: isso **não** comprova existência no cadastro
IBGE. CFOP/NCM/CEST também são conferidos por formato, sem inferência de incidência
ou homologação cadastral. Referência HTTPS `*.gov.br` é procedência declarada,
não conteúdo consultado/validado automaticamente. Não há requisição a essas URLs.

## Cálculos e limites matemáticos

Os valores são strings decimais e frações exatas (`big.Rat`), sem `float64`.
Há percentual, redução de base, MVA, adição/exclusão de tributos anteriores,
dedução, valor por unidade e isenção explicitamente aprovada. Não existe regra
padrão zero para contexto desconhecido. Dependências devem apontar para componentes
anteriores, sem ciclos, duplicatas ou referências contraditórias na política.

Foi acrescentado `gross_up`: `base / (1 - aliquota/100)`. Serve para uma base
que **ainda não contém** esse tributo; não se deve aplicar automaticamente a
preços que já o incluem. Alíquota de 100% ou maior é recusada. A escolha desse
método permanece na política aprovada, não no parâmetro de uma venda.

O modo de arredondamento da base é obrigatório na publicação:

- `before_tax`: arredonda a base transformada antes do percentual, mantendo o
  comportamento anterior da calculadora.
- `tax_final`: preserva a fração nas transformações de base e arredonda o tributo
  no final. A resposta preserva `unrounded_base` como fração racional exata.

Ambos arredondam os valores monetários iniciais da operação e cada tributo em
centavos, com half-up não negativo. Dependências usam o tributo anterior já
arredondado. Não são um motor RTC oficial nem uma garantia de compatibilidade
com toda regra de precisão de documentos fiscais. Valores fora dos limites são
recusados; multiplicação monetária muito grande deixou de poder causar panic.

## Persistência e recuperação

Migração incremental 75: tabelas `fiscal_tax_policies` e `fiscal_tax_quotes`,
FKs para perfil/política, unicidade de operação por empresa/aparelho e triggers
que impedem atualizar/apagar o histórico. Migrações 1–74 não são modificadas.
Backup aceita schemas 25–75; teste histórico inclui também snapshots 72, 73 e 74.
Restauração não publica nova política nem cria nova cobrança/documento fiscal.
O relatório `pilot_cash_v1` do comando de inspeção continua contando somente
suas tabelas financeiras explicitamente documentadas; não é um resumo tributário.

O ensaio novo usa HTTP loopback em processos reais, SQLite descartável e backup
criptografado. Identificadores e hashes permanecem após reinício/restauração;
sessões anteriores à restauração são recusadas. As alíquotas dos testes são
vetores sintéticos de matemática, jamais padrões pré-carregados para clientes.
O CI exige esse ensaio no gate de integrações e inclui os testes novos no race.

## O que continua sem cobertura completa

Não foi entregue enquadramento automático de todo NCM/CEST/CST/CSOSN/cClassTrib,
benefícios/cBenef, DIFAL/FCP completo por operação, ST/protocolos estaduais,
Simples/DAS, monofásicos, desoneração/diferimento/créditos e ajustes de apuração.
A calculadora pode representar fórmulas explícitas, mas isso não equivale à
implementação e atualização de todas essas regras legais. Importação/exportação,
NFS-e/ISS municipal e tributação de serviços são recusadas nesta publicação.
Não há catálogo nacional de alíquotas nem atualização automática de legislação.

As memórias de cálculo ainda não preenchem automaticamente grupos tributários
do XML do emissor de produção. Este vínculo requer mapeamento validado por modelo
e classificação, incluindo novos grupos RTC. A revisão/aprovação do emissor atual
permanece independente. Esta entrega não conclui DANFE de produção, SPED/PVA,
contingência, NFS-e de produção ou homologação oficial de um cliente.

## Referências oficiais consultadas

- [LC87/1996 consolidada](https://www.planalto.gov.br/ccivil_03/leis/lcp/lcp87.htm):
  bases e hipóteses de ICMS dependem da operação; não aplicar uma alíquota genérica
  de UF como solução de todos os enquadramentos.
- [Convênio ICMS142/2018 consolidado](https://www.confaz.fazenda.gov.br/legislacao/convenios/2018/CV142_18):
  NCM/CEST não substituem verificação de responsabilidade e regras aplicáveis.
- [Documentos NF-e da SVRS](https://dfe-portal.svrs.rs.gov.br/NFe/Documentos) e
  [CST/cClassTrib](https://dfe-portal.svrs.rs.gov.br/CFF/ClassificacaoTributaria):
  classificações têm tabelas e versões próprias; não são deduzidas só pelo NCM.
- [FAQ oficial da calculadora RTC](https://www.gov.br/receitafederal/pt-br/centrais-de-conteudo/publicacoes/apresentacoes/reforma-tributaria-do-consumo/faq-calculadora-v1-4.pdf):
  distingue consulta online de módulo local para ERP.
- [OpenAPI RTC](https://consumo.tributos.gov.br/servico/calcular-tributos-consumo/api/api-docs):
  respostas podem conter `x-warning-dados-simulados`. Nenhuma resposta dessa
  API foi usada como regra válida nesta entrega, e não foi incorporado motor
  oficial offline. Integração futura deve preservar e bloquear resultados
  provisórios, em vez de apresentá-los como tributação concluída.

Contrato completo das quatro rotas:
[fiscal-tax-policy.openapi.json](../api/fiscal-tax-policy.openapi.json).
Resultados observados serão registrados no mesmo registro de continuidade e
no arquivo de evidência desta unidade. Não há ZIP, instalação ou alteração dos
serviços existentes nesta entrega.
