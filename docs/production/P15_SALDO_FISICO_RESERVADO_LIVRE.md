# P15 — saldo físico, reservado e livre por produto/local

Base efetiva: **2a93445**, branch **feat/producao-receitas**, P14 registrada
com árvore limpa. Consulta de estoque integrada às reservas P05, sem interface
nova. **Nenhuma migração nova**: SQLite permanece **35**, backup intacto.

## API e significado

GET /local/v1/stock/availability?product_id=flour&location_id=production-room

Exige sessão, aparelho autorizado e manage_stock, como a consulta de saldo
físico já existente. Não amplia permissões do papel production para consultar
estoque. Contexto de empresa/loja deriva da sessão. Produto deve existir na
empresa e local na loja autorizada; ausência de referência retorna 404.

Somente product_id e location_id são aceitos, obrigatórios, uma vez e não
vazios. Até 128 bytes UTF-8 por ID, sem espaços nas bordas, NUL, CR ou LF.
Query desconhecida, duplicada ou ID inválido retorna 400. Não aceita empresa,
loja, limite, offset, operation_id ou quantidade do cliente.

| Campo | Significado |
| --- | --- |
| product_id, location_id | Referências no escopo autorizado |
| unit | Unidade atual do catálogo, explícita |
| physical_milli | Soma exata dos movimentos registrados, antes das reservas |
| reserved_milli | Soma das reservas P05 active nesse produto/local |
| free_milli | physical_milli menos reserved_milli |

Os três componentes são lidos na mesma transação. Invariante da resposta:
physical_milli = reserved_milli + free_milli. Referências existentes sem
movimentos/reservas retornam zeros. Valores são inteiros 0..9007199254740991,
em milésimos da unidade indicada; não confundir essa escala com gramas.
Não converte volume em massa, aplica densidade ou muda a escala do estoque.

```json
{"product_id":"flour","location_id":"production-room","unit":"g","physical_milli":2000000,"reserved_milli":1500000,"free_milli":500000}
```

Exemplo: 2000 g físicos, 1500 g reservados e 500 g livres. Reservar não baixa
o físico; liberar remove a reserva; consumir baixa o físico e a reserva deixa
de ser active, evitando desconto duplo. Reservas released/consumed ficam no
histórico, mas não contribuem para reserved_milli. Reservas de outro local,
produto, loja ou empresa não são somadas.

Contrato autocontido: docs/api/stock-availability.openapi.json.

## Precisão e inconsistências

Soma os movimentos com precisão arbitrária nos intermediários, sem SQL SUM
de int64. Movimentos muito grandes que se cancelam não provocam overflow
intermediário; saldo final deve caber no limite inteiro seguro. Não arredonda.
Saldo negativo, saldo final acima do limite, reserva agregada inconsistente
ou maior que o físico retornam 409. Não limita silenciosamente a zero nem
fabrica disponibilidade para encobrir divergência.

Usa HeldTx do pacote stockreservation, a regra já utilizada pelos escritores
da P05. Não altera essa regra, reservas, consumo ou proteção dos escritores.

## Unidade atual e limite do esquema existente

Movimentos genéricos de estoque não preservam a unidade original neste schema.
A consulta retorna a unidade atual do catálogo, como a interpretação vigente
do estoque, sem converter números históricos. Pelo contrato público atual,
catálogo cria produtos; esta entrega não acrescenta edição de unidade.

Recusa divergência conhecida entre unidade atual e unidades preservadas em
reservas active/consumed ou resultados de produção daquele produto/local.
Exemplo: reserva preservada em g e catálogo alterado para kg em fixture
descartável resultam em 409; 1500000 g não viram 1500000 kg.

Isso não prova a unidade original de todos os movimentos genéricos. Alteração
externa de unidade sem snapshot conhecido pode ser indetectável. Não anunciar
que esta consulta permite mudar unidade ou reconcilia esse histórico. Uma
futura edição de unidade terá que definir e validar seu contrato de escrita;
P14 auxilia a revisão dos vínculos, limitada à loja autorizada.

## Observação, sem reserva ou aprovação

Esta resposta é observação do estado local. Não é reserva, promessa de saldo
futuro, disponibilidade somada para alternativas simultâneas, saldo físico por
lote, FEFO ou aprovação de venda. Uma escrita posterior deve revalidar saldo,
reservas, autorização e contrato dentro de sua própria transação.

Não escreve movimentos, reservas, operações, auditoria, outbox, relógio do
contrato ou dados comerciais. Leitura autorizada permanece disponível após
expiração do contrato, como a API de saldo anterior; permissões e aparelho
atuais continuam obrigatórios. Requisições distintas podem ver mudanças.

## Contratos compartilhados e erros

Única edição compartilhada: server.go monta uma rota GET protegida separada.
/stock/balance mantém quantity_milli como saldo físico original; não passa a
retornar saldo livre. /stock/operations, contagem, venda, reservas, consumo e
capacidade anteriores mantêm contratos. Nenhum escritor é alterado.

400 query inválida; 401 sem sessão; 403 sem manage_stock/aparelho; 404 produto
ou local ausente no escopo; 409 saldo/unidade inconsistente nas validações;
500 falha interna. Não informa preços, custos ou dados pessoais das reservas.

Sem mudança em autenticação, sessões, infraestrutura, PostgreSQL, migrações,
formato/validações do backup ou bancos comerciais/demonstração. Sem merge/push.

## Verificação

Testes com SQLite temporário e Fiber App.Test verificam reserva/liberação/
consumo sem desconto duplo; outro local e saldo zero; saldo negativo,
overcommit e overflow final; soma com movimentos grandes que se cancelam e
escopos distintos; unidades divergentes em reserva ativa/consumida e resultado;
query estrita, referências ausentes, sessão/papéis/contexto; leitura após
expiração sem atualizar relógio; ausência de escrita; concorrência com
reserva/consumo; backup/restauração de saldo e reserva ativa. Não inicia
servidor externo nem usa banco comercial/demonstração.

aplicar.sh confere base 2a93445, branch, árvore limpa e checksum, aplica apenas
P15, testa backup/CLI, stock/stockreservation e API específica, suíte completa
uma vez, vet, race e build sem executar o binário. Registra apenas os sete
arquivos listados após sucesso, com visualizador desativado.

Limites: consulta local de um produto/local, sem painel frontend, novos
movimentos, reserva automática, inventário físico, edição de unidade,
reconciliação de movimentos sem snapshot ou disponibilidade por lote.
