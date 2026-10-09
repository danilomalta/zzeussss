# P10 — avaliações humanas de qualidade por lote

Base efetiva: **dd6cf9e**, branch **feat/producao-receitas**, P09 registrada
com árvore limpa. Somente a frente de produção recebe alterações.

Migração nova: **SQLite 0035_production_quality.sql**. O número 0035 estava
livre na base P09; as migrações 0001..0034 permanecem intactas.

## Parecer sobre o lote inteiro

Um usuário autorizado registra avaliação humana do lote inteiro cadastrado na
P09, com critério declarado e motivo/observação. O autor responsável é a pessoa
autenticada: não aceitar actor_id ou responsible_id enviados pelo cliente.
O servidor registra o parecer, sem inventar padrões ou executar testes físicos.

Os pareceres permitidos são passed e failed. Não existe aprovação automática.
Antes da primeira avaliação, a consulta retorna not_assessed, revisão 0 e latest
null. Avaliação não muda o status recorded/voided do lote, sua quantidade, datas,
produto, unidade, resultado, receita, ordem, consumo, rendimento ou perdas.

A avaliação é uma declaração auditada, não uma certificação nem uma regra nova
para o estoque ou PDV. Nenhum produto é bloqueado, liberado, reservado, baixado
ou devolvido por este endpoint. Controle de quarentena e bloqueio de vendas
exigem integração transacional própria, em entrega separada. Uma interface deve
mostrar o estado atual do lote junto do parecer, sem tratar um lote anulado
como disponível por causa de uma avaliação passada.

## Revisões distintas e reavaliação

| Situação | expected_revision do parecer | Resultado |
| --- | --- | --- |
| Lote sem avaliação | 0 | Nova avaliação, revisão 1 |
| Parecer atual na revisão N | N | Reavaliação, revisão N+1 |
| Revisão esperada diferente da atual | qualquer divergente | 409, sem alteração |
| Lote voided | qualquer | Nenhuma avaliação nova, 409 |

A revisão de qualidade não é a revisão P09 do lote. Um lote recorded permanece
na revisão 1 da P09 mesmo após várias avaliações. Reavaliação usa nova
operation_id, critério e motivo explícitos. Pode repetir o parecer, mas deve
criar outra revisão auditada, sem sobrescrever o histórico. Não há edição,
exclusão, reabertura do lote ou anulação de parecer; um novo parecer motivado
é a forma de corrigir a declaração atual.

Os inteiros são exatos: expected_revision entre 0 e 2147483646; a última revisão
persistível é 2147483647. A próxima revisão nunca é calculada com base apenas
no valor enviado. O histórico precisa ser contínuo, com revisões únicas 1..N:
contagem diferente da revisão máxima indica inconsistência e retorna 409 nas
consultas e antes de novas avaliações.

## Histórico e snapshot

Cada revisão preserva lot_id, revisão, operação, autor, aparelho, parecer,
critério, motivo, data de registro e snapshot completo do lote no momento da
avaliação. O snapshot inclui identificação, result_id, produto, unidade,
quantidade exata, fabricação, validade declarada, estado e metadados originais.
Unidade histórica nunca é substituída pela unidade atual do catálogo.

A consulta atual mostra a revisão mais recente e o **lot_status atual**. Depois
de anular o lote P09, histórico e parecer continuam consultáveis; o estado
voided é explícito, enquanto o snapshot da avaliação preserva recorded.
O vínculo result_id permite consultar o resultado P06, a ordem e a versão
imutável da receita, seus ingredientes e local, sem alterar esses registros.

## Autorização, auditoria e atomicidade

Todas as operações exigem manage_production, empresa/loja corretas, usuário
ativo e aparelho aprovado. Escritas e replay exigem contrato Production válido.
Leituras históricas autorizadas continuam após expiração do contrato; revogação
humana ou do aparelho continua impedindo acesso. O papel atual de produção pode
avaliar: esta entrega não cria permissão nova nem exige uma segunda pessoa como
avaliador independente. Não altera os arquivos de identidade/autenticação.

Empresa, loja, autor e aparelho vêm da sessão/contexto confiável. Critério e
motivo após trim têm 1..255 bytes UTF-8. IDs seguem o limite de 128 bytes sem
espaços nas extremidades. O snapshot valida unidade, quantidade exata e datas
já declaradas; não converte massa/volume nem infere validade.

A chave idempotente é empresa + aparelho + operation_id dentro da P10. Loja,
autor e pedido normalizado devem coincidir. Critério e motivo usam trim. Replay
exato retorna a revisão e parecer originais, mesmo após reavaliação ou anulação
do lote, sem duplicar registro ou outbox. A consulta atual representa o parecer
mais recente; não inferir estado atual da resposta de um replay antigo.

Parecer, pedido, resultado, snapshot e outbox são gravados em uma transação,
com comparação da revisão persistida e unicidade por lote/revisão. As mutações
do núcleo local serializam os pedidos concorrentes; dois pareceres diferentes
para a mesma revisão esperada não vencem simultaneamente. Toda escrita exige
exatamente uma linha afetada. Erro ou trigger IGNORE/ABORT não deixa avaliação,
outbox ou relógio do contrato parcialmente atualizado.

Outbox: production.quality.reviewed, versão 1. Payload inclui autor, pedido,
resultado e snapshot do lote. Nenhum formato de transporte/sincronização de
outra frente é alterado. A tabela production_quality_reviews é o histórico
de auditoria completo, com request_json e result_json originais.

## API

Contrato: docs/api/production-quality.openapi.json.
Prefixo /local/v1, sessão humana autenticada.

| Método e rota | Contrato |
| --- | --- |
| POST /production/quality-reviews | Parecer novo 201; replay exato 200 |
| GET /production/lots/:id/quality | Estado atual do lote, revisão e último parecer |
| GET /production/lots/:id/quality/history | Histórico por revisão, até 50 itens |

Exemplo de primeira avaliação:

```json
{"operation_id":"review-1","lot_id":"lot-1","expected_revision":0,"status":"failed","criterion":"Criterio interno informado pelo operador","reason":"Parecer humano registrado"}
```

Uma reavaliação deve informar nova operation_id e expected_revision 1. O campo
status pode ser passed ou failed, conforme decisão humana, com critério/motivo.

Somente o histórico aceita offset, uma vez e não negativo. As demais rotas
recusam query strings. Campos obrigatórios, desconhecidos, duplicados, null,
revisões fracionárias e JSON extra são rejeitados; corpo máximo 8192 bytes.

Erros: 400 entrada inválida; 401 sem sessão; 403 autorização/contrato; 404 lote
não encontrado no escopo; 409 revisão, idempotência, lote anulado ou histórico
inconsistente; 500 falha interna. Um lote existente sem avaliações retorna
not_assessed, não 404. O histórico vazio retorna items [], sem inventar eventos.

## Arquivos compartilhados e backup

server.go monta três rotas protegidas. backup.go acrescenta **somente schema
35** à lista explícita 25..34. Preserva versões anteriores, ValidateSchema,
checksums, integrity_check, foreign_key_check e validação do aparelho.
Formato, criptografia e recuperação não foram alterados. Schema futuro continua
recusado. compatibility_test.go passa a cobrir snapshots históricos 25..34
restaurados até 35; os testes gerais esperam 35. O teste de falha de migração usa
36 fictício, sem reservar ou criar migração 0036.

Nenhum arquivo de autenticação, sessões, infraestrutura, PostgreSQL, banco
comercial ou banco da demonstração foi alterado. Sem merge e sem push.

## Verificação e limites

Testes usam SQLite temporário e Fiber App.Test, sem iniciar servidores.
Cobrem revisão 0, reavaliação, parecer repetido com nova revisão, preservação de
snapshot e histórico, replay antigo, lote anulado, escopos, papel, contrato,
JSON estrito, limites inteiros/texto, rollback de avaliação/outbox/relógio,
concorrência, paginação e recusa de histórico incompleto. Saldo, resultado,
ordem e classificações P09 permanecem inalterados.

Backup/restauração verifica failed seguido de passed, critério/motivo, autor,
definição original e lote posteriormente anulado, receita/ingredientes, ordem
concluída e saldo do produto acabado. O aplicar.sh confere base dd6cf9e e árvore
limpa, número 0035 livre e checksum; testa backup/CLI primeiro, depois P10,
suíte completa uma vez, vet, race e build sem executar binário. Só então
registra a lista explícita P10, com visualizador do Git desativado.

Esta entrega não implementa inspeção parcial, medições estruturadas, anexos,
critérios automáticos, aprovação independente, quarentena, liberação física,
FEFO, bloqueio de venda ou retrabalho. Esses contratos ficam separados desta
trilha auditada de avaliação humana do lote inteiro.
