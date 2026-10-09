# P08 — etapas sequenciais por ordem

Base efetiva autorizada: `5b133c5`, branch `feat/producao-receitas`.
Migração utilizada: **SQLite `0033_production_stages.sql`**. O número 0033
estava livre na base P07. As migrações 0001..0032 permanecem intactas.

## Contrato e sequência

Uma ordem pode ter um plano opcional de 1..20 etapas. A configuração ocorre uma
única vez em ordem `planned` ou `approved`, antes de existir reserva ativa ou
consumida de ingredientes. Reservas já liberadas não impedem a configuração.
A ordem continua vinculada à versão imutável da receita e ao local da P04.
O plano pertence à ordem: esta entrega não cria modelos de etapas em receitas.

O array informado determina posições consecutivas 1..N. Cada etapa tem ID único
na ordem, nome e responsável autorizado na empresa/loja. Nome, responsável e
sequência ficam fixados; não há edição, exclusão, reordenação ou reabertura por
esta API. Configurar não aprova a ordem, não reserva e não consome ingredientes.

| Estado atual | Comando | Revisão esperada | Estado e revisão resultantes |
| --- | --- | --- | --- |
| pending | running | 1 | running, 2 |
| running | completed | 2 | completed, 3 |

Somente o responsável atribuído, com permissão atual `manage_production`, pode
executar sua etapa. Não existe substituição silenciosa pelo proprietário.
A ordem precisa estar aprovada e o consumo P05 comprovado pelos movimentos
negativos dos ingredientes, com quantidades e unidades do snapshot da ordem.
Todas as etapas anteriores precisam estar concluídas para iniciar ou concluir
uma etapa posterior. Isso impede execução simultânea de etapas da mesma ordem.
Cada transição exige motivo não vazio e nova operação idempotente.

As etapas não registram quantidades, perdas ou movimentos de estoque.
O resultado medido e o produto acabado continuam sendo registrados pela P06;
as perdas declaradas continuam na P07. Unidades e escala inteira existentes são
preservadas, sem conversões novas de massa/volume.

## Mudança compartilhada na P06

`production/results.go` passa a exigir que um plano configurado esteja inteiro,
com todas as etapas concluídas, antes de aceitar um resultado novo. A contagem
persistida e posições consecutivas detectam definições incompletas: remover uma
linha não torna o restante suficiente para concluir. Falha responde 409 e não
publica produto acabado nem resultado. A guarda pertence à mesma transação dos
movimentos, revisão da ordem, resultado e outbox.

Ordens sem plano conservam o contrato P06. Um replay exato de resultado já aceito
continua devolvendo seu resultado original, depois de conferir autorização e
contrato atuais. Configurar após reserva/consumo ou conclusão é proibido, para
não impor requisitos retroativos a um processo em execução.

O plano não aumenta a revisão da ordem: as revisões de ordem e de etapa têm
finalidades distintas. Concluir todas as etapas não conclui automaticamente a
ordem; a P06 ainda exige o resultado medido e a revisão esperada da ordem.
Cancelar uma ordem planejada/aprovada segue as regras P04/P05. O plano e histórico
permanecem consultáveis, mas nenhuma transição nova é aceita após cancelamento.

## API

Especificação: `docs/api/production-stages.openapi.json`.
O contrato compartilhado de resultado também foi atualizado em
`docs/api/production-results.openapi.json`.

| Método e rota (prefixo /local/v1) | Contrato |
| --- | --- |
| POST /production/stage-plans | Configurar uma vez; 201 ou 200 no replay |
| POST /production/stages/state | Iniciar/concluir com expected_revision; 200 |
| GET /production/orders/:id/stages | Definição e estados, por posição |
| GET /production/orders/:id/stages/history | Auditoria, até 50 eventos, offset opcional |

Exemplo de configuração:

```json
{"operation_id":"plan-op","order_id":"order-1","reason":"Sequencia de preparo","stages":[{"stage_id":"mix","name":"Misturar","responsible_id":"staff-1"},{"stage_id":"bake","name":"Assar","responsible_id":"staff-1"}]}
```

Exemplo de início:

```json
{"operation_id":"mix-start","order_id":"order-1","stage_id":"mix","expected_revision":1,"status":"running","reason":"Inicio autorizado"}
```

Para concluir a etapa, envie nova operation_id, expected_revision 2 e status
completed. Não reaproveite a operação de início para outro comando.

Empresa, loja, aparelho e autor vêm da sessão e configuração confiável do
processo; não são aceitos no JSON. Campos obrigatórios, desconhecidos,
duplicados, nulos e JSON adicional são rejeitados, inclusive nos objetos das
etapas. Limites: corpo do plano 16384 bytes; transição 8192; IDs 128 bytes UTF-8;
nome 120 bytes; motivo 255 bytes após trim; até 20 etapas.

Sem plano, GET retorna 404. Referências ausentes no escopo retornam 404;
autor/aparelho/contrato negado ou executor diferente retorna 403; conflito de
estado, revisão, idempotência, consumo ou sequência retorna 409. Dados históricos
continuam legíveis por usuário autorizado após expiração do contrato.

## Atomicidade, auditoria e idempotência

Plano, etapas, evento e outbox são gravados na mesma transação. Transição usa
comparação da revisão e estado anteriores e exige exatamente uma linha alterada.
Uma falha ou trigger IGNORE não publica uma alteração parcial; o relógio do
contrato também sofre rollback. As transações do núcleo local serializam as
mutações, incluindo tentativas concorrentes de iniciar a mesma etapa.

A chave idempotente é empresa + aparelho + operation_id dentro das operações
P08. Autor, loja, ação e pedido normalizado precisam coincidir. Um replay exato
não duplica evento/outbox e devolve o resultado original, mesmo se a etapa tiver
avançado ou a ordem tiver sido concluída. O GET representa o estado atual.
Permissão atual e contrato válido são exigidos também no replay de escrita.

Auditoria preserva sequência local, dispositivo, autor, motivo, pedido
normalizado completo e resultado original. A configuração auditada inclui todas
as definições e responsáveis. O histórico é ordenado pela sequência persistida,
sem depender de comparação textual de timestamps. A sequência é local ao banco,
pode ter lacunas e não deve ser usada como contador global entre aparelhos.
Os tipos de outbox são production.stage.configured, production.stage.running e
production.stage.completed, versão de evento 1. Sua publicação usa o contrato
já existente de outbox; nenhum protocolo de sincronização foi alterado.

## Backup e arquivos compartilhados

`backup.go` acrescenta **somente schema 33** à lista explícita anterior 25..32.
Não aceita schemas futuros. ValidateSchema, checksums, integrity_check,
foreign_key_check, prova do aparelho, formato e criptografia permanecem intactos.
O teste de compatibilidade agora inclui snapshots históricos 25..32 restaurados
até 33. Os testes gerais de migração esperam 33 e usam 34 apenas como número
fictício para provar rejeição/rollback, sem reservar ou criar a migração 0034.

`localapi/server.go` monta as quatro rotas protegidas; `production.go` mapeia
ErrStages para 409. Os arquivos de autenticação, sessões, infraestrutura e
migrações PostgreSQL não são alterados. Nenhum merge ou push faz parte da entrega.

## Verificação e limites

Os testes usam bancos SQLite temporários e Fiber App.Test sem abrir servidores.
Cobrem sequência, guarda de resultado, ausência de movimentos adicionais,
replay histórico, conflitos, limites, JSON estrito, escopos, papel, responsável,
revogação, expiração, rollback de todas as gravações relevantes, concorrência,
configuração antes de aprovação/cancelamento, reservas liberadas e definição
incompleta. Backup/restauração verifica definições, responsáveis, estados,
histórico, ordem concluída e saldo do produto acabado.

O aplicar.sh verifica base 5b133c5 e árvore limpa, ausência de 0033, checksum do
patch e aplicação possível. Executa backup/CLI primeiro, testes específicos,
suíte completa uma vez, vet, race nas etapas e resultado compartilhado e build
sem executar o binário. Só então registra exclusivamente a lista P08.

Limites intencionais: etapas são opcionais para preservar ordens existentes;
não há configuração em lote, template por receita, duração planejada, pausa,
substituição de responsável ou retrabalho. Se o responsável perder autorização,
a etapa não poderá avançar até que sua autorização seja restaurada; uma futura
substituição exigirá contrato e auditoria próprios. Esta entrega não cria novas
reservas, movimentos, rendimentos ou perdas por etapa.
