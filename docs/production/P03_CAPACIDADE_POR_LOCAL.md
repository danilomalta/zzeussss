# P03 — capacidade por versao da receita e local

Base confirmada pelo proprietario: `7180c3b`, branch `feat/producao-receitas`,
arvore limpa. P02 registrada em `f343de8` e `7180c3b`; suite completa, vet e
race de production/backup aprovados no PC. A P03 usa o codigo fornecido mais
a P02 confirmada; o pacote original nao inclui historico Git.

| Etapa | Estado | Base | Verificacoes | Dependencias |
| --- | --- | --- | --- | --- |
| P02 | Confirmada no PC | f343de8, 7180c3b | Suite completa, vet, race production/backup | Ativacao/inativacao nao consta desta implementacao; permanece lacuna do escopo ampliado |
| P03 | Entregue para aplicacao | 7180c3b | Patch, shell, JSON e referencia matematica conferidos localmente. Go indisponivel no gerador | Executar testes, vet, race e build no PC |
| P04 | Pendente | — | — | Ordens vinculadas a versao imutavel |

## Regra de calculo

A entrada indica um unico `location_id` da loja da sessao e 1–20 `version_ids`
distintos. Cada versao e recuperada pelo escopo empresa/loja, com seus
ingredientes, rendimento e unidades imutaveis. A transacao verifica permissao
humana `ManageProduction` e aparelho aprovado antes de ler os saldos.

O saldo de cada produto e a soma exata de `stock_movements.quantity_milli`
no local informado, da empresa e loja da sessao. Nao soma locais, empresas
ou lojas. Movimentos compensatorios participam da mesma soma. Saldo ausente
vale zero; saldo negativo, quantidade final acima de 9007199254740991 ou
unidade inconsistente gera 409, sem apresentar capacidade inventada.

Cada quantidade continua em milesimos de sua unidade. Exemplo:
`4000000` de `g` = 4000 g; `4000` de `kg` = 4 kg. Ambos representam a mesma
massa, mas seus inteiros nao sao intercambiaveis. O helper `UnitRatio`
representa kg→g e litro→ml por 1000/1, e a volta por 1/1000. Nao ha densidade
implicita nem conversao de unidade/embalagem, metro, massa e volume entre si.

**Limite do estoque existente:** o produto tem unidade atual, mas os movimentos
nao tem snapshot da unidade. P02 exige unidade da receita igual ao catalogo
na publicacao. Se hoje essas unidades divergirem, mesmo kg/g, o calculo recusa
o saldo: converter movimentos antigos com unidade desconhecida seria incorreto.
Os helpers matematicos de conversao sao testados, mas a API usa somente saldos
com unidade coerente com a receita. Alteracao de unidade exige migracao de
estoque com historico proprio, fora desta consulta.

Para ingrediente i:

`lotes_i = floor(saldo_milli * numerador / (necessario_milli * denominador))`

`lotes_possiveis = min(lotes_i)`

`saida_milli = lotes_possiveis * rendimento_milli_da_versao`

Intermediarios usam `math/big` sem float; a saida respeita o limite exato da API.
Somente lotes completos da quantidade de referencia: saldo fracionario nao e
arredondado para cima, e nao se inventa rendimento parcial. Todos os ingredientes
empatados no minimo sao retornados como limitantes, inclusive em capacidade zero.

## Alternativas e disponibilidade

Todas as versoes solicitadas usam **a mesma leitura transacional** de estoque.
Cada resultado e uma alternativa independente. A resposta marca
`alternatives_independent=true` e `simultaneous_total_available=false`.
Nao fornece total conjunto. Exemplo: estoque permite cinco lotes de v1 OU cinco
lotes de v2 com outro rendimento; isso nao autoriza executar dez lotes.

`basis=local_recorded_balance_without_reservations`: nao existe modelo de
reservas na base atual. Isto e capacidade teorica pelo saldo registrado nesse
local, sem considerar reservas, lotes, validade, equipamentos, trabalhadores
ou estoques globais de aparelhos offline. P05 deve definir a reserva e conectar
esse saldo as saidas antes de prometer disponibilidade comprometivel.

Consulta nao cria entidade, movimento, reserva, auditoria, outbox nem atualiza
o relogio do contrato. Consulta historica segue P02: permissao/aparelho atuais
obrigatorios; preservada depois da expiracao do contrato. Resposta e fotografica,
com `measured_at`; nao e autorizacao para consumir material depois da transacao.

## API autenticada

| Metodo | Caminho sob /local/v1 | Entrada |
| --- | --- | --- |
| GET | /production/capacity | version_id e location_id obrigatorios, exatamente uma vez |
| POST | /production/capacity/alternatives | Objeto exato location_id + version_ids; consulta sem escrita |

```json
{"location_id":"local-producao","version_ids":["pao-v1","bolo-v1"]}
```

Corpo ate 8192 bytes. Rejeita campos extras, repetidos, null, JSON posterior,
IDs vazios/repetidos, mais de 20 versoes e query extra. GET exige os dois
parametros e POST nao aceita query. Uma versao/local inexistente no escopo
gera 404; nao devolve resultado parcial se uma alternativa falhar.

Retorna 200 com `location_id`, `measured_at`, `basis`, marcadores de alternativas
e `alternatives`. Cada alternativa inclui IDs, revisao, produto/unidade de saida,
rendimento por lote, lotes possiveis, saida exata, IDs limitantes e materiais:
produto, unidade da receita/estoque, quantidade necessaria/saldo em milesimos,
razao de conversao, capacidade individual e marcador de limitante.

Erros: 400 entrada invalida; 401 sem sessao; 403 permissao recusada; 404 versao
ou local nao encontrado; 409 unidade/saldo/capacidade inconsistente ou overflow;
413 corpo excedido; 500 falha interna. Nenhuma mensagem exibe SQL ou credenciais.

## Arquivos e coordenacao

- Novos arquivos de production: `capacity.go`, `capacity_test.go`.
- Novos arquivos de API: `production_capacity.go`, `production_capacity_test.go`.
- Compartilhado: uma chamada `mountProductionCapacity(protected)` em
  `localapi/server.go`, depois de `mountProduction`.
- OpenAPI separado `docs/api/production-capacity.openapi.json`.
- **Nenhuma nova migracao**, SQLite permanece 0028; nenhum arquivo de backup,
  autenticacao, PostgreSQL, estoque ou frontend foi alterado.
- Nao introduz evento, sincronizacao ou contrato de reserva. Ordem futura deve
  repetir a verificacao dentro da transacao de reserva; nao confiar neste GET.

## Testes e demonstracao

`TestHTTPProductionCapacity*` usa API real sobre SQLite descartavel e verifica
local, limitante/empates, quantidades fracionarias, versoes historicas/alternativas,
ausencia de escritas e observacao temporal, ambiguidades, limites, tenant/loja,
sessao/papel/aparelho, saldo negativo, overflow, unidade alterada, soma com
intermediarios acima de int64, consultas concorrentes e reabertura.
`TestCapacityExactUnitRatiosAndBatchFloor` verifica conversoes racionais,
dimensoes recusadas, arredondamento e overflow.

Demonstracao automatizada: `TestHTTPProductionCapacityExactLocationAndLimitingMaterial`
usa 4000 g de farinha, 500,005 ml de oleo e receita de 500 g + 100,001 ml para
10 unidades. Mostra cinco lotes (50 unidades), oleo limitante. Outro local nao
altera esse resultado. O teste de alternativas publica v2 e comprova que ambas
leem os mesmos saldos sem consuma-los nem criar um total simultaneo.

Testes Go nao executados no gerador por ausencia do toolchain. Script testa
primeiro P03, depois suite completa uma vez, vet, race focado em P03 e build
antes do commit. Os testes nao iniciam servidores externos nem usam bancos reais.
