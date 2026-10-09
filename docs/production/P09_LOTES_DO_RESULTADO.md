# P09 — lotes do produto acabado

Base efetiva: **40b8e24**, branch **feat/producao-receitas**, P08 registrada
com árvore limpa. Esta entrega continua apenas a frente de produção.

Migração nova: **SQLite 0034_production_lots.sql**. O número 0034 foi conferido
livre na base P08. As migrações 0001..0033 não são alteradas.

## Quantidade e rastreabilidade

Um lote identifica uma quantidade do produto bom já registrado por um resultado
P06. Ele aponta para result_id, que permite chegar à ordem, local e snapshot da
versão da receita, incluindo ingredientes e rendimento planejado. Produto e
unidade vêm do resultado histórico, nunca do catálogo atual ou de campos
livres enviados pelo cliente.

Esta etapa não cria outra produção, movimento, reserva ou consumo. O produto
acabado já foi contabilizado pela P06. Lotes são uma classificação histórica da
quantidade produzida; não representam o saldo atual disponível. Por isso podem
ser declarados depois de movimentos posteriores de estoque, sem reescrevê-los.

| Campo da consulta por resultado | Significado |
| --- | --- |
| produced_milli | Quantidade boa registrada na P06 |
| assigned_milli | Soma de todos os lotes recorded desse resultado |
| unassigned_milli | produced_milli menos assigned_milli |

A soma dos lotes recorded nunca pode superar produced_milli. Um resultado com
produção zero não admite lote positivo. Não usar o rendimento planejado como
limite de lotes: ele pode ser maior que o produto bom efetivamente produzido.
A classificação das perdas P07 usa a diferença de rendimento, separadamente;
um lote nunca consome o limite de perdas, e uma perda nunca vira produto bom.

Todos os inteiros públicos respeitam o limite 9007199254740991. Quantidades são
milésimos da unidade histórica: unit exige múltiplos de 1000, g permite 1 para
0,001 g, e assim por diante. Não há conversão nova de massa/volume. A soma usa
inteiros de precisão arbitrária para detectar overflow e inconsistências antes
de converter para a escala pública. Isso evita overflow de SUM do SQLite.

## Identificação, datas e correção

Cadastro exige lot_id, result_id, quantity_milli, code, manufactured_on,
expires_on, reason e operation_id. IDs têm até 128 bytes UTF-8. Código após trim
tem 1..64 bytes, sem caracteres de controle; motivo após trim tem 1..255 bytes.
O código é sensível a maiúsculas/minúsculas e único na empresa/loja entre todos
os lotes, inclusive anulados. Um código anulado não pode identificar outro lote.

As datas são declarações explícitas, no formato calendário YYYY-MM-DD, anos
0001..9999. Datas inexistentes, timestamp no lugar de data e validade anterior
à fabricação são inválidos. expires_on é obrigatório no JSON: string vazia
significa validade **não informada**. Null e ausência do campo são rejeitados.
O sistema não calcula vida útil, não transforma validade desconhecida em prazo
infinito, nem prova que a data declarada corresponde à fabricação física.
Não há prazo ou rejeição automática com base no relógio da consulta.

O lote é imutável em identificação, datas, produto, unidade, quantidade e motivo
original. Correção usa anulação autorizada e motivada, sem apagar dados:

| Estado atual | Ação | Revisão esperada | Resultado |
| --- | --- | --- | --- |
| inexistente | cadastrar | — | recorded, revisão 1 |
| recorded | anular | 1 | voided, revisão 2 |

Anular libera apenas o limite de classificação daquele resultado. Não devolve
estoque, não reabre ordem nem anula resultado. Uma nova classificação corrigida
usa novos lot_id, code e operation_id. Não existe reativação, edição ou exclusão.

## Autorização, atomicidade e idempotência

Todas as operações validam manage_production, empresa, loja, usuário e aparelho
aprovado. Escritas, inclusive replay, também exigem contrato Production válido.
Empresa/loja/autor/aparelho vêm do contexto confiável, não do JSON.
Consultas autorizadas continuam disponíveis após vencimento do contrato;
revogação de papel ou aparelho continua impedindo acesso.

A chave idempotente é empresa + aparelho + operation_id no módulo P09. Loja,
autor, ação e pedido normalizado devem coincidir. Código e motivo são
normalizados por trim; datas não recebem correção automática. Repetição exata
não duplica lote, auditoria ou outbox. Repetição alterada retorna 409.
Replay de cadastro depois da anulação retorna o resultado original recorded;
o GET mostra o estado atual voided.

Lote, evento e outbox são gravados na mesma transação, com comparação da revisão
na anulação. Toda gravação exige exatamente uma linha afetada. Falhas e triggers
IGNORE/ABORT nos testes não deixam estado parcial; o relógio do contrato também
sofre rollback. Escritas concorrentes passam pelo mesmo núcleo transacional,
para não atribuir o mesmo produto a dois lotes acima da quantidade produzida.

Auditoria preserva pedido normalizado, resultado, autor, aparelho, motivos e
datas de registro. Histórico público lista criação e eventual anulação por
revisão. O GET preserva a definição original. Outbox usa production.lot.changed,
versão 1, com ator, ação, pedido e definição atual completa. Não foi alterado o
protocolo de transporte ou sincronização de outra frente.

## API documentada

Contrato: docs/api/production-lots.openapi.json.
Todas as rotas têm prefixo /local/v1 e sessão humana autenticada.

| Método e rota | Resposta |
| --- | --- |
| POST /production/lots | 201; replay autorizado 200 |
| POST /production/lots/void | 200 |
| GET /production/lots/:id | Lote com definição e estado atual |
| GET /production/lots/:id/history | Eventos de criação/anulação |
| GET /production/results/:id/lots | Totais globais e página de até 50 lotes |

A última rota aceita somente um offset não negativo. As demais recusam query
strings. A página inclui lotes anulados, em ordem created_at,id, mas os totais
consideram todos os recorded, independentemente da página, em uma transação.
Página vazia continua com totais corretos. Corpo máximo: 8192 bytes. JSON estrito
rejeita duplicatas, desconhecidos, null, fracionários e documentos adicionais.

Exemplo de cadastro de vinte unidades já produzidas:

```json
{"operation_id":"lot-op","lot_id":"lot-1","result_id":"result-1","quantity_milli":20000,"code":"BREAD-A","manufactured_on":"2026-10-08","expires_on":"","reason":"Identificar producao conferida"}
```

Exemplo de anulação:

```json
{"operation_id":"void-lot-op","lot_id":"lot-1","expected_revision":1,"reason":"Corrigir identificacao"}
```

Erros: 400 para entrada inválida; 401 sem sessão; 403 para autorização/contrato;
404 para referência ausente no escopo; 409 para limite, duplicidade,
idempotência, estado/revisão ou soma inconsistente; 500 para falha interna.
A API nunca revela dados de outra empresa/loja para resolver referências.

## Compatibilidade e arquivos compartilhados

server.go monta cinco rotas protegidas. backup.go acrescenta somente schema 34
à lista explícita 25..33. Preserva versões anteriores, ValidateSchema, checksums,
integrity_check, foreign_key_check e validação do aparelho. Formato, criptografia
e recuperação não foram alterados. Schema futuro continua recusado.

compatibility_test.go cobre snapshots 25..33 restaurados até 34. Os testes
gerais passam a esperar 34; o teste de falha de migração usa 35 fictício, sem
reservar ou criar migração 0035. Nenhuma migração PostgreSQL, autenticação,
sessão, infraestrutura, main, banco comercial ou demonstração foi alterada.
Não há merge nem push.

## Verificação e limites desta entrega

Bancos SQLite temporários e Fiber App.Test; sem abrir servidores. Testes cobrem
limite pelo produto bom, unidade histórica, 0,001 g, calendário/bissexto, datas
vazias, códigos, JSON estrito, escopos, papel, contrato, replay, anulação,
rollback inclusive de relógio, concorrência e soma com overflow. Totais da
consulta são validados em páginas completas e vazias. Backup/restauração inclui
lotes recorded/voided, datas, definição, auditoria, resultado, ordem/receita,
ingredientes e saldo do produto acabado.

O aplicar.sh verifica base e árvore limpa, número 0034 livre e checksum; aplica
apenas P09 e formata a lista de Go. Testa backup/CLI primeiro, depois P09, suíte
completa uma vez, vet, race e build sem executar binário. Registra só a lista
explícita da entrega após sucesso. O visualizador do Git é desativado para o
script não ficar parado em (END) antes do commit.

Não implementa saldo físico por lote, FEFO, consumo por lote, alocação da venda,
recolhimento, controle de qualidade ou vida útil automática. A validade aqui é
metadado rastreável; não muda a capacidade P03, o estoque livre ou a regra do
PDV. Esses contratos deverão ser implementados em entregas separadas.
