# B02B4 — lançamentos operacionais do dinheiro

Base no PC: B02B3 `5dbeb69`, branch `feat/backend-b02a-compat`, worktree
`zzeus-frontend-backend-b02a`. A instalação anterior foi confirmada por saída
de testes Go, frontend e commit limpo. Esta entrega não altera a worktree onde
estão os frontends 97–100 nem a branch de produção `ba29cbd`.

## O que funciona

Cada abertura, venda em dinheiro, estorno, suprimento e sangria grava uma
entrada em `cash_journal_entries`, na mesma transação de sua fonte, auditoria,
estoque quando aplicável e outbox. Falha na inserção, inclusive inserção ignorada
por trigger, impede o commit da operação inteira. Reenvio usa a idempotência
da operação original e não cria nova entrada. Não há rota para lançar um
valor avulso diretamente no journal.

A entrada tem ID estável `opening:<session_id>` ou `movement:<movement_id>`,
origem, operação, autor, instante, valor em centavos e contrapartida de controle.
As duas pernas são derivadas de forma determinística: entrada positiva debita
`cash_drawer` e credita o controle; saída faz o inverso. Estorno acrescenta
movimento negativo, preservando a venda original. Recebido e troco não duplicam
a receita: venda de 3198 com recebido 4000 e troco 802 lança apenas 3198.
Abertura zero mantém a entrada identificável e duas pernas de valor zero.

As contrapartidas `opening_control`, `cash_sales_control`, `supply_control`,
`withdrawal_control` e `unclassified_control` são controles operacionais, não
contas contábeis fiscais ou prova de origem bancária do dinheiro. Não registra
lucro, tributos, custo, contas a pagar/receber ou liquidação de adquirente.

## Consulta e contrato

`GET /local/v1/cash/sessions/{id}/journal?limit=50&offset=0`

Requer sessão válida, permissão Sell e o operador, loja e dispositivo originais
do turno. Consulta histórica não depende de licença POS vigente. Caixa aberto
retorna409 sem expor saldo esperado; outro operador ou turno inexistente404.
Query estrita: `limit` de1 a100, `offset` de0 a1000000000; duplicações, parâmetros
extras e corpo GET retornam400. Autenticação continua anterior à validação.

A resposta tem `scope=cash_operational`, `items`, `total_count`, `has_more`,
`next_offset` e `summary`. Ordenação por sequência persistida; a sequência
global não é exposta. Resumo cobre o turno inteiro, independentemente da página.
`source_complete=true` indica cobertura das fontes locais daquele turno, não
que todas as páginas foram baixadas, nem dados de outros aparelhos.

`summary` compara saldo dos lançamentos com `expected_cents` do fechamento,
informa declarado, diferença, débitos, créditos e quantidade não classificada.
Diferença de contagem permanece diferença: não gera despesa automática.
Valores e totais são exatos; somatórios usam inteiros arbitrários. Se um total
exceder o inteiro exato do cliente JavaScript, retorna422 sem números parciais.

Entradas não podem receber UPDATE/DELETE por SQL. A consulta compara os campos
com a fonte e seus vínculos de auditoria, detecta fontes sem lançamento e
lançamentos sem fonte; ausência/divergência de fechamento ou fonte retorna409.
Ela nunca repara dados durante leitura. Isso não é assinatura criptográfica nem
proteção contra um administrador que altere deliberadamente o esquema SQLite.
A conferência de parcelas e venda continua no relatório B02B2, que também deve
ser consultado; o journal não substitui esse relatório nem confirma adquirente.
O indicador `financial_ledger_available=false` daquele relatório continua
correto para o financeiro completo, ainda não entregue.

## Migração, arquivos compartilhados e integração

Nova `0047_cash_journal.sql`; scripts0001–0046 permanecem byte a byte iguais.
Reserva47 nesta linha, depois de0045_checkout_plans e0046_checkout_cash_sales.
A produção `ba29cbd` continua em44. Antes de integrar branches, combinar essa
numeração e conferir alterações em `migrate.go`, `backup/backup.go`,
`register/register.go`, `register/movement.go`, `sale/sale.go`, `sale/cancel.go`,
`localapi/cash.go` e inventários de API. Não aplicar patches dessas áreas por cima
de trabalho concorrente. O aplicador confere revisão e hashes das preimagens,
recusa alterações alheias e verifica a referência da produção antes e depois.

Dependência real encontrada: o executor antigo separava scripts por `;`,
impedindo os triggers de imutabilidade. `migrate.go` passa o script completo
ao parser do driver SQLite na mesma transação já existente. Testes exercitam
trigger, ponto e vírgula em string, falha posterior, rollback de DDL/histórico,
retry e reabertura. Validação de versões/checksums permanece obrigatória.
Nenhum código de autenticação/sessão foi alterado.

Migração47 copia as fontes existentes. Classifica suprimentos, sangrias, vendas
e estornos somente quando o vínculo auditado e o valor conferem; texto livre do
motivo, sozinho, não comprova venda. Fontes sem procedência inequívoca mantêm
`unclassified`, `actor_id=null` e `operation_id=null`. Abertura sem vínculo
humano preserva autor/operação nulos. Não inventa responsáveis ou operações.
Não elimina saldo antigo; não reclassifica histórico por meio de leitura.

Backup aceita versões25–47 mantendo verificações de histórico, integridade,
chave, aparelho e revogação existentes. Restauros históricos25–46 migram para47.
Entradas, IDs, ordem e proteções sobrevivem ao backup e à reabertura. Executável
antigo recusa esquema novo; não há downgrade nem remoção de histórico entregue.
Sincronização comercial entre aparelhos desses lançamentos não foi integrada.

## Evidências e pendências

Testes usam SQLite temporário e servidor HTTP de teste. Exercitam ciclo real
de dinheiro: abertura1000 + venda3198 + suprimento500 − sangria300 − estorno3198
= saldo1200. Cinco lançamentos, total de débitos e créditos8196 em cada lado.
Testam reenvio, paginação, histórico após novo turno, precisão, autorização,
rollback com falhas injetadas, migração de46 com fontes conhecidas/desconhecidas,
backup, reabertura e concorrência no mesmo processo. Resultados executados
acompanham o pacote em `EVIDENCIAS.txt`; não são aceite no aparelho do cliente.

Pendentes: confirmação eletrônica real, pagamento misto R$315, financeiro
completo, exportação contábil, recebimento remoto de malotes, integração visual
97–100, hardware, SEFAZ e aceite entre dispositivos. Nenhum conector simulado
é habilitado. O frontend completo dessa base não está disponível no ambiente
de preparação; testes e build dele são exigidos no PC antes do commit.

Sem merge, push, servidor iniciado ou acesso ao banco da demonstração.
