# B02B14 — fluxo local de contas a pagar com espécie auditada

## Estado e fronteira da entrega
O log do proprietário confirma B02B13 aplicada e testada sobre B02B12
`af33023895374d22e7e18e1178943e570c52a7fe`, branch `feat/backend-b02a-compat`,
worktree `~/Downloads/zzeus-frontend-backend-b02a`, ainda com nove arquivos
pendentes e sem commit. Go completo/vet/race,89 testes locais e build19,35s
passaram no PC. Isso valida aquela worktree, não as interfaces113/114/115 da
outra pasta.

Este pacote consolida B02B13 e o fechamento do fluxo sem reaplicar pacotes antigos.
Reutiliza integralmente serviços B02B11 (documentos/parcelas), B02B12 (vínculo e
devolução em espécie) e B02B13 (fontes/saldos). Acrescenta somente o recibo GET do
movimento original, testes de aceite e contrato único com exemplos verificáveis.
Não altera campos/semântica dos endpoints existentes. Novo GET é aditivo.

Esquema53 e SQL0001–0053 permanecem inalterados. Nenhuma nova migração é necessária.
Não altera frontend, autenticação/sessões, política central, PostgreSQL, estoque,
produção, backup, smartphone ou integração de pagamentos. Foco desktop
Windows/Linux; validação executada no Linux, sem ensaio nativo Windows.

## Contrato único para o frontend
`docs/api/payables-flow.openapi.json` é OpenAPI autocontido:19 caminhos com schemas,
parâmetros, permissões e32 exemplos de requisição/resposta de teste, incluindo
recusa, replay, recuperação, devolução e relatório. Sem dependência de resolver
schemas de outros arquivos. Os contratos originais continuam válidos.

`docs/api/examples/payables-flow.json` registra a sequência executável dos exemplos.
`docs/api/examples/payables-flow.csv` é o CSV final efetivo, com BOM UTF-8,
separador ponto e vírgula e CRLF. Um atributo Git restrito a esse fixture preserva
os bytes CRLF entre Windows/Linux e permite somente esse fim de linha no check
de whitespace; não altera configuração global. Exemplos são dados de teste, nunca tokens ou
dados comerciais. Apenas IDs gerados de contexto/ator/movimento e timestamps
foram normalizados; valores, estados, IDs de operação, vínculos e status HTTP
vieram dos handlers reais e SQLite descartável. Testes comparam os exemplos e
OpenAPI ao resultado atual e recusam divergência. A exportação deliberada de
fixtures é opção de teste para diretório scratch, nunca uma ação de produção.

A seguir todos os caminhos têm prefixo `/local/v1` e token opaco
`Authorization: Bearer <sessão em memória>`. Não persistir token com comandos.
JSON de escrita usa `Content-Type: application/json`; exemplos omitem o token.
Empresa/loja/aparelho/ator vêm da sessão. Não enviá-los no corpo nem em query.

### Endpoints do fluxo

| Método e caminho | Entrada | Sucesso e efeito |
| --- | --- | --- |
| POST /finance/payables | operation_id, payable_id, supplier_id, document_reference, description, issued_on, currency=BRL, total_cents, installments[{id,due_on,amount_cents}] |201 PayableResult; cria documento/parcelas/histórico/evento privados, sem mover dinheiro; replay200 |
| GET /finance/payables | state opcional; limit1–100 padrão50; offset0–1000000000 |200 PayablePage: items,total_count,limit,offset,has_more; não modifica |
| GET /finance/payables/{id} | ID documental; sem query/corpo |200 Payable, incluindo cash atual; não modifica |
| POST /finance/payables/{id}/approve | operation_id,reason |200 PayableResult; revisão independente, sem dinheiro |
| POST /finance/payables/{id}/reject | operation_id,reason |200 PayableResult; recusa independente, sem dinheiro |
| POST /finance/payables/{id}/cancel | operation_id,reason |200 PayableResult; cancela se estado/ator/saldo permitem, sem dinheiro |
| GET /finance/payables/operations/{operation_id} | sem query/corpo; próprio ator |200 recibo documental original;404 ausente/inacessível; não modifica |
| GET /finance/payables/{id}/installment-balances | sem query/corpo |200 InstallmentBalances; todas as parcelas e resumo num snapshot, sem dinheiro |
| GET /finance/payables/{id}/cash-sources | installment_id obrigatório; limit/offset como acima |200 CashCandidates; sangrias elegíveis próprias; nenhuma reserva |
| POST /finance/payables/{id}/cash-payments | operation_id,payment_id,installment_id,withdrawal_operation_id,recipient,reference |201 CashResult; vincula uma sangria já registrada; replay200; não cria saída |
| GET /finance/payables/{id}/cash-payments | limit/offset como acima |200 CashPage; pagamentos/devoluções atuais, resumo completo e paginação, sem dinheiro |
| GET /finance/payable-cash/{payment_id}/return-sources | limit/offset como acima |200 CashCandidates; suprimentos próprios elegíveis para devolução integral, nenhuma reserva |
| POST /finance/payable-cash/{payment_id}/return | operation_id,supply_operation_id,reason |201 CashResult; vincula suprimento já registrado; replay200; não cria entrada |
| GET /finance/payable-cash/operations/{operation_id} | sem query/corpo; próprio ator |200 recibo original do vínculo/devolução;404 ausente/inacessível; não modifica |
| GET /finance/payables/report | from,to,as_of obrigatórios, YYYY-MM-DD; sem outros filtros |200 Report completo, sem paginação ou dinheiro |
| GET /finance/payables/report.csv | mesma janela |200 CSV oficial formato2; sem paginação ou dinheiro |
| GET /cash/movements/operations/{operation_id} | novo: sem query/corpo; ID percent-encoded uma vez |200 MovementReceipt do próprio ator; fonte auditada validada, caixa aberto/fechado; nenhuma mutação/reserva |

Todos os GET recusam corpo e parâmetros extras/repetidos/vazios. Onde há
paginação, página vazia continua com resumo/total reais; não usar sua lista vazia
para inferir saldo zero. As escritas financeiras não aceitam query e exigem
campos exatos, sem extras/duplicação/null. Limites de corpo: criação32KiB,
transição4KiB, vínculo/devolução8KiB. Falha413 antes de executar o comando.

### Pré-requisitos de fornecedor e movimento físico — operações separadas

Fornecedor deve existir em `/purchase-suppliers`, consultado com ViewOrders por GET com offset
(páginas de50). Cadastro POST e seu contrato estão no OpenAPI consolidado como
pré-requisito; requer ManageReplenishment e Orders/Core, não permissão de pagamento.
Este fluxo não cadastra fornecedor automaticamente nem chama criação ao abrir tela.
Fornecedor/documento local não significam comunicação ou aceite remoto.

O OpenAPI também inclui, explicitamente como pré-requisitos independentes:
GET `/cash/current`, POST `/cash/open` e POST `/cash/movements`.
Abertura exige Sell + POS/Core e ID de sessão estável; não tem operation_id:
recupera pelo turno/session_id e consulta do caixa. POST de movimento exige
ManageCash + POS/Core, turno aberto e saldo/fonte segundo as regras de caixa.
Corpo: session_id,operation_id,kind=withdrawal|supply,amount_cents positivo,reason.
Retorna MovementResult200, com repeated=true somente no replay exato. Essa é a
operação separada que registra uma nova saída/entrada declarada física e seu
journal/outbox. Não faz transferência bancária nem comprova entrega ao fornecedor.

Após resposta perdida do movimento, usar o novo GET de recibo, inclusive com turno
aberto e licença expirada para a leitura autorizada. Mesma chave/conteúdo no POST
continua idempotente, mas não é substituto da consulta nem dispensa licença atual.
Movimentos existentes com valores fora do limite de inteiros seguros para este
fluxo financeiro não são apresentados como recibos utilizáveis (409).

### Autorizações

Todos os GET financeiros, inclusive recibos e fontes, exigem ViewAccounting;
não exigem licença Finance vigente para leitura histórica. Recibos de operação
e movimento são exclusivos do ator original; outro ator recebe404, inclusive
quando consegue consultar o documento atual. Perda de permissão403 ou sessão401
não significa operação ausente: preservar pendência e recuperar acesso.

Escritas financeiras exigem ManageCash + Finance/Core vigentes e verificados
na transação. Aprovar/rejeitar exige ator diferente do criador. Pendente só pode
ser cancelado pelo criador; aprovado só por outro ator e sem valor aplicado.
A interface precisa dos acessos de leitura e escrita para completar o fluxo.
A fonte deve pertencer ao autor do movimento, mas quem devolve pode ser outro
ator desde que seja autor do suprimento. Nenhum privilégio é concedido pela UI.

### Estados, correção e histórico

Transições permitidas: criação → pending; pending → approved/rejected por outro
ator; pending → cancelled pelo criador; approved → cancelled por outro ator e
somente com aplicado zero. rejected/cancelled são terminais. Não há edição nem
exclusão de parcelas/documento/pagamento/devolução. Referência documental é
reservada por fornecedor/contexto sem distinção ASCII entre maiúsculas e
minúsculas; cancelamento não libera referência ou IDs.

Para erro antes de pagamento, pode cancelar conforme a regra e registrar outro
documento real com identificação distinta. Não inventar nova referência fiscal
para contornar a unicidade. Com histórico financeiro, não alterar nem apagar o
original: qualquer devolução deve corresponder a retorno físico realmente
registrado, nunca ser criada só para liberar edição. Após devolução de todos os
pagamentos pode cancelar segundo a regra; isso não é estorno fiscal/bancário.
Correção sob a mesma referência exige futura substituição documental versionada,
ainda não suportada. Essa limitação deve aparecer como operação indisponível,
não como um formulário de edição ou cancelamento automático.

### Valores, fontes e limites

Centavos inteiros1–9007199254740991; nunca converter por float multiplicado e
arredondamento posterior. Até120 parcelas positivas por documento, soma exata,
IDs distintos, vencimentos em ordem não decrescente e não anteriores à emissão;
datas civis válidas YYYY-MM-DD, ano>=1900. Até1000 documentos locais e1000
pagamentos históricos por documento, incluindo devolvidos. Bruto histórico
limitado ao máximo seguro mesmo depois de devolução.

IDs financeiros: ASCII alfanumérico na primeira posição, depois também `_ . : -`,
até128bytes. document_reference e referência do recibo seguem esse formato.
Description/reason:1–500 caracteres UTF-8, texto sem NUL e sem espaços externos;
recipient:1–255. supplier_id deve ser ID existente, até128 caracteres. Para o novo
GET de movimento, preservar o alfabeto legado UTF-8 até128bytes e codificar uma
vez no caminho, inclusive `%` e `/`; não confundir o ID com rota ou URL externa.

Principal vem de installment.amount_cents. Bruto pago é paid_cents; devolvido é
returned_cents; aplicado=bruto−devolvido. Saldo=principal−aplicado somente aprovado,
zero ativo nos demais estados. Não somar principal/aplicado/saldo como receitas.
`basis=declared_cash_audited_sources`, `settled_cents=null` e
`payment_execution_available=false` preservam a distinção de banco/operadora.

Sangria usa valor inteiro da fonte em uma parcela; admite aplicações parciais
por várias sangrias até o saldo, mas não divide uma sangria. Origem utilizada em
qualquer histórico de malote ou pagamento fica excluída para sempre. Devolução
é integral por pagamento, via um suprimento de mesmo valor, não anterior à
sangria original, não usado nem financiado por cofre; uma devolução por pagamento.
Devoluções parciais ainda não são suportadas. Fonte física/ator/contexto/journal
são revalidados na escrita. Consultar fonte não a reserva nem aprova pagamento.

Fontes retornam amount_rule (até saldo ou valor exato), amount_limit_cents,
blocked_reason, total_count,limit,offset,has_more, complete=true,
reservation_created=false e write_authorization_checked=false. Cada item mostra
operation_id,session_id,movement_id,actor_id,kind,amount_cents,occurred_at.
Blocked_reason: document_not_approved,installment_fully_applied,payment_history_limit,
historical_amount_limit,payment_already_returned; vazio indica ausência desses
bloqueios. Com total_count=0 e motivo vazio, nenhuma fonte própria desse tipo
atende às regras; não há autorização para criar movimento automaticamente.
Com total_count>0 e items vazio, é apenas página fora do intervalo.

Ordem de documentos: created_at,id; parcelas: posição original; pagamentos:
created_at,id; fontes: operation_id. São snapshots por requisição, sem cursor
congelado entre páginas. Novas operações podem alterar páginas; atualizar e
deduplicar por ID, nunca declarar uma coleta incompleta como saldo fechado.
Todas as fontes do ator/tipo são validadas antes da página; limite10000 incluindo
antigas/usadas; acima disso422. Inconsistência fora da página também409.

Agenda: janela de até366 datas inclusivas, ambos limites incluídos, ordenada por
vencimento,payable_id,installment_id. Até10000 parcelas e CSV4MiB, sem truncamento.
Valida documentos/fontes completos, mesmo com filtro de datas ou página vazia.
`complete=true` significa esse conjunto autorizado e essa janela, não todos os
aparelhos ou bancos. `as_of` classifica overdue/today/future; os saldos são atuais,
não uma reconstrução histórica. CSV2 tem20colunas: formato/tipo/escopo/janela/moeda,
métrica/valor e identificação/principal/vencimento/classificação/aplicado/saldo.
Linhas summary e installment são seções distintas; não somar ambas. Texto CSV
incompatível com exportação segura é recusado409, sem arquivo parcial.

### Idempotência e recuperação de resultado incerto

Salvar intenção e IDs estáveis antes do envio; confirmação explícita do usuário
precede cada POST. Separar pendências por empresa,loja,aparelho,ator. Mesmo ID e
conteúdo retorna recibo original e repeated=true no replay; conteúdo diferente409;
ID de outro ator404. GET de recibo retorna conteúdo original repeated=false e não
estado atual mutável; original de pagamento mantém return=null depois da devolução.
Para estado atual usar documento/lista/parcelas/pagamentos, não reescrever recibo.

| Observação | Interpretação e próxima ação |
| --- | --- |
| GET recibo200 e IDs/contexto/conteúdo conferem | Confirmado; consultar projeções atuais sem novo POST |
| GET recibo404 com mesmo ator/contexto original | Ausente nessa consulta; não é prova de recusa persistida. Só retry explícito com os mesmos IDs/conteúdo |
| POST recebeu400/403/409/413/422, primeira tentativa sem incerteza prévia | Solicitação recusada; consultar recibo para conferir ausência antes de liberar/corrigir intenção |
| Timeout/queda/5xx/401/403 na recuperação ou resposta inválida | Resultado desconhecido; manter pendência. Falha na consulta não autoriza envio |
| Tentativa anterior incerta, retry retorna recusa | A recusa posterior não elimina a incerteza anterior; continuar recuperando a operação original |

Não existe registro durável de todas as recusas;404 não distingue “nunca enviado”
de “enviado e recusado”. O cliente preserva a recusa que efetivamente recebeu e
sua incerteza anterior. Não apresentar um enum fictício de recusa persistida.
Perda de resposta depois do commit foi testada com HTTP loopback real e descarte
intencional da resposta no transporte do teste; GET recuperou, sem segundo POST.
Isso não comprova operação no PC comercial nem sincronização entre aparelhos.

### Erros estáveis
Solicitar `X-Titan-Error-Format: v1` para JSON genérico `{error:{code,message}}`.
Sem o header, preservar o formato legado; status HTTP é autoridade.
400 invalid_request;401 unauthorized;403 forbidden;404 not_found;409 conflict;
413 payload_too_large;422 unprocessable_request;429 rate_limited;
503 unavailable; demais5xx internal_error.409 pode ser estado, conteúdo diferente,
reuso de origem ou inconsistência: consultar e preservar histórico, não assumir
qualquer um deles nem inventar detalhe não retornado pelo servidor. Erro não
expõe SQL, credenciais ou conteúdo do pedido.503 não autoriza execução offline.

## Exemplo verificado e demonstração curta de API
O fixture cria “Padaria”, documento NFe-123 de31500centavos, com parcelas10000,
9600 e11900. Outro ator aprova. Sangrias6000 e4000 são selecionadas e vinculadas à
primeira parcela; uma nova aplicação acima do saldo é recusada. Suprimento6000
devolve integralmente o primeiro pagamento. Após reinício:

| Métrica | Documento | Primeira parcela |
| --- | --- | --- |
| Principal |31500 |10000 |
| Pago bruto |10000 |10000 |
| Devolvido |6000 |6000 |
| Aplicado |4000 |4000 |
| Saldo aprovado |27500 |6000 |

Demais parcelas permanecem9600 e11900 de saldo. Relatório e CSV exibem aplicado4000
e saldo27500, sem marcar liquidação bancária.32 etapas HTTP de teste no JSON/Swagger
incluem reenvios e recuperação. Nenhum dinheiro físico foi executado pelo ensaio.

## Aceite, pacote e retomada
O pacote contém um único patch consolidado e payload final, não uma pilha de
patches antigos. Instalador aceita baseB02B12 af33023 limpa ou com o estado exato
B02B13 já aplicado; também reconhece commitB02B13 direto sobre essa base se os
conteúdos conferirem. Qualquer outra revisão ou edição independente interrompe.
Antes de aplicar, valida raiz/branch/HEAD/status, hashes, SQL e gera um único diff
correspondente ao estado reconhecido; usa git apply --check. Não reaplica B02B13.
Após falha de teste retoma a validação do payload já aplicado, sem duplicar patch.
Se houver conflito, preservar checkout e dados e adaptar a partir da saída;
nunca reset/clean/renumeração. Nenhum commit, merge ou push automático.

Regressão completa,vet,race e build devem passar; marcador só fica aprovado após
todas as verificações. Testes usam SQLite descartável e opt-ins externos são
removidos; não iniciam o servidor comercial nem abrem o banco de demonstração.
Frontend verificado é apenas o existente na worktree de backend; interfaces da
outra pasta e aceite visual continuam com o outro chat. A aplicação deste pacote
no PC só fica confirmada após log do proprietário.

Mudanças compartilhadas: duas chamadas de montagem acumuladas em server.go
(B02B13+B02B14), inventário/mapa e índice de contratos. Demais arquivos próprios
são fontes, testes e documentação. Nenhuma migração/backup/frontend alterado.
A documentação de produção permanece apenas referência histórica; seu roteiro
não é declarado concluído por esta entrega.

## Dependências e próximos fluxos, sem implementação nesta entrega
1. Compras/recebimento conferido → estoque/financeiro: já existem serviços e
migrações de produção até44; inventariar compras, recebimentos, autorizações,
rejeições/anulações e saldos antes de ligar obrigação financeira. Falta vínculo
comprovado entre recebimento local e este documento financeiro; não duplicar
estoque/título. Frontend115 cria pedido local, não aceite/recebimento remoto.
2. Comunicação autorizada com fornecedor/agenda: transporte técnico existente
não equivale ao contrato comercial de confirmação/doca; definir identidade,
consentimento, entrega idempotente e aceite remoto sem enviar dados privados.
3. PDV/pagamentos/fiscal: reaproveitar planos/cash/recibos existentes; Pix/TEF,
venda mista e emissão fiscal dependem de contratos/provedores e aceite próprios.
4. Produção: reaproveitar receitas/ordens/materiais/etapas/resultados/perdas/lotes
existentes, verificar rastreabilidade/estoque antes de expandir a interface.
5. Desktop Windows/Linux: inventariar instalação/atualização/backup/restauração
existentes; ensaio nativo Windows e recuperação de instalação ainda necessários.

O próximo fluxo sugerido é recebimento conferido ligado a estoque e obrigação
financeira, após inventário real e coordenação de migrações. Não se presume
pronto nenhum desses fluxos pela existência de uma pasta ou tela.
