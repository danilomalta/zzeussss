# B02B8 — conferência física e relatório dos cofres

## Base observada

B02B7 instalada no PC em `c686c44`, branch `feat/backend-b02a-compat`, worktree
`zzeus-frontend-backend-b02a`. O log confirma testes Go, race,89 testes locais do
frontend, build e árvore limpa. Esta entrega reúne dois fluxos de backend:
conferência de cofre com revisão independente e relatório/exportação consolidados.

Não altera autenticação, sessões, frontend, movimentos de estoque, pagamentos
eletrônicos ou protocolo legado. As etapas de produção P01–P12 não recebem novo
aceite por causa deste pacote.

## Conferência e bloqueio do saldo

Um gestor com `ManageCash` e contrato POS inicia a conferência de um cofre ativo,
com motivo e outra pessoa responsável pela contagem. Essa pessoa precisa ter
`ManageCash` na mesma loja e aparelho no momento da atribuição. O backend deriva
autor, tenant, loja e aparelho da sessão. O criador não pode ser o contador.

Na mesma transação, a conferência registra o saldo exato e o último ID de operação
do cofre. Uma única conferência pode permanecer aberta por cofre. O saldo fica
congelado nos estados `counting` e `submitted`:

| Ação | Estado e responsável | Efeito |
| --- | --- | --- |
| Criar | Cofre ativo; criador e contador diferentes | `counting`; congela saldo |
| Submeter | `counting`; somente o contador atribuído | `submitted`; registra contagem/diferença |
| Aceitar | `submitted`; gestor diferente do contador | `accepted`; registra motivo e libera cofre |
| Rejeitar | `submitted`; gestor diferente do contador | `rejected`; registra motivo e libera cofre |
| Cancelar | `counting`; somente o criador | `cancelled`; registra motivo e libera cofre |

Criador pode revisar, pois não é o contador. Depois de submetida, a contagem não
pode ser editada nem cancelada; cabe revisão. Após o estado terminal, nova
conferência exige novo ID, preservando o histórico anterior. Não há reatribuição
de conferência aberta: antes da submissão, o criador pode cancelar e criar outra.
Escritas continuam exigindo permissões e licença vigentes; não há bypass para
usuário revogado ou contrato expirado. Outro gestor autorizado pode revisar uma
contagem já submetida, respeitando a separação do contador.

O bloqueio recusa depósito, suprimento, arquivamento e transferência de saída ou
entrada. A verificação fica dentro da transação B02B7, antes do movimento real de
caixa. Uma trigger protege também a atualização direta do cofre enquanto houver
conferência aberta. Operações antigas idempotentes podem retornar seus recibos
originais durante o bloqueio; não movimentam dinheiro novamente.

## Valores e limites da contagem

Contagem é uma declaração da pessoa autenticada em centavos inteiros, de zero
até9007199254740991. Diferença = contado − esperado, calculada exatamente. Um
saldo290 e contagem280 geram diferença−10. Aceitar ou rejeitar registra a revisão,
mas mantém saldo290. Uma contagem máxima em cofre zerado não fabrica dinheiro.
Não há ajuste automático, baixa de perda, perdão da divergência ou correção de
lançamentos anteriores. Uma divergência aceita ainda pode exigir apuração externa.

O campo esperado é `null` no registro/recibo de uma contagem ainda não submetida
ou cancelada sem contagem. Ele aparece junto à diferença depois da submissão.
Isso evita sugerir o valor no formulário de contagem, mas NÃO é isolamento de
saldo contra gestores: as consultas de cofre, extrato e recibos B02B7 continuam
autorizadas para `ManageCash`. Não se anuncia conferência cega com garantia de
sigilo, validação de cédulas, câmera ou prova física do dinheiro.

## Relatório e CSV

GET `/local/v1/cash/vaults/report` consulta todos os cofres do tenant/loja/aparelho
atual em uma transação. Mostra saldo total, ativos/arquivados, conferências em
contagem, aguardando revisão e quantidade de últimas contagens divergentes.
Cada linha contém o cofre e sua conferência mais recente, quando existente.
Todos os históricos de conferência desses cofres são conferidos, inclusive os
substituídos, antes da resposta. Os saldos conferem com as fontes/recibos B02B7.

O saldo total inclui apenas cofres. Não soma caixa, malotes ainda não creditados,
contagem física ao saldo, banco ou estoque. Transferência entre cofres não aumenta
esse total. Última contagem pode ser histórica e anterior aos movimentos atuais;
`vault_operation_id`, datas e estado deixam esse vínculo explícito. Uma nova
contagem passa a ser a última; divergências antigas continuam no histórico de
conferências. O indicador não afirma que todas as diferenças antigas foram resolvidas.

GET `/local/v1/cash/vaults/report.csv` usa o mesmo serviço de snapshot completo,
com linha de resumo e uma linha por cofre. Formato1, UTF-8 com BOM, separador `;`,
CRLF e valores em centavos inteiros. A consulta JSON e o CSV feitos em momentos
diferentes podem observar movimentos diferentes; cada resposta é internamente
consistente. O CSV não faz uma segunda consulta para preencher linhas ou totais.

Limites:1000 cofres e2MiB por CSV. Soma fora da faixa exata ou quantidade acima
do limite retorna422 sem relatório/arquivo parcial. Texto incompatível com CSV
seguro (início de fórmula, controles, caracteres iniciais não suportados) retorna409
sem reescrever nomes ou IDs; JSON permanece canônico. Campos monetários numéricos
podem ser negativos na diferença. Download tem nome fixo, `no-store` e `nosniff`.
Não há benchmark de carga comercial; conferência completa cresce com os históricos.

## Contratos HTTP

Dez operações novas, todas protegidas por sessão e `ManageCash`. Escritas exigem
POS; leituras históricas não exigem contrato POS ativo.

| Método | Caminho após `/local/v1` |
| --- | --- |
| POST / GET | `/cash/vault-counts` |
| GET | `/cash/vault-counts/operations/{id}` |
| GET | `/cash/vault-counts/{id}` |
| POST | `/cash/vault-counts/{id}/submit` |
| POST | `/cash/vault-counts/{id}/accept` |
| POST | `/cash/vault-counts/{id}/reject` |
| POST | `/cash/vault-counts/{id}/cancel` |
| GET | `/cash/vaults/report` |
| GET | `/cash/vaults/report.csv` |

Lista: `limit`1–100, padrão50; `offset`0–1000000000, padrão0. GET singular/relatório
e POST recusam query; corpo GET, parâmetros duplicados/desconhecidos ou JSON extra
também são recusados. Corpo de escrita até8192bytes. IDs novos ASCII seguros para
caminhos, até128 caracteres. Motivo obrigatório na abertura, revisão e cancelamento.
Contrato: `docs/api/cash-vault-counts.openapi.json`; inventário e OpenAPI principal
atualizados. Relatórios são montados antes da rota genérica de cofre para evitar
que `report` seja interpretado como ID.

Erros:400 entrada,401 sessão,403 acesso/licença/ator,404 recurso/recibo alheio,
409 estado, bloqueio, histórico inconsistente ou texto CSV incompatível,413 corpo,
422 limite,500 falha interna,503 serviço de contratos indisponível.

## Persistência, integridade e coordenação

Migração nova `0050_cash_vault_counts.sql`. Scripts0001–0049 inalterados. Backup
passa a25–50; bases históricas25–49 são restauradas e migradas até50 em teste.
Fixtures de esquema futuro passam para51. Não modifica o executor de migração.

Três tabelas: conferências, operações imutáveis e eventos privados imutáveis.
Snapshot, estado, histórico, recibos e evento são gravados na mesma transação.
Triggers impedem apagar conferências, reescrever fonte/atribuição e modificar
estados terminais. Leituras conferem sequência de ações, atores, snapshots,
recibos e eventos. Conferência aberta também confere saldo/head congelados do
cofre. Inconsistência causa recusa sem reparo ou números parciais no relatório.
Isso não protege contra administrador que reescreva banco e todas as evidências.

Idempotência usa `operation_id` estável por tenant/aparelho: mesmo conteúdo/autor
recupera recibo original; conteúdo diferente409; recibo de outro autor404. Um
recibo de abertura continua sem esperado após a contagem ser revisada. Reenvio
não substitui a consulta do estado atual. Histórico permanece após reinício e backup.

Compartilhados: `cashvault/write.go`, novo `cashvault/count_lock.go`,
`localapi/server.go`, `backup/backup.go`, testes de backup/migração e contratos.
`cashvault.InspectTx` valida cofre usando transação do chamador, sem commit.
Bloqueio consulta0050; por isso a migração precisa existir antes do uso do código.
O aplicador valida numeração com a produção, hashes/preimagens e branch/base
B02B7 `c686c44`, e repete as guardas antes do commit. Sem merge/push.

Eventos `cash.vault.count.*` ficam na tabela privada local. Nenhum evento novo
entra na outbox legada nem recebe transporte remoto fictício. Conferências não
criam movimentos de caixa, estoque ou dinheiro em cofre.

## Validação e pendências

Comandos/resultados executados em `EVIDENCIAS.txt` do pacote. Testes em SQLite
descartável e HTTP cobrem congelamento em ambas as pontas, contador/revisor,
cancelamento, zero/máximo, divergência sem ajuste, rollback, concorrência,
reinício/backup, recibos históricos, contratos, CSV e inconsistências.

O frontend completo do PC não está disponível aqui. O aplicador exige npm ci,
testes locais e build no PC antes do commit. Integração visual, prova física,
ajustes autorizados de diferenças, banco/adquirente, cartão/Pix, pagamentos
mistos dosR$315, despesas e financeiro contábil completo permanecem pendentes.
