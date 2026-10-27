# Backend ampliado — carrinhos, recuperação e conferência

## Base real e integração

Fonte obtida diretamente do repositório do proprietário na revisão857febb588b522fdac45d3773e1b772094b19816, branch de origemfeat/backend-b02a-compat. A revisão coincide com a saída enviada pelo usuário. Trabalho isolado emfeat/backend-expanded-20261024, com a árvore inicialmente limpa. Não usa a reconstrução sintética das entregas anteriores.

Frota/Workshop permanece na0059_fleet_maintenance.sql e0060_fleet_workshop.sql. A0058_supplier_returns.sql é idêntica à entrega anterior. Nova migração0061_suspended_carts.sql. Nenhum SQL0001–0060 é modificado. O número59 de carrinhos existia somente no pacote recusado, que não foi aplicado noPC; não é uma renumeração de banco existente. Qualquer ocupação inesperada de61 é recusada, para coordenação.

Compartilhados alterados: montagem das nove operações emserver.go, inventáriosOpenAPI, contadores/testes de migração e compatibilidade de backup até61. O teste de atualização da Frota passa a comparar o total com a cadeia real de SQL, mantendo todas as verificações de schemas/checksums/recibos. Implementações de Frota e Workshop, autenticação/sessões/online, frontend eElectron não foram editadas. O pacote anteriorConsolidacao_Local não deve ser reaplicado; esta entrega o substitui sobre a base857febb.

## Frente1 — carrinhos duráveis e venda em dinheiro

Backend salva500linhas com produto/local/quantidade exata e rótulo; preços/unidades/totais são lidos do catálogo e saldo livre real. Save não exige caixa aberto, mas exige produto ativo e saldo suficiente; não reserva estoque nem cria pagamento. IDs de venda/parcela são gerados pelo servidor, estáveis e imutáveis.

Cada alteração exigeexpected_revision eoperation_id. Gravação só emsuspended. Claim exige caixa do operador/aparelho aberto e compatibilidade atual de preço/unidade/saldo. Uma segunda operação concorrente conflita. Release permite recuperarclaimed após reinício/fechamento do turno e voltar asuspended; Cancel somente emsuspended. Completed/cancelled são terminais.

Finalização só em dinheiro: revalida tudo, usa completeTx existente, registra valor líquido no caixa e retorna troco. Cartão/Pix não são confirmados por declaração do navegador. Venda, estoque, journal, outbox e histórico do carrinho são uma transação; falha no evento final desfaz todos os efeitos. Cancelamento posterior da venda não reabre carrinho. Seu estado completed é histórico; o recibo de venda mostra a compensação.

Consulta da operação devolve o recibo original e permite recuperar resposta perdida sem novoPOST. Todas as leituras são do mesmo tenant/store/device/actor; gerente não adota carrinho privado de outro usuário. GET histórico preserva acesso autorizadoSell mesmo sem licençaPOS atual; escritas exigemPOS. Limites:4096revisões/carrinho,128KiB no save e4096bytes na ação, rótulo160bytes, motivo255bytes e inteiros seguros até9007199254740991.

## Frente2 — diagnóstico antes de retomar

GET /local/v1/pos/carts/:id/reconcile?cash_session_id=ID lê o carrinho e fontes reais na mesma transação. Statusready/blocked/terminal. Bloqueios explícitos: caixa fechado,claim noutro turno, produto inativo, catálogo/quantidade inválidos, saldo insuficiente ou preço/unidade alterados.

Quando a fonte permite cotação integral, current_quote apresenta os valores atuais sem substituir a versão salva. Falta de saldo ou catálogo inválido não produz cotação parcial. Diagnóstico não grava versão, reserva, venda, dinheiro ou confirmação de pagamento; pode apenas avançar a referência temporal da licença já usada pelo backend. Exige licençaPOS atual. É uma fotografia e não garante o estado numa futura finalização. A ação efetiva revalida novamente.

## Frente3 — conferência operacional e CSV

GET /local/v1/pos/carts/report?from=YYYY-MM-DD&to=YYYY-MM-DD e/report.csv usam o mesmo relatório validado. Intervalo inclusivo pelas datas de atualizaçãoUTC, até366dias. Fonte restrita ao usuário/aparelho da sessão. Resumo por estado, idade, linhas, revisão e referência de venda. Não há colunas de dinheiro: carrinho salvo não é faturamento, recebível ou liquidação.

Valida integralmente todos os históricos da seleção antes de retornar. Limites explícitos de1000carrinhos e10000eventos; fonte maior retorna422, sem resumo/CSV parcial. Fonte inconsistente retorna409. CSV só é escrito após validação completa e neutraliza IDs que possam virar fórmulas de planilha. Não há envio ao contador ou mensagem automática.

## APIs e evidência

Nove operações reais emdocs/api/suspended-carts.openapi.json, todas sob/local/v1:POST/GET pos/carts;GET pos/carts/:id;POST pos/carts/:id/actions;GET pos/carts/:id/history;GET pos/cart-operations/:id;GET pos/carts/:id/reconcile;GET pos/carts/report;GET pos/carts/report.csv. Cache-Control:no-store. Campos desconhecidos/duplicados/null são recusados; quantidades são inteiros exatos. Nenhum contexto alegado pelo cliente concede autorização.

Testes novos cobrem as três frentes, preço/estoque/turno, concorrência, rollback, compensação, licença, usuário original, histórico, reinício, restauração, respostaHTTP perdida porloopback, relatório/CSV sem dinheiro, CSV seguro, fonte corrompida e limite de seleção. Aceite transversal de produção por etapas→resultado→gôndola→carrinho→venda→fechamento oficial. Dados de catálogo/identidade são fixtures, não demonstração de interface.

O aplicador verifica a revisão/branch, todas as migrações e pré-requisitos por conteúdo; executa testesGo completos,vet,race selecionado ebuild numa candidata temporária antes de escrever fontes. Logs e backup privados, sem banco comercial, serviço, commit/merge/push. Um segundo uso verifica hashes e não reaplica. Fonte de referência completa inclui Frota/Workshop reais desta revisão, mas o instalador escreve somenteARQUIVOS_ALTERADOS.txt.

## Limites comerciais

Frontend/IndexedDB não são migrados automaticamente. Carrinhos não são compartilhados entre usuários/aparelhos; não existe expiração automática declaim. Relatório usaUTC e não faz fechamento fiscal/financeiro. Entrega em três frentes não é medida de percentual ou triplicação artificial de linhas/bytes.

Pagamento eletrônico, fiscal, financeiro completo, crédito/reembolso comercial e aplicação/reconciliação de todos os eventos comerciais entre bancos independentes continuam pendentes. Frota/Workshop existentes são preservados e testados como parte da suíte, sem converter registros operacionais em pagamento externo. Para concluir os conectores, faltam os provedores/contratos técnicos escolhidos; nunca fornecer senhas/certificados pelo chat. Esta entrega não declara o backend comercial completo.
