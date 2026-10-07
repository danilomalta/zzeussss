# P02 — receitas versionadas no nucleo local

Base fornecida: `8fdf5fe77ed11360df56a4f4c569bd8a8621e402`, branch de destino
`feat/producao-receitas`. O pacote fonte nao inclui `.git`; o historico do PC
nao foi consultado. P01 documental pode ser preservada e registrada com P02.

| Etapa | Estado | Base | Arquivos/migracao | Verificacoes | Dependencias |
| --- | --- | --- | --- | --- | --- |
| P01 | Documento preparado no historico; registro no PC nao confirmado | 8fdf5fe | P01_BASE_E_DEPENDENCIAS.md, se presente no PC | Nao disponivel no pacote fonte | Preservar documento existente |
| P02 | Entregue para aplicacao; testes Go pendentes no PC | 8fdf5fe | production/recipes.go, testes, localapi/production.go, server.go, SQLite 0028 | SQL 0001–0028 aplicado em SQLite descartavel; foreign_key_check sem erros; patch verificado. Go indisponivel neste ambiente | Executar gofmt, go test, race e vet no PC |
| P03 | Pendente | — | — | — | Capacidade e conversoes exatas |

## Contrato de dominio

Uma receita pertence a empresa e loja. Cada publicacao acrescenta uma versao
imutavel por meio do servico; nao existe endpoint para editar/excluir versoes.
`recipe_id`, `version_id` e `operation_id` sao definidos antes do envio.
`expected_revision=0` cria a receita; demais publicacoes exigem a revisao atual.
Versoes publicadas nunca sao reescritas por uma revisao posterior ou por mudanca
de nome/unidade no catalogo. Ordens em P04 referenciarao `version_id`.

Rendimento e ingredientes sao inteiros entre 1 e 9007199254740991, em milesimos
da unidade informada: 500000 de `g` representa 500 g; 10000 de `unit` representa
10 unidades; 100001 de `ml` representa 100,001 ml. Nao usar numeros decimais no
JSON. A revisao vai de 1 a 2147483647. Uma versao possui de 1 a 100 ingredientes,
produtos distintos, sem o produto de saida entre eles. Cada produto deve existir
na empresa; unidade deve coincidir com o catalogo na publicacao. Unidades:
`unit`, `kg`, `g`, `liter`, `ml`, `meter`. Conversoes e densidades sao P03;
esta entrega recusa uma unidade diferente em vez de converter silenciosamente.

Nome (1–255 bytes apos trim), ingredientes ordenados por produto e todos os
demais campos formam o pedido canonico. Mesma operacao/aparelho/empresa, mesmo
autor/loja e mesmo conteudo devolvem a versao original, inclusive depois de
novas revisoes. Outro conteudo ou autor gera conflito. Nova operacao nao pode
reutilizar `version_id` na loja. Ordem dos ingredientes nao muda o conteudo.

Publicacao revalida `ManageProduction`, aparelho aprovado e contrato assinado
`Production` com dependencias `Core` e `Inventory`, dentro da transacao.
Repeticao tambem exige contrato vigente. Leitura exige a mesma permissao humana
e aparelho, mas preserva consulta depois de vencimento do contrato. Papel
`production` e dono possuem essa permissao no codigo existente; gerente nao a
recebe automaticamente. Delegacoes seguem a politica existente.

Cabecalho da receita, versao, ingredientes, auditoria, observacao temporal do
contrato e outbox confirmam juntos. Toda gravacao exige exatamente uma linha;
falha SQL ou trigger que ignora a gravacao provoca rollback. Nao movimenta
estoque, calcula capacidade, reserva materiais nem executa ordens nesta etapa.

## API local autenticada

Base `/local/v1`; autenticacao Bearer existente. Empresa, loja, pessoa e aparelho
vem da sessao/estacao, nunca de campos do pedido. Publicacao aceita ate 64 KiB.
Objetos completos e exatos em ambos os niveis: rejeita campos extras, repetidos,
ausentes, null, JSON posterior e quantidades fracionarias.

| Metodo | Caminho | Resultado |
| --- | --- | --- |
| POST | /production/recipe-versions | 201 nova versao; 200 repeticao |
| GET | /production/recipe-versions?recipe_id=ID&limit=50&offset=0 | items de versoes historicas e atuais; filtro opcional; limit 1–100 |
| GET | /production/recipe-versions/:version_id | Snapshot duravel de uma versao |

Erros: 400 pedido invalido; 401 sem sessao; 403 permissao/contrato recusado;
404 versao inexistente no escopo; 409 revisao/ID/conteudo conflitante ou recuo
temporal; 413 corpo excedido; 503 publicacao sem verificador configurado;
500 falha interna, sem expor SQL. Queries extras e repetidas sao recusadas.

Exemplo de pedido (IDs de produtos ja cadastrados):

```json
{"operation_id":"receita-op-1","recipe_id":"pao","version_id":"pao-v1","expected_revision":0,"name":"Pao","output_product_id":"produto-pao","output_unit":"unit","yield_milli":10000,"ingredients":[{"product_id":"produto-farinha","unit":"g","quantity_milli":500000}]}
```

Depois de timeout, consultar `GET .../pao-v1`; consulta falhada nao comprova
ausencia e nao deve disparar escrita automatica. Caso seja necessario repetir,
preservar o pedido e os IDs originais. Nao gerar outro ID para esconder incerteza.

## Coordenacao com Chat A

- SQLite `0028_production_recipes.sql` reservado no plano e livre nesta base.
  Nenhuma migracao PostgreSQL ou de autenticacao foi modificada.
- Arquivo de runtime compartilhado: apenas uma chamada `mountProduction` em
  `localapi/server.go`. Testes compartilhados de migracao/backup atualizados
  para 28, incluindo restauracao a partir da versao 27. Testes de falha usam 29.
- Evento outbox `production.recipe.published`, schema 1, aggregate = version_id;
  payload = snapshot completo Version, autor e horario, sem preco/custo ou
  credenciais. A recepcao/aplicacao desse evento em outro aparelho ainda nao foi
  implementada. Nao conceder pareamento para ele nem afirmar sincronizacao.
- Auditoria em `production_recipe_audit`, por publicacao. Nao integra ainda a
  consulta administrativa unificada `/access/audit`; consulta SQL administrativa
  local e os testes permitem verificar a trilha.
- Frontend, permissoes, capacidades, middleware online e estoque preservados.
  Nenhum percentual de ERP concluido ou aceite de producao integral e alegado.

## Verificacao e demonstracao

Testes `TestHTTPProduction*` usam API Fiber real e SQLite descartavel com contrato
de teste assinado. Cobrem snapshots, revisoes, replay, JSON ambiguo, limites,
permissoes/papel production, contrato ausente/expirado, outro escopo, revogacao,
falhas ABORT/IGNORE em cada tabela/outbox, rollback temporal, concorrencia e
fechamento/reabertura do banco. `TestNormalize*` cobre limite e canonicalizacao.
O fluxo `TestHTTPProductionImmutableVersionsReplayAndConflicts` demonstra
publicar v1 → repetir → publicar v2 → consultar v1 intacta → paginar versoes.

O script do pacote executa testes completos, race nos pacotes afetados, vet e
build do executavel local em diretorio temporario, antes do commit. Essas
verificacoes Go NAO foram executadas no ambiente que gerou o patch, por falta
do toolchain. O estado so muda para confirmado no PC depois da saida aprovada.
