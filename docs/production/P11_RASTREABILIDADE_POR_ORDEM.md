# P11 — rastreabilidade consolidada por ordem

Base efetiva: **bb7d923**, branch **feat/producao-receitas**, P10 registrada
com árvore limpa. Consulta do backend, sem interface nova.

**Nenhuma migração nova.** SQLite permanece no schema **35**. Migrações,
compatibilidade, formato, criptografia e validações do backup não são alterados.

## Contrato da consulta

GET /local/v1/production/orders/:id/trace reúne em uma transação de leitura:

| Bloco | Conteúdo |
| --- | --- |
| order | Ordem, local, responsável e snapshot da versão da receita |
| ingredients | Quantidades planejadas, reservadas e consumidas por produto/unidade |
| materials | Contagens active/consumed/released e reserva corrente |
| stages | Plano e estados das etapas, ou null quando não configurado |
| result | Resultado efetivamente registrado, ou null antes da conclusão |
| losses | Totais globais declarados/não classificados da diferença de rendimento |
| lots | Totais globais, página de lotes e último parecer de qualidade por lote |

É uma visão dos registros existentes. Não calcula capacidade, infere saldo
disponível, reserva, consome, produz mercadoria, altera o relógio do contrato,
aprova qualidade ou gera auditoria/outbox por consultar. As APIs P02..P10
mantêm seus contratos de escrita e histórico.

## Planejado, reservado e consumido

planned_milli é a quantidade do ingrediente da receita preservada multiplicada
pelo número de lotes de receita planejados na P04. A validação antecede a
multiplicação: limite 9007199254740991, zero, overflow, duplicidade e versão.
A unidade é a unidade histórica explícita. Não somar produtos/unidades
diferentes nem confundir milésimos da unidade com gramas inteiras.

reserved_milli representa apenas a reserva active da própria ordem P05.
consumed_milli representa apenas seu consumo, com prova dos movimentos
negativos e do snapshot dos ingredientes. Não representa disponibilidade para
outras ordens. Não introduz conversões de massa/volume.

Reservas released contam como histórico, sem contribuir para quantidades
reservadas/consumidas. A reserva corrente é active ou consumed; na ausência de
ambas, current é null. Mais de uma reserva corrente, unidade, quantidade ou
local divergente do plano tornam o relatório inconsistente: 409.

## Resultado, perdas e lotes

Antes da conclusão, result, losses e lots são null. Uma ordem aprovada ou com
consumo não é apresentada como mercadoria concluída. stages é null quando não
houve configuração, preservando ordens legadas da P06.

Depois da conclusão, resultado deve corresponder à ordem, produto/unidade,
local, quantidade planejada, revisão e consumo comprovado. Etapas configuradas
devem estar concluídas e a definição inteira preservada. Diferença de rendimento
continua separada do produto bom efetivamente produzido.

losses.recorded_milli soma perdas recorded P07, excluindo anuladas.
losses.unclassified_milli é a diferença ainda não classificada. Lista detalhada
e motivos permanecem na API P07.

lots.assigned_milli soma lotes recorded P09; unassigned_milli é o produto bom
ainda não identificado em lotes. São classificações históricas, não saldo
físico, reserva ou capacidade. Lotes voided continuam na página.

Cada item contém lot e quality. Qualidade informa estado atual recorded/voided
do lote e último parecer humano; passed histórico não reativa um lote voided.
Sem avaliação: not_assessed, revisão 0, latest null. Não decide qualidade nem
bloqueia venda. Históricos completos continuam na P10.

## Paginação e coerência

Somente lot_offset é permitido: inteiro 0..9007199254740991, uma vez, padrão 0.
A página tem até 50 lotes, inclusive anulados, ordenados por created_at,id.
total_count conta todos os lotes; offset informa a posição e has_more indica
continuação. Página vazia mantém totais globais. Totais de produção, perdas e
lotes são do resultado inteiro; pareceres detalhados são apenas da página.

Todas as tabelas usam o mesmo snapshot SQLite e autorização. Não chamar APIs
públicas que abrem outras transações dentro da consulta: o pool local tem uma
conexão, e transações separadas perderiam coerência. Cursores são fechados antes
das consultas de detalhes. Reavaliação concorrente não mistura revisão e parecer
de momentos diferentes no mesmo item.

Snapshots não são reinterpretados pelo catálogo atual. Alterar nome/unidade
do produto não reescreve receita, resultado ou lote do relatório.

## Autorização e erros

Exige usuário ativo, manage_production, empresa/loja corretas e aparelho
aprovado. Contexto vem da sessão. Histórico autorizado continua legível após
expiração do contrato, sem atualizar module_contract_state.last_observed_unix.
Revogação humana/aparelho continua bloqueando acesso. Sem operation_id: a rota
não escreve nem cria uma operação comercial.

400 para query/ID inválido; 401 sem sessão; 403 sem autorização atual; 404 ordem
ausente no escopo; 409 registros inconsistentes nas validações do módulo; 500
falha interna. Não fabricar resultados, quantidades ou pareceres para completar
blocos ausentes. null e not_assessed representam estados reais.

```text
GET /local/v1/production/orders/order-1/trace
GET /local/v1/production/orders/order-1/trace?lot_offset=50
```

Contrato autocontido: docs/api/production-trace.openapi.json, com schemas dos
blocos históricos e referências locais.

## Arquivos compartilhados e validação

Única mudança compartilhada: server.go monta uma rota GET protegida. Nenhuma
escrita P02..P10, autenticação, sessão, infraestrutura, PostgreSQL, CI ou
compatibilidade do backup é alterada. Sem merge ou push.

Testes usam SQLite temporário e Fiber App.Test, sem iniciar servidores. Cobrem
planejamento, reserva/liberação/consumo, conclusão com etapas, perdas, lotes e
qualidade, lote anulado, unidade histórica/catálogo alterado; páginas completas
e vazias com totais globais; escopos, query estrita, expiração/papel; prova de
consumo e plano inconsistentes; concorrência com reavaliação. Consultar não muda
movimentos, outbox, auditoria ou relógio do contrato.

Backup/restauração verifica receita/ingredientes, ordem, consumo, etapas,
produto acabado, perdas, lote e qualidade. Não exige schema novo. aplicar.sh
confere base bb7d923 e árvore limpa, checksum e aplicação; testa backup/CLI,
P11, suíte completa uma vez, vet, race e build sem executar o binário. Registra
somente os sete arquivos listados após sucesso, com visualizador desativado.

Limites: consulta de uma ordem; sem painel frontend, exportação, custo,
consolidação monetária, saldo físico por lote, FEFO, retrabalho ou planejamento
automático. Não integra outra empresa ou a outra branch.
