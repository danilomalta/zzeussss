# Ciclo de compras, entregas e origem financeira — 22/10/2026

## Base e origem verificadas

Referência local reconstruída dos patches B02B5–B02B12, arquivos finais da B02B14 e oito ZIPs fornecidos pelo usuário. Os hashes dos arquivos e migrações históricos foram conferidos antes da reconstrução. O HEAD de referência local é sintético (`cef21df`); não representa um commit instalado no PC. A última revisão do usuário observada é `58a26bbbdf08903d62f6dd1f0542417171b460dd`, branch feat/backend-b02a-compat, com os oito pacotes posteriores nos fontes, sem commit. Esta entrega não audita commits do frontend.

Predecessores necessários: Inventário/Custos R2, Gestão/Produção, Agenda, Indicadores, Utilização, Conferência, Painel e Tratativas. Esta linha possui56migrações antes da entrega. Nova migração0057_purchase_supply_cycle.sql. Não renumerar SQL; bases44–47 usadas em outro desenvolvimento não são suportadas pelo aplicador desta entrega.

## Fluxo implementado

1. Um pedido já criado a partir de reposição aprovada recebe registro humano da resposta de fornecedor externo: confirmado/parcial ou recusado; quantidade, prazo prometido, contato, referência, instante observado e justificativa. Revisões preservam a operação anterior. É `manual_external` e `external_verified=false`; não há autenticação do fornecedor, envio ou prova de concordância remota.
2. O comércio cria uma entrega com ID estável, pedido, referência única por fornecedor/loja e quantidade. Todas as entregas ativas compartilham a quantidade confirmada. Recebimentos legados também entram no orçamento. Planejar não reserva estoque físico nem autoriza recebimento.
3. Pode vincular uma agenda ativa do mesmo pedido e registrar a referência manual de aceite da janela. A agenda continua com seu próprio histórico e revisão; o vínculo não transforma reserva local em aceite externo. Pode vincular tratativa aberta/em andamento do mesmo pedido/fornecedor com encaminhamento reposição. Planejar não resolve a tratativa.
4. Estados: planned → loaded → arrived → received. Cancelamento explícito permitido em planned/loaded/arrived, desde que não exista recebimento/recusa legada para a mesma referência. Carga e chegada exigem referência e instante de conferência manual. Não são observações de aplicativo do motorista ou GPS.
5. Em arrived, conferência registra entregue e aceito. Reutiliza exatamente o serviço existente de recebimento: autorização local e licenças Orders/Inventory, unidade preservada, saldo/reservas, entrada de estoque, auditoria e outbox. Tudo e a revisão da entrega confirmam na mesma transação. Falha no último evento não deixa entrada parcial.
6. A quantidade aceita efetiva consome o compromisso; diferenças recusadas/faltantes podem compor nova entrega por ação humana, sem ultrapassar o compromisso. Estorno remove a aceitação efetiva, mas mantém entrega, entrada original e operação de estorno. Não confirma retorno físico.
7. Uma entrega recebida pode originar uma única conta a pagar nova, com documento/referência conferida, valor declarado e parcelas. A entrada não calcula automaticamente preço, imposto ou custo histórico. O serviço financeiro existente valida valor, fornecedor, referência única, datas e soma das parcelas. Documento pendente, parcelas, recibo, origem e eventos confirmam na mesma transação. Uma falha não deixa conta órfã. Reenvio com mesmo conteúdo recupera o original; outro ID não financia novamente o mesmo recebimento.
8. A aprovação financeira exige responsável diferente do criador, pelas regras anteriores. Pagamento parcial e devolução integral de cada pagamento seguem os serviços B02B12 existentes, vinculados a sangria/suprimento auditados; não executam transferência externa. O teste vertical percorre esses serviços reais em SQLite descartável.
9. Aprovação e pagamento novos de uma conta vinculada são bloqueados se a entrada de origem foi estornada ou não confere com os registros preservados. A consulta financeira de origem marca `review_required=true`. Não cancela dívida, estorna pagamento, reembolsa nem ajusta documento fiscal automaticamente. Retornos, cancelamento autorizado e recibos anteriores continuam disponíveis pelas regras existentes.

## Autorização e limites

Todos os contextos vêm da sessão local. Não aceitar tenant/store/device/actor no corpo. Leitura física requer view_orders. Escrever resposta, planejar e registrar estados exige manage_replenishment e Orders; receber exige adicionalmente manage_stock e Inventory. Criar obrigação exige manage_cash, Finance, view_accounting e leitura da evidência física por view_orders/manage_stock. Consulta de origem financeira exige view_accounting, no aparelho da conta. Recibo de operação exige ator/aparelho originais; recibos financeiros exigem view_accounting; físicos, view_orders. A política efetiva existente continua soberana.

Consultas e histórico físicos não incluem valores de contas a pagar. Histórico da entrega contém somente revisões físicas; origem financeira é consultada separadamente. A licença vencida bloqueia novas escritas e não é requisito adicional das leituras autorizadas.

Quantidades em milésimos da unidade existente, sem conversão implícita; máximo9007199254740991. Valores em centavos inteiros, BRL, máximo9007199254740991, uma a120parcelas conforme serviço financeiro. Máximo1000entregas/recebimentos analisados por pedido; exceder gera422, sem total truncado. Paginação de entregas/histórico: limit1–100, padrão50; offset0–1000000. Totais do ciclo são da fonte completa limitada, não somente da página.

Corpos de até32768bytes. Todos os campos de cada DTO são obrigatórios; vínculos opcionais são strings vazias. Campos desconhecidos, duplicados, null, JSON concatenado, quantidades fracionárias JSON e parâmetros desconhecidos/duplicados são recusados. Cabeçalho Cache-Control:no-store. Identificadores preservam UTF-8 e até128bytes, sem controles nem espaços nas pontas; identificadores financeiros obedecem adicionalmente à regra anterior de contas a pagar.

Erros principais:400entrada inválida;401sessão;403permissão/licença;404ausência no contexto/ator;409estado/revisão/origem/duplicidade conflitante;413corpo grande;422limite de fonte;503indisponibilidade. Erros internos nunca incluem SQL ou dados privados. O formato de erros negociado anterior da API continua aplicável.

## Recuperação e integridade

Gerar e persistir operation_id antes do POST. Em perda de resposta, consultar GET /local/v1/purchase-cycle-operations/{id} no contexto original. O corpo é a fotografia da operação original, mesmo após revisões, aprovação ou estorno posteriores. Não substituir por fotografia atual. Reenvio exato retorna repeated=true; conteúdo diferente gera409. Consulta404 por si só não comprova recusa ou ausência em outro aparelho/contexto. Não emitir novo ID automaticamente para uma operação incerta.

A revisão esperada impede sobrescrita. O contrato de módulos adquire o escritor SQLite antes das verificações de quantidade/estado/idempotência. Operações, eventos locais e origens são imutáveis por triggers. Os eventos novos são privados/duráveis; não são entrega confirmada pelo protocolo remoto legado. O recebimento continua produzindo sua outbox preexistente. Não há assinatura externa nova nem resistência a rollback malicioso declarada.

## Banco, backup e instalação

0057 é incremental e não modifica as56SQL anteriores. Backup aceita versões25–57 preservando autenticação, checksum, ValidateSchema, integridade, chaves estrangeiras e identidade do aparelho. Teste específico migra56→57 duas vezes preservando sentinela; restauração percorre arquivos históricos reais, sem apagar tabelas para simular versão antiga. Teste vertical restaura um backup com entrega, entrada e origem financeira e recupera recibos após reinício.

O ZIP contém uma única árvore de referência de fontes Go/SQL e documentação, sem binários, caches, bancos, segredos, frontend ou Electron. O aplicador grava somente a lista de mudanças do manifesto; não substitui toda a árvore backend. Aceita --check, testa candidata descartável, faz backup dos fontes anteriores, compara novamente preimagens e preserva alterações concorrentes. Não inicia serviço, aplica SQL em banco operacional, faz commit, merge ou push. O esquema será migrado pela inicialização normal futura do backend, sob responsabilidade da instalação.

## Limites e pendências explícitas

Uma entrega corresponde a um pedido/produto. Não consolida vários pedidos em uma viagem/manifesto de transporte. Referência estável distingue entregas locais; não é identificação regulatória do transportador. Não há comunicação autenticada fornecedor–comércio, convite comercial, envio, notificações, scanner/câmera, motorista conectado, rastreamento ou teste de hardware. Reserva de janela e conferência de doca continuam operações próprias.

Recusa total de carga continua pelo serviço legado de recusa; esta rota de conferência exige aceito>0. Não há adoção automática de entrada ou conta já criada por outra rota. Quantidades/unidades de pedido seguem o modelo de um item existente. A correção de quantidade planejada exige cancelamento explícito e nova entrega; a revisão do fornecedor não pode baixar quantidade abaixo do compromisso já usado.

Uma conta nova por entrada; não agrupa várias entradas na mesma fatura nem divide a mesma entrada entre documentos. Rejeitar/cancelar preserva a origem e não libera duplicação da entrada; uma substituição financeira formal ainda precisa de fluxo específico. Crédito, abatimento, reembolso externo, adiantamento, coleta/retorno físico e compensação financeira não são implementados por esta etapa. O valor é declarado/conferido pelo responsável, não calculado nem validado contra XML fiscal. fiscal_authorization_verified=false. Não há Pix/cartão/boleto/fiscal/provedor externo implementados.

Frontend não foi alterado nem demonstrado nesta entrega. Compilação cruzada Windows não equivale a execução nativa. Evidências executadas e limitações adicionais estão no EVIDENCIAS.txt do pacote.
