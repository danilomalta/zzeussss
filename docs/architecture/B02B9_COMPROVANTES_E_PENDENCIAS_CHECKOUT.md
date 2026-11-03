# B02B9 — comprovantes informados e pendências do checkout

## Base e efeito real

B02B8 instalada no PC em `9eec12d`, branch `feat/backend-b02a-compat`, worktree
`zzeus-frontend-backend-b02a`. O log confirma Go completo, race,89 testes locais
do frontend, build e árvore limpa. Esta entrega implementa dois fluxos:
registro/revisão documental de referências de comprovantes de parcelas e fila
com relatório/CSV das pendências e evidências aceitas.

Documento informado e revisão humana NÃO são confirmação de pagamento pela
adquirente/banco. Todo DTO tem `effect=evidence_only` e `provider_confirmed=false`.
O relatório tem `scope=unverified_checkout_evidence` e `settled_cents=null`.
Não gera venda, recebível, liquidação, dinheiro no caixa, consumo de estoque,
desconto, crédito ao cliente, aprovação de cartão/Pix ou confirmação fiscal.

O cenárioR$315 continua alocado em dinheiro10000, débito9600 e crédito11900.
Mesmo depois de aceitas as duas referências eletrônicas, o plano permanece
`prepared`, parcelas `not_requested`, `can_finalize=false`. A rota cash-only
recusa esse plano. A API de evidência é separada do contrato de pagamento real.

## Vínculo e cadastro

Somente o operador original, com `Sell` e contrato POS, pode registrar uma
referência em um plano próprio preparado, no mesmo tenant/loja/aparelho. A parcela
precisa existir e ter método `debit`, `credit` ou `pix`. Dinheiro e `creditAccount`
são recusados: documento não autoriza concessão de crédito nem altera o caminho
de dinheiro já implementado. Valor e método vêm da parcela persistida, não do JSON.

Entrada: `operation_id`, `proof_id`, `provider`, `terminal`, `reference` e
`occurred_at`. O identificador da parcela e do plano vem da URL. Provider/terminal
são identificadores manuais ASCII, até80 caracteres; não são conectores configurados.
Referência tem até160 caracteres; data exige RFC3339 e é armazenada em UTC. O
horário é uma informação do operador, sem validação do relógio de um provedor.
Não há upload de imagem, OCR, leitura do conteúdo ou validação de autenticidade.

Uma referência pode ser registrada depois do encerramento do caixa ou mudança
de preço/estoque, pois descreve atividade informada anteriormente. O plano e
seus valores históricos continuam sendo a fonte. Isso não reabre o turno nem
reserva estoque. Todas as escritas continuam exigindo contrato e permissões atuais.

## Revisão, retirada e substituição

| Ação | Estado e responsável | Resultado |
| --- | --- | --- |
| Registrar | Plano preparado; operador original | `submitted` |
| Aceitar documentalmente | `submitted`; `ManageCash`, diferente do operador | `accepted` |
| Rejeitar | `submitted`; `ManageCash`, diferente do operador | `rejected` |
| Retirar | `submitted`; operador original com `Sell` | `withdrawn` |
| Revogar aceite documental | `accepted`; `ManageCash`, diferente do operador | `revoked` |

Aceite, rejeição, retirada e revogação exigem motivo. O autor do documento
não revisa a própria evidência, mesmo se for dono ou gerente. Reviewer atual e
histórico preservam quem fez cada decisão, inclusive se outro gestor revogar.
Somente `submitted` e `accepted` são ativos. Rejeitado, retirado e revogado são
terminais; uma substituição cria novo comprovante e nova referência, sem editar
ou remover o anterior. Só pode haver um comprovante ativo por parcela.

O banco mantém unicidade de tenant/loja/provider/terminal/referência, inclusive
nos documentos terminais. Provider/terminal usam comparação ASCII sem distinguir
maiúsculas; referência é exata. Evita reutilizar a mesma chave para outra parcela,
mas não detecta outra referência inventada nem comprova identidade externa.
É uma restrição neste SQLite, não uma deduplicação global entre bancos/aparelhos.

## Cancelamento conservador do plano

Com qualquer histórico de comprovante externo, `CancelPlan` retorna409, inclusive
se todos forem rejeitados, retirados ou revogados. Essas decisões não provam que
a cobrança externa inexistiu ou foi estornada. Uma trigger protege também a
inserção direta de cancelamento incompatível; outra impede comprovante em plano
já cancelado. Disputa de cadastro/cancelamento admite apenas uma das operações.

Planos sem comprovantes mantêm o cancelamento existente. A B02B9 não implementa
a resolução externa necessária para desbloquear um plano que já teve evidência.
Essa pendência é deliberadamente preservada; apagar documento ou retornar um
status local não será tratado como estorno. Não existe endpoint de reembolso
nem baixa financeira neste pacote.

## Fila, relatório e CSV

Gestores com `ManageCash` consultam fila paginada, filtro de estado, relatório e
CSV no tenant/loja/aparelho atual. Operador com `Sell` consulta seu documento pelo
ID estável; gestor pode consultar documentos da loja/aparelho. Recibos de operação
são exclusivos do próprio autor, inclusive quando o solicitante é gerente.
Leituras históricas não dependem de contrato POS ativo; sessão/permissão continuam
obrigatórias. Operador de caixa não recebe acesso à fila gerencial por ter `Sell`.

Fila padrão: `submitted`. `state` aceita submitted/accepted/rejected/withdrawn/
revoked/all; `limit`1–100, padrão50; `offset`0–1000000000, padrão0. Cada item
retornado confere fonte, histórico, snapshots e evento antes da resposta.

Relatório completo limitado a1000 documentos, incluindo históricos. Soma somente
evidências ativas nos campos `reported_active_cents` e, dentro delas, aceitas em
`accepted_evidence_cents`. Não soma rejeitados/retirados/revogados como valores
ativos. Resumo separado para débito/crédito/Pix informa contagens de submetidos,
aceitos e históricos. Substituir um comprovante não duplica o saldo documental
da parcela. Esses campos não são dinheiro recebido, contas a receber ou receita.
`plans_with_evidence` conta planos distintos, inclusive com documentos terminais.

Exemplo: débito9600 revogado e substituído por outro submetido, crédito11900
aceito: ativo21500; aceito11900; três documentos, um histórico. A parcela cash10000
não entra nesse relatório. Valor liquidado continua desconhecido, `null`.

JSON e CSV usam o mesmo serviço de snapshot integral, em uma transação SQLite.
O CSV não faz nova consulta para preencher as linhas. Duas requisições separadas
podem observar instantes diferentes. Formato1, UTF-8/BOM, separador `;`, CRLF,
linhas de resumo/método/documento, valores exatos em centavos. Liquidado vazio
significa desconhecido. Linhas documentais históricas não devem ser somadas como
dinheiro recebido. Download fixo `titan-comprovantes-checkout.csv`, no-store/nosniff.

CSV até2MiB. Texto incompatível com CSV seguro, incluindo início de fórmula ou
controles, retorna409 sem reescrever referência/IDs; JSON permanece canônico.
Mais de1000 documentos ou soma fora da faixa segura9007199254740991 retorna422.
Inconsistências, overflow e erros não geram números ou arquivo parcial. Não há
benchmark de volume comercial; a validação cresce com documentos/planos/históricos.

## Dez operações HTTP

Prefixo `/local/v1`. JSON estrito e corpo de escrita até8192bytes. GET não recebe
corpo. Somente a fila admite query; parâmetros extras/duplicados são recusados.
IDs novos ASCII seguros para caminho, até128 caracteres. Planos/parcelas mantêm
seus identificadores legados e pertencimento original. Nenhuma autenticação ou
permissão nova é criada.

| Método | Caminho após o prefixo |
| --- | --- |
| POST | `/checkout/plans/{id}/parts/{part}/proofs` |
| GET | `/checkout/proofs` |
| GET | `/checkout/proofs/report` |
| GET | `/checkout/proofs/report.csv` |
| GET | `/checkout/proofs/operations/{id}` |
| GET | `/checkout/proofs/{id}` |
| POST | `/checkout/proofs/{id}/accept` |
| POST | `/checkout/proofs/{id}/reject` |
| POST | `/checkout/proofs/{id}/withdraw` |
| POST | `/checkout/proofs/{id}/revoke` |

Erros:400 entrada,401 sessão,403 acesso/licença/ator,404 documento/recibo/plano
invisível,409 estado/referência/consistência/CSV incompatível,413 corpo,422 limite,
500 erro interno,503 serviço de contratos indisponível. Contrato em
`docs/api/checkout-proofs.openapi.json`; inventário e OpenAPI principal atualizados.
O contrato de planos documenta o novo bloqueio do cancelamento, sem mudar DTOs.

## Persistência e coordenação

Nova `0051_checkout_payment_proofs.sql`; nenhuma alteração em0001–0050. Backup
aceita25–51; snapshots históricos25–50 são restaurados e migrados para51 em teste.
Fixtures futuras usam52, sem mudar executor, checksums, integridade/FKs ou aparelho.

Registro, estado, histórico, recibo e evento privado são transacionais. Triggers
preservam fonte, referência, autor, estados terminais e históricos. Leituras
conferem sequência, atores, snapshots, evento e parcela/plano original. Erro não
repara o passado. Isso não protege contra reescrita de todas as evidências por
administrador do banco. Operações usam `operation_id` estável por tenant/aparelho:
mesmo conteúdo/autor recupera recibo original, conteúdo diferente409, autor alheio404.
Reenvio de um aceite já revogado retorna recibo histórico e não reativa documento.

Compartilhados: `sale/checkout_plan.go` (bloqueio de cancelamento), `localapi/server.go`,
`backup/backup.go`, testes de migração/backup e contratos de rotas/planos. Código
novo usa o pacote sale para validar os snapshots reais de checkout sem transações
aninhadas. Cancelamento passa a depender da existência da migração51.

Eventos `checkout.proof.*` são privados e locais, sem consumidor/transporte remoto
inventado e sem entrada na outbox legada. Protocolo de pagamentos/sincronização,
sale.committed, autenticação/sessões, frontend e deps Go não mudam. O aplicador
confere base B02B8 `9eec12d`, arquivos, produção e migrações antes de aplicar e de
commitar. Nenhum merge ou push.

## Validação e limites

Comandos/resultados em EVIDENCIAS.txt do pacote. Testes em SQLite temporário e
HTTP incluem R$315 sem promoção de pagamento, independência de revisão, caixa
fechado, operador apenas Sell, unicidade/substituições, concorrência, rollback,
cancelamento concorrente, reinício/backup, precisão, relatórios/CSV e integridade.

Frontend completo da base do PC não disponível aqui. O aplicador exige npm ci,
test:local e build antes do commit. Não conectar estas respostas diretamente ao
`Charge96` como `approved`: o contrato desta entrega é documental, não de cobrança.
Adapter visual de comprovantes, confirmação real do provedor, resolução/estorno
externo, pagamentos mistos, financeiro/fechamento por parcelas reais, contador,
cartão/Pix comercial e integração visual permanecem pendentes. Não se marca o
cenárioR$315 nem o financeiro completo como concluído.
