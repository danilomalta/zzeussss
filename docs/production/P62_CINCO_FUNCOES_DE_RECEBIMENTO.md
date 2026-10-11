# P62 — Cinco funções de recebimento na interface

Base: `95ce8a7`, branch `feat/producao-receitas`. Um ZIP, um patch e um commit após validação.

## Cinco funções

1. Autorizar recebimento por decisão humana local, com referência e motivo.
2. Registrar aceite parcial ou integral de uma entrega, preservando quantidade entregue, aceita, unidade, local e motivo.
3. Recusar uma entrega integralmente, sem entrada de estoque e sem reduzir a quantidade pendente do pedido.
4. Anular um recebimento lançado por engano, mediante motivo e saldo livre suficiente para o movimento compensatório existente.
5. Consultar o recibo original de cada uma dessas ações pelo tipo e ID da operação, independentemente do estado atual do pedido.

Interface: **Pedidos e histórico de recebimento**, `/local/receiving`.

## Uso

- Para autorizar: informe o ID do pedido e clique em **Consultar pedido**. Só aceita pedido local não enviado e ainda não autorizado. Preencha referência e motivo, revise e confirme.
- Para aceitar, recusar ou anular: informe o ID do pedido e clique em **Consultar recebimentos e pendência**. A consulta exibe unidade preservada, quantidades globais e recebimentos da página.
- Aceite: informe referência da entrega, entregue, aceito, ID do local e motivo. Revisão mostra a diferença recusada. Aceito deve ser positivo, não exceder o entregue nem o pendente consultado. O servidor valida tudo novamente e grava apenas o aceito.
- Recusa integral: informe referência, quantidade entregue e motivo. Nenhuma entrada é enviada ao estoque.
- Anulação: selecione um recebimento ativo da página e informe motivo. Confirme a quantidade e o local originais. O servidor decide se o saldo livre permite compensar; esta ação não comprova devolução física ao fornecedor.
- O painel de recibo original e as consultas históricas anteriores permanecem disponíveis. Autorizações e pedidos locais não comprovam transmissão ou confirmação remota do fornecedor.

Quantidade usa milésimos da unidade explícita preservada no pedido, com parser BigInt e limite 9007199254740991. Sem arredondamento, sem conversão massa/volume. A interface exige quantidade inteira para produtos `unit`, como os demais formulários locais de estoque; registros antigos com frações continuam consultáveis. Motivos limitados a 255 bytes UTF-8. Não há recalculo automático de quantidade pendente nem troca automática de ID após conflito.

## Contratos e isolamento

`GET /local/v1/purchase-receiving/operations/{kind}/{id}/receipt`, `kind` em `authorize`, `receipt`, `reject`, `void`.

Resposta: `{kind,input,result}`. `input` é o pedido original completo com `order_id` e `receipt_id` quando aplicável; `result` é o resultado original com `repeated:false`. Recibo de aceite não incorpora anulação posterior. A situação corrente continua sendo consultada separadamente na rastreabilidade/histórico.

Consulta exige sessão, aparelho aprovado e `view_orders`; aceite, recusa e anulação exigem também `manage_stock`. Somente empresa, loja, aparelho e operador originais podem obter o recibo. Fora desse escopo: 404. Tipo/query inválido: 400. Registro/auditoria/vínculo de movimento inconsistente: 409. Não presume confirmação em erro.

Gravações mantêm exatamente os contratos existentes: autorização exige `manage_replenishment` e módulo Orders ativo; demais ações exigem também `manage_stock` e Inventory. Backend controla referência única por fornecedor/loja, unidade preservada, saldo, reservas, overflow, auditoria e atomicidade.

Mudanças compartilhadas: `server.go` acrescenta montagem de uma consulta GET autenticada; `LocalReceivingHistory.tsx` acrescenta os cinco painéis na rota existente; `package.json` acrescenta os testes locais. Ajuste de apresentação da P61: o histórico de estoque também inclui `stock_operations` geradas por aceites e anulações de compras. `StockOperations.tsx`, o comentário de `stock/operation_reads.go` e a documentação/API da P61 passam a descrever corretamente esse conjunto; consulta e cálculos permanecem iguais. Nenhum dos quatro escritores de compras é modificado. Nenhuma alteração em autenticação/sessões, infraestrutura ou migrações PostgreSQL.

Sem nova migração SQLite. Schema **44** preservado, última migração `0044_production_receipt_voids.sql`; whitelist e formato do backup permanecem intactos.

## Recuperação e idempotência

Fila própria por empresa/loja/aparelho/operador, persistida antes do POST, com Web Lock entre abas. Uma pendência bloqueia novas decisões dos quatro formulários. Consulta primeiro o recibo original; GET não pode causar POST automaticamente. Só uma repetição explícita reutiliza o ID e o payload preservados. POST bem-sucedido sozinho não limpa a fila: exige confirmação por GET correspondente ao pedido original. Resultado incerto não pode ser descartado. Recusa definitiva inicial pode ser removida por ação explícita após GET 404; recusa posterior não elimina incerteza anterior.

Após confirmação, consulte o estado atual novamente antes de outra decisão. Revisão é invalidada por mudança de campos, consulta, página ou sessão. Dados do pedido são imutáveis, mas o progresso consultado pode ficar desatualizado; validação transacional do servidor prevalece.

## Verificação

Testes novos cobrem originais das quatro ações após acontecimentos posteriores, consulta sem gravação, restauração em SQLite temporário, escopo de empresa/loja/aparelho/operador, permissões e auditorias malformadas. Testes da interface cobrem payloads exatos, caminhos vinculados ao pedido/recebimento, saldo pendente, limites, unidades, recusa, anulação, paginação e snapshot, perda de resposta, armazenamento bloqueado e confirmação incompatível. Testes existentes de recebimento, recusa e anulação continuam cobrindo rollback por etapa, referência concorrente, recebimento concorrente e anulação contra retirada concorrente de estoque.

Aplicador executa testes específicos, suíte completa, vet, detector de corrida dos fluxos de recebimento/recusa/anulação, testes locais e build TypeScript/Vite. Só então registra os arquivos explícitos e cria commit; em falha preserva alterações. Sem servidor aberto, banco comercial/demo, merge ou push.

Aceitação visual no computador do usuário e integração revisada com main continuam pendentes. Transmissão/aceite real pelo fornecedor e devolução comercial efetiva continuam fora desta entrega.
