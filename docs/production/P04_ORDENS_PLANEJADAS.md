# P04 — ordens planejadas por versao imutavel

Base confirmada: `6cc37c6`, branch `feat/producao-receitas`, arvore limpa.
P03 confirmada no PC com suite completa, vet, race e build aprovados.

| Etapa | Estado | Base/migracao | Verificacoes | Dependencias |
| --- | --- | --- | --- | --- |
| P03 | Confirmada no PC | 6cc37c6; SQLite 0028 | Testes, race, vet, build | — |
| P04 | Preparada para aplicacao apos coordenacao | 6cc37c6; proposta SQLite 0029 | SQL 0001–0029, diff/patch/shell/JSON conferidos localmente; Go indisponivel | Confirmar reserva 0029 e compatibilidade de backup 29 |
| P05 | Pendente | A combinar | — | Reserva/liberacao/consumo de materiais |

## Ordem e estados

Cria uma ordem vinculada a empresa, loja, `version_id`, local e pessoa
responsavel. `planned_batches` e inteiro positivo: numero de lotes completos
da quantidade de referencia da receita. Guarda o snapshot completo da versao,
o numero planejado e `planned_output_milli` exato. Cada total de ingrediente
planejado e `recipe.ingredients[].quantity_milli * planned_batches`; limites
sao verificados antes da confirmacao. Quantidades permanecem em milesimos
da unidade, como P02/P03, sem float, conversao ou arredondamento escondido.

Nao depende da versao mais recente da receita. Edicao posterior de receita
ou catalogo nao muda o snapshot da ordem. Local, responsavel e plano nao sao
editados nesta etapa. Correcao do plano exige cancelar a ordem e criar outra
com IDs novos, preservando ambas e o motivo do cancelamento.

| Estado atual | Destino permitido | Significado |
| --- | --- | --- |
| Nova ordem | planned | Plano registrado, sem comprometer materiais |
| planned | approved | Plano aprovado por pessoa com ManageProduction |
| planned | cancelled | Plano cancelado com motivo |
| approved | cancelled | Aprovacao retirada com motivo |
| cancelled | Nenhum | Terminal; historico preservado |

`approved` nao garante saldo nem autoriza consumo. Nao existe transicao para
execucao, completed ou producao real nesta entrega. Ordem pode ser planejada
sem saldo no local; P05 revalidara disponibilidade ao comprometer materiais.
Nao cria reserva, movimento de estoque, venda nem entrada de mercadoria.

## Autoridade e consistencia

Escritas exigem identidade humana, aparelho aprovado e permissao
`ManageProduction`, alem de contrato assinado `Production/Core/Inventory`.
Todas essas verificacoes compartilham a transacao da entidade/evento/outbox.
O responsavel deve estar ativo e autorizado para producao na loja, tanto na
criacao quanto na aprovacao. A atribuicao nao e uma acao/assinatura do
responsavel; `actor_id` registra quem fez a alteracao pela sessao.

`operation_id` e definido antes do envio. Mesmo ID/aparelho/empresa, autor,
loja, tipo e conteudo retorna o resultado original duravel; divergencia gera
409. Replay revalida permissao e contrato, inclusive depois de expirar.
Resultado de replay e historico: pode conter planned mesmo quando a ordem
atual esta cancelled; consultar GET para estado atual. Mudancas de estado
exigem `expected_revision` atual e motivo de 1–255 bytes apos trim.

Transacao confirma ordem, evento de auditoria e outbox juntos. Falhas SQL,
trigger ABORT/IGNORE ou update sem exatamente uma linha revertem tudo,
inclusive a observacao temporal do contrato. Cancelamento preserva plano,
snapshot e eventos. Consulta exige permissao/aparelho atuais, mas continua
disponivel apos vencimento do contrato, conforme politica de P02.

## API local autenticada

| Metodo | Caminho sob /local/v1 | Entrada |
| --- | --- | --- |
| POST | /production/orders | operation_id, order_id, version_id, location_id, responsible_id, planned_batches |
| POST | /production/orders/state | operation_id, order_id, expected_revision, status, reason |
| GET | /production/orders | offset opcional; 50 por pagina |
| GET | /production/orders/:id | Plano, snapshot, estado e revisao atuais |
| GET | /production/orders/:id/history | offset opcional; eventos ordenados por revisao, 50 por pagina |

Corpos ate 8192 bytes; objetos exatos com todos os campos obrigatorios.
Recusa campos extras/repetidos/null, JSON posterior e numeros fracionarios.
Consulta so aceita offset onde documentado; query extra/repetida e invalida.
Empresa/loja/aparelho/pessoa autora derivam da sessao, nunca do navegador.

```json
{"operation_id":"ordem-op-1","order_id":"ordem-1","version_id":"pao-v1","location_id":"local-producao","responsible_id":"pessoa-autorizada","planned_batches":3}
```

Respostas: 201 criacao; 200 replay/transicao/consulta; 400 pedido/limite invalido;
401 sem sessao; 403 permissao/contrato/responsavel recusado; 404 referencia
inexistente no escopo; 409 conflito de ID, revisao ou transicao; 413 corpo
excedido; 503 sem verificador; 500 falha interna. Nao expor SQL/credenciais.

Depois de timeout, consultar a ordem e seu historico. Consulta falhada nao
dispara outra escrita; eventual repeticao conserva os IDs e conteudo originais.

## Migracao, backup e coordenacao

SQLite **0029_production_orders.sql** e proposta na sequencia livre da copia
de producao fornecida (ate 0028). Confirmar sua reserva com Chat A/integrador
antes da aplicacao; o numero nao foi conferido na main e nao e reserva global.
Nenhuma migracao antiga ou PostgreSQL e alterada.

Patch principal cria production_orders e production_order_events e ajusta os
testes globais para 29 (falhas ficticias usam 30). Compartilhado de runtime:
uma chamada `mountProductionOrders(protected)` em localapi/server.go.

Patch separado **TitanSystem_P04_Compatibilidade_Backup.patch** propoe somente
aceitar schema 29 na lista explicita de backup.go, mantendo 25–28,
ValidateSchema/checksums/integrity_check/foreign_key_check/aparelho. Atualiza
teste de compatibilidade para schema 29 e inclui origem 28. Este ajuste depende
de coordenacao/autorizacao adicional: a autorizacao anterior era para 28.
O aplicar.sh para ANTES de alterar arquivos se a coordenacao nao estiver
explicitamente indicada por `TITAN_P04_BACKUP29_COORDENADO=sim`.

Evento `production.order.changed`, schema 1, aggregate = order_id. Payload
inclui ID/revisao/estado/tipo/autor e pedido canonico. Recepcao/aplicacao em
outro aparelho nao implementada; nao conceder pareamento para esse tipo nem
afirmar sincronizacao. Auditoria consultavel em /orders/:id/history, separada
da auditoria administrativa unificada. Sem alteracao de autenticacao/infra
alem do ajuste de compatibilidade proposto separadamente.

## Verificacao

Testes TestHTTPProductionOrders cobrem criacao sem estoque, snapshot mesmo
apos nova versao/catalogo, replay, IDs conflitantes, estados/revisoes/historico,
autorizacao e referencias, limites exatos/overflow, JSON ambiguo, rollback de
entidade/auditoria/outbox, relogio do contrato, concorrencia, backup/restauracao
de ordem e ingredientes e escopo de empresa/loja. Suporte ao schema 29 no
backup e necessario para passar esse teste e a suite global.

Go nao estava disponivel no ambiente gerador. Script testa primeiro backup/CLI,
depois P04, suite global uma vez, vet, race focado em P04 e build antes do
commit. Nao inicia servidor externo nem usa banco comercial. Estado muda
para confirmado no PC somente com saida aprovada e commit real.
