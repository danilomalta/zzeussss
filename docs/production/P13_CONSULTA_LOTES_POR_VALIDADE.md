# P13 — consulta de lotes de produção por validade

Base efetiva: **03f1a5c**, branch **feat/producao-receitas**, P12 registrada
com árvore limpa. Consulta do backend, sem nova interface.

**Sem nova migração.** SQLite permanece **35**. Migrações, compatibilidade,
formato, criptografia e recuperação de backup permanecem intactos.

## Consulta e filtros

GET /local/v1/production/lot-search

| Query opcional | Significado |
| --- | --- |
| status | recorded ou voided, estado atual do lote |
| quality | not_assessed, passed ou failed, último parecer humano |
| product_id | Produto histórico do lote, ID exato |
| location_id | Local histórico do resultado, ID exato |
| expiry | dated para validade declarada; undated para validade desconhecida |
| expires_from | Início inclusivo da faixa de validade declarada |
| expires_to | Fim inclusivo da faixa de validade declarada |
| offset | Posição na lista filtrada, padrão 0 |

Filtros exatos são combinados com AND e passados como parâmetros SQL. Omitir
um filtro não restringe esse campo. Sem status, inclui recorded e voided.
Sem quality, inclui lotes sem avaliação e lotes avaliados. Sem filtros de
validade, inclui datas declaradas e desconhecidas. IDs ausentes no escopo
retornam página vazia, sem revelar dados de outra empresa/loja.

Dates são declarações calendáricas YYYY-MM-DD, com ano 0001..9999 e calendário
válido. expires_from deve ser <= expires_to quando ambos existirem. Limites
são inclusivos; qualquer limite exclui validade desconhecida. expiry=undated
com uma faixa é combinação inválida, retornando 400. Não usa o relógio da
máquina para inventar um limite ou mudar automaticamente o estado do lote.

IDs têm até 128 bytes UTF-8, sem espaços nas bordas, NUL, CR ou LF. offset
aceita somente dígitos decimais, de 0 a 9007199254740991; sem sinal ou fração.
Query desconhecida, valor explicitamente vazio, duplicidade, estado ou data
inválidos são recusados. Não aceita tenant_id, store_id ou limit da query.

```text
GET /local/v1/production/lot-search?status=recorded&expires_to=2026-10-31
GET /local/v1/production/lot-search?expiry=undated&product_id=bread
GET /local/v1/production/lot-search?quality=failed&status=voided
GET /local/v1/production/lot-search?expiry=dated&offset=50
```

Contrato autocontido: docs/api/production-lot-search.openapi.json.

## Resultado histórico, sem disponibilidade presumida

Resposta: filters, total_count, limit (50), has_more e items. Cada item contém
lot P09, order_id, location_id e quality P10. Permite localizar a ordem e
consultar /production/orders/:id/trace da P11. Não soma quantidades de produtos
ou unidades diferentes. Quantidade é classificação histórica do produto bom
registrado, não saldo físico por lote, estoque atual ou capacidade disponível.

O filtro de qualidade usa a maior revisão do parecer no mesmo escopo. Um
passed antigo seguido de failed não satisfaz quality=passed. not_assessed
significa nenhum parecer, não aprovação implícita. voided preserva o histórico
e último parecer; passed histórico não reativa o lote nem libera venda.

expiry=undated significa data desconhecida, não validade infinita. Datas
ordenadas não implementam FEFO: esta API não escolhe lote de venda, bloqueia
saída, reserva, baixa estoque, descarta mercadoria ou cria quarentena.

Não reinterpreta unidade, quantidade, produto ou local com base no catálogo
atual. Confere metadados, quantidades e relação produto/unidade com o resultado
histórico; qualidade valida continuidade das revisões conforme a P10/P11.
Inconsistência nessas validações retorna 409; não fabrica dados para corrigir
o relatório. Não substitui uma verificação completa de integridade do banco.

## Paginação e coerência

Ordenação: datas de validade crescentes, created_at e id crescentes em empate.
Lotes sem data declarada vêm depois dos datados, também com desempate estável.
total_count conta todos os lotes que atendem aos mesmos filtros, inclusive
fora da página. items sempre é array, mesmo vazio. Página vazia mantém a
contagem filtrada. has_more informa existência de registros depois da página.

Contagem, página, resultado e parecer usam uma única transação de leitura
SQLite, com cursores fechados antes de consultas de detalhes. Não chama APIs
públicas que abrirão outra transação dentro dela. Revisões concorrentes não
misturam filtro passed e parecer failed de snapshots diferentes na resposta.

Requisições distintas podem ver alterações, novas avaliações e anulações.
Paginação por offset não é cursor persistido: não garante uma exportação
imutável de várias páginas sob escritas concorrentes.

## Autorização e efeitos

Exige sessão, manage_production, empresa/loja corretas e aparelho autorizado.
Contexto deriva da sessão; correlações de resultado e qualidade incluem
tenant_id e store_id. Histórico continua legível por usuário autorizado após
expiração do contrato, sem mudar last_observed_unix. Permissões atuais são
obrigatórias. Não muda autenticação, sessões ou regras de contrato.

Consulta não escreve auditoria, outbox, movimentos, reservas, resultados,
perdas, lotes ou pareceres. Sem operation_id. 400 query inválida; 401 sem
sessão; 403 sem autorização atual; 409 inconsistência nas validações; 500
falha interna. Filtro válido sem resultados retorna 200 com total_count 0,
has_more false e items [].

## Arquivos compartilhados e verificação

Única mudança compartilhada: server.go monta a rota GET protegida separada,
/production/lot-search. P09..P12 mantêm contratos. Não altera escritores de
estoque, autenticação, sessões, infraestrutura, PostgreSQL ou backup.

Testes com SQLite temporário e Fiber App.Test cobrem datas inclusivas e ano
bissexto, validade desconhecida, qualidade atual após reavaliação, anulações,
filtros combinados, IDs semelhantes a SQL tratados como dados, paginação de
histórico recorded/voided, empate e página vazia; query estrita, sessão/papel,
contexto de empresa/loja e leitura após expiração sem alterar relógio;
catálogo alterado sem reinterpretação histórica; contagem/filtro/parecer sob
concorrência; backup/restauração de lote datado com parecer e vínculo à ordem.
Nenhum servidor de teste usa banco comercial ou demonstração.

aplicar.sh confere base 03f1a5c, branch, árvore limpa e checksum, aplica só P13,
executa backup/CLI, testes específicos, suíte completa uma vez, vet, race e
build sem executar o binário. Registra apenas os sete arquivos listados após
sucesso; visualizador desativado. Sem merge ou push.

Limites: consulta de lotes históricos de produção. Sem painel frontend,
exportação, saldo físico por lote, FEFO, descarte, quarentena, bloqueio de
venda ou cálculo automático de prazo de validade.
