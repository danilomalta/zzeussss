# Entrega 36 — Revisão humana durável de sugestões de desconto

Base do PC: main 0767e9e. Cinco funções do backend online, sem integrar a outra branch.

| Função | API (prefixo /api/v1) | Resultado |
|---|---|---|
| Consultar sugestão | GET /discounts/suggestions/:id | Sugestão real e produto atual para conferência |
| Aprovar | POST /discounts/suggestions/:id/approve | Decisão humana definitiva, com motivo e recibo |
| Recusar | POST /discounts/suggestions/:id/reject | Recusa definitiva, preservando motivo e estado anterior |
| Histórico | GET /discounts/suggestions/:id/reviews | Recibos desta API, paginados e isolados por empresa |
| Recuperar decisão | GET /discounts/reviews/:operation_id | Recibo original da operação do próprio operador |

A geração já existente continua calculando sugestões. A revisão não modifica preço do produto,
venda, pagamento, caixa, saldo, documento fiscal nem reserva. Evento outbox indica intenção durável;
não comprova entrega, sincronização ou aplicação no PDV.
O endpoint legado /discounts/suggestions/:id/review permanece indisponível (501).
O frontend e checkout ainda precisam integrar estes cinco novos contratos.

## Decisão

Corpo JSON obrigatório e estrito: operation_id UUID, expected_product_version inteiro positivo,
expected_discount_bps inteiro 0–10000 e reason (motivo não vazio, até 500 caracteres).
1500 bps corresponde a 15,00%. Percentuais usam inteiro exato; não se calcula dinheiro em float.
Sem campos extras/duplicados/null, sem trailing JSON, content type application/json, até 8192 bytes.
ID numérico da sugestão precisa ser canônico, positivo e seguro; sem query no comando de decisão.

Consultar primeiro a sugestão e conferir o produto atual. A geração histórica não gravou a versão
do produto: esta API não afirma que a análise antiga continua atual. O operador confirma versão
e percentual observados no momento da revisão. A integração futura da venda deverá revalidar
contexto e regras; não aplicar desconto antigo automaticamente após mudanças de preço.
Aprovação exige produto ativo; recusa permite produto inativo. Ambas exigem sugestão PENDING,
versão de produto e percentual correspondentes. Uma sugestão só pode ter uma decisão desta API.
Não existe reabertura, troca de decisão ou sobrescrita automática.

## Consistência e autorização

Owner/admin/manager. Stock, cashier, employee e accountant são negados.
Empresa e operador vêm da sessão. Transação revalida usuário/papel atual, empresa ativa,
sessão não revogada e não expirada; frontend não concede permissão.
Ordem: sessão/membro/empresa sob share; lock de operação; recibo anterior; sugestão FOR UPDATE;
produto FOR SHARE. Alteração da sugestão, recibo JSON antes/depois e outbox commitam juntos.
Leituras usam repeatable-read e revalidam sessão. Nenhuma SQL comercial é executada pelo instalador.
As APIs não recebem tenant_id ou usuário no payload.

Mesmo UUID + mesmo payload normalizado + mesmo operador retorna recibo original mesmo depois
de outras mudanças no catálogo. Motivo diferente, outra sugestão, outra ação ou outro operador
com mesmo UUID retorna conflito. Duas operações diferentes para a mesma sugestão disputam o lock:
uma vence; a outra recebe conflito. Falha ao gravar recibo/evento desfaz toda a decisão.
Resposta perdida ou commit incerto: preserve UUID e payload, consulte GET do recibo e só faça
nova tentativa explícita com a mesma identidade. 404 não autoriza repetir com outro UUID.
Não há retry automático nem pagamento confirmado por timeout.

Histórico é da nova API. Decisões legadas eventualmente existentes sem recibo não são inventadas.
Permissão de histórico é empresarial; consulta por operation_id é restrita ao próprio operador.
limit 1–100, offset 0–1.000.000.000; defaults 50/0. Query extra/duplicada e corpo GET são rejeitados.
Respostas autenticadas no-store. Falhas não expõem SQL, segredo, token ou credenciais.

## Migração PostgreSQL 16

Arquivo 000016_online_discount_reviews.sql, incorporado pelo Go.
Acrescenta índice composto tenant/id às sugestões, recibos únicos por sugestão/empresa,
FK de contexto e outbox. Não altera sugestões existentes ou tabela products.
Migrador independente, transacional, serializado, repetível, com checksum explícito e rejeição de
histórico futuro ou adulterado. Preserva versões 8–15; exige schema de lote GTIN 15 já conferido.
Não há migração SQLite ou mudança de backup/produção.

Manutenção comercial deliberada, com servidor em janela apropriada e backup verificado:
titan-online migrate-discount-reviews
titan-online check-discount-reviews
Os comandos existentes migrate-catalog/check-catalog não passam a migrar este módulo implicitamente.
A instalação do patch não executa esses comandos sobre o banco comercial.
Sem a migração necessária, gravação/consulta de recibos retorna indisponibilidade; sem DDL automático.

## Aceite demonstrável no banco isolado

1. Consultar sugestão real e confirmar versão/percentual.
2. Aprovar; repetir mesmo pedido e recuperar mesmo recibo sem nova auditoria/evento.
3. Recusar outra sugestão com motivo; impedir troca de decisão já finalizada.
4. Consultar histórico/página vazia e impedir acesso de outra empresa/operador ao recibo.
5. Forçar falha de outbox, verificar PENDING e ausência de recibo, desativar gatilho de erro apenas
   na fixture e repetir com o mesmo UUID.
6. Fazer quatro chamadas simultâneas com UUID idêntico: um recibo/evento.
7. Fazer aprovação e recusa concorrentes com UUIDs diferentes: um sucesso e um conflito.
8. Alterar versão/percentual, usar papel não permitido e revogar sessão: nenhuma revisão indevida.
9. Confirmar preço e saldo originais após aprovação.
10. Repetir migração e adulterar histórico futuro/checksum em schema descartável: falhar sem publicar versão.

O teste PostgreSQL opt-in existente inclui todos esses cenários. O LEIA_ME distingue verificações
executadas aqui das exigidas no PC. Testes com sqlmock não comprovam execução real no PostgreSQL.
Não anunciar prontidão comercial, percentuais de conclusão ou aplicação de desconto no caixa.
