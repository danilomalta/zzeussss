# B02B10 — caixas encerrados para contabilidade

## Base e entrega

B02B9 instalada no PC em `1d5aa41`, branch `feat/backend-b02a-compat`, worktree
`zzeus-frontend-backend-b02a`. O log confirma Go completo/race,89 testes locais
do frontend, build e árvore limpa. Esta entrega implementa duas frentes de leitura:
fechamentos de vários operadores por período para gestão/contador e consulta dos
lançamentos operacionais correspondentes com exportação CSV integral.

Quatro operações HTTP, serviço real de consulta, contratos e testes. Nenhuma
migração ou mudança no backup; esquema51 mantido. Não altera autenticação,
sessões, regras de permissões, pagamentos, protocolo legado ou frontend.

## Permissão e origem

Todas as consultas novas exigem sessão válida e `ViewAccounting`, permissão já
existente. Dono, gerente e contador usam as regras efetivas existentes, incluindo
negação por loja e revogação. Um operador apenas `Sell` não recebe acesso ao
período dos outros operadores. Tenant/loja/aparelho vêm da sessão; a API não aceita
operador, tenant, loja ou aparelho na query ou corpo.

As consultas antigas de relatório/journal do caixa continuam limitadas ao operador
original com `Sell`. As novas funções transacionais verificam `ViewAccounting`
do solicitante e derivam o operador original de cada turno no banco. Não trocam
a identidade da sessão nem exigem que o ex-operador continue empregado/ativo.
O histórico permanece consultável pelo contador autorizado após a revogação do
operador original. A permissão do contador é revalidada em toda requisição.

Histórico não exige contrato POS ativo. Isso não concede escrita, acesso livre
a outras lojas/aparelhos, banco financeiro ou transporte de dados ao contador.
Nenhum relatório é enviado a terceiros por este serviço.

## Seleção do período

`from` e `to` são obrigatórios, em RFC3339 com frações opcionais e timezone.
São normalizados para UTC na resposta. Intervalo de encerramento `[from,to)`:
inclui o instante inicial e exclui o final, com precisão de nanossegundos.
O instante inicial precisa ser menor que o final; máximo31dias de24h.
Filtro é pelo encerramento, não pela data de cada venda ou lançamento.

Só turnos encerrados do aparelho/loja atual entram. Caixas abertos não têm saldo
esperado revelado por estas rotas e não entram nos totais. Cada turno selecionado
inclui todo seu relatório e journal, mesmo se a abertura/movimentos forem anteriores
ao período. Não é relatório de competência contábil, calendário fiscal ou horário
local escolhido automaticamente: o cliente informa os limites com timezone.

A seleção lê os metadados de até10000 turnos encerrados do escopo e faz comparação
temporal exata em Go, sem arredondar em SQL. Data de encerramento inválida nesse
conjunto impede seleção completa e retorna409, em vez de excluir silenciosamente
um turno. No período selecionado, abertura/encerramento também são validados;
encerramento anterior à abertura é recusado.

## Resumo e significado dos valores

O relatório junta as fontes oficiais B02B2 e o journal B02B4 na mesma transação.
Cada turno confere operador/vínculo, fechamento, pagamentos, movimentos, auditoria
e journal. Os valores esperado, declarado e diferença precisam conferir entre
relatório e journal. Fonte ausente ou inconsistente não produz resumo parcial.

`scope=local_closed_cash_sessions`. `complete=true` significa que o conjunto local
selecionado passou nas verificações, não que todos os aparelhos/bancos foram
consultados. `bank_settlement_available=false` e `statutory_ledger_available=false`.

Resumo fixo:

| Campo | Significado |
| --- | --- |
| `session_count` / `sale_count` | Turnos selecionados / vendas persistidas, incluindo canceladas |
| `gross_sales_cents` | Vendas originais antes dos cancelamentos registrados |
| `cancelled_sales_cents` | Vendas canceladas preservadas no histórico |
| `net_sales_cents` | Vendas persistidas ainda válidas |
| `sum_openings_cents` | Soma das aberturas dos turnos selecionados |
| `sum_expected_at_closures_cents` | Soma dos valores esperados nos encerramentos |
| `sum_declared_at_closures_cents` | Soma dos valores declarados nos encerramentos |
| `sum_closure_differences_cents` | Soma assinada das diferenças dos encerramentos |
| `supply_cents` / `withdrawal_cents` | Suprimentos / sangrias registrados nos turnos |
| `journal_entry_count` | Quantidade de lançamentos do conjunto inteiro |
| `journal_debit_cents` / `journal_credit_cents` | Soma das pernas operacionais balanceadas |
| `unclassified_count` | Fontes legadas sem classificação/autoria completa preservadas |

Soma de aberturas e soma de fechamentos NÃO representam dinheiro disponível em
um instante. O mesmo numerário pode ter sido transportado de um turno para outro.
Não se deve somar esses campos para obter saldo de caixa. Cofres, malotes ainda
não creditados, bancos e comprovantes manuais B02B9 não são incluídos como dinheiro
ou pagamentos confirmados. Os saldos de cofre continuam em seu relatório próprio.

Pagamentos preservam o contrato legado: cash/card/pix/other, partes e valores
`recorded_confirmed_cents`, `recorded_pending_cents`, `recorded_reversed_cents`.
Card não distingue débito/crédito e confirmed é estado persistido, não confirmação
de adquirente. O comprovante aceito B02B9 permanece fora desses totais financeiros.
Uma venda cash cancelada aparece como venda original, cancelamento e pagamento
revertido; o journal preserva entrada e estorno, sem apagar o passado.

Cada lançamento tem o operador do turno e o ator original da própria fonte.
Eles podem ser diferentes, como gerente que fez suprimento no caixa de um operador.
Não há substituição desses IDs pelo contador que está consultando. Fontes antigas
sem autoria/operação continuam `null`, sem atribuição inventada.

## APIs e paginação

Prefixo `/local/v1`. Nenhuma rota aceita corpo GET. Query estrita: `from`, `to`,
e somente nas duas páginas, `limit`1–100 (padrão50), `offset`0–1000000000 (padrão0).
Parâmetros desconhecidos, repetidos, ausentes ou inválidos retornam400.

| GET | Resposta |
| --- | --- |
| `/accounting/cash/sessions` | Página de turnos, relatório individual, resumo/pagamentos do período inteiro |
| `/accounting/cash/journal` | Página de lançamentos, resumo do período inteiro |
| `/accounting/cash/report` | Relatório integral com turnos, lançamentos, resumo e pagamentos |
| `/accounting/cash/report.csv` | CSV completo do mesmo serviço de snapshot |

Todo o período é conferido antes de paginar, inclusive se a página está vazia.
Ordem: encerramento, ID do turno e sequência do journal dentro de cada turno.
Paginação não implica que o cliente já consultou todas as páginas.

Limites:10000 metadados de turnos encerrados no escopo,500 turnos selecionados,
10000 lançamentos no período inteiro. Quantidade acima do limite ou total fora
da faixa exata9007199254740991 retorna422 sem números/arquivo parcial. Somas usam
inteiros grandes, inclusive para diferenças assinadas, evitando overflow dependente
da ordem. Não houve benchmark de carga comercial; o custo cresce com as fontes.

Erros:400 entrada,401 sessão,403 permissão,409 fonte/data/CSV incompatível,
422 limite/precisão,500 falha interna. Nenhuma resposta de erro inclui resumo
utilizável ou download parcial. Contrato: `docs/api/cash-accounting.openapi.json`;
inventário e OpenAPI principal atualizados.

## CSV e snapshot

JSON, páginas e CSV usam o mesmo serviço de validação integral. Cada requisição
tem uma transação de leitura; requisições separadas podem observar instantes
diferentes. O CSV não concatena páginas consultadas em transações independentes.

Formato1, UTF-8/BOM, separador `;`, CRLF, valores em inteiros exatos. Linhas:
`period_metric` (métricas do resumo), `period_payment`, `session_metric`,
`session_payment` e `journal`. `metric_value` contém a unidade indicada pelo nome
da métrica; campos monetários terminam em `_cents`. Não somar linhas de resumo,
turno e journal entre si: são representações distintas das mesmas fontes.

CSV até16MiB, nome fixo `titan-caixas-encerrados.csv`, Cache-Control:no-store e
X-Content-Type-Options:nosniff. IDs/textos incompatíveis com CSV seguro são recusados
com409 sem reescrever o identificador, acrescentar apóstrofo ou alterar JSON.
Valores numéricos negativos legítimos, como diferença e estorno, continuam números.
Não gera layout SPED, arquivo fiscal, plano contábil estatutário ou liquidação.

## Dependências, preservação e validação

Compartilhados: `register/report.go`, `cashjournal/journal.go`,
`localapi/server.go` e contratos HTTP. Dois helpers novos recebem transação do
chamador, validam `ViewAccounting` e derivam a identidade histórica nas fontes.
Os wrappers antigos mantêm `Sell`, operador próprio e commit de sua transação.
Não há transação aninhada, escrita de relatório, outbox ou autorização por frontend.

Esquema51/backup25–51 mantidos, sem edição de migrações0001–0051, fixtures futuras,
executor, autenticação/sessões, permissões, dependências Go ou protocolo legado.
Backup/restauração do esquema51 preserva os relatórios calculados. O aplicador
confere base B02B9 `1d5aa41`, hashes e produção/migrações pendentes, testa e reconfere
antes de commitar os arquivos explícitos. Sem merge ou push.

Comandos/resultados em EVIDENCIAS.txt. Testes em SQLite temporário/HTTP cobrem
operadores diferentes, contador sem Sell, negação/revogação, licença histórica,
intervalos/timezones/nanossegundos, fonte inconsistente, página vazia, valor máximo,
limites, exportação integral, estorno cash versus prova B02B9 e backup. Fixtures
de tamanho e timestamps futuros são dados sintéticos de teste, não uso comercial.

Frontend completo da base do PC indisponível aqui. O aplicador exige npm ci,
test:local e build no PC antes do commit. Integração visual/envio ao contador,
outros aparelhos, conectores, pagamentos mistosR$315, financeiro/contabilidade
completos e fiscalização permanecem pendentes. Verificações de consistência não
protegem contra administrador que reescreva todas as fontes/evidências do banco.
