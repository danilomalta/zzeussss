# P12 — consulta operacional filtrada de ordens

Base efetiva: **be04563**, branch **feat/producao-receitas**, P11 registrada
com árvore limpa. Evolução da consulta do backend para localizar ordens e,
depois, consultar a rastreabilidade P11. Sem interface nova.

**Sem nova migração.** SQLite permanece **35**, sem edição de migrações,
backup, formato, criptografia ou recuperação. Sem merge ou push.

## API

GET /local/v1/production/order-search

| Query opcional | Significado |
| --- | --- |
| status | planned, approved, cancelled ou completed |
| location_id | Local planejado preservado na ordem |
| responsible_id | ID do responsável preservado na ordem |
| version_id | Versão imutável da receita |
| offset | Posição na lista filtrada, padrão 0 |

Filtros são exatos, combinados com AND e tratados como parâmetros SQL.
Omitir um filtro não restringe aquele campo. Valor explicitamente vazio,
duplicidade, query desconhecida, estado inválido e ID inválido retornam 400.
IDs têm até 128 bytes UTF-8, sem espaços nas bordas, NUL, CR ou LF.
offset aceita somente dígitos decimais, de 0 a 9007199254740991; sem sinal,
fração ou notação exponencial. Não aceita tenant_id ou store_id da query.
Referências ausentes no escopo retornam página vazia, sem revelar existência
de outro local, responsável ou versão em outra empresa/loja.

Resposta: filters, total_count, limit (50), has_more e items. filters repete
os filtros efetivos e o offset. total_count conta todos os registros que
atendem aos mesmos filtros, mesmo fora da página. items sempre é um array,
inclusive quando vazio. Nenhuma soma de quantidades é apresentada: ordens
podem produzir produtos e unidades diferentes.

```text
GET /local/v1/production/order-search?status=approved&location_id=production-room
GET /local/v1/production/order-search?status=completed&offset=50
GET /local/v1/production/orders/order-1/trace
```

Contrato autocontido: docs/api/production-search.openapi.json.

## Estado real e histórico

completed é o estado efetivo da P06: a presença do resultado da própria ordem
no mesmo escopo. A coluna física production_orders.status permanece approved.
Por isso o filtro completed consulta production_results; os demais estados
excluem ordens com resultado. Contagem e página usam o mesmo predicado.
A consulta não escreve completed na coluna antiga nem altera contratos P04/P06.

Cada item mantém a estrutura Order da P04/P06, incluindo snapshot da versão da
receita, quantidades planejadas, local, responsável, revisão e completion_id
quando existir. Catálogo ou responsável atuais não reinterpretam os campos
históricos. A lista não informa reserva, consumo, saldo físico ou qualidade.
Para o histórico detalhado, usar as rotas P05..P11.

O plano preservado é validado antes de retornar a página, reutilizando a
validação exata P11: versão correspondente, rendimento, ingredientes,
multiplicações e limites. Inconsistência nessa validação retorna 409 para a
consulta, sem fabricar um item válido. Isso não substitui a validação da cadeia
completa na P11 nem uma verificação de integridade de backup.

## Coerência, autorização e ausência de escrita

Contagem e página usam uma única transação SQLite, com ordenação determinística
created_at DESC,id DESC, inclusive em empate de timestamp. Não abrem APIs
públicas com outra transação dentro dela. Autorização deriva da sessão e exige
manage_production, empresa/loja corretas e aparelho autorizado. Leitura
histórica permanece autorizada após expiração do contrato, como P04/P11,
sem alterar o relógio do contrato. Permissões atuais continuam obrigatórias.

Uma página representa um snapshot coerente. Requisições distintas podem ver
novas ordens e transições: a paginação por offset não é um cursor persistido.
Pode haver repetição/deslocamento entre páginas se houver escritas concorrentes.
Não prometer uma exportação imutável por várias requisições.

Consulta não grava auditoria, outbox, operação de estoque, reserva, consumo,
resultado, perda, lote ou avaliação. Não recebe operation_id: é uma leitura.
Não altera bancos comerciais ou demonstração; testes usam bancos temporários.

400 query inválida; 401 sem sessão; 403 sem autorização atual; 409 plano
inconsistente nas validações; 500 falha interna. Filtro válido sem registros
retorna 200, total_count 0, has_more false e items [].

## Mudança compartilhada e testes

Único arquivo compartilhado: server.go monta uma rota GET protegida separada,
/production/order-search. Rotas de listagem, criação, estados, histórico e
rastreabilidade anteriores mantêm comportamento. Não altera autenticação,
sessões, infraestrutura, PostgreSQL ou escritores de estoque.

Testes verificam estados efetivos e conclusão P06 sem alterar a coluna física;
filtros exatos combinados, IDs semelhantes a SQL como dados, IDs inexistentes;
limites e páginas de 50 registros, empate de timestamp e página vazia com total
global preservado; snapshots após alteração do catálogo; query estrita,
sessão/papel e contexto de empresa/loja; contrato expirado sem mudar relógio;
contagem/página sob transição concorrente; backup/restauração de receita com
ingredientes e ordem concluída. Fiber App.Test não abre servidor externo.

aplicar.sh verifica base be04563, branch, árvore limpa e checksum, aplica
somente P12, executa backup/CLI, testes específicos, suíte completa uma vez,
vet, race e build sem executar binário. Só registra os sete arquivos listados
após sucesso. Visualizador desativado para não interromper em (END).

Limites: consulta de ordens, sem painel frontend, exportação, filtros por datas,
nome atual de produto, custo, replanejamento ou ações automáticas. Não passa a
tratar saldo, lotes ou qualidade como produção simultaneamente disponível.
