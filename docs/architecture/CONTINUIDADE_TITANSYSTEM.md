# Continuidade do TitanSystem

Revisão 62 — 26/10/2026. Registro único de continuidade a atualizar a cada entrega.
Este arquivo faz parte das fontes do projeto; não implica commit ou publicação.

Estado resumido do BACKEND atualizado na revisão62: CI aprovado em `3270029`, schema66/devolução em turno posterior, transporte vinculado à sessão original e geração de contexto autenticado, sete executáveis instalados e serviços existentes preservados. As matrizes históricas não são recertificadas por esta atualização; consultar as evidências datadas e a revisão62 ao final.

## Base, localização e responsáveis

- Interface oficial: `/home/danillo-malta/Downloads/zzeus-frontend-backend/frontend-web/`.
- Branch frontend: `feat/backend-checkout-authority`; base documental da revisão56 `d3d727d535b3446f71d782027b1ce1f435d6f07b`, com os oito arquivos locais do relatório de carrinhos preservados e pausados. A base histórica auditada era `c5579808c62826174b43a2b7a45fbc2ceda1e47b`; entregas posteriores constam abaixo. O HEAD sozinho não representa alterações locais ainda não publicadas.
- Frontend116 presente; Frontend117 CORRETO de conferência por fornecedor aplicado. Confirmados `SupplierReport117.tsx`, `core/payables117`, cinco dicionários, teste e documento193. O usuário informou em21/10: compatibilidade aprovada, backup,136testes sem falhas, TypeScript/build aprovados em20,05s, sem banco/commit/merge/push.
- Frontend passa a ser responsabilidade deste chat/Codex, com preservação do trabalho anterior. Em24/10 o proprietário autorizou explicitamente este chat a desenvolver também o backend; a restrição anterior de somente leitura foi superada. Preservar a autoria e as entregas do outro chat.
- Backend desta frente: `/home/danillo-malta/Downloads/zzeus-frontend-backend-b02a/backend/`; branch `feat/backend-b02a-compat`, HEAD `327002939a517a314864cb2d42cfb984ca09f79e`, schema66 de devoluções de cliente em turno posterior escolhido, transporte/fila de pedidos assinados com worker vinculado ao contexto e à sessão original, com geração autenticada do contexto, backup cifrado da fila e recuperação de recibo com sessão nova no destino, preservando pedidos autenticados/schema64. O aplicador do ZIP antigo de devoluções/schema64 NÃO foi executado; seus fontes declarados foram comparados e conciliados manualmente nesta revisão53, sem substituir migrações históricas. Não aplicar o ZIP antigo sobre a base atual. Revisões49–62 registram pilotos com APIs reais, persistência, restauração, empresas independentes e CI. Serviços8182/8282 continuam nos executáveis anteriores, preservados por escolha do proprietário; instalar binários não os atualiza.
- `~/Downloads/zzeus-frontend` é uma árvore anterior. `Documents/Codex/.../work` contém cópias de laboratório, não a fonte principal.
- `desktop/` executa as telas compiladas de `frontend-web/`. Até a revisão43 não foi alterado nesta continuidade; revisão44 organiza entrada, build, origem, configuração e transporte desktop. Não contém outra implementação do ERP. `website/` fica reservado ao site público futuro.
- Meus antigos `workflows117` são o **rascunho de documentos e recebimento**, diferente do117 aplicado. `production118` é outro rascunho não homologado. Não executar seus aplicadores, não copiar seus routers/menus e não usar sua numeração como base de produto.

## Fontes e interpretação de evidências

Fontes: `PROJECT_RULES.md`; documentos172–193 desta pasta; contratos e referências em `docs/api`; código/testes locais; relatório do proprietário `TitanSystem_Relatorio_Completo_Produto_Arquitetura_Continuidade_2026-10-21.pdf` (66páginas,24capítulos, especialmente16–18 e20–23).

Atualizações posteriores ao PDF: auditoria confirmou preservação dos originais e B02B13 no código; usuário confirmou aplicação do117. Esses pontos não devem reaparecer como ausências. O relatório não prova o serviço atual nem homologa todos os módulos.

- Código encontrado, teste isolado, aplicação por log, API respondendo e aceite comercial são evidências diferentes.
- Mock somente em prévia explícita. Não introduzir dados fictícios nas rotas reais.
- Dinheiro em centavos exatos; quantidade conforme unidade/contrato. Autoridade comercial, permissão, transação, auditoria e idempotência continuam no backend.
- Não confundir `/local/v1` (SQLite; padrão8181), API online`:8080/api/v1` e Vite`:3000` (interface/proxy de desenvolvimento).
- Não iniciar backend escolhendo banco, estação ou chaves. Não usar banco comercial para testes de interface.

## Pendências consolidadas por fluxo

A lista é um mapa de trabalho, não uma declaração de que todo contrato parcial resolve o fluxo completo. “No código” abaixo se refere ao backend separado auditado, não a um serviço em execução.

| Fluxo | Rota | Implementação existente | Pendência frontend | Contrato backend | Dependência externa | Teste | Estado |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Frota e manutenção | /local/fleet | Rascunhos preservados; consulta oficial de14recursos; bens/chamados, prestadores/orçamentos e oficina com revisão, fila durável e recuperação. | ACK, viagens, planos, despesas/perfis e edição operacional de bens ainda precisam de comandos; nenhum rascunho é enviado automaticamente. | Fleet/Workshop do backend857febb, schema60. | Serviço e banco descartáveis autorizados não fornecidos. | 192regressões nas revisões25–28, mais5do ciclo de vida; suíte2078, TS/build aprovados nesta revisão. | Operações descritas implementadas; demais parciais; integração real não homologada. |
| Acesso, cadastro e atividade | /local/login; /register; /local/settings | Login/contexto e cadastro112 existem. Conectar criação/recuperação/perfil real e seleção autorizada de loja onde os contratos suportarem; usar atividade central, sem repetir questionário em módulos. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | APIs locais de conta, staff e política existem; não presumir cadastro completo de empresa, licenciamento comercial e filiais. Demonstrar autorização/isolamento. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Sessão financeira | /local/payables; /local/orders | Correção116 preservada; extensão de origem vigente ao115; callbacks consideram token, sessão e capacidades atuais. | Aceite de integração real com perfis descartáveis. | Mesmos contratos, sem mudanças no backend. | Serviço local e perfil a identificar pelo responsável. | Regressões de troca de quatro campos de contexto, logout, corpo tardio, licença revogada e IDs originais; suíte total858 passou. | Implementado e validado em testes isolados; integração real pendente. |
| Venda/PDV | /local/pos | Carrinhos persistidos: salvar/atualizar, retomar/liberar, cancelar e concluir em dinheiro; recibo e histórico originais, fila durável e exclusão mútua com POS legado. Preservar foco, tela cheia, CPF separado do clube, cotação e troco após concluir. Conectar contratos avançados com recuperação sem afrouxar cliente legado. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Cotação/venda local e planos checkout estão no código. Gateway95 inicia desconectado; a existência dos planos não prova PSP ou pagamento misto integrado. Demonstrar venda/reinício/recibo sem duplicação. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Pagamentos, descontos e fiscal | /local/pos | Seletores/drafts não são confirmação. Integrar somente meios e estados reais. Separar cancelamento de carrinho, venda, pagamento e nota. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Pix/TEF/adquirente e fiscal exigem provedor/ambiente e contratos completos. Não considerar papel como liquidação. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Fechamento e comprovantes | /local/cash | 97–108 existem: dinheiro físico, provas, revisão, OCR e relatório oficial. Integrar estados oficiais adicionais sem duplicar parcela por papel+integração; não guardar imagem por padrão. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Relatório oficial e contratos de proofs estão no código. Revisão de papel não é confirmação bancária. Falta demonstrar fluxo combinado com permissões e hardware reais. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Movimentos, livro, malote e cofre | /local/cash | 99/105/109 têm registros/revisões locais. Conectar a cadeia oficial e distinguir o registro local de recebimento remoto. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Há APIs de movimentos, bags, custódia e vaults. Não declarar esses contratos inexistentes. Revisar política do conferente/origem e demonstrar recuperação sem autocustódia/crédito duplicado. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Catálogo, estoque e perdas | /local/catalog; /local/stock; /local/losses | Novo estoque oficial por produto/local com físico/reservado/livre, histórico genérico, entrada/perda/transferência, CSV e cópia de página para planilha. Catálogo anterior preservado; edição cadastral, estado operacional e histórico integrados na revisão17. | Inventário revisado implementado nesta revisão10 (abertura, contagem parcial/zero, aprovação independente, recuperação/histórico/CSV). Gestão, busca, filtros, tabela e criação do produto em cinco idiomas; demais ações de perdas ainda precisam de revisão. Periodicidade da empresa e custo histórico das perdas continuam pendentes. Valoração atual por página e análise estimada de diferenças implementadas na revisão11. | Availability e stock operations/history/receipt existentes; suplemento inventory-costs da referência agora contém contrato e Go do ciclo de inventário revisado. Isso não comprova serviço atualizado. | Homologação em serviço de teste; política de inventário da empresa. |23testes novos do estoque, inclusive preservação da entrada antiga; regressões completas858. | Unidade de movimentos implementada; fluxo amplo de estoque ainda parcial. |
| Reposição e pedido local | /local/orders | 115 preservado, decisão manual na revisão31 e ciclo de entregas identificadas/recebimento/obrigação documental na revisão32. Preservados mínimo/alvo, sugestão e decisão humana. Confirmação autenticada e acordo interempresa continuam distintos dos registros manuais locais. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | API local existente não demonstra envio/aceite do fornecedor. Vínculo e condições comerciais não podem ser deduzidos da aprovação local. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Fornecedor, doca, expedição e recebimento | /local/orders | Recebimento oficial integrado aos detalhes do pedido: autorização humana local, aceitação parcial, recusa integral, estorno de entrada e histórico. | Agenda operacional de docas integrada na revisão20: recursos, reserva/reagendamento, chegada/descarga, ocupação e histórico. Painel agrupado e conferência por pedido integrados como consultas na revisão21. Tratativas administrativas locais integradas na revisão29. Indicadores de doca e utilização histórica integrados na revisão30. Decisão externa manual e consulta paginada do ciclo/entregas integradas na revisão31, explicitamente sem autenticação externa. Planejamento de entregas identificadas, carga/chegada/cancelamento, recebimento parcial e obrigação documental vinculada integrados na revisão32. Permanecem recusa integral nominal, confirmação autenticada interempresa, coleta e política de créditos; nenhum desses efeitos foi simulado como concluído. | Contratos de receiving authorization/trace/history/operations e receipt/rejection/void usados. | Acordo com fornecedor, motorista e integrações externas. |26testes novos do recebimento; prévia descartável de autorização e entrada parcial; suíte858. | Unidade local implementada; circuito interempresa ainda parcial. |
| Troca, devolução, coleta e reembolso | /local/pos | 100/110 são solicitações e revisão locais. Integrar execução oficial sem apresentar solicitação como reembolso; coleta/chegada são confirmações distintas. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Cancelamento de venda local existe parcialmente. Política do fornecedor, coleta, chegada e crédito/reposição/reembolso exigem contrato e aceite de ponta a ponta. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Documentos e financeiro | /local/payables; /local/payables/agenda |113–117 preservados. Criação documental, parcelas exatas e revisão independente agora integradas aos contratos. | Contas a receber, centros de custo, fluxo de caixa/DRE, OFX e outras fontes continuam a conferir e integrar; não classificados como API inexistente sem inspeção. | Payables e vínculos existentes usados. Demais contratos precisam ser identificados por fluxo. | Bancos/PSP e aceite contábil são separados. | Novos testes de documentos mais regressões113/114/116/117;858 na suíte completa. | Unidade de documentos implementada; módulo financeiro amplo ainda parcial. |
| Contador e exportações | /local/payables/agenda | 117 já entrega CSV/XLSX/impressão/cópia para planilha do escopo consultado; não recriar. Falta acesso externo autorizado, carteira/checklist/entrega quando suportados. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Exportação local não é portal do contador. Compartilhamento revogável, trilha, limites maiores e visão empresarial/histórica exigem contratos próprios. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Receitas, produção e qualidade | /local/recipes; /local/operation | Consulta/capacidade, publicação imutável, ordens, reserva/liberação/consumo, etapas, resultado medido e declarações de perdas/lotes/qualidade integrados em /local/recipes. Planejamento86/90 preservado como rascunho. | Estado da receita, responsáveis autorizados, histórico de etapas e proteção de formulários já implementados na revisão8; planejamento simultâneo/estimativas na revisão9. Permanecem custo real completo, integrações/aceite e demais limites documentados, sem recriar as unidades entregues. | Contratos reais existentes consumidos, incluindo recibos originais e trace; backend separado não alterado. | Serviço/perfis descartáveis ainda a identificar com responsável. |1028testes; TypeScript/build; circuitos visuais em memória e5idiomas. | Subfluxos aplicados; produção ampla ainda parcial, sem homologação real nem instalador novo. |
| RH e equipe | /local/hr; /local/staff | Staff real e painel71 existem. Conectar fontes autorizadas de ponto, tarefas, metas, escalas, férias/documentos/folha e avaliações privadas. Não confundir RH com cadastro de acesso. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Não houve comprovação do ciclo completo de RH/portal móvel; APIs/fonte e permissão por dado devem ser coordenadas. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Planilhas | /local/sheets | Editor e presets preservados. Cinco idiomas carregados sob demanda, com proteção para mudança tardia de idioma/contexto. Cópias117 e estoque permanecem cópias explícitas. | Estoque físico por local e obrigações a pagar por vencimento integrados a contratos existentes; revisar demais fontes, blocos/versões/importação, compartilhamento e homologar editor completo. Separação do motor reduz carga estática inicial, não o pacote total. | Fontes parciais existentes; contratos locais de estoque/financeiro disponíveis para integração posterior. | Medir abertura/memória no hardware alvo; colaboração/compartilhamento dependem de contrato. |Fontes e carregamento: testes incrementais de idioma, contexto, estoque, obrigações e texto literal; suíte1444, TypeScript/build aprovados. Prévia isolada não homologa serviço real. | Otimização implementada e medida em build; planilha ampla ainda parcial. |
| Gestão, comercial, documentos e integrações | /local/enterprise; /local/commercial; /local/documents | Áreas85/89 existem; diferenciar arquivo de trabalho de fonte oficial e conectar cada adaptador disponível. Não criar menu duplicado. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Há fontes parciais/indisponíveis. Cadastro comercial, CRM, integrações e relatórios completos precisam de contrato e aceite delimitados. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Entrega, mapa e frota | /local/delivery; /local/map; /local/fleet | Preservar geografia, clientes externos, janelas negociadas e separação de delivery/cozinha; conectar carga, confirmação e localização autorizada. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Não presumir telemetria/ETA ao vivo, app de motorista ou roteamento comercial completo pela presença do mapa. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Offline e móvel | /local/integrations/diagnostics | Preservar estados de conexão e limites; não prometer venda móvel independente do PC sem prova. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Direitos exclusivos, transferência durável, backup antigo, revogação e reconciliação precisam de protocolo/aceite. P2P, timestamp e reenvio idempotente sozinhos não resolvem saldo global. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Desktop, segurança e desempenho | desktop/; frontend-web/ | Revisão44: Electron com UI compilada sem Vite, origem estável, perfis por destino, conexão local/HTTPS e atalho Linux. | Funções do frontend pausadas; medir operação/memória e concluir aceites por plataforma. | /local/v1 e /api/v1 separados; cliente não ativa cloud/hybrid no núcleo Go. | Endereços/servidor/perfil reais, hardware e provedores não homologados. | 35testes desktop,2smokes Electron Linux e2452regressões frontend; detalhes/limites na revisão44. | Organização desktop entregue; instaladores, Internet integrada, atualização/restauração e Windows pendentes. |

| Documentos a pagar — unidade aplicada | /local/payables → Documentos a pagar | Criar obrigação BRL, até120parcelas ordenadas, soma exata, histórico, aprovar/recusar por outro responsável, cancelar conforme autor/estado/saldo, recuperação por recibo e lock. | Homologação integrada; não inclui pagamento bancário nem fiscal. | GET/POST finance/payables, GET documento, POST approve/reject/cancel e GET operations/id; fornecedores existentes. | Serviço local e usuários de teste com permissões reais. |28testes novos; criação e aprovação pela interface em memória, cinco idiomas e estados de falha. | Implementado, aguardando integração real. |
| Recebimento — unidade aplicada | /local/orders → Detalhes → Recebimento conferido | Autorizar, conferir entregue/aceita, recusar, estornar entrada, totais e histórico50porpágina. Identificadores originais e fila por quatro campos do contexto. | Homologação integrada e verificação visual de recusa/estorno; autorização não confirma fornecedor. | Endpoints purchase-orders/authorize-receiving, receipts, rejections, receipts/id/void, receiving-trace/history e purchase-receiving/operations/kind/id/receipt. | Serviço, licenças Orders/Inventory e operadores autorizados. |26testes novos; autorização e parcial5kg/4kg na prévia em memória. | Implementado, aguardando integração real. |
| Estoque — unidade aplicada | /local/stock | Saldo físico/reservado/livre, histórico50porpágina, entrada/perda/transferência no mesmo escopo, recibo, recuperação de fila nova e entrada legada, CSV e cópia da página. | Homologação integrada; inventário revisado conectado nesta revisão10, sem política automática de frequência. | Catalog search, locations, stock availability, operations, operations/history e operations/id/receipt. | Serviço local, licença Inventory e permissão manage_stock; consulta de catálogo conforme servidor. |23testes novos; consulta10kg e perda1kg com saldo9kg na prévia em memória. | Implementado, aguardando integração real. |
| Idiomas da planilha — unidade aplicada | /local/sheets | Presets core/conditional-formatting/data-validation/filter/sort preservados; apenas idioma escolhido na abertura. | Teste visual do editor completo e medidas de runtime ainda não executados. | Sem alteração de API. | Biblioteca e hardware já usados pelo produto. |5testes de seleção, cancelamento e resposta tardia; build medido antes/depois. | Implementado e validado em testes/build; aceite visual pendente. |

Expansões do capítulo19 (advocacia e novos segmentos, entre outras) continuam registradas no relatório; não são dependências silenciosas da próxima unidade.

## Coordenação com o backend

Nenhuma mudança de contrato, banco, migração, licença ou permissão é solicitada pela correção de sessão abaixo. Foram relidos `backend/internal/localapi/payable_cash.go`, `payable_readiness.go`, montagem do servidor e contratos `payable-cash.openapi.json`, `payable-readiness.openapi.json` e `payables-flow.openapi.json` na árvore do backend.

Para o responsável: manter os recibos originais e o escopo de empresa/loja/aparelho/ator; informar versão e perfil efetivamente em execução antes de ensaio integrado. Futura unidade de documentos/recebimento/produção deve confirmar contrato, concorrência, recuperação e trabalho em curso com esse responsável, sem alterar sua árvore para demonstrar a UI.

Nenhum chat foi identificado com segurança como o responsável pelo backend na lista de conversas disponível. Não foi enviada mensagem a conversa não identificada. Este registro mantém a situação e os pontos de coordenação; não representa resposta ou aprovação do responsável.

## Unidade atual — proteção da sessão no financeiro116

Problema reproduzido em transporte exclusivamente em memória: após trocar a sessão durante o GET do recibo, o cliente anterior ainda tentava1POST; esperado0. Nenhuma API real foi chamada para reproduzir.

Correção delimitada:
1. `frontend-web/src/core/payables116/source.mjs`: callback `isCurrent` opcional para compatibilidade; conferir sessão antes/depois de transporte e após decodificar corpos; comparar com o escopo original capturado, inclusive se os objetos forem alterados no mesmo lugar; bloquear envio e descarte de fonte antiga. Fila incerta continua preservada.
2. `frontend-web/src/core/payables116/source.d.mts`: declarar o callback.
3. `frontend-web/src/modules/financial/pages/LocalPayables113.tsx`: vincular o callback ao token e objeto de sessão atuais; uma falha da fonte antiga não encerra o login novo.
4. `frontend-web/tests/finance116.test.mjs`: regressões de logout/troca de sessão durante lock, consulta, leitura de corpo e POST; troca de empresa/loja/usuário/aparelho, inclusive mutação no mesmo objeto; recuperação posterior com um único envio e os mesmos IDs.
5. Este registro de continuidade.

Não altera valores, regras de elegibilidade, IDs, recibos, contratos ou dicionários. Cinco idiomas e temas existentes permanecem. Não impede retroativamente requisição que já chegou ao servidor; nesse caso mantém incerteza e exige consulta pelo contexto original. Não confirma operação após troca de sessão nem apaga sua prova local.

### Evidências desta unidade

- Preparação em apenas4arquivos de fonte/teste, sem outra cópia do ERP; preimagens e backup privado no diretório de trabalho da tarefa.
- Teste focado da preparação:46testes aprovados, incluindo9regressões novas. Transporte e storage em memória.
- Aplicado no projeto correto em21/10/2026, após conferir HEAD, branch, os4hashes de preimagem e escrita efetiva nas4pastas autorizadas. Nenhum conflito de conteúdo encontrado.
- Regressões aplicadas:157testes aprovados, zero falhas, zero ignorados: `node --test tests/finance116.test.mjs tests/suppliers117.test.mjs tests/agenda114.test.mjs tests/payables113.test.mjs`. São58testes do116, incluindo21regressões de sessão/contexto acrescentadas nesta unidade, mais as regressões113/114/117.
- Cenários de empresa, loja, usuário e aparelho: a consulta antiga não envia POST após troca; nova origem não lê/apaga fila antiga; resultado incerto não dispara reenvio automático; retorno ao contexto original recupera pelo mesmo operation_id/payment_id/documento/parcela, com somente1POST. Também foram reproduzidas e corrigidas4falhas quando objetos de sessão/capacidades eram alterados no mesmo lugar.
- TypeScript: `node node_modules/typescript/bin/tsc --noEmit --project tsconfig.json`, código de saída0, sem diagnósticos.
- Build Vite5.4.21 da árvore aplicada: aprovado em19,93s. API `build`, mesmo plugin React do vite.config.ts, `configFile:false` e `envFile:false`; destino de verificação separado em `work/continuidade-117-financeiro/build-verificacao-UyaWtg`. O dist existente do projeto não foi substituído. Nenhuma dependência instalada; nenhum ZIP gerado.
- Permanece aviso preexistente de chunks grandes, principalmente LocalSpreadsheet (~9,53MB minificado). Não foi anunciado ganho de desempenho.
- Preservação:1040arquivos anteriores de src/testes/docs conferidos por SHA-256; somente os4arquivos previstos mudaram e este documento foi acrescentado; nenhum removido.117, agenda114 e demais arquivos preservados. `git diff --check` passou; as comparações diretas dos4arquivos não rastreados com suas preimagens não apontaram whitespace inválido. Backend separado permaneceu limpo.
- Backup privado e evidências: `/home/danillo-malta/Documents/Codex/2026-10-20/github-plugin-github-openai-curated-remote-2/work/continuidade-117-financeiro/backup`; preimagens, hashes aplicados, logs e verificação de preservação no diretório pai. Não executar aplicadores antigos. Nenhum commit/merge/push/publicação.
- Não executados: backend/SQLite real, banco comercial, adquirente, SEFAZ, hardware, Windows e ensaio físico entre aparelhos. Sem alteração de endpoint, não foi iniciado serviço para esta correção.

### Como verificar e reverter

Os testes116 junto a113/114/117, TypeScript e build fora do dist usado pelo proprietário foram executados conforme o bloco anterior. Em ambiente de teste, iniciar consulta/confirmar com resposta atrasada e encerrar/trocar sessão: a origem antiga não pode enviar novo POST, exibir resultado antigo nem encerrar a sessão nova. Uma operação enviada antes da troca continua pendente para consulta pelo contexto original, sem reenvio automático.

Aplicação: conferir hash de TODOS os quatro arquivos contra a preimagem antes de qualquer substituição; preservar modo, copiar somente os quatro caminhos e este documento. Se um arquivo mudou, interromper e comparar. Reversão: comparar a versão aplicada; somente se ela ainda corresponder, restaurar as quatro preimagens do backup. Preservar mudanças posteriores e a fila financeira. Não usar reset/clean ou restauração geral.

## Diagnóstico dos erros de acesso

Na tentativa anterior, a concessão por caminho de arquivo levou o sandbox a tentar montar diretórios de controle abaixo de arquivos (por exemplo `finance116.test.mjs/.git`), causando `Not a directory` antes da execução. Outra ferramenta retornou `EROFS: read-only file system`. Não houve aplicação naquela tentativa; os hashes desta tarefa confirmaram as4preimagens intactas.

Neste turno o caminho real foi confirmado por realpath e o trabalho passou a executar com cwd `/home/danillo-malta/Downloads/zzeus-frontend-backend`. A raiz geral do sandbox continua marcada ro; isso sozinho não determina a escrita nas pastas autorizadas. Nas4pastas, abertura sem truncar e gravação/fsync/releitura de arquivo temporário passaram. Só então houve aplicação com backup, conferência antes/depois e substituição localizada. Não foi feita alteração de permissões do projeto nem remontagem do disco Ubuntu.

## Regra para a próxima atualização

Atualizar este mesmo arquivo com data, base efetiva, arquivos exatos, contrato utilizado, testes realizados, limitações e aceite. Não criar outra lista concorrente de continuidade. Não repetir o117 aplicado nem executar aplicadores dos rascunhos. Sem commit, merge, push ou publicação automáticos.


## Execução da primeira entrega — em andamento

21/10/2026: base e escrita efetiva reconferidas. Plano: (1) documentos a pagar com parcelas e revisão independente, usando contrato existente; (2) recebimento/produção e demais ligações do circuito, conforme inspeção dos contratos; (3) fontes transversais, planilha e desempenho; (4) regressões completas, traduções, TypeScript, build e validação visual descartável. Não iniciar backend comercial.

Aplicadas as unidades abaixo diretamente na árvore correta. Não há aplicador ou pacote novo. Os demais fluxos continuam parciais até evidência específica; a primeira entrega completa ainda NÃO está concluída.


## Entrega aplicada em21/10/2026 — documentos, recebimento, estoque e idiomas de planilha

### Aplicação e decisões

- Somente frontend e este registro. Branch/HEAD preservados. As permissões do sandbox foram concedidas por diretórios e verificadas com arquivo temporário/fsync antes da escrita; os erros anteriores não foram considerados resolvidos apenas por autorização.
- `core/payableDocuments`: adaptador tipado com contrato real de obrigação documental. Aprovar não paga; cancelamento com dinheiro aplicado é recusado; outro responsável revisa o documento. Valores em centavos exatos, inclusive limite de inteiro seguro, sem arredondar parcelas. IDs são gerados ao revisar e preservados ao confirmar/recuperar.
- `core/receiving`: contratos reais de autorização, entrada aceita, recusa integral, estorno e histórico. Unidade e quantidades em milésimos exatos; totais grandes do histórico permanecem strings/BigInt. Divergência entre snapshots de trace/history pede atualização. Autorizar não dá entrada; recusar não dá entrada; estornar retira a quantidade aceita original, sem fingir reembolso.
- `core/stockWorkspace`: estoque oficial por produto/local; cada movimento revisado carrega `expected_unit` e motivo. Nenhum ajuste automático por contagem. A entrada pendente legada usa o mesmo nome de lock e é preservada no armazenamento original, sempre tratada como incerta até consulta de recibo. Contexto legado ambíguo por delimitador dois-pontos bloqueia recuperação e preserva os bytes para análise, sem consulta ou reenvio. Não transforma a fila antiga em outra operação ou novos IDs.
- `orders115`: resposta de origem antiga não continua em POST nem apaga fila. Conferência antes/depois do transporte e da leitura do corpo, quatro IDs do contexto, sessão/token/capacidades atuais e licença antes de cada POST. Pedidos cancelados não são mais rejeitados como DTO desconhecido nem rotulados como não enviados no CSV.
-116/117, agenda114, seus valores/relatórios e núcleo da correção anterior não foram substituídos. Somente o contêiner116 recebeu composição com a nova área de documentos e guarda de capacidades atuais.
- `LocalCatalog` continua com seu cadastro anterior. A rota stock carrega a nova unidade sob demanda; a implementação antiga foi preservada no arquivo e não é executada nessa rota. Não houve troca de router ou menu.
- Planilha: removeram-se somente imports estáticos dos25pacotes de tradução, agora agrupados em5cargas por idioma. Editor, fórmulas e presets permanecem. Carga assíncrona antiga não monta editor após navegação nem seleciona um idioma ultrapassado.
- Novos textos têm pt-BR/en/es/fr/zh-CN; estados reais, revisão antes do envio, labels/foco, temas existentes e controles com alvo de44px. Nenhuma demonstração é ativada nas rotas reais. Prévia apenas em `work/frontend-release/preview`, com faixa explícita e transporte em memória.

### Verificações executadas e limites

- `node --test tests/*.test.mjs`:858aprovados,0falhas,0ignorados. O npm test:local atual é uma lista menor; foi executada a suíte completa disponível por glob.
- TypeScript: `node node_modules/typescript/bin/tsc --noEmit --project tsconfig.json`, exit0 e nenhum diagnóstico.
- Build da árvore aplicada via API Vite, mesmo plugin React, `configFile:false`, `envFile:false`, em destino privado de verificação, sem sobrescrever dist: aprovado em22,064s na rodada final antes deste registro. Nenhuma dependência instalada.
- `git diff --check` aprovado. Conferência SHA-256 da base1041arquivos: somente caminhos desta entrega mudaram, nenhum arquivo anterior removido.16arquivos protegidos de117/agenda114/núcleo e testes116 mantiveram seus hashes. Backend separado limpo.
- Primeiro build medido antes dos idiomas sob demanda: LocalSpreadsheet9.981.842bytes,19,966s. Após: núcleo6.659.122bytes e português746.315bytes, total desses dois arquivos7.405.437bytes (cerca de25,8% menos que o chunk anterior). Inglês596.995, espanhol633.569, francês809.246, chinês539.140bytes. Continua grande; o aviso não foi ocultado. Build final22,064s não demonstra ganho no tempo de compilação.
- Grafo de imports do build: menu e PDV não alcançam módulos Univer por dependência estática. Idiomas e nova unidade stock têm chunks sob demanda. Não foi medido tempo de abertura, memória, FPS ou hardware antigo.
- Navegador, prévia descartável: criação documental315,00 com revisão/recibo; aprovação por outro usuário; textos nos5idiomas; claro/escuro; vazio/offline/sem permissão; autorização local; entrega5kg, aceita4kg, recusa parcial1kg e restante6kg; saldo de estoque10kg, perda1kg e saldo9kg, todos somente em memória.
- Viewports solicitados390/768/1280: larguras efetivas375/753/1265 com `scrollWidth===clientWidth`, fontes de14px no estoque e rolagem interna das tabelas. Os tamanhos efetivos descontam a barra vertical. Não declarar que foi percorrida cada tela histórica em cada tamanho/idioma.
- Problemas de automação encontrados: abas descartáveis perderam vínculo em tentativas iniciais, preenchimento pelo clipboard perdeu foco e o processo de prévia terminou com143. A prévia foi reiniciada e novas abas/controlos acessíveis permitiram os ensaios descritos. Não são erros constatados do backend comercial.
- Não executados: API/SQLite real, fluxo completo por navegador do estorno/recusa integral/transferência, exportação real para planilha no shell completo, editor Univer completo após a mudança, zoom50–100%, auditoria completa de teclado/leitor de tela, concorrência entre aparelhos físicos, PSP/bancos/fiscal/hardware/Windows. Testes em memória não homologam essas integrações.
- Nenhum commit, merge, push, publicação, ZIP, instalador, backend ou banco comercial. Aplicadores dos rascunhos não foram executados.

### Como verificar estas unidades

Usar apenas serviço e perfis descartáveis explicitamente definidos pelo responsável. O serviço em execução ainda não foi identificado; não iniciar uma base escolhida automaticamente. As mudanças estão nas fontes locais; AppImages/DEBs antigos não foram atualizados.

1. Na interface atualizada, `/local/payables`: abrir Documentos a pagar, carregar fornecedor ativo, preencher emissão/referência/descrição e parcelas. Valor315,00 pode ser dividido100,00+96,00+119,00. Uma soma incorreta ou data fora de ordem não permite revisão. Confirmar somente após conferir a prévia.
2. Outro operador autorizado consulta o documento e aprova/recusa com motivo. O criador não recebe essas ações. Cancelar segue autor/estado/saldo; consultar o histórico. A agenda114 e a Visão por fornecedor117 continuam nas rotas existentes.
3. `/local/orders`: abrir Detalhes; na área Recebimento conferido, autorizar localmente, conferir entregue/aceita/local/motivo. Receber5kg e aceitar4kg, para pedido10kg, deve apresentar1kg recusado e6kg restante. Testar recusa integral e estorno somente em base descartável.
4. `/local/stock`: buscar produto, escolher local e consultar. Conferir físico=reservado+livre. Revisar entrada/perda/transferência e consultar recibo/histórico. CSV exporta somente a página. Copiar para planilha é cópia explícita, permitida conforme perfil, sem vínculo automático.
5. Durante consulta de recibo com resposta atrasada, trocar empresa/loja/usuário/aparelho ou encerrar sessão. A origem antiga não deve iniciar POST. Operação enviada antes da mudança permanece incerta; retornar ao contexto original e Consultar resultado, sem reenvio automático.
6. `/local/sheets`: abrir em cada idioma e alternar idioma durante a carga. Nenhuma carga antiga deve remontar editor fora da rota. Verificar fórmulas, estrutura, filtros, validação e formatação no hardware alvo antes de homologar a otimização.

### Ponto exato de continuidade

A primeira entrega ampla permanece em andamento. Não há preparação pendente de aplicar: as unidades acima estão na árvore principal e passaram nos checks descritos. Atualização22/10: a consulta oficial de receitas e capacidade em `/local/recipes` foi aplicada conforme a seção seguinte; publicação e execução produtiva ainda não foram integradas. Reaproveitar perfil central e preservar planejamento86/90 como rascunho identificado. Ler os contratos de versões, capacidade, ordens, materiais, etapas, resultados, lotes/qualidade/perdas e recibos, sem confundir ausência de integração frontend com falta de API.

Foi consultado o rascunho isolado `production118`: a página contém textos em português e depende de `workflows117`, que não é o117 oficial. Não atende à integração atual nem aos5idiomas; nenhum arquivo dele foi aplicado. Revisão de reaproveitamento deve ser localizada e testada, não cópia de router/menu/applicador.

Também permanecem frontend a concluir: ligação avançada PDV/planos/provas e meios eletrônicos honestamente bloqueados, cadeia oficial de fechamento/malote/cofre, demais ações de catálogo/perdas/inventário, fontes de planilha, contas a receber/contabilidade, perfil/contexto/configurações, RH e comercial/documentos/logística conforme contratos. A matriz distingue a unidade entregue do fluxo amplo. Não afirmar que essas pendências são todas do backend.

Responsável do backend ainda não identificado com segurança em outro chat; nenhuma mensagem enviada. Contratos consumidos não foram alterados. Confirmar serviço, versão, licença e perfis para ensaio conjunto antes de qualquer integração real.

### Caminhos exatos desta entrega


Arquivos anteriores alterados:

- `frontend-web/src/core/orders115/model.d.mts`
- `frontend-web/src/core/orders115/source.d.mts`
- `frontend-web/src/core/orders115/model.mjs`
- `frontend-web/src/core/orders115/source.mjs`
- `frontend-web/src/core/local/purchaseClient.mjs`
- `frontend-web/src/modules/financial/pages/Finance116.tsx`
- `frontend-web/src/modules/financial/pages/LocalPayables113.tsx`
- `frontend-web/src/modules/tenant/pages/LocalSpreadsheet.tsx`
- `frontend-web/src/modules/tenant/pages/sheetRuntime70.ts`
- `frontend-web/src/modules/catalog/pages/LocalCatalog.tsx`
- `frontend-web/src/modules/catalog/pages/LocalOrders.tsx`
- `frontend-web/src/modules/catalog/pages/Orders115.tsx`
- `frontend-web/tests/orders115.test.mjs`
- `docs/architecture/CONTINUIDADE_TITANSYSTEM.md`

Arquivos novos:

- `frontend-web/src/core/stockWorkspace/model.d.mts`
- `frontend-web/src/core/stockWorkspace/words.ts`
- `frontend-web/src/core/stockWorkspace/source.d.mts`
- `frontend-web/src/core/stockWorkspace/model.mjs`
- `frontend-web/src/core/stockWorkspace/source.mjs`
- `frontend-web/src/core/payableDocuments/model.d.mts`
- `frontend-web/src/core/payableDocuments/words.ts`
- `frontend-web/src/core/payableDocuments/source.d.mts`
- `frontend-web/src/core/payableDocuments/model.mjs`
- `frontend-web/src/core/payableDocuments/source.mjs`
- `frontend-web/src/core/receiving/model.d.mts`
- `frontend-web/src/core/receiving/words.ts`
- `frontend-web/src/core/receiving/source.d.mts`
- `frontend-web/src/core/receiving/model.mjs`
- `frontend-web/src/core/receiving/source.mjs`
- `frontend-web/src/core/sheets70/language.d.mts`
- `frontend-web/src/core/sheets70/language.mjs`
- `frontend-web/src/modules/financial/pages/PayableDocuments.tsx`
- `frontend-web/src/modules/financial/pages/payableDocuments.css`
- `frontend-web/src/modules/tenant/pages/sheetLanguage70.ts`
- `frontend-web/src/modules/tenant/pages/sheetLocales70/enUS.ts`
- `frontend-web/src/modules/tenant/pages/sheetLocales70/esES.ts`
- `frontend-web/src/modules/tenant/pages/sheetLocales70/ptBR.ts`
- `frontend-web/src/modules/tenant/pages/sheetLocales70/frFR.ts`
- `frontend-web/src/modules/tenant/pages/sheetLocales70/zhCN.ts`
- `frontend-web/src/modules/catalog/pages/Receiving.tsx`
- `frontend-web/src/modules/catalog/pages/LocalStock.tsx`
- `frontend-web/src/modules/catalog/pages/StockWorkspace.tsx`
- `frontend-web/src/modules/catalog/pages/receiving.css`
- `frontend-web/tests/stockWorkspace.test.mjs`
- `frontend-web/tests/payableDocuments.test.mjs`
- `frontend-web/tests/receiving.test.mjs`
- `frontend-web/tests/sheetLanguage.test.mjs`

Evidências e backups privados: `work/frontend-release/` nesta tarefa Documents/Codex. Preimagens de arquivos existentes em `backup/`, continuidade anterior em `CONTINUIDADE-antes.md`, hashes iniciais `base-hashes.json`, logs de testes/TypeScript/build e relatórios de chunks. Para reverter, comparar hashes finais antes de restaurar somente preimagens correspondentes; não remover novos arquivos que tenham recebido trabalho posterior, não limpar fila pendente e não usar reset/clean. Não restaurar a árvore inteira.

Conferência final adicional:858testes, inclusive contexto legado ambíguo; TypeScript exit0; último build22,064s. Verificados whitespace de todos os47caminhos da entrega e links novos para rotas existentes, sem nova rota de backend. As verificações de módulos pesados se referem a imports estáticos do build, não a tempo de runtime.


## Entrega de22/10/2026 — consulta de receitas oficiais e capacidade

### Escopo aplicado e localização

Na árvore principal, branch `feat/backend-checkout-authority`, HEAD `c5579808c62826174b43a2b7a45fbc2ceda1e47b`, sem alteração de branch/commit. `/local/recipes` recebe um painel de consulta oficial acima do planejamento86 existente. Nenhuma rota/menu novo; rascunhos locais e seu armazenamento permanecem. O painel tem filtro exato por recipe_id, páginas de50 (sem inventar total não fornecido), detalhe da versão imutável e composição, escolha de local e capacidade calculada pelo serviço.

Contratos lidos no backend separado: `docs/api/production-recipes.openapi.json` e `production-capacity.openapi.json`, conferidos com os handlers e implementação. Usados somente GET `/local/v1/production/recipe-versions`, `/production/recipe-versions/{version_id}`, `/production/capacity` e `/locations`. Nenhum POST, publicação, reserva, consumo, fila, recibo financeiro ou banco alterado por esta unidade.

Validação tipada de versões, identidade, revisão, unidades e quantidades seguras; capacidade corresponde à versão/local solicitados, saída=lotes×rendimento, menor capacidade dos insumos e limitantes. Aritmética intermediária BigInt e exibição decimal sem converter quantidades para float. A implementação atual do backend recusa divergência histórica de unidade; o adaptador não inventa conversão de densidade/embalagem. Mostrar momento da consulta e aviso de alternativas independentes, sem soma ou promessa de reserva.

Autorização conferida antes/depois de transporte e depois da leitura do corpo: contexto original de empresa/loja/usuário/aparelho, expiração, token/sessão/capacidades vigentes. Resposta antiga não aparece após troca de contexto; cancelamento e geração de consulta descartam resultados ultrapassados no painel. Esta unidade não toca operações incertas de outros módulos. O adaptador permite leitura histórica autorizada sem exigir licença ativa; a guarda preexistente da página86 ainda exige licença ativa e perfil pertinente. Não foi ampliada para esta entrega.

### Arquivos exatos

Novos:
- `frontend-web/src/core/productionRead/source.mjs`
- `frontend-web/src/core/productionRead/source.d.mts`
- `frontend-web/src/core/productionRead/words.ts`
- `frontend-web/src/modules/tenant/pages/sectors86/OfficialRecipes.tsx`
- `frontend-web/src/modules/tenant/pages/sectors86/officialRecipes.css`
- `frontend-web/tests/productionRead.test.mjs`

Anteriores alterados:
- `frontend-web/src/modules/tenant/pages/sectors86/Recipes86.tsx`: imports, composição com painel oficial e título explícito de planejamento local — rascunho. Não substitui a implementação anterior.
- Este registro único.

### Verificações reais

- Preparação:42testes passaram. Aplicação somente após escrita efetiva via arquivo temporário/fsync/releitura nas4pastas concedidas e comparação das preimagens; nenhum conflito.
- `node --test tests/productionRead.test.mjs tests/finance116.test.mjs tests/suppliers117.test.mjs tests/agenda114.test.mjs tests/payables113.test.mjs`:199aprovados,0falhas.
- `node --test tests/*.test.mjs`:900aprovados,0falhas,0ignorados. Os42novos cobrem contrato, valores exatos, capacidade zero, insumo limitante, divergências, páginas, contexto mutado nos4campos, logout durante transporte/corpo, permissão revogada, aborto e HTTP401/403/404/409/500/503.
- TypeScript: `node node_modules/typescript/bin/tsc --noEmit --project tsconfig.json`, exit0, sem diagnósticos.
- Build via API Vite/plugin React existentes, `configFile:false`, `envFile:false`, em diretório de trabalho separado: aprovado em19,629s. Dist usado pelo proprietário preservado. Aviso de chunks grandes permanece; não foi medido ganho de runtime nesta unidade.
- Prévia explícita com dados em memória e faixa de demonstração: consultar versão, composição, local; resultado3lotes/6unidades; farinha limitante,3,5kg disponíveis,1kg por lote. Cinco idiomas percorridos, temas claro/escuro, estados vazio/offline/negado. Navegação do seletor por teclado verificada. Tabelas com rolagem interna em janela estreita. Não declarar auditoria completa de acessibilidade ou múltiplos viewports medidos.
- Erro inicial apenas na configuração da prévia: `SyntaxError: The requested module '/.vite/deps/zustand.js?v=f0a439c0' does not provide an export named 'create'`. Removidos aliases extras do servidor de laboratório; renderização e ensaios passaram depois. A tentativa de seletor Playwright retornou `Playwright selector deadline exceeded`; seleção por controle acessível/teclado funcionou. Não alteradas dependências do produto.
- Preservação por SHA-256 dos1074arquivos anteriores de src/testes/docs: apenas Recipes86 e este registro alterados; nenhum removido.116/117 e demais arquivos anteriores preservados. Backend separado permaneceu limpo. `git diff --check` e whitespace dos novos caminhos verificados.
- Não executados: serviço/SQLite real, banco comercial, perfil real, integração completa do shell, produção/consumo real, Windows/Linux empacotado, zoom ou auditoria de leitor de tela. Nenhum backend iniciado, instalação, commit, merge, push ou ZIP.

### Verificação e continuidade

Em serviço descartável explicitamente identificado pelo responsável, abrir `/local/recipes`, consultar receita por identificador, conferir versão e composição, escolher local e consultar capacidade. Confrontar com reservas oficiais; zero estoque deve produzir zero capacidade. Troca de local limpa resultado anterior. Logout/troca de contexto durante resposta atrasada deve descartar a resposta. Conferir também os rascunhos e agenda117 preservados.

Produção permanece PARCIAL. Próxima unidade: publicação versionada com revisão explícita, idempotência/recuperação por recibo e planejamento/aprovação de ordens oficiais; depois etapas, materiais, resultado, perdas, lotes/qualidade. Ler todos os contratos correspondentes antes de implementar. Não iniciar backend escolhendo banco/estação/licença. Responsável do backend ainda não identificado para coordenação; contratos não foram modificados.

Preimagens, hashes e logs privados em `work/production-read-20261022/` nesta tarefa Documents/Codex, sem pacote novo. Reversão localizada: comparar hashes aplicados; somente restaurar preimagens correspondentes de Recipes86/registro se não receberam alterações posteriores. Novos arquivos só podem ser retirados após verificar ausência de referências e trabalho posterior. Sem reset/clean/restauração geral.


## Entrega de22/10/2026 — publicação de receitas e planejamento oficial

Aplicada na mesma branch/HEAD, sem conflitos, com conferência de preimagens e escrita efetiva/fsync. Novos: `frontend-web/src/core/productionRead/commands.mjs`, `commands.d.mts`, `commandWords.ts`; `frontend-web/src/modules/tenant/pages/sectors86/ProductionCommands.tsx`; `frontend-web/tests/productionCommands.test.mjs`. Alterados localmente: `core/productionRead/words.ts` (distinguir consulta de envio), `sectors86/OfficialRecipes.tsx` (composição), `officialRecipes.css` e este registro. Rascunhos86,117 e demais fontes anteriores preservados.

- Publicação imutável com seleção de produtos do catálogo, unidades oficiais, rendimento e insumos em milésimos exatos; nova receita ou nova versão com revisão esperada. Não usa JSON livre nem converte rascunho automaticamente.
- Ordens sobre versão congelada, lotes inteiros, local e operador atual responsável. Cadastro/atribuição de outro responsável ainda não compõe este formulário. Aprovação e cancelamento exigem motivo e revisão, sem apresentar aprovação como reserva/consumo.
- Lista de ordens, detalhes e histórico paginados. Resultado incerto mantém tentativa e IDs; consulta original sem reenvio; reenvio apenas explícito, com payload idêntico. Fila por empresa/loja/usuário/aparelho, storage verificado e Web Locks. Fila corrompida não é apagada. Recibos originais validados independentemente de estados posteriores. Leitura histórica não depende de licença ativa no adaptador; guarda86 existente permanece.
- Formulários não enviados ficam em memória e têm aviso beforeunload. Navegação interna da página86 ainda não incorpora esse estado ao seu bloqueador; não confundir com persistência da fila de operação já confirmada para envio, que é durável.
- Contratos: receitas, ordens e recibos de operação do backend separado, sem alteração. Catálogo exige sua permissão própria; erro de consulta não inventa produto ou autoriza escrita.
-42testes novos; suíte completa `node --test tests/*.test.mjs`:942aprovados,0falhas,0ignorados. TypeScript exit0. Build isolado/envFile:false aprovado em19,157s; dist existente preservado. `git diff --check` e whitespace dos novos caminhos passaram. Hashes de1080arquivos base: apenas3fontes acima e este registro alterados, nenhum removido. Sem instalação/commit/merge/push/ZIP/backend/banco comercial.
- Navegador em memória: publicar receita com rendimento2unidades e1kg de farinha; planejar3lotes, saída6unidades; aprovar, revisão2 e histórico; perder resposta do cancelamento, bloquear descarte e confirmar por consulta ao recibo, revisão3, sem reenvio. Cinco dicionários definidos e TypeScript conferidos; os novos formulários ainda não foram percorridos visualmente em todos os idiomas/temas/tamanhos. Não foi ensaiado serviço real, banco comercial, concorrência física, Windows ou instalador.
- Evidências/preimagens/hashes/logs privados em `work/production-write-20261022/` desta tarefa. Próxima unidade em andamento: reserva/liberação/consumo, etapas e resultado, preservando esta fila e a distinção de efeitos. Produção ampla ainda parcial; lotes, perdas e qualidade não entregues nesta unidade.


## Entrega de22/10/2026 — reserva, consumo, etapas e resultado

### Escopo e decisões

Mesma árvore principal, branch/HEAD preservados. Unidade aplicada sem conflito após hashes de preimagem e prova de escrita efetiva. Extensão da MESMA fila `titan-production-commands:v1`, mantendo compatibilidade de comandos pendentes anteriores. Não há aplicador/ZIP, migração ou mudança de backend/Electron.

- `/local/recipes` → ordem → detalhes/histórico → Execução da produção: consulta P11 de um snapshot com quantidades planejadas, reservadas e consumidas distintas. Valores históricos não são saldo disponível atual.
- Reserva atômica de todos os insumos da ordem aprovada; liberar não movimenta estoque; consumir solicita uma única baixa das quantidades congeladas. O contrato atual consome as quantidades da ordem: esta tela não ajusta consumo real arbitrário nem calcula custo completo.
- Etapas opcionais configuradas antes de reserva ativa/consumida, sequência imutável até20etapas, responsáveis atuais atribuídos neste formulário ao operador da sessão. Iniciar/concluir exige consumo, ordem aprovada, identidade responsável e etapas anteriores concluídas. Responsáveis de planos existentes são respeitados; não se amplia permissão para executar etapa alheia.
- Resultado após consumo e etapas configuradas completas, quantidade boa medida entrezero e o plano. Produto em unidades exige inteiro. Zero conclui sem entrada positiva; diferença de rendimento não causa nova baixa de estoque. Nova reserva/consumo e cancelamento após consumo conhecido são bloqueados na interface; backend continua autoridade final.
- Todas as ações passam por revisão humana e recibo original. Session/contexto/permissões/licença/locks/storage continuam verificados. Nenhum resultado incerto é apagado ou reenviado por troca de sessão. Nenhum comando é disparado por carregamento de tela.
- Cinco idiomas; Intl e BigInt na exibição exata; labels, foco e controles44px herdados. Campos de execução também têm aviso beforeunload. O bloqueador de navegação INTERNA da página86 ainda não incorpora os formulários novos; formulários não enviados podem se perder nessa navegação. A fila confirmada para envio permanece durável.

### Arquivos

Novos: `frontend-web/src/core/productionRead/execution.mjs`, `execution.d.mts`, `executionWords.ts`; `frontend-web/src/modules/tenant/pages/sectors86/ProductionExecution.tsx`; `frontend-web/tests/productionExecution.test.mjs`.

Anteriores alterados nesta unidade: `frontend-web/src/core/productionRead/commands.mjs`, `commands.d.mts`; `frontend-web/src/modules/tenant/pages/sectors86/ProductionCommands.tsx`; este registro único. Antes de reutilizar, comparar os hashes finais; não substituir arquivos com trabalho posterior.

Contratos lidos e usados: production-materials, production-results, production-stages, production-trace e operation-receipts no backend separado, conferidos com Go. Novos endpoints: POST material-reservations e state, stage-plans, stages/state, results; GET orders/{id}/trace. Recibos `reserve`, `materials`, `stage_plan`, `stage_state`, `result`. Não altera contratos.

### Evidências e limites

-38testes novos, preparação aprovada. Suíte completa aplicada: `node --test tests/*.test.mjs`,980aprovados,0falhas,0ignorados. TypeScript aprovado, sem diagnósticos. Build isolado final aprovado em19,283s com envFile:false, sem sobrescrever dist. Aviso de chunk grande da planilha permanece.
- Verificações: valores e insumos do snapshot, reserva/consumo, IDs e recibos, bloqueio de sessão para cada tipo de comando, perdas de resposta sem repetir consumo, etapas contíguas e sequenciais, resultado vinculado a consumo e movimento, zero produção sem movimento, unidade e limites de inteiro.
- Navegador/memória: ordem aprovada3lotes, plano6unidades,3kg de farinha e0,030g de sal; configurar Preparação e forno; reservar (consumidozero), consumir (reservadozero), iniciar/concluir etapa, medir5unidades, confirmar resultado/revisão3 e diferença1unidade. Painel e resultado percorridos em pt-BR/en/es/fr/zh-CN; claro e escuro. Não foi percorrida toda combinação formulário×idioma×tema×tamanho.
- Revisão visual detectou cancelamento ainda oferecido após consumo e número do resultado sem locale/unidade no resumo. Ajustados: bloqueio por consumo conhecido e resumo medido com Intl/unidade. TypeScript/build posteriores passaram.
- `git diff --check`, whitespace e preservação por SHA-256: apenas os caminhos previstos; nenhum arquivo anterior removido.117, financeiro116 e backend preservados. Evidências privadas em `work/production-execution-20261022/`, além da unidade anterior em `work/production-write-20261022/`.
- Não executados: serviço/SQLite real, banco comercial, ensaio físico entre aparelhos, Windows/instalador, acessibilidade completa ou consumo produtivo real. Liberação e saída zero cobertas em testes, não no circuito visual descrito. Não iniciar banco/estação/licença escolhidos automaticamente.

Produção continua PARCIAL. Perdas declaradas/estorno, lotes/validade/qualidade, custos, seleção de outros responsáveis e aceite real ainda precisam fechar. Próxima unidade: consultar contratos desses itens e integrar revisão/recibo sem duplicar baixa física ou tratar qualidade declarada como inspeção automática. Demais pendências da primeira entrega permanecem na matriz.


## Entrega de22/10/2026 — perdas declaradas, lotes e parecer humano

### Aplicação e efeitos

Aplicada na árvore principal sem conflito após preimagens e prova de escrita/fsync. Mantém branch/HEAD,117 e todas as alterações anteriores fora do escopo. Usa a mesma fila/locks/validação de contexto e recibos originais. Não há aplicador/ZIP, instalação ou modificação de backend/Electron.

- Declaração de perda sobre a diferença do resultado, com limite global ainda não classificado. Anular devolve apenas a parcela à classificação pendente, preservando motivos/atores/histórico. Não baixa/recompõe estoque ou reabre a ordem.
- Identificação de lote a partir da saída boa já registrada, código único, quantidade exata e datas explicitamente declaradas; validade vazia é desconhecida. Anular libera apenas a capacidade histórica de identificar essa produção, preservando o código e a história. Não cria estoque, saldo por lote, FEFO, recall ou disponibilidade comercial.
- Parecer humano explícito para o lote completo, critério/motivo/ator e revisão própria, distinta da revisão do lote. Não avaliado é revision0/latestnull, sem parecer fictício. Parecer antigo continua visível após anulação do lote, com estado atual anulado e novos pareceres bloqueados. Não é inspeção automática, certificação ou liberação para venda.
- Consultas paginadas de50 mostram totais globais fornecidos pelo serviço, mesmo em página vazia; não somam apenas a página como total. Detalhes/históricos de declaração e parecer; unidades históricas e quantidades exatas com Intl/BigInt. Formulários têm cinco dicionários e aviso beforeunload. Continua pendente o bloqueador de navegação interna descrito nas unidades anteriores.

### Arquivos exatos desta unidade

Novos: `frontend-web/src/core/productionRead/aftermath.mjs`, `aftermath.d.mts`, `aftermathWords.ts`; `frontend-web/src/modules/tenant/pages/sectors86/ProductionAftermath.tsx`; `frontend-web/tests/productionAftermath.test.mjs`.

Alterados localmente: `frontend-web/src/core/productionRead/commands.mjs`, `commands.d.mts`; `frontend-web/src/modules/tenant/pages/sectors86/ProductionCommands.tsx`, `ProductionExecution.tsx`; este registro único.

Contratos de production-losses, production-lots, production-quality e operation-receipts, conferidos também em handlers/modelos Go. POST losses/void/lots/void/quality-reviews; GET summaries por resultado, declaração/histórico, qualidade/histórico. Recibos loss/loss_void/lot/lot_void/quality. Nenhum contrato alterado.

### Verificações e correções

-48testes novos passaram na preparação. Suíte completa aplicada: `node --test tests/*.test.mjs`:1028aprovados,0falhas,0ignorados, incluindo todas as regressões anteriores116/117. Cobertura de datas reais/leap-year, limites globais, unidades, identidade, recibos e sessão para cada tipo, anulação com histórico e recuperação sem novo parecer/efeito físico.
- TypeScript inicialmente falhou com `TS2550: Property 'at' does not exist on type 'ProductionDeclarationEvent[]'`. Corrigido apenas o acesso ao último evento por indexação ES2020. TypeScript final exit0, sem mudar tsconfig ou dependências.
- Revisão visual encontrou recibo confirmado com total anterior ainda na tela, pois uma renderização rápida preservava o componente e suas dependências de consulta não mudavam. Corrigido propagando revisão da operação para renovar totais/detalhes/parecer. Repetição visual confirmou criação/anulação e totais atualizados; não foi reenviado o comando para resolver cache.
- Build final isolado/envFile:false aprovado em20,268s, preservando o dist existente. Aviso de chunk grande permanece. `git diff --check`, whitespace e hashes dos caminhos previstos passaram. Backend separado segue limpo.
- Navegador/memória: resultado5unidades, diferença1; classificar1unidade e consultar histórico; identificar2unidades em DEMO-LOTE-22, fabricação22/10/2026 e validade desconhecida, deixando3sem lote; declarar parecer humano revision1; anular lote e confirmar5sem identificação, histórico e parecer preservados, novas avaliações bloqueadas; anular perda e retornar1à classificação pendente sem baixa/estorno físico. Painéis/pareceres nos5idiomas e claro/escuro. Não foi percorrida toda combinação de formulário/idioma/tamanho nem auditoria completa de acessibilidade.
- Automação do date input não preencheu a data nas primeiras tentativas; valor DOM vazio e revisão rejeitada corretamente. Seleção dos segmentos visíveis corrigiu a data para2026-10-22 antes da confirmação. Avisos createRoot nas prévias vieram de HMR do laboratório, não de serviço real. Não atribuir esses resultados a integração SQLite.
- Não executados: backend/SQLite real, banco comercial, outro dispositivo físico, instalador/Windows, PSP/fiscal/hardware. Nenhum serviço comercial iniciado, credencial lida, commit/merge/push/publicação/ZIP.

Evidências/preimagens/hashes em `work/production-aftermath-20261022/`; unidades anteriores nos diretórios production-write e production-execution desta tarefa. Reversão somente localizada após comparar hashes finais e referências; preservar alterações posteriores e filas pendentes. Não restaurar a árvore inteira.

### Continuidade após estas três unidades

A primeira entrega inteira ainda NÃO está concluída. Produção tem agora o circuito de consulta → publicação → plano → aprovação → reserva → consumo → etapas opcionais → resultado → declarações/qualidade. Isso é integração de frontend aos contratos existentes, com validação isolada; serviço e aceite comercial continuam não confirmados.

Pendências concretas desta área: seleção autorizada de outros responsáveis, custos completos/consumo real variável conforme contrato, estado ativo/inativo e edição de receita, histórico próprio das etapas na UI, proteção de navegação interna para os novos formulários e ensaio real com perfil/banco descartável explicitamente definido pelo responsável do backend. Fontes86/90 e seus rascunhos continuam separados; nenhum rascunho118 aplicado.

As demais áreas da primeira entrega permanecem na matriz. Não converter pendência de frontend em suposta ausência de API. Não ampliar permissões para teste. Não iniciar backend comercial ou pacote desktop sobre combinação desconhecida. Próxima entrega deve continuar neste mesmo registro e comparar hashes antes de editar; não precisa repetir auditoria geral.


## Entrega de22/10/2026 — navegação, estado operacional, histórico e responsáveis

### Base atual e coordenação

Árvore principal preservada: `/home/danillo-malta/Downloads/zzeus-frontend-backend`. Desenvolvimento iniciado nesta unidade na branch `feat/backend-checkout-authority`, HEAD `c5579808c62826174b43a2b7a45fbc2ceda1e47b`, com alterações locais anteriores. Durante a conferência solicitada pelo usuário para salvar no GitHub foi encontrado o commit `9a510880ba1cfcd4065254d4cb7206ae230a2c85` já criado localmente e publicado em `origin/feat/backend-checkout-authority`, com420arquivos de frontend/documentação. A criação desse commit não foi executada pelas ferramentas desta sequência; foi constatada e conferida. Não houve checkout/reset/clean/merge/force-push. O registro posterior é incremental sobre essa base.

Correção explícita do proprietário: backend desenvolvido EM OUTRO CHAT, entregue por ZIPs; não existe segundo Codex local confirmado nem ambiente integrado autorizado. A pasta `zzeus-frontend-backend-b02a` é referência de código/contratos. Sua existência não comprova processo ou banco. Não alterar backend, aplicar seus ZIPs, iniciar8181/8080 ou escolher banco/estação/chaves. No último status, a referência do backend tinha alterações e arquivos novos externos a esta tarefa (incluindo contratos de gestão/produção e inventário/custos); não os atribuir a este trabalho, apagar ou restaurar. Ler os contratos atuais antes da próxima integração independente.

### Unidades e arquivos

1. Proteção dos campos em memória: novos `frontend-web/src/core/productionRead/FormGuard.tsx`, `formGuard.mjs`, `formGuard.d.mts` e `frontend-web/tests/productionFormGuard.test.mjs`. Integração em `frontend-web/src/modules/tenant/pages/LocalSectors86.tsx`, `sectors86/ProductionCommands.tsx`, `ProductionExecution.tsx`, `ProductionAftermath.tsx` e `core/productionRead/commandWords.ts`. Rota, parâmetros e âncora consultam o mesmo registro dos formulários. Permanecer mantém os campos; sair não dispara POST nem apaga/reenvia a fila. O diálogo não oferece salvar rascunho local como se isso enviasse operação oficial. Fechar formulário/trocar ordem ou detalhe passa por confirmação quando pode perder campos. Logout/expiração não são bloqueados para manter uma sessão antiga válida; a fila permanece vinculada ao contexto original.

2. Retenção durante consultas: atualizar snapshot/recibo não desmonta formulários vizinhos. Confirmação usa o ID original para limpar apenas os campos correspondentes e iguais aos revisados. Um lote em edição não é apagado pela confirmação de uma perda. Snapshot retido fica com ações bloqueadas durante leitura/erro; informação conhecida de consumo não é esquecida apenas porque uma atualização falhou. Tentativa incerta não é descartada por navegação/sessão.

3. Estado ativo/inativo: novos `core/productionRead/recipeState.mjs`, `recipeState.d.mts`, `recipeStateWords.ts`, `sectors86/RecipeState.tsx` e `tests/productionRecipeState.test.mjs`. Extensões localizadas de `commands.mjs`, `commands.d.mts`, `ProductionCommands.tsx`, `OfficialRecipes.tsx`. GET `/production/recipes/{id}/state`, POST `/production/recipe-state`, recibo `recipe_state`. Revisão operacional independente da versão imutável: inicial active/revision0, mudança com motivo/revisão esperada. Estado desconhecido/inativo bloqueia novos planos na UI; servidor revalida no envio. Inativação não cancela ordens existentes nem reescreve versões. Edição da composição continua publicação de NOVA versão, não edição retroativa. Mesma fila/locks/recuperação e IDs originais das unidades anteriores.

4. Histórico das etapas: novos `core/productionRead/stageHistory.mjs`, `stageHistory.d.mts`, `stageHistoryWords.ts`, `sectors86/ProductionStageHistory.tsx`, `tests/productionStageHistory.test.mjs`. Extensões de commands/tipos e ProductionExecution. GET `/production/orders/{id}/stages/history?offset=N`, páginas de50, ator/aparelho/motivo/data/revisão e pedidos/resultados originais. Sequência persistida pode ter lacunas por eventos de outras ordens; não exige contiguidade artificial. Valida ordem, operação, sequência crescente, status, revisão e resultado original. Não executa etapa ao consultar.

5. Responsáveis: novos `core/productionRead/responsibles.mjs`, `responsibles.d.mts`, `responsibleWords.ts`, `sectors86/ProductionPeople.tsx`, `tests/productionResponsibles.test.mjs`. Extensões de commands/tipos, ProductionCommands, ProductionExecution e dicionários commandWords/executionWords. Consulta explícita e compartilhada GET `/staff` somente com manage_staff e licença core/staff ativa, além da autorização de produção já exigida. Apenas membros ativos da loja retornada; sem entrada livre de ID, ampliação de permissão, nome inferido ou nova consulta por etapa. Limite100/sem paginação informado; ausência na lista não prova ausência no cadastro. Papel nominal não prova permissão efetiva: aceita candidato ativo e o servidor revalida regras/vínculos por pessoa. Sem acesso ao cadastro, operador atual permanece disponível. Escolha conservada no payload/revisão/recibo de ordem ou etapa; não transforma o operador no responsável escolhido e não permite executar etapa de outra identidade.

Todos esses caminhos são relativos ao mesmo frontend-web; backend e Electron não foram editados. Cinco dicionários mantidos. Não foram instaladas dependências nem gerado novo ZIP/instalador.

### Verificações reais

-7testes de registro/navegação,29de estado da receita,19de histórico e19de responsáveis:74novos. Suíte completa `node --test tests/*.test.mjs`:1102aprovados,0falhas,0ignorados, incluindo financeiro116 e conferência117. TypeScript exit0. Build Vite/plugin React existentes, configFile:false/envFile:false e saída isolada: aprovado em24,869s, dist do usuário preservado. Aviso de chunk grande da planilha permanece; sem medição de runtime nesta unidade.
- Navegador/memória: abrir receita, editar nome, tentar sair/trocar parâmetros/fechar formulário, permanecer preserva; fechamento explícito libera proteção sem POST. Declarar perda1mantendo código de lote em edição: apenas perda confirmada/limpa, código do lote e bloqueio de saída preservados. Fila em memória do laboratório não é prova de persistência comercial.
- Estado: active0 → inactive1, novo plano bloqueado e ordem anterior preservada. Reativação com perda simulada de resposta: fila incerta/descarte bloqueado, consulta do recibo confirma active2 sem repetir POST. Limpeza apenas do motivo correspondente.
- Histórico: configuração, início e conclusão originais com sequências4/7/8, revisões1/2/3; painel percorrido pt-BR/en/es/fr/zh-CN, claro/escuro. Ao usar a fixture de outra ordem no novo plano, a consulta incompatível foi rejeitada e mostrou indisponível; não foi fabricada história. Isso foi limitação da fixture, não ensaio de serviço real.
- Responsáveis: perfil fictício restrito mostra apenas operador e aviso de cadastro indisponível; cenário separado explicitamente fictício/autorizado lista Padeiro descartável, exclui desativado. Plano e etapa revisados/confirmados com responsável baker, distinto de user. Não se ampliou sessão/permissão real. Não foi homologada permissão efetiva de pessoa real.
- Erros de laboratório: preparação tentou ler caminho relativo errado de PROJECT_RULES e depois base-hashes (FileNotFoundError); corrigidos para a pasta de trabalho correta antes da escrita correspondente. Servidores de prévia encerrados entre mensagens produziram ERR_CONNECTION_REFUSED; reiniciada somente a prévia conhecida. Tentativa redundante de iniciar4326 retornou `Error: Port 4326 is already in use`; reutilizado processo já existente, sem encerrar serviço desconhecido. Não são erros de backend comercial. Revisões de locale corrigiram textos antigos de responsável atual.
- Preservação contra1095fontes/testes/docs da unidade anterior: nenhum removido.19novos;9arquivos anteriores alterados por esta tarefa, mais count107.css alterado externamente e já incluído no commit observado (preservado, não sobrescrito). O registro passa agora de revisão7para8. Auditoria de420arquivos do commit observado:1.661.455bytes nos arquivos alterados, apenas frontend/docs, sem paths de banco/.env/chaves/ZIP/node_modules/dist e sem as assinaturas de segredos verificadas. Isso é verificação delimitada, não auditoria completa de todos os segredos do histórico Git.
- Evidências/preimagens/hashes/logs privados em `work/production-navigation-20261022/`, `work/production-state-20261022/`, `work/production-stage-history-20261022/`, `work/production-responsibles-20261022/` desta tarefa Documents/Codex. Sem cópia desses builds para o produto. Reversão só localizada, comparando hashes atuais e trabalho posterior; sem restauração geral.

### Ambiente e cenários de integração ainda necessários

O proprietário confirmou que NÃO existe serviço, perfil ou banco descartável identificado. Essas dependências não suspendem integrações independentes de frontend. Para homologar depois, o responsável do backend deve fornecer versão/commit e contrato exatos, executável correto, URL/perfil explícitos, banco novo descartável isolado, empresa/loja/aparelho/usuários fictícios e licenças/permissões mínimas para cada cenário. Nenhuma credencial comercial deve entrar nos logs ou no repositório. Definir também reinício, restauração e retirada desse ambiente sem afetar produção.

Ensaiar: isolamento entre empresas/lojas/usuários/aparelhos; logout/contexto durante consulta; storage/locks ausentes; resposta perdida e reinício com recibo original sem efeito duplicado; receita inativa impedindo plano mas preservando ordens; publicação/revisões concorrentes; reserva/liberação/consumo uma vez; etapa só pelo responsável autorizado; produção medida/zero; perda/lote sem segunda movimentação; parecer histórico após anulação; lista Staff negada/revogada,100membros e pessoa com regra efetiva de negação; atualização de totais preservando outro formulário; regressões116/117 contra banco descartável. Ainda não executados serviço/SQLite real, banco comercial, hardware, dois aparelhos físicos, Windows/instalador ou acessibilidade completa.

### Próximo ponto de continuidade

Não declarar o frontend inteiro concluído: a matriz da primeira entrega permanece válida. Navegação interna, estado de receita, histórico de etapas e seleção de responsáveis acima deixam de ser pendências de implementação dessas telas; aceite real permanece pendente. Custos/consumo real variável/gestão e inventário devem ser comparados com os NOVOS contratos encontrados na referência do backend, sem aplicá-los nem supor serviço ativo. Continuar unidades independentes dos demais fluxos, atualizar este mesmo registro e salvar checkpoints revisados conforme autorização explícita do proprietário. Não misturar rascunhos workflows117/production118 com o117oficial aplicado.


## Entrega de22/10/2026 — planejamento simultâneo e estimativas de fabricação

Base incremental `a41a8cf75279b4d7fdcc2e212bbc86e910b0f6c6`, mesma árvore/branch. Esse checkpoint e o anterior `9a51088` foram conferidos no GitHub; a main não recebeu merge. Autorização explícita do proprietário para salvar o progresso no GitHub substitui a proibição anterior de commit/push automáticos, sem autorizar reset/clean/merge/force-push ou alteração do backend. Push do registro anterior concluiu e hash remoto foi conferido apesar do aviso `fatal: unable to get credential storage lock in 1000 ms: Read-only file system` do cache de credenciais. Não houve leitura/exibição de credenciais nem tentativa de mudar sua configuração.

### Unidade funcional

`/local/recipes` recebe planejamento de até20versões imutáveis, quantidades inteiras de bateladas e local explícito. Soma cada insumo compartilhado antes de confrontar o mesmo snapshot físico/reservado/livre; não soma capacidades independentes nem credita saídas intermediárias da hipótese como insumo disponível. Mostra falta/saldo e saídas separadas por unidade. Não cria ordem, reserva, compromisso ou evento de estoque. Troca de hipóteses invalida o resultado anterior, sem cálculo automático.

Materiais da receita e da ordem congelada usam consulta autorizada separada. Quantidades, unidades, saída e custo atual por item são conferidos contra o snapshot original. Zero no catálogo exige revisão; mão de obra/despesas desconhecidas permanecem null. A consulta da ordem usa quantidades PLANEJADAS e custo ATUAL; não é custo real da saída boa registrada depois.

Cenário de fabricação combina materiais atuais e hipóteses explícitas para a execução inteira: minutos, pessoas, custo/hora, despesas e preço por unidade comercial. Não multiplica mão de obra/despesas novamente pelas bateladas. Omitido é desconhecido; zero explícito é zero informado. Contribuição apenas com mão de obra E despesas fornecidas; preço zero não produz percentual. Não anuncia custo histórico, custo completo, DRE ou lucro e não preenche tributos/frete/encargos por suposição.

Numerador/denominador exatos com BigInt; conferência de soma, custo unitário, arredondamento half-up só no agregado e truncamento dos pontos-base em direção a zero. Exibição reduz frações e só apresenta decimal quando exato com até6casas; não arredonda uma dízima como se fosse exata. Dinheiro de entrada aceita no máximo duas casas sem conversão por float.

Produção sem view_accounting pode consultar a combinação de insumos, mas não solicita custos. Consultas financeiras exigem manage_production e view_accounting. POSTs de simulação são leituras sem efeitos conforme contrato e Go; não usam licença de escrita, fila durável, recibo de mutação ou Web Locks como se fossem ordens. A função de transporte distingue explicitamente essas duas rotas de cálculo; os POSTs de mutação anteriores continuam com a mesma autorização de escrita. Sessão/contexto/permissão revalidados antes/depois da resposta e leitura do corpo. Dados do pedido são clonados antes de aguardar a resposta; dados atrasados de outra hipótese/sessão não são exibidos.

### Contratos e arquivos

Referência exclusivamente lida: `zzeus-frontend-backend-b02a/docs/api/inventory-costs.openapi.json` e `management-production.openapi.json`, com os fontes Go de material_cost, cost_scenario e simultaneous. O suplemento é código local não homologado em serviço; não se aplicou ZIP/backend nem se iniciou processo/banco.

Rotas integradas: GET `/local/v1/production/recipe-versions/{id}/material-cost?batches=N`, GET `/local/v1/production/orders/{id}/material-cost`, POST `/local/v1/management/production/recipe-versions/{id}/cost-scenario` e POST `/local/v1/management/production/simultaneous-plan`.

Novos arquivos: `frontend-web/src/core/productionRead/planning.mjs`, `planning.d.mts`, `planningWords.ts`; `frontend-web/src/modules/tenant/pages/sectors86/ProductionPlanning.tsx`; `frontend-web/tests/productionPlanning.test.mjs`.

Anteriores alterados: `core/productionRead/commands.mjs`, `commands.d.mts`, `FormGuard.tsx`, `formGuard.mjs`, `formGuard.d.mts`; `sectors86/ProductionCommands.tsx`, `OfficialRecipes.tsx`, `ProductionExecution.tsx`, `ProductionAftermath.tsx`; `tests/productionFormGuard.test.mjs` e este registro. Caminhos core/sectors acima estão sob o mesmo frontend-web/src. Sem dependências novas, backend, Electron, banco comercial ou pacote desktop.

A revisão visual encontrou confirmação excessiva ao consultar a segunda receita, embora a combinação permanecesse montada. O registro de formulários agora distingue campos que a troca de receita realmente substitui: cesta de combinação, ordem e execução continuam protegidas ao sair da rota, sem interromper a seleção de outra receita. Hipóteses/motivo da receita substituída continuam exigindo confirmação quando sujas. Fechar/remover formulários segue proteção; fila pendente nunca é apagada. Também corrigidos o botão Remover receita e o cabeçalho Quantidade na execução, pois a tabela apresenta totais da execução, não uma única batelada.

### Verificação e continuidade

58testes novos de planejamento/custos e1regressão de seleção/navegação. Suíte completa final:1161aprovados,0falhas,0ignorados. TypeScript aprovado. Build Vite/React isolado, configFile:false/envFile:false, em22.494s; dist do proprietário preservado. `git diff --check` e whitespace conferidos. Aviso de chunk grande continua; sem medição em PC antigo.

Navegador/memória explícita: Bread2bateladas e Cake1 compartilham2,5kg de farinha; físico3kg, reservado1kg, livre2kg → falta0,5kg. Reduzir Bread para1retira resultado anterior; nova consulta cabe em2kg livres. Selecionar outra receita conserva a cesta e não exige aviso indevido após a correção; campos de hipótese substituídos continuam protegidos.

Custos: receita de2unidades com1kg de farinha a300centavos e0,010g de sal a100centavos por g →301centavos,150,5por unidade. Preço20,00sem mão de obra/despesas mostra receita hipotética, contribuição desconhecida. Com60minutos,2pessoas a10,00/h e1,00de despesas →2401centavos conhecidos; receita4000; contribuição1599;3997pontos-base. Cinco idiomas percorridos, claro/escuro, quantidades/local/hipóteses e ausência diferente de zero. Não se percorreu toda combinação formulário×idioma×tema×viewport nem leitor de tela completo.

A prévia de desenvolvimento foi recarregada durante mudanças de módulo; resultados descartáveis foram refeitos após a última revisão. A primeira abertura do navegador apresentou demora extraordinária da ferramenta; isso não foi usado como medida de desempenho do frontend ou backend. Uma preparação voltou a usar caminho relativo errado para base-hashes e falhou antes da escrita; refeita com caminho absoluto e preimagens verificadas. Evidências/logs/preimagens/hashes privados em `work/production-planning-20261022/` nesta tarefa; imagem da prévia em outputs. Não copiar esses builds para o projeto nem para pacotes.

Ensaios integrados pendentes: suplemento correto em serviço e esquema identificado; sessão sem/ com accounting, licença expirada para leituras históricas, contexto trocado durante cálculo, versão/unidade divergente, limites e overflow, custos zero/ausentes, estoque reservado concorrente, hipóteses desconhecidas/zero, saída intermediária não creditada e margem negativa. Usar somente ambiente descartável fornecido pelo responsável, conforme revisão8. API em código não é serviço em execução.

O frontend inteiro ainda não está concluído. A estimativa de fabricação e combinação simultânea deixam de ser lacunas de implementação desta tela, mas custo histórico industrial/consumo real variável não foram inventados. Próxima unidade independente: fluxo oficial de inventário revisado (abertura, contagem, envio, outro revisor, reabertura/rejeição/cancelamento, histórico e CSV), preservando ajuste legado e zero distinto de não contado. Demais fluxos da matriz permanecem pendentes e devem ser integrados um a um, sem bloquear trabalho independente pela ausência de ambiente integrado.


## Entrega de22/10/2026 — inventário revisado

Base incremental `7086128eae4ef08e857a0f1ea821f467050295c5`, branch `feat/backend-checkout-authority`, árvore limpa antes da aplicação. Local real e escrita efetiva conferidos com arquivo temporário, fsync, leitura e remoção exclusivamente nos diretórios liberados. Hashes das três preimagens preservadas conferidos antes da escrita; nenhum conflito encontrado. Backup/preparação/logs privados em `work/reviewed-inventory-20261022/` desta tarefa. Não se substituiu Frontend117, backend, Electron, router ou menu; não se instalou dependência nem se gerou pacote.

### Comportamento entregue

`/local/stock` compõe o novo Inventário com conferência e o estoque genérico anterior, com filas independentes. Escopo imutável de1–200produtos/unidades em um local; busca e seleção paginadas. Abertura, contagens parciais, envio, reabertura, aprovação, rejeição e cancelamento usam revisão oficial, revisão humana e IDs originais. Cada nova contagem é digitada explicitamente: vazio conserva a anterior e não vira zero; zero é permitido. Quantidades exatas em milésimos; unidades comerciais inteiras. Diferença negativa exibida corretamente inclusive -0,001.

Contar/enviar/reabrir/rejeitar/cancelar não movimentam estoque. Aprovação/rejeição só para dono/gerente autorizado que não registrou NENHUMA contagem atual; o próprio dono contador é bloqueado. Cancelamento por criador ou revisor autorizado; estados terminais não reabrem. Reabertura conserva histórico/contagens, não gera nova contagem nem aceita diferenças automaticamente. Movimentos posteriores, revisão concorrente e reservas são revalidados pelo servidor; seus conflitos são exibidos sem forçar alteração. Horários, política de frequência e contagem cega não foram inventados: a empresa abre manualmente cada escopo.

Fila durável separada por empresa/loja/usuário/aparelho, leitura de retorno após gravação e Web Locks. Sem autoenvio/autoexclusão. Antes de POST: consulta ao recibo original e preflight da revisão, estado, escopo/unidade e independência do revisor. Só o404 do próprio recibo significa ausência. Um404 do histórico depois de recibo conhecido BLOQUEIA o POST. Confirmação exige recibo mais evento original, com conteúdo, ator, aparelho, IDs, revisão, resultado e data compatíveis. O endpoint de recibo sozinho não devolve o pedido; isso não foi presumido.

Resultado incerto permanece preservado após falha, logout, troca de contexto ou resposta perdida. A consulta não o reenvia; reenvio é escolha explícita com os mesmos dados/IDs. Só tentativa certamente não transmitida/definitivamente recusada pode ser descartada, após consulta de ausência do recibo; isso não apaga inventário/histórico. Tentativa já incerta nunca vira descartável por receber409depois. Sem licença de escrita, consulta histórica autorizada continua disponível; não há permissão ampliada para demonstrar. Sessão/contexto/permissão são conferidos antes/depois do transporte/corpo e antes do download/confirmação de interface.

Lista/histórico paginados pelo total global, revisão monotônica e evento original expandido sob demanda. CSV oficial contém o estado persistido, não campos não enviados; cabeçalho identifica milésimos. Cinco idiomas (pt-BR/en/es/fr/zh-CN), temas, foco e controles mínimos44px. Navegação protege campos em memória, enquanto preserva a fila gravada.

### Contratos e caminhos exatos

Somente leitura da referência externa: `zzeus-frontend-backend-b02a/docs/api/inventory-costs.openapi.json`, `backend/internal/localapi/reviewed_inventory.go` e `backend/internal/localdb/stocktaking/{model,common,read,write,export}.go`. Rotas GET/POST `/local/v1/stock/inventories`, GET `/{inventory_id}`, POST `/{inventory_id}/counts`, POST `/{inventory_id}/state`, GET `/{inventory_id}/history`, GET `/{inventory_id}/report.csv` (sob stock/inventories) e GET `/local/v1/stock/inventory-operations/{operation_id}`. O suplemento continua sem serviço/banco homologado; não se aplicou backend.

Novos: `frontend-web/src/core/reviewedInventory/model.mjs`, `model.d.mts`, `source.mjs`, `source.d.mts`, `words.ts`; `frontend-web/src/modules/catalog/pages/ReviewedInventory.tsx`, `reviewedInventory.css`; `frontend-web/tests/reviewedInventory.test.mjs`.

Anteriores alterados: `frontend-web/src/core/stockWorkspace/words.ts` (texto antigo de inventário nos cinco idiomas), `frontend-web/src/modules/catalog/pages/LocalStock.tsx` (composição e proteção de navegação), este registro. Fontes do estoque genérico/filas anteriores permanecem preservados.

### Evidências e limites

58testes novos aprovados, abrangendo7mutações, zero/nulo, unidade/overflow, autoaprovação inclusive dono, revisões/escopo/ator/aparelho, quotas/corrupção/lock, troca de todas as identidades, logout entre consulta e POST, logout depois do envio, perda da resposta, recuperação original sem repetir POST, histórico404, licença/sessão expirada, CSV/tipo/cabeçalho e contexto atrasado. Suíte completa:1219aprovados,0falhas,0ignorados. TypeScript completo aprovado. Build Vite/React isolado em23.448s (configFile:false/envFile:false); dist existente preservado. Aviso de chunk grande da planilha continua. `git diff --check` aprovado.

Navegador em prévia EXPLÍCITA apenas em memória, sem backend/banco: abertura com produto em unidade e kg; contagem parcial0deixa outro Não contado e bloqueia envio; segunda contagem1,250resulta diferença+0,250; envio não ajusta estoque; dono contador não aprova/rejeita. Outro revisor vê as ações. Conflito simulado inventory_recount_required exibido, tentativa recusada preservada e descartada só após ausência de recibo; reabertura conserva histórico e exige entrada explícita para recontagem. Nova contagem por outra identidade, envio e aprovação por quem não contou atualmente. Perda simulada da resposta após aprovação deixa resultado incerto, bloqueia descarte e é recuperada consultando recibo/evento originais, estado final aprovado/revisão8/movimentos. CSV preparado para download. Cinco idiomas percorridos e claro/escuro observados; não é cobertura completa de todas as combinações, viewport ou leitor de tela. Capturas de dados descartáveis em outputs. O harness tem transporte em memória rotulado, nunca embutido na rota real.

Falha inicial da compilação candidata: import relativo payableDocuments não estava vinculado no laboratório; criado vínculo apenas de leitura e checagem refeita com sucesso antes da aplicação. Uma ação da ferramenta visual perdeu o alvo após atualização da prévia; estado inspecionado e ação retomada sem repetir envio. Não se confunde isso com falha do produto.

Para homologação: responsável deve fornecer suplemento/commit/build/schema identificados, perfil local explícito, URL autorizada e banco DESCARTÁVEL, dois usuários revisores com permissões reais, aparelho/empresa/loja, catálogo/local/reservas de teste. Validar401/403/licença, concorrência entre aparelhos, movimento posterior inclusive saldo líquido zero, reserva acima da contagem, transação/auditoria/outbox/recibo únicos, reabertura/recontagem, rejeição/cancelamento, histórico/CSV e perda de resposta/reinício. Não iniciar serviço nem escolher banco por suposição. O limite de offset do histórico é1.000.000: tentativa cujo evento não pode ser conferido fica bloqueada para revisão técnica, sem reenvio por outro ID.

O frontend inteiro continua parcial conforme matriz: este ciclo deixa de ser lacuna de implementação, mas política de frequência, custo histórico/valoração, PDV/fechamento avançado, demais integrações/fluxos e homologação integrada permanecem pendentes. Continuar unidades independentes com contratos disponíveis. Não anunciar percentual ou conclusão integral com base nesta suíte isolada.


## Entrega de22/10/2026 — posição/valoração e diferenças de inventário

Base `49d870e0ae8725cea7018a9a4b0feebe0ac6dafc`, publicada e hash remoto conferido na mesma branch. Sem merge/main/reset/clean, dependências, pacote, backend, Electron ou banco. Preimagens e escrita efetiva verificadas novamente; sem conflito. Fontes/preimagens/logs/hashes privados em `work/stock-reports-20261022/`; capturas de dados descartáveis em outputs.

`/local/stock` passa a selecionar três painéis: movimentos genéricos anteriores, inventário revisado e posição/valoração. Botões com estado pressionado e rótulos nos cinco idiomas. Trocar de painel conserva os componentes/campos montados; não há envio ou descarte implícito. Os fluxos financeiro116/117 não foram substituídos; a navegação de rota mantém a proteção de campos e filas. A consulta física anterior e sua fila continuam independentes.

### Consultas e limites

Posição por local obrigatório, produto opcional escolhido na busca paginada,50produtos/página, total global e horário do snapshot. Físico/reservado/livre são conferidos no mesmo retrato; não autorizam escrita nem reserva. Sem permissão financeira não se solicita o endpoint de valoração, não se aceita vazamento de campos financeiros na resposta física e não se mostra opção de custo. Leituras autorizadas não exigem licença de escrita nem Web Locks/fila/recibo de mutação.

Com manage_stock E view_accounting: valoração usa físico × custo ATUAL do cadastro, com soma SOMENTE da página. Valores são racionais exatos de centavos; BigInt, sem arredondamento por linha ou conversão por float. Valores acima do limite numérico seguro permanecem strings/racionais exatos. Custo zero exige revisão, não significa produto sem custo. Não anuncia custo histórico, patrimônio total, DRE, lucro ou lançamento financeiro. Alteração de local/produto/modalidade invalida a consulta visível e aborta a exportação antiga. Nova página consulta outro retrato explicitamente; os totais não são somados como se compartilhassem instante único.

CSV oficial usa os mesmos filtros/página e autorização, confere MIME/cabeçalho e é consultado novamente; a tela explica que o horário exportado pode ser posterior e não representa o catálogo inteiro. Numerador/denominador de valores fracionários são identificados em centavos. Resultado atrasado após logout/troca de contexto/permissão não pode ser usado nem baixado.

Inventário revisado recebe análise estimada de faltas, sobras, iguais e não contados. Confere TODAS as linhas/contagens/atores/datas/movimentos e a mesma revisão/estado do detalhe exibido antes de exibir custos. Mudança concorrente exige atualizar o inventário, sem misturar retratos. Calcula falta/sobra/net exatos para validar os dados devolvidos, usando custo atual, sem registrar movimento/financeiro. O custo não é congelado e o contrato não fornece horário próprio de valoração desta análise; não se inventa data oficial de custo. Não contado continua null, separado de diferença0. Nenhuma consulta dispara POST.

### Arquivos e referência

Novos: `frontend-web/src/core/stockReports/model.mjs`, `model.d.mts`, `source.mjs`, `source.d.mts`, `words.ts`; `frontend-web/src/modules/catalog/pages/StockReports.tsx`; `frontend-web/tests/stockReports.test.mjs`. Alterados: `frontend-web/src/modules/catalog/pages/LocalStock.tsx`, `ReviewedInventory.tsx` e este registro. Ajuste genérico/recuperação de filas/autorizações não foram ampliados.

Referência lida, não aplicada: `zzeus-frontend-backend-b02a/docs/api/management-production.openapi.json`, `backend/internal/localapi/management_reports.go`, `backend/internal/localdb/stock/report.go`, `availability.go`, `backend/internal/localdb/stocktaking/variance.go` e modelos/contratos relacionados. GET `/local/v1/management/stock/positions`, `/positions.csv`, `/valuation`, `/valuation.csv` (estes sob management/stock), e `/local/v1/management/inventories/{id}/variance`. API em arquivo não comprova serviço em execução. As primeiras buscas por positions.go/management.go falharam porque esses nomes não existiam; rg localizou report.go, efetivamente lido. Não se criou arquivo fictício no backend.

### Verificações reais

46testes novos; suíte completa final1265aprovados,0falhas,0ignorados. TypeScript final aprovado; build Vite/React isolado em23.856s, sem arquivo de ambiente/configuração carregado, dist anterior preservado. `git diff --check` aprovado. Aviso de chunk da planilha permanece; não houve benchmark em hardware antigo.

Prévia explícita em memória:51produtos, primeira página50 →62centavos estimados; segunda página1 →1,25centavos, total global51 e indicador2/2. Filtro de produto retorna1/1e2centavos, sem conservar o resultado do filtro anterior. Custo zero visível com aviso. CSV de página preparado. Diferenças:2contadas,1não contada,0iguais; faltas100centavos positivos, sobras0,25, saldo-99,75; valor de linha não contada permanece Não contado. Cinco idiomas percorridos, claro/escuro observados. Perfil de teste sem view_accounting mantém posição física e não mostra controles de valoração. São identidades/capacidades de laboratório, sem ampliar permissão real.

Composição REAL LocalStock/LocalAccess/estado de sessão/cliente, envolvida em harness explicitamente em memória: campo de novo inventário preservado ao abrir relatórios e retornar; painéis ocultos não aparecem no estado de acessibilidade do painel selecionado. O harness intercepta somente a origem própria da prévia, não acessa servidor/banco e não é entregue no produto. Uma importação absoluta de módulo i18next gerou incompatibilidade CommonJS na primeira abertura; corrigida no laboratório para resolução normal, sem instalar dependência ou mudar o produto. A otimização de dependências recarregou a outra prévia; dados e consultas descartáveis foram refeitos. Um clique no fechamento aguardou a confirmação nativa e a ferramenta reportou timeout; estado final foi inspecionado e mostrou formulário fechado, sem repetir envio.

Limites: não percorremos todas as combinações tela×idioma×tema×viewport, leitor de tela ou múltiplos aparelhos. Pendente homologação com suplemento/build/schema corretos, banco descartável autorizado, catálogo/custos/local/reservas, permissões reais com/sem accounting e licenças expiradas; conferir snapshot transacional, valores nulos/zero, custo modificado, revisões concorrentes, CSV e revogação durante resposta. Nunca usar banco comercial ou escolher URL/estação/chaves por suposição.

Custo atual por página e estimativa de diferenças deixam de ser lacunas de interface. Custo histórico/contábil de perdas, periodicidade da empresa e demais fluxos da matriz continuam pendentes. Próximas unidades independentes devem usar contratos existentes (por exemplo, análise comercial real de produtos vendidos e sugestão de reposição), mantendo aprovação humana e sem reimplementar o117de conferência por fornecedor.


## Entrega de22/10/2026 — análise oficial de vendas por produto

Base `3d36870aad46d4b7acbe6d20e9fbfacc5f156ebf`, hash local/remoto conferidos e árvore limpa. Preimagens das três fontes existentes e deste registro conferidas antes da escrita. Acesso efetivo a core/router e tenant/pages verificado com arquivo temporário/fsync/leitura/remoção. Sem conflito, instalação, pacote, alteração de backend/Electron/banco ou substituição do117. Evidências privadas em `work/sales-analysis-20261022/`.

### Unidade entregue

Nova rota sob demanda `/local/reports`, intitulada Vendas confirmadas por produto, com link próprio no grupo Gestão. Disponível SOMENTE com a permissão existente view_accounting; dono não recebe exceção. Contador autorizado pode consultar sem licença de escrita, em conformidade com a consulta histórica do backend; isso não habilita PDV, caixa, estoque ou qualquer operação. O cartão genérico planejado de relatórios não é duplicado para quem pode abrir esta unidade. O painel legado do dono permanece parcial e separado; esta entrega não o anuncia como concluído.

Períodos no fuso do aparelho, início incluído/fim excluído. Padrão de sete dias completos e comparação anterior adjacente de mesma duração; usuário pode escolher comparação própria. Cada intervalo limitado a366dias, anos2000–2200 e datas válidas; lacunas de horário local inválido recusadas, com conversão explícita para instantes RFC3339. Contrato e comparação de fronteiras preservam nanossegundos com BigInt, sem arredondar milissegundos. Períodos sobrepostos têm aviso: mesma venda pode estar nos dois, não somar totais. Durações diferentes só comparam totais, sem percentual.

Fonte EXATA: itens de vendas com status committed da mesma empresa/loja. Não comprova pagamento, liquidação, lucro, DRE, nem receita líquida de devoluções registradas em fluxo separado. Nome/SKU/unidade são metadados do cadastro ATUAL conforme Go, não snapshot histórico inventado; mudança de unidade precisa de conferência. Baixa venda isolada não vira alerta de prejuízo.

Totais globais dos dois períodos, diferença exata em centavos, produtos distintos e linhas-fonte examinadas; tabela paginada50com quantidade milésima/unidade, valores atual/anterior/diferença, percentual/bases e ABC. Fonte anterior zero é Sem base/Atividade nova, não0%ou infinito. Percentual truncado em pontos-base pelo servidor e conferido com BigInt; fora do intervalo exato permanece desconhecido. ABC usa total atual da loja, ordenação por valor decrescente e ID UTF-8 no empate, limites80/95do acumulado ANTERIOR à linha. Não é margem/lucro/giro/ranking de outra empresa. Critério global e categorias são validados onde os dados da página permitem, sem inventar acumulado de páginas anteriores.

Limite de100.000linhas-fonte é tratado como erro que exige período menor, sem vender resultado parcial como completo. Inconsistência da fonte tem mensagem específica. Ausência de API permanece indisponível, nunca mock na rota real. Só GET, sem fila/lock/recibo de mutação ou sucesso comercial. Sessão/empresa/loja/usuário/aparelho e permissão conferidos antes/depois da resposta/corpo. Pedidos clonados antes da espera; trocar filtros invalida o retrato e impede download antigo. CSV reconsulta a página com os mesmos dois períodos; contém somente a página e pode refletir alterações posteriores, conforme aviso.

### Caminhos e referência

Novos: `frontend-web/src/core/salesAnalysis/model.mjs`, `model.d.mts`, `source.mjs`, `source.d.mts`, `words.ts`; `frontend-web/src/modules/tenant/pages/LocalSalesAnalysis.tsx`; `frontend-web/tests/salesAnalysis.test.mjs`.

Alterados: `frontend-web/src/core/router/index.tsx` (rota lazy), `frontend-web/src/core/local/accessModel.mjs` (área reports/read permission), `frontend-web/src/modules/tenant/pages/LocalMainMenu.tsx` (link existente autorizado, sem duplicar cartão planejado) e este registro.

Referência somente lida: `zzeus-frontend-backend-b02a/docs/api/management-production.openapi.json`, `backend/internal/localapi/management_reports.go`, `backend/internal/localdb/management/{model,sales,export}.go`. GET `/local/v1/management/sales/products` e `.csv`, com from/to/compare_from/compare_to/limit/offset. Nenhum desses arquivos foi aplicado ou executado por este chat. A referência local continua sem serviço/banco homologado. Buscas iniciais de nomes supostos de páginas/router não existentes foram corrigidas com rg --files e leitura dos caminhos reais, antes de qualquer escrita.

### Verificações

50testes novos aprovados. Suíte completa1315aprovados,0falhas,0ignorados; TypeScript completo aprovado; build Vite/React isolado em24.169s, sem carregar arquivo de ambiente/configuração, preservando dist anterior. `git diff --check` aprovado. Chunk grande da planilha continua pendente; não se mediu PC antigo.

Prévia EXPLÍCITA em memória: totais1100e600centavos, diferença500,3produtos,5linhas-fonte; Arroz+100%, atividade nova sem percentual, Sal-100%; ABC A/B/sem valor atual. Mudança de períodos remove resultado anterior. Período desigual retira todos os percentuais; sobreposição mostra aviso e mantém comparações independentes. Erro422de limite mostra mensagem de reduzir período, sem vazio/sucesso fictício. CSV preparado para download; cinco idiomas e claro/escuro percorridos. Captura com dados descartáveis em outputs. Sem POST permitido no transporte da prévia.

A ferramenta de preenchimento direto dos campos datetime-local não alterou seus valores; inspeção DOM confirmou vazio. Preenchimento pelos segmentos nativos foi conferido antes de consultar (01–02/10 para duração desigual,14–21/10 para sobreposição); não se anunciou cobertura pela tentativa de fill sem efeito. Isso é limite da automação, não integração contra servidor. Não se percorreu todo formulário×idioma×tema×viewport/leitor de tela. O menu/guardas foram conferidos por código, TypeScript/build e regressões de permissão; a prévia da análise não equivale a teste completo da navegação autenticada do produto.

Homologação pendente com build/schema/banco descartável autorizados: sessões reais view_accounting com/sem dono, licença expirada, fuso/horário de verão, fronteiras exclusivas com nanossegundos, janelas sobrepostas/desiguais, produtos gratuitos/anteriores/zerados, empate ABC entre páginas, volume100.000/100.001, metadados/unidades alterados, cancelamento e devolução em fluxos distintos, CSV e revogação durante resposta. Não usar este relatório como liquidação ou DRE; coordenar contrato futuro para receita líquida de devoluções se exigido. Não iniciar serviço nem escolher banco/estação/chaves por conta própria.

A consulta comercial de produtos deixa de ser lacuna de implementação. Painel executivo amplo, caixa/fechamento/pagamentos/fiscal, demais ciclos de compras/fornecedor/logística/financeiro/RH e integrações continuam conforme matriz. O frontend inteiro ainda não está concluído nem homologado. Continuar unidades independentes usando contratos atuais e preservar ideias de expansão fora do fechamento inicial.


## Entrega de22/10/2026 — prévia de reposição por demanda observada

Base incremental `ff98a15c105cad6ef1bb6d5d60ce36ee69b28e87`, hash local/remoto conferidos. A retomada confirmou a mesma branch e as mesmas alterações preparadas. Permissões do ambiente foram atualizadas; escrita efetiva testada novamente após liberação técnica apenas de frontend/docs/Git. Hashes das preimagens e dos arquivos já aplicados conferidos, sem conflito. Fonte principal continua Downloads/zzeus-frontend-backend; backend não alterado/iniciado, sem banco, dependência, ZIP, Electron, main/merge/reset/clean.

### Fluxo preservado e complemento

O115já configura política, calcula sugestão durável, exige revisão humana e pode criar pedido local. NÃO foi recriado. A nova seção aparece na aba Reposição do115, sob o produto escolhido com política existente, apenas com manage_stock E manage_replenishment (além do acesso já exigido à área de pedidos). O117de conferência por fornecedor e as duas filas115 permanecem preservados.

Prévia é hipótese READ-ONLY por contrato, embora use POST `/local/v1/management/replenishment/preview`. Não tem operation_id, não grava fila, não adquire lock de mutação, não exige licença de escrita e não modifica política/sugestão/decisão/pedido. O transporte específico revalida suas duas permissões e contexto; não ampliamos o scopedFetch das mutações115 nem sua autorização. A interface com licença expirada continua bloqueando salvar política/criar pedido, mas permite esta consulta autorizada. Nenhuma política, compra, reserva ou contato com fornecedor é assumido.

Usuário informa prazo0–365dias inteiros, cobertura1–365e segurança explícita (zero permitido), com observação de1hora a366dias. Esses campos começam vazios; não se inventa condição de fornecedor nem segurança comercial. Períodos no fuso do aparelho, início incluído e fim excluído; cálculo usa instantes exatos. Produto/unidade são vinculados ao item selecionado. Edição pendente da política/revisão bloqueia a prévia e retira o resultado anterior; trocar hipóteses invalida o resultado sem recálculo automático.

Demanda observada de itens de vendas committed da mesma loja, não garantia futura ou receita/devolução líquida. Estoque físico/reservado/livre agrega TODOS os locais locais da loja no mesmo snapshot. Não representa estoque global offline. Mínimo = demanda diária × prazo + segurança; alvo = demanda diária × (prazo+cobertura) + segurança; ambos respeitam os pisos da política oficial. Arredondamento para cima a unidade comercial inteira ou milésimo das demais. Sugestão apenas quando livre<=mínimo e livre<alvo, descontando o livre, sem confundir físico com disponível. Sem vendas, demanda0e pisos/segurança explícitos, sem forecast inventado.

Cálculo devolvido é conferido com BigInt/racionais exatos, inclusive demanda fracionária, overflow, diferença de unidade/período e quantidade comercial. Exibição de dízima como fração exata, não decimal arredondado apresentado como verdade. Política é a oficial do momento da consulta; se revisão/mínimo/alvo divergirem da lista antiga, a tela avisa para atualizar/conferir antes de gravar qualquer alteração no115. A prévia nunca copia automaticamente a sugestão aos campos da política e não aprova nada.

Sessão, empresa, loja, usuário, aparelho e permissões conferidos antes/depois de resposta/corpo e antes de apresentar o resultado. Hipóteses/produto clonados antes da espera. Falha não vira zero nem cria tentativa de mutação. API ausente/política inexistente, conflito e limite/invalidez aparecem com mensagens; nenhuma resposta fictícia na rota real. A empresa decide comprar; fornecedor confirma atendimento e condições pelo fluxo próprio. Sazonalidade, validade, margem e compromissos devem ser considerados, sem concluir prejuízo pela baixa saída.

### Arquivos e referência

Novos: `frontend-web/src/core/restockPreview/model.mjs`, `model.d.mts`, `source.mjs`, `source.d.mts`, `words.ts`; `frontend-web/src/modules/catalog/pages/RestockPreview.tsx`, `restockPreview.css`; `frontend-web/tests/restockPreview.test.mjs`.

Alterados: `frontend-web/src/core/orders115/source.mjs`, `source.d.mts` (adaptador e verificação de acesso somente da prévia); `frontend-web/src/modules/catalog/pages/Orders115.tsx` (composição sob política/produto escolhido) e este registro. Lógica de submit/recover/locks/filas anteriores intacta.

Referência somente lida: `zzeus-frontend-backend-b02a/docs/api/management-production.openapi.json`, `backend/internal/localdb/stock/restock_preview.go`, `backend/internal/localapi/management_reports.go`, disponibilidade/identidade relacionados. Contrato local não comprova serviço atualizado. Busca inicial por diretório replenishment115 não existente foi corrigida usando os fontes reais orders115 e local/replenishmentClient; não se implementou função duplicada a partir de nome suposto.

### Verificação real e limites

47testes novos aprovados; suíte completa final1362aprovados,0falhas,0ignorados, incluindo regressões anteriores115/financeiro116/117. TypeScript final aprovado; build Vite/React final isolado em20.004s, sem carregar arquivo de ambiente/configuração, preservando dist existente. `git diff --check` aprovado. Aviso do chunk grande de planilha continua. Não houve benchmark em PC antigo ou homologação de serviço.

Testes: política/pisos, mínimo exato, acima do mínimo sem pedido, estoque reservado/livre, unidade comercial e kg, fração/overflow/tempo/nanossegundo, ausência de vendas, pedidos proibidos na resposta, ambos os permissions, contexto/troca de sessão/corpo atrasado, abort, clone, ausência de política e falhas. Adaptador115consultado com storage/locks que lançam erro se acessados: a prévia não toca filas nem pede lock. Sem Web Locks/licença de escrita a leitura permanece autorizada; remover uma das permissões bloqueia até o POST de cálculo.

Prévia EXPLÍCITA em memória usando a composição REAL Orders115 e seu cliente, sem serviço ou banco:7unidades vendidas/7dias →1un/dia; físico4, reservado2, livre2. Prazo2, cobertura3, segurança1 →mínimo3, alvo6, sugestão4, sem pedido criado ou fila gravada. Sem vendas →pisos2/5e sugestão3. Política simulada alterada para revisão3/10/15 →aviso de atualizar e sugestão13, sem gravar a política. Cinco idiomas percorridos e claro/escuro observados. Perfil kg:1kg/7dias →1/7kg por dia, mínimo0,143kg, alvo/sugestão0,286kg. Edição de política na prévia remove resultado e desabilita consulta; desbloquear não consulta automaticamente. Nova consulta explícita refez o resultado. Não cobrimos todas as combinações de idioma/tema/viewport nem leitor de tela completo.

A revisão visual encontrou herança de `.o115 dl>div` que apertava rótulos/números. CSS próprio e restrito à prévia separa rótulo/valor e amplia as colunas, sem alterar o115inteiro. Aplicação/estilo conferidos com captura de dados descartáveis em outputs. Na retomada o vínculo da ferramenta de navegador havia sido reinicializado; a aba existente foi recuperada e o ajuste final recarregado, com hipóteses reintroduzidas. No laboratório TypeScript inicial faltava vínculo de leitura dos cinco JSON115; vínculo corrigido e checagens refeitas, sem mudar traduções legadas ou instalar dependência.

Evidências/preimagens/hashes/logs privados em `work/restock-preview-20261022/`. O servidor da prévia desta unidade foi encerrado após a verificação; nenhum backend comercial foi executado. Não copiar laboratório/builds para o projeto ou pacotes.

Integração pendente: responsável fornecer build/schema/URL/perfil e banco DESCARTÁVEL identificados. Validar catálogo/política, ambos os permissions, licença expirada de leitura, concorrência de política/reservas, todos os locais (limite500), origem de vendas (limite100.000linhas), ausência de vendas, unidades alteradas, fronteiras/horário de verão, volume/overflow e reconsulta após revogação. Backend sinaliza alguns limites como400genérico; frontend não inventa qual limite exato ocorreu. Não iniciar serviço nem selecionar banco/estação/chaves sem definição do responsável.

Esta hipótese de reposição deixa de ser lacuna de interface. Confirmação entre empresas, atendimento do fornecedor, doca, motorista/coleta, financeiro/fiscal e outros fluxos da matriz ainda precisam das unidades e integrações correspondentes. O frontend inteiro não está concluído/homologado; os1362testes isolados não substituem esses aceites. Próxima frente independente deve priorizar as lacunas comerciais/operacionais ou o desempenho da planilha já medido como chunk grande, sem confundir otimização de bundle com redução comprovada de tempo/memória no aparelho.


## Entrega de22/10/2026 — inicialização adiada do editor de planilhas

Base `a4fb332b337db54c554a39dd0d771c85ef6f6817`, branch feat/backend-checkout-authority, árvore limpa na retomada. Regras relidas; caminho real Downloads/zzeus-frontend-backend confirmado. Nova concessão técnica de escrita foi seguida de teste efetivo criar/fsync/reler/remover arquivo temporário exclusivamente em frontend/docs. Preimagens conferidas antes da aplicação; sem conflito. Backend, Electron,117e suas filas preservados; nenhuma dependência instalada, ZIP ou banco usado.

O editor Univer deixou de ser dependência estática da página. Interface inicial e indicador de carregamento podem aparecer antes do motor. Editor e idioma são importados em paralelo; idioma é conferido novamente quando o motor termina, protegendo mudança durante a espera. Navegação/contexto obsoleto não monta editor. Falha mantém estado indisponível e oferece nova tentativa. A falha tardia de uma instância descartada não desativa uma nova instância válida.

Comandos, campos e persistência automática aguardam editor pronto. Substituir/reabrir documento desativa a prontidão imediatamente. A biblioteca permanece montada entre reinicializações, preservando identidade, versões e blocos. Atualizar blocos após abrir arquivo aguarda o workbook correto, substituindo a espera fixa de300ms. Função TITAN e verificações de acesso preservadas; não foi movida a worker, pois captura estado de permissão/cache. Todos os presets atuais foram mantidos. Paleta primária do motor usa o cinza existente, retirando os valores dourados explícitos.

Arquivos novos: core/sheets70/editorLoader.mjs, editorLoader.d.mts, editorWords.ts; tests/sheetEditorLoader.test.mjs. Alterados: LocalSpreadsheet.tsx, Sheets70.tsx, sheetRuntime70.ts, spreadsheet.css, neste caso dentro de frontend-web/src/modules/tenant/pages; além deste registro. Rascunhos antigos não usados.

### Medição e verificação

Builds isolados antes/depois com a mesma configuração Vite/React, sem configFile/envFile, em work/spreadsheet-performance-20261022. Grafo de imports ESTÁTICOS da página, descontados os chunks já estáticos da entrada principal:6.717.446→118.885bytes; gzip calculado por arquivo:1.809.197→48.107bytes. Chunk que contém LocalSpreadsheet:6.659.122→60.358bytes. O motor foi adiado, NÃO removido: todos os chunks JS somados18.629.097→18.630.914bytes. Continua existindo chunk grande do runtime. Não chamar isso de diminuição do instalador ou ganho medido de abertura/memória em PC antigo. Build antes19,638s, depois19,887s; são duas observações, sem inferência de desempenho de execução.

13testes novos: importações paralelas, contexto inativo, logout/navegação, troca de idioma antes do motor e durante nova carga, falhas e cinco idiomas. Suíte completa1375aprovados,0falhas,0ignorados; TypeScript aprovado; build aprovado; diff --check conferido antes do checkpoint.

Prévia EXPLÍCITA isolada em127.0.0.1:4332, sem backend e sem dados comerciais, com usuário employee sem permissões adicionais. Grade real: A1=10,A2=20,A3=SUM(A1:A2)→30. Salvamento IndexedDB local, recarga da página e reabertura pela biblioteca mantiveram valores/fórmula; controles foram bloqueados durante a reinicialização e liberados depois. Claro/escuro e cinco seleções de idioma percorridos; francês e chinês conferidos também no motor após a importação terminar. Trocas rápidas podem mostrar a tradução anterior por instantes enquanto o pacote carrega; resultado final chinês/português conferido. Nenhum erro de console no circuito final. Captura em outputs/planilha-carregamento-validado-2026-10-22.png, somente dados descartáveis.

O laboratório inicialmente usou papel operator que o contrato não reconhece; rejeição das capacidades foi preservada e o papel de teste corrigido para employee, sem ampliar permissões do produto. A primeira composição do laboratório não reproduzia o contêiner de altura limitada da aplicação e escondia a grade; somente o HTML do laboratório foi ajustado. Timeout da captura de acessibilidade após digitar não foi considerado falha das células: screenshot posterior confirmou10/20/30. A biblioteca inicialmente mostrou vazio durante leitura assíncrona; resultado posterior confirmou o documento. Não se afirmou êxito com base nesses estados intermediários.

Continuam pendentes medição no hardware alvo, matriz completa teclado/touch/leitor de tela/viewport, fontes reais e compartilhamento/colaboração da planilha; não houve homologação do backend ou do frontend inteiro. Próximas unidades devem conectar fontes reais existentes e concluir os fluxos parciais da matriz, sem converter arquivo local em persistência comercial.


## Entrega de22/10/2026 — fonte física de estoque na planilha

Base incremental `9280c05966d388a559ddd748f3faeb5bbce45a96`, checkpoint anterior publicado e hash remoto conferido. Preimagens novamente verificadas; nenhuma divergência. Fonte principal mantida em Downloads/zzeus-frontend-backend, branch feat/backend-checkout-authority. Sem backend, Electron, ZIP, dependência, banco ou alteração do117.

A fonte Estoque70 deixava a rota real indisponível e apenas retornava demonstração quando habilitada por ambiente. Foi substituída por adaptador tipado para os contratos já existentes GET /local/v1/locations e GET /local/v1/management/stock/positions. Não existe fallback de demonstração nesta fonte. Autorização exige manage_stock e módulo inventory pelos mesmos critérios da área; não amplia operador, não consulta custos nem exige permissão contábil. Licença expirada permite a leitura histórica/autorizada conforme a regra existente; não autoriza mutação.

Escolha explícita de local, inicialmente vazio; nunca escolhe o primeiro ou agrega locais silenciosamente. Colunas: ID/SKU/nome/unidade, ID do local, físico/reservado/livre em MILÉSIMOS inteiros (1000=1), instante de observação de cada página. Filtro textual aplicado ao recorte examinado, não busca global. Limite de500posições, páginas50; parcial conserva aviso mesmo quando busca retorna zero. Se contagem global mudar entre páginas ou houver produto duplicado entre páginas, pede nova consulta em vez de apresentar total enganoso. Não é snapshot global atômico/offline, custo, inventário aprovado ou disponibilidade de outro aparelho.

Resultado parcial passa a constar também no bloco vinculado após inserir/atualizar. Resumo do bloco mostra o local; detalhes repetem o limite e a unidade. Rótulos/ajuda novos nos cinco idiomas. Cabeçalho do diálogo foi localizado como Dados do sistema em vez da chave data. Campos de busca/local e fonte invalidam/abortam prévia anterior; datas permanecem desabilitadas para estoque atual. Blocos antigos sem local não são convertidos por suposição: devem ser editados escolhendo um local.

Adaptadores recebem sessão e callback de atualidade; estoque verifica quatro IDs, token, autorização, abort e corpo antes/depois de cada página. O chamador também protege erro tardio de consulta/atualização: depois de mudar sessão/empresa/loja/usuário/aparelho, a tentativa antiga não limpa o workbook nem invalida a nova sessão por um401/403antigo. Revogação na própria sessão continua usando a limpeza existente. Nenhuma fila/operação de resultado incerto do financeiro foi apagada ou reenviada; esta unidade é somente GET.

### Arquivos e evidências

Novos em frontend-web: src/core/sheets70/stockSnapshot.mjs, stockSnapshot.d.mts, stockWords.ts; tests/sheetStockSnapshot.test.mjs. Alterados: src/core/sheets70/stock.ts, contracts.ts, model.d.mts; src/modules/tenant/pages/Sheets70.tsx; este registro. Contrato/validação física de stockReports reaproveitados, não recriados. Nenhum aplicador antigo usado.

30testes novos aprovados, incluindo local obrigatório, paginação51/500/501, busca parcial sem matches, inteiro exato, vazio confirmado, todas as quatro identidades, logout/corpo tardio/permissão/módulo, aborto, clone, 401/403/404/503, diretório inválido, duplicação entre páginas, contagem alterada e rejeição de custos em resposta física. Suíte final1405aprovados,0falhas,0ignorados, incluindo regressões116/117; TypeScript final aprovado; build Vite/React isolado final20,529s, sem envFile/configFile nem sobrescrever dist. `git diff --check` aprovado antes do checkpoint. O runtime da planilha continua grande, conforme medição da entrega anterior.

Prévia EXPLÍCITA na composição real LocalSpreadsheet/Sheets70 com transporte somente em memória e GET obrigatório,127.0.0.1:4333. Usuário employee de teste com apenas manage_stock; demais fontes desabilitadas. Local inicialmente vazio bloqueou Prévia. Após escolher depósito,51posições consultadas; exemplo kg: físico1250,reservado500,livre750milésimos. Confirmação inseriu bloco; recarga redigiu dados persistidos e reconsultou a fonte, recuperando os valores, conforme regra70. Capturas planilha-estoque-previa-2026-10-22.png e planilha-estoque-restaurado-2026-10-22.png em outputs. Prévia não homologa serviço real. Não se percorreu toda matriz de idioma/tema/viewport/leitor de tela.

Cenário de regressão adicional em ?profile=late: corpo403da consulta antiga troca para second-preview-user e segunda sessão descartável antes de concluir. Nova identidade permaneceu autenticada, workbook vazio/separado e sem reapresentar dados do operador anterior; console final sem erro. Captura planilha-sessao-tardia-2026-10-22.png. Um primeiro ensaio do transporte tinha referência sombreada de location; isso gerou falha na própria prévia, foi corrigido para window.location e o cenário repetido antes de anunciar resultado. Falha do seletor por getByLabel na automação foi resolvida pelo controle nativo observado, sem alterar o produto. Uma busca de caminho work executada no diretório do repositório retornou No such file or directory; corrigido o diretório do laboratório, sem efeito em fontes.

Laboratórios de4332e4333encerrados após a verificação. Evidências, hashes e preimagens privados em work/sheet-stock-20261022; não incluir dependências/builds desses laboratórios em entregas. Houve aviso do helper Git de lock de credenciais read-only no push anterior, mas o processo concluiu e o hash9280foi confirmado remotamente; não se alterou configuração de credenciais.

Pendências de integração: URL/build/schema/perfil/banco DESCARTÁVEL identificados pelo responsável, manage_stock e licença em estados distintos, locais autorizados/negados, volume500/501e mudanças concorrentes, revogação durante corpo, reabertura de blocos e limites do servidor. Não iniciar backend escolhendo banco, estação ou chaves. Fonte financeira e demais fontes70, compartilhamento, importação e aceites completos continuam parciais; não afirmar conclusão do frontend inteiro por esta consulta ou pelos1405testes. Prosseguir pelos contratos disponíveis e registrar dependências reais por fluxo.


## Entrega de22/10/2026 — obrigações a pagar na planilha e integridade dos vínculos

Base `d19715de9207da57a937fb73c2d504dc6f984ecc`, branch feat/backend-checkout-authority, árvore limpa na retomada. Regras relidas e AGENTS aplicáveis procurados (nenhum encontrado). Caminho real confirmado; nova autorização técnica seguida de teste efetivo criar/fsync/reler/remover temporário somente em frontend/docs. Preimagens conferidas antes de aplicar, sem divergência. Financeiro116 e conferência117 preservados; nenhum aplicador antigo, backend, Electron, banco, instalação ou ZIP.

A fonte Financeiro70 só oferecia demonstração por variável de ambiente. Agora se chama Obrigações a pagar e usa GET /local/v1/finance/payables/report.csv com o parser/cliente114 existentes. Não recria a agenda114 nem o agrupamento por fornecedor117. Requer autorização da fonte contábil (view_accounting e módulo accounting conforme regra70), sem conceder manage_cash. Não cria obrigação/pagamento/vínculo nem usa fila/idempotência de mutação. Ausência de endpoint/erro é indisponibilidade, sem fallback de demonstração. Tipo MIME CSV, versão2, escopo, período, moeda, métricas e somas passam pelas validações do contrato existente.

Datas from/to inclusivas, até366datas, e as_of explícito, inicialmente vazio; datas reais são validadas antes do pedido. as_of classifica atraso; aplicado e saldo são ATUAIS, não um corte histórico de pagamentos. Valores BRL em CENTAVOS INTEIROS, sem dividir por100nem arredondar. Colunas separadas para principal, aplicado ao documento e saldo aprovado; estado e classificação temporal têm código estável e rótulo nos cinco idiomas. IDs do documento/fornecedor/parcela, referência, vencimento, moeda e janela seguem junto às linhas. Resumo do servidor é validado, mas nunca transformado em parcelas extras nem somado novamente. Pendentes/recusados/cancelados conservam principal e saldo aprovado0conforme contrato, sem interpretação de pagamento.

Busca por fornecedor/referência documental filtra o relatório integral VALIDADO antes do limite da planilha. Mais de500parcelas correspondentes é recusa explícita: refinar busca/período; nunca truncar silenciosamente. Limites422do servidor exigem período menor, sem resultado parcial. Consulta fonte pode conter até10000parcelas/1000documentos locais e4MiB segundo o contrato; não se apresenta isso como financeiro empresarial completo, DRE, confirmação bancária ou cobertura de pagamentos externos desconhecidos.

Sessão, quatro IDs, token, permissão, módulo, aborto, expiração e corpo são conferidos nas fronteiras da leitura. Alterar período, referência, busca ou fonte invalida/aborta a prévia anterior. Proteção já adicionada contra erro de sessão antiga é mantida. Atualização de vínculo também confere o workbook original e presença do mesmo bloco antes de escrever ou publicar erro; uma troca de documento não aceita resposta antiga. Essa corrida de documento foi revisada no código e passou TypeScript/build; não foi exercitada como corrida completa de troca de arquivo na interface nesta unidade.

### Texto externo, erros e preservação

O código instalado do Univer core/facade confirmou que setValues converte strings iniciadas por=em fórmulas e pode interpretar strings numéricas. Inserção/atualização de blocos consultados agora fornece células explicitamente tipadas (texto/número/booleano), sem fórmula ou rich text anteriores. Nome de fornecedor, SKU, IDs e referências não viram fórmulas; zeros à esquerda são preservados. Fórmulas que o usuário digita continuam habilitadas. Isso se aplica aos blocos das fontes, sem reescrever toda a biblioteca/rotas nem o117.

A verificação visual encontrou erros de atualização que só eram renderizados dentro do diálogo fechado. Agora cada bloco tem seu erro visível fora do diálogo; sucesso retira apenas o erro daquele bloco. Troca de contexto/arquivo limpa os erros pertinentes. Recusa501fica explícita ao lado da planilha, sem inserir500linhas como se fossem tudo. Em uma reabertura os valores vinculados são redigidos e reconsultados; uma fórmula manual SUM de células ainda vazias pode mostrar0durante falha/espera. Isso não é saldo oficial nem consulta bem-sucedida; o aviso de fonte indisponível/limite deve ser respeitado. Não se atribui persistência comercial ao IndexedDB ou a essas fórmulas.

### Arquivos e validações reais

Novos em frontend-web/src/core/sheets70: financeSnapshot.mjs/.d.mts, financeWords.mjs/.d.mts, literalData.mjs/.d.mts; novo tests/sheetFinanceSnapshot.test.mjs. Alterados: core/sheets70/finance.ts, contracts.ts, model.d.mts; modules/tenant/pages/Sheets70.tsx, LocalSpreadsheet.tsx, spreadsheet.css; este registro. Fonte de backend somente lida: zzeus-frontend-backend-b02a/docs/api/supplier-payables.openapi.json (report.csv). Não se alterou cliente/parser114 nem arquivos117.

39testes novos aprovados: consulta GET/autorização, datas/referência,366datas, relatório vazio validado,500/501e busca após validação integral, valores atéMAX_SAFE_INTEGERem centavos, quatro estados, quatro IDs, logout/corpo atrasado, permissão/módulo/expiração, abort, clone, HTTP401/403/404/409/422/503, MIME, CSV inválido, cinco dicionários completos e dados externos literais. Suíte final1444aprovados,0falhas,0ignorados; regressões116/117 incluídas. TypeScript final aprovado; build Vite/React isolado20,510s (envFile/configFile desabilitados; dist preservado); diff --check conferido antes do checkpoint. Runtime da planilha ainda grande; não houve benchmark no hardware alvo.

Prévia EXPLÍCITA em127.0.0.1:4334, composição real LocalSpreadsheet/Sheets70, dados descartáveis, transporte que recusa qualquer método diferente deGET. Usuário employee apenas com view_accounting, licença de teste expirada permitindo leitura conforme regra existente; sem permissão de escrita financeira. Campos vazios impediram consulta. Datas01–31/10/2026, referência20/10; duas parcelas de10001centavos,1aplicado e10000de saldo cada. Nome de fornecedor=1+1permaneceu texto na grade, sem virar2. Fórmula manual SUM(I2:I3)=20002. Reabertura reconsultou dados e manteve a fórmula. Cinco idiomas percorridos, claro/escuro observados; atualização em inglês localizou o vínculo. Não se testou toda combinação idioma/tema/viewport/leitor de tela.

Cenário501confirmou aviso visível com diálogo fechado, sem truncagem. Cenário de resposta403da tentativa anterior trocou para segunda identidade/sessão antes da resposta: nova sessão permaneceu autenticada com workbook separado. Nenhum erro de console no circuito final. Capturas em outputs: planilha-financeiro-centavos-2026-10-22.png, planilha-financeiro-limite-2026-10-22.png, planilha-financeiro-sessao-2026-10-22.png. Somente dados descartáveis.

Limites da automação corrigidos durante a verificação: fill das datas alterava o DOM, mas não o estado React e a consulta continuava desabilitada; não foi considerado êxito. Entrada pelos segmentos nativos confirmou as três datas e habilitou a consulta antes do GET. A espera inicial do aviso501expirou porque ele estava oculto no diálogo; isso levou à correção de erros por vínculo e ao ensaio repetido. O primeiro transporte403aguardava mudança no método text, mas o cliente corretamente recusa HTTP antes de ler o corpo; transporte corrigido para mudar sessão antes de devolver o403e o cenário foi repetido. As abas descartáveis adicionais foram encerradas antes do ensaio final de reabertura, evitando concorrência de autosave no mesmo documento de laboratório.

Evidências/preimagens/hashes/logs privados em work/sheet-finance-20261022. Não incluir builds, node_modules ou esses laboratórios em pacotes. Serviço de prévia encerrado após verificar. Não iniciar backend/selecionar banco/estação/chaves.

Aceite integrado pendente: responsável fornecer URL, build, schema, perfil e banco DESCARTÁVEL identificados. Validar report.csvformato2, view_accounting com/sem módulos/licença, limites1000/10000/4MiB, contagem500/501, concorrência de saldo, referência vs instante atual, revogação durante resposta/corpo, troca de documento, restauração de blocos e isolamento por identidades. Contas a receber, fluxo de caixa completo, DRE/centros de custo/OFX, acesso externo do contador e demais fluxos comerciais/logísticos/RH da matriz continuam sem conclusão/homologação. Não atribuir percentual ou conclusão global aos1444testes. Estoque e obrigações deixam de ser fontes demonstrativas na planilha; demais fontes e recursos do editor ainda exigem revisão e aceites próprios.


## Entrega de22/10/2026 — gestão cadastral e estado operacional do produto

### Base e escopo

Base limpa observada `9acda306c7b5fe5b2c28376220fa8f19c5fafbfa`, branch `feat/backend-checkout-authority`, pasta principal confirmada. Permissão efetiva de escrita conferida por arquivo temporário exclusivo com fsync/leitura/remoção nas pastas autorizadas. Preimagens de LocalCatalog e deste registro conferidas; nenhuma substituição de cópia antiga. Frontend116/117 preservados, backend/Electron não alterados, sem dependências/ZIP novo. Publicação de checkpoint autorizada pelo pedido anterior do proprietário para salvar no GitHub.

Novos arquivos: `frontend-web/src/core/catalogManagement/{model.mjs,model.d.mts,source.mjs,source.d.mts,words.mjs,words.d.mts}`, `frontend-web/src/modules/catalog/pages/{CatalogManagement.tsx,productManagement.css}` e `frontend-web/tests/catalogManagement.test.mjs`. Integração localizada em `frontend-web/src/modules/catalog/pages/LocalCatalog.tsx`, mantendo busca/criação/rota de estoque existentes. Cabeçalho da matriz de produção corrigido para não repetir como faltantes quatro unidades já entregues na revisão8.

### Comportamento e contratos

- `/local/catalog` → Gerenciar: consulta cadastro autorizado, revisão cadastral independente do estado, SKU/nome/código/unidade/preço/custo, comparação antes/depois e confirmação humana. Valores em centavos exatos, inclusive limite seguro; não transforma edição de unidade em conversão de saldo. Dependências comerciais/históricas podem impedir a mudança no servidor.
- Estado global da empresa: ativar/inativar com motivo e revisão esperada, histórico de50porpágina, loja/operador/aparelho/data/ID original. Aceita eventos legítimos de outras lojas da mesma empresa. Inativação não apaga estoque nem cancela documentos antigos.
- GET `catalog/products/{id}/edit-details`, GET/POST `.../state`, POST `.../update`, GET `.../state-history`, GET `catalog/operations/{edit|state}/{operation_id}`. Contratos e implementação lidos somente na referência separada: `catalog-edit`, `catalog-product-state`, `catalog-product-state-history`, `catalog-operation-receipts`. Não foi testado serviço real nem copiado backend.
- GET gerencial exige manage_stock; estado pode ser lido com view_catalog. Escrita exige permissões, licença core/inventory e locks. Não inventar requisito view_accounting para o custo gerencial explicitamente autorizado pelo contrato. Licença vencida não bloqueia leitura histórica autorizada.
- Fila durável única por empresa/loja/usuário/aparelho. Verifica armazenamento, recibo original e snapshot integral antes de POST; bloqueia consulta antiga após logout/contexto/permissão/licença alterados. Resultado incerto não é descartado nem reenviado automaticamente. Retentativa explícita conserva o mesmo ID/payload; revisão atual ou cadastro parecido não substituem recibo original. Confirmado só depois de validar recibo correspondente.
- Rascunho conserva sua base durante atualização; confirmação limpa apenas campos correspondentes. Diálogos da interface protegem fechar produto, trocar produto, restaurar consulta e navegar internamente; beforeunload protege fechamento externo. Mudança de token ou quatro identidades remonta o conteúdo e não conserva tela de outro contexto. Cinco dicionários, estilos por variáveis de tema, controles de44px e foco de teclado na área nova. Os textos antigos de busca/criação ainda têm português fixo e não são declarados traduzidos nesta unidade.

### Verificações e limites reais

-47testes novos aprovados: revisões independentes, exatidão monetária/UTF8, edição/estado, recibo anterior, perda de resposta, recuperação sem POST, retentativa explícita, quatro identidades/logout/corpo tardio/revogação, locks/storage corrompido/falha na remoção, dependência de unidade409, histórico global e cinco dicionários.
- Após os ajustes finais: `node --test tests/*.test.mjs`:1491aprovados,0falhas,0ignorados; TypeScript `--noEmit --project tsconfig.json`:exit0; Vite/React existentes com configFile:false/envFile:false e saída isolada:build aprovado em21,192s. Dist do usuário preservado. Aviso de chunks maiores que500kB permanece; não é medição de runtime nem homologação comercial.
- Prévia explícita descartável, sem proxy/backend/banco: interface real abriu cadastro, revisou nome e12,34→19,99, confirmou1POST e recibo/revisão1; inativação perdeu resposta após segundoPOST, mostrou fila incerta/descarte bloqueado; consulta do recibo recuperou inactive/revision1 sem terceiroPOST e preservou revisão cadastral1. Motivo correspondente limpo e histórico original exibido.
- Limitação visual: confirmação nativa de saída bloqueou automação do navegador (`Timed out running CDP command "Input.dispatchMouseEvent"`, depois `Emulation.setFocusEmulationEnabled`). Foi substituída por diálogo da própria interface. TypeScript/testes/build posteriores passaram; o circuito visual final desse diálogo, cinco idiomas/temas, perfil restrito e conflito de unidade não foi concluído no navegador. Não tratá-lo como aceite executado. Screenshot da tela inicial da nova área salvo nos outputs desta tarefa; não prova esses cenários pendentes.
- Erros de preparação: leitura relativa do manifesto de trabalho a partir da raiz do projeto produziu `No such file or directory`; retomado caminho correto. Parser de dicionários na preparação do diálogo retornou JSONDecodeError antes de qualquer escrita daquela tentativa; corrigido fechamento e aplicado script revisado. Não houve perda do produto.

### Homologação ainda necessária

Backend deve fornecer commit/versão dos quatro contratos, URL/perfil explícitos, banco NOVO descartável, identidades fictícias e permissões/licenças mínimas, sem banco comercial. Ensaiar revisão concorrente entre lojas, edição direta legada com mesma revisão, unidade com histórico/receita/ordem, reativação, histórico com mais50eventos e outra loja, autorização de custo, reinício após perda de resposta, recibo incompatível, sessão revogada e armazenamento/locks indisponíveis. Não iniciar serviço automaticamente.

Esta unidade não conclui todo o ERP. As pendências por fluxo da matriz continuam abertas onde indicado; manutenção de catálogo não substitui pagamentos/fiscal, circuito interempresa, RH, offline/móvel ou homologação desktop. Próxima continuidade deve revisar textos/criação antigos do catálogo e integrações de outros fluxos com contrato, sem duplicar117 nem as unidades de produção já aplicadas. Evidências, hashes e logs privados em `work/catalog-management-20261022/` desta tarefa; reversão só localizada comparando alterações posteriores, sem restauração geral.


## Entrega de23/10/2026 — concorrência nas atualizações de blocos da planilha

Prioridade recebida do proprietário após revisão externa dos commits d19715d/9acda30: impedir consulta anterior de sobrescrever resultado mais novo do mesmo bloco e proteger filtros/posição. Base atual publicada `716a9e171b8a38aee07f207a5d52cfbbe0f78ddd`. A continuação de tradução/criação do catálogo está preservada em arquivos separados e não faz parte deste checkpoint de planilha. Não sobrescrever nem atribuir esses arquivos à correção.

### Alteração delimitada

Novos `frontend-web/src/core/sheets70/blockRefresh.mjs`, `blockRefresh.d.mts` e `frontend-web/tests/sheetBlockRefresh.test.mjs`; integração em `frontend-web/src/modules/tenant/pages/Sheets70.tsx`; um aviso novo nos cinco JSONs de `frontend-web/src/i18n/sheets70/`. Backend/Electron e contratos não alterados. Limites de500 do estoque/financeiro preservados.

Cada consulta de bloco possui identidade própria e cópia dos filtros/colunas/posição originais. Iniciar outra consulta do MESMO bloco aborta a anterior e a torna inválida mesmo se o transporte ignorar aborto. Somente o ticket atual pode escrever células, cache, atividade, confirmação de atualização ou erro. Uma falha antiga, inclusive403, não pode apagar a fonte nem invalidar sessão depois de resultado mais novo. Blocos diferentes mantêm consultas independentes.

Antes de aplicar, compara workbook, existência/vínculo do bloco e snapshot completo (fonte, filtros, colunas, folha, posição e dimensões), além das guardas de sessão/permissão existentes. Alterações estruturais, edição da origem, intervalo ou desvínculo invalidam a consulta pendente correspondente. Trocar/criar/reabrir documento, contexto/token ou desmontar invalida tickets, inclusive ao reutilizar mesmo ID do workbook. Os callbacks de editor/cache são os vigentes. Não há POST nem retentativa automática nesta correção de leitura.

Aviso visível fora do diálogo/detalhes, em cinco idiomas: blocos ainda não validados nesta sessão podem ter células vazias, zeros de fórmulas ou valores anteriores. Isso NÃO prova saldo zero nem atualidade do total. Nova consulta retira sua marca de validação; somente sucesso vigente a repõe. Falha mantém aviso e o erro específico do bloco. Reabrir ou trocar contexto/token perde a marca, sem desativar fórmulas ou transformar falha em zero comercial confirmado. Os avisos de recorte parcial continuam separados.

### Evidências e limites

-18regressões com Promises controladas: segunda resposta primeiro, primeira resposta chegando durante/apos a segunda, falha antiga403, última falha sem restauração por sucesso velho, independência entre blocos, filtros/local/colunas/posição/folha/dimensões/fonte/desvínculo, remoção, workbook diferente/mesmo ID reaberto, retorno da posição original após invalidação, aborto ignorado, finalização velha sem apagar ticket novo e snapshot independente da ordem das chaves.18aprovadas,0falhas.
- Suíte inteira na árvore corrente:1509testes aprovados,0falhas,0ignorados, incluindo116/117; TypeScript exit0; build isolado Vite/React aprovado em21,210s. Essas verificações também enxergaram os arquivos de catálogo em andamento, que não serão incluídos no commit desta correção. Não houve CI remoto confirmado, serviço real ou banco integrado.
- A hipótese foi convertida em regressão determinística do mecanismo utilizado pela tela; ainda não há reprodução visual concluída do circuito de duas consultas da planilha. Prévia própria explícita sem backend preparada com respostas controladas; navegador carregou editor/diálogo, porém edição de datas não liberou a ação na automação. Não declarar esse ensaio concluído. Confirmação nativa pendente do ensaio anterior já limitava ações do navegador.
- Preparação: pesquisa inicial usou glob inexistente (`No such file or directory`), corrigida por rg --files. Quatro casos iniciais de teste repetiam uma mutação idempotente na segunda consulta e por isso esperavam incorretamente invalidá-la; corrigidos para fixtures novas, cobrindo mudança real em ambos os caminhos. TypeScript inicialmente detectou três imports/variáveis não usados na preparação paralela do catálogo; corrigidos sem incluir catálogo neste checkpoint. Prévia teve resolução de i18next/react-i18next corrigida por aliases para dependências existentes, sem instalar. Porta4336 ocupada ao reiniciar (`Error: Port 4336 is already in use`); não encerrou processo desconhecido, usou4337 somente para prévia própria.
- Hashes/preimagens/logs e laboratório em `work/sheets-concurrency-20261023/` desta tarefa. Reversão somente localizada, comparando trabalho posterior. Sem ZIP, instalação ou alteração de dados comerciais.

### Pendências de aceite

Concluir circuito visual com atualizações manuais/automáticas intercaladas, filtros/posição/desvínculo durante leitura, falha403antiga e reabertura com fonte indisponível. Depois, ambiente integrado exige versão correta do backend, URL/perfil explícitos, banco novo descartável, identidades/lojas/permissões fictícias e autorização do proprietário. A ausência desse ambiente não suspende as demais unidades independentes de frontend. Não afirmar frontend completo nem CI verde com base apenas nestes testes.


## Entrega de23/10/2026 — conclusão da revisão de busca/criação do catálogo

Continuação preparada na base716a9e1, preservada durante a correção prioritária da planilha c658274 e agora concluída como unidade separada. Arquivos: `frontend-web/src/core/catalogManagement/creation.mjs`, `creation.d.mts`, extensão de `words.mjs`; `frontend-web/src/modules/catalog/pages/LocalCatalog.tsx` e `CatalogManagement.tsx`; `frontend-web/tests/catalogCreation.test.mjs` e este registro. Não alterou fontes de estoque/financeiro da planilha, backend ou Electron.

- Busca, filtros, tabela, resultados/vazio, colunas, paginação, criação, mensagens e unidades visíveis do catálogo usam os cinco dicionários. Mantidos dados reais da API e os limites existentes. A matriz deixa de listar esses textos como pendência. Isso não comprova revisão linguística nativa ou acessibilidade completa.
- Criação continua no endpoint legado `/local/v1/products`, via localClient existente. Novo adaptador protege contexto/quatro identidades, expiração, permissões view_catalog/manage_stock e módulos core/inventory antes de enviar e antes de confirmar o retorno. Valida UTF8 e centavos inteiros, clona o payload e impede submissão concorrente nesta instância. Callback antigo não invalida sessão nova nem apresenta sucesso/limpa formulário novo. Não amplia papel/permissão.
- Valores decimais da criação usam a mesma conversão exata do editor novo. Campos só são limpos após confirmação válida na mesma sessão; custo/unidade retornam aos padrões. Campos de criação passam a participar da proteção de navegação interna/beforeunload, sem fazer seleção/fechamento de outro produto descartar o formulário de criação. Os dois fluxos continuam distintos.
- O endpoint legado de criação NÃO fornece recibo idempotente. Falha/perda de resposta não dispara reenvio automático nem sucesso fictício. Campos permanecem; instrução visível manda consultar SKU antes de repetir. Não promete fila durável, proteção global entre abas ou recuperação por ID para essa criação. A edição/estado usam a fila/recibos próprios da revisão17 e permanecem preservados.

Verificações finais:14testes novos de criação aprovados (incluindo quatro identidades, logout, revogação/licença/módulo/expiração, sucesso tardio, perda de resposta sem retry, concorrência/clonagem e limites UTF8/dinheiro);61junto dos47da gestão. Suíte completa `node --test tests/*.test.mjs`:1523aprovados,0falhas,0ignorados, incluindo18da concorrência da planilha e116/117. TypeScript exit0; build isolado existente Vite/React:21,279s, sem alterar dist do usuário nem ler .env. Aviso de chunk grande permanece. `git diff --check` sem erros. Não houve ensaio visual completo posterior à alteração, CI confirmado ou integração real; as limitações visuais registradas nas revisões17/18 continuam pendentes.

Prévia própria de planilha4337 e catálogo4335 encerradas por Ctrl+C com exit130; não encerrou serviço/backend desconhecido. Laboratório/evidências em `work/catalog-completion-20261022/`, iniciado no dia22 e concluído no dia23. Reversão localizada por hashes, sem apagar/restaurar a árvore. A conclusão desta unidade não significa conclusão de todo o frontend: a matriz mantém pagamentos/fiscal, circuito interempresa, RH, offline/móvel e demais integrações/aceites delimitados.


## Continuidade aceita em23/10/2026 — ordem e critérios da próxima entrega

O proprietário consolidou17frentes, preservando o código existente e exigindo conferência antes de acrescentar: PDV; abertura/fechamento; faturamento/contas a receber; financeiro; compras/fornecedores; logística/docas/recebimento; trocas/devoluções; estoque/inventário; receitas/produção/qualidade; fiscal/documentos; contador; RH/acessos; clientes/comercial; planilhas; configurações/contexto/integrações; notificações/visão do dono; validação geral do frontend desktop. Essa lista NÃO redefine todas as funções como ausentes.

Ordem atual: concluir verificações das últimas entregas → integrar logística/recebimento com contratos disponíveis → faturamento/contas a receber → PDV/fechamento → demais áreas. Aplicativo completo de funcionário/motorista, GPS ao vivo, venda móvel independente, P2P avançado e novos segmentos ficam em etapas próprias, sem serem anunciados como operacionais.

Preservar especialmente: CPF fiscal separado de clube; meios mistos/cotação autoritativa/troco depois de confirmar; venda, pagamento e fiscal distintos; sessões de vendedores separadas de gavetas; dinheiro físico/contagem cega/OCR revisável e correlação não duplicada de parcelas; livro/malote/custódia; faturamento parcial, recebimento antecipado e cobrança incerta com IDs originais; projeção versus realizado e recortes versus totais; aceite humano/fornecedor e recusas configuráveis, fornecedores externos manuais e versões do pedido; agenda não garante chegada e completar doca não recebe estoque; troca aprovada antes da coleta e chegada à origem distinta; inventário conforme política central, lotes/validade/rastreio e custos separados de quantidades; produção existente, insumo limitante/disputa simultânea/perdas/retrabalho/qualidade e custo estimado versus realizado; fiscal/contabilidade somente pelos contratos reais; RH funcional separado de credencial e avaliações privadas; indicadores com origem/período/cobertura/porte comparável; permissões, contexto, cinco idiomas, temas/teclado/touch e estados reais por integração. O restante da matriz permanece referência de pendências específicas; não expandir poderes para demonstrar telas.

## Verificação visual concluída em23/10/2026 — planilha e catálogo

Base `2ed977bd27336a39a0758d5b2fc57d87a7dc4845`, árvore principal e branch confirmadas, inicialmente sem alterações. Não foi repetida auditoria geral nem implementado117outra vez.

- Planilha real, prévia EXPLÍCITA sem backend/banco: inseriu bloco com2parcelas; reteve a primeira atualização de10.001centavos; segunda respondeu30.003; liberou primeira depois e manteve I2=30003. Header de laboratório mostrou2consultas e nenhuma ainda retida. Isso conclui o ensaio visual principal de resposta fora de ordem.
- Filtro: durante outra primeira consulta retida, editou a origem e consultou NF-1; confirmou vínculo com apenas DOC1/PART1/30.003. Liberar resposta antiga não repôs DOC0 nem o filtro anterior.
- Posição: com leitura pendente, inseriu linha antes do bloco. Liberou resposta antiga, manteve valor30003 no novo endereço I3 e o aviso de validação pendente. Nenhuma atualização escreveu no endereço antigo.
- Reabertura: documento salvo reaberto com422da fonte; células vinculadas não reidratadas, aviso externo de não validação/zeros e erro financeiro de limite visíveis. Avisos lidos em pt-BR/en/es/fr/zh-CN; temas escuro/claro selecionados. Não foi adicionado saldo fictício nem declarado período completo. Combinação de atualização automática/manual, falha403antiga visual e hardware limitado continuam sem aceite específico; regressões automatizadas cobrem cancelamentos/erros antigos.
- Catálogo: editar nome, fechar e navegar internamente abriu diálogo comum. Escape/Voltar mantiveram “Rascunho preservado”, com0POSTs; recarga exigiu descarte explícito e voltou ao snapshot. Cabeçalhos principais e gestão percorridos nos5idiomas; temas claro/escuro. Revisão kg→g recebeu409da prévia; mensagem de conflito e confirmação bloqueada, sem forçar substituição. Perfil restrito manteve consulta de estado,0campos gerenciais e nenhum custo/histórico ou fila expostos. Criação completa, reativação e perfil de licença vencida não receberam novo ensaio visual nesta rodada; testes isolados continuam como evidência delimitada.
- Capturas em outputs desta tarefa: `planilha-concorrencia-validada-2026-10-23.png`, `planilha-filtro-concorrente-2026-10-23.png`, `planilha-posicao-concorrente-2026-10-23.png`, `planilha-reabertura-aviso-2026-10-23.png`, `catalogo-rascunho-protegido-2026-10-23.png`. Não contêm dados comerciais. Formato de data teve atualização React concluída por teclado, sem alterar código do produto para simular sucesso.

## Entrega de23/10/2026 — agenda operacional de docas

### Fonte e arquivos

Contrato lido somente na referência separada: `docs/api/dock-agenda.openapi.json`, versão2026-10-22.1, SHA-256 `1a4656355135a8b75256888bc0c26af22b79963c868e6420856079112713025e`; implementação `backend/internal/localapi/dock_agenda.go` e `backend/internal/localdb/dockagenda/{model,common,write,read}.go`, dependências de módulos consultadas. Também identificados, sem afirmar integração concluída: dock-insights, dock-utilization, receiving-overview, receiving-board, receiving-cases. A referência não foi alterada/aplicada e sua existência não comprova serviço ativo.

Novos em `frontend-web/src/core/dockAgenda/`: `schemas.mjs` (projeção tipada do contrato, sem servidor), `contracts.ts`, `model.mjs`/`model.d.mts`, `source.mjs`/`source.d.mts`, `words.mjs`/`words.d.mts`. Novos `frontend-web/src/modules/catalog/pages/DockAgenda.tsx`, `dockAgenda.css` e `frontend-web/tests/dockAgenda.test.mjs`. Integração localizada em `LocalOrders.tsx`, na rota existente /local/orders, sem duplicar menu/ERP. Recebimento oficial existente e116/117preservados. Diálogo comum de rascunho também reutilizado na navegação de Compras; não altera fila nem envia ao sair.

### Funções e limites

- Recursos: listar em páginas50; proprietário/gerente pode revisar criar/editar nome e disponibilidade com revisão esperada. Desabilitar com operações ativas continua sujeito à rejeição do servidor. Consulta mostra Sim/Não, sem checkbox editável fictício.
- Reserva usa o pedido selecionado nos detalhes de Compras, com fornecedor derivado no servidor; IDs próprios para operação/agendamento. Janela5min..8h, datas no fuso visível do navegador e revisão também com UTC; consulta [from,to) de até31dias. Nada confirma fornecedor ou promete horário de chegada.
- Reagendar somente reservado; chegada, início e término de descarga, cancelamento reservado/chegou e não comparecimento depois do fim planejado. UI considera a hora consultada do servidor para oferecer não comparecimento; o servidor revalida. Revisão esperada/snapshot conservados. Nenhuma expiração libera automaticamente recurso/unidades.
- Lista de agendamentos, detalhe com diferença de chegada assinada, espera/descarga em segundos (null = desconhecido), ocupação real INDEPENDENTE do período, espera por doca e histórico paginado de eventos/operação/ator/aparelho/motivo. Descarga de dia anterior pode continuar bloqueando outro início. Concluir descarga não dá entrada em estoque, não paga e não confirma recebimento comercial.
- CSV é cópia da PÁGINA validada, com intervalo e campos originais, texto protegido de fórmulas; não é exportação de todos os registros nem gera arquivo no servidor. Recorte e total consultado explicitados. Usa a leitura JSON existente, sem inventar um contrato de escrita.
- Escrita exige view_orders+manage_stock, licença active e módulos core/inventory/orders/logistics; recursos exigem owner/manager. Leituras históricas autorizadas não exigem renovação. Proteção entre abas e leitura/gravação/conferência de storage antes de POST. Troca de qualquer identidade, token/contexto/capacidades, expiração ou revogação bloqueia tentativa antiga. Sem ampliação de papel.
- Fila única de docas por quatro identidades, separada de compras/recebimento existentes. Recibo original consultado antes de enviar; confirmação exige resultado original e evento correspondente, incluindo request_json integral, ator e aparelho originais. POST é seguido de nova conferência. Resultado incerto não é apagado nem reenviado automaticamente; reenvio explícito conserva ID/payload.404no histórico depois de recibo existente NÃO é interpretado como ausência da operação. Revisão atual não substitui o recibo anterior. Recuperação por evento além do offset máximo1000000permanece bloqueada com aviso próprio, preservando a tentativa.
- Cinco dicionários, semântica nativa de campos/tabelas, focos, botões44px e tema pelas variáveis existentes. Formulário guarda sua base, exige revisão humana e descarte explícito; fila persistida não some ao descartar campos. A fonte exige consistência de datas/estado/contadores, ordenação UTF8 como SQLite e IDs corretos por espécie de página.

### Verificações reais

-49testes novos aprovados: recursos/revisões, reserva e todas as transições; provas originais após estado posterior; resposta perdida/recuperação sem POST e reenvio explícito; contexto em4campos/logout/corpo tardio/revogação; storage/locks/falha na remoção;401/403/409e limite; recibo/evento/ator/aparelho/JSON incompatíveis; leitura histórica com licença vencida; ocupação atrasada, cancelamento congelando espera; datas inválidas/UTC/nanos/janelas/UTF8/CSV; filtros, duas reservas na mesma doca e ordenação Unicode. Suíte completa após correção:1572aprovados,0falhas,0ignorados, incluindo116/117. TypeScript exit0. Build isolado Vite/React existentes aprovado em21,159s, configFile:false/envFile:false; dist preservado, aviso de chunks grandes continua.
- Na prévia EXPLÍCITA da agenda (sem backend/banco), consultou recursos/ocupação/período, revisou reserva ORDER2, simulou aplicação com resposta perdida, mostrou tentativa incerta/descarte bloqueado e recuperou por recibo+evento original com apenas1POST. IDs na revisão/fila/recibo mantidos. Não é prova de persistência real SQLite, conflito transacional ou permissão no serviço.
- O ensaio visual seguinte foi bloqueado por confirmação nativa de descarte da prévia. A API de diálogo retornou undefined e chamadas de foco expiraram; foi solicitada liberação ao proprietário sem interromper verificações independentes. Controles novos de descarte/navegação passaram a reutilizar o diálogo comum já testado do catálogo. Ainda NÃO concluídos na interface final: circuito chegar→iniciar→concluir, reagendamento/cancelamento/no_show, perfil worker/expired/denied, todos os idiomas/temas desta agenda, download CSV e ligação visual com a seleção real de Orders115. Não anunciar esses ensaios como executados.
- Erros encontrados ANTES da publicação: runner Node dentro do sandbox contou só arquivos; execução direta e runner fora do sandbox confirmaram casos reais. Prévia inicialmente omitiu remaining_days das capacidades e foi corrigida, sem mudar permissões do produto. Teste de ordenação encontrou falsa duplicidade ao usar dock_id em vez de appointment_id; corrigido e acrescentada regressão de dois agendamentos da mesma doca, com49testes e suíte final aprovados. Porta4337ocupada/EPERMno sandbox; não encerrou serviço desconhecido, prévias próprias4340/4341/4342sem backend. Sem dependências, ZIP, banco, migração ou Electron alterados.

### Continuidade e homologação

Preparação, hashes/preimagens, contrato projetado e logs em `work/dock-agenda-20261023/` desta tarefa. Antes da aplicação: HEAD2ed977b/caminho real e hashes de LocalOrders/registro conferidos; arquivo temporário exclusivo confirmou fsync/leitura/escrita efetiva; somente12fontes/testes foram aplicados, além deste registro. Reversão localizada por comparação de hashes, sem checkout/reset/clean/restauração geral. Salvamento na branch existente segue autorização anterior do proprietário; sem merge/main/force-push.

Próxima unidade: visão de recebimento por pedido e painel agrupado, depois tratativas e indicadores/utilização, usando os contratos já identificados. Não recriar recebimento anterior nem confundir andamento administrativo com acordo, reembolso ou estoque. Em seguida, faturamento/contas a receber, PDV/fechamento e demais frentes da lista consolidada. Não afirmar conclusão total do frontend.

Ambiente integrado continua NÃO autorizado/configurado. Exigir versão/commit compatível com contrato, URL/perfil explícitos, banco NOVO descartável, empresa/loja/aparelho/operadores fictícios, permissões e licenças mínimas. Ensaiar corridas de reserva, janela adjacente com descarga atrasada, um agendamento ativo por pedido, revisão concorrente, order_cancelled/fornecedor inativo, não comparecimento segundo hora do servidor, reenvio com payload divergente, recuperação após reinício e isolamento entre quatro identidades. Não iniciar backend, aplicar ZIPs ou escolher banco/estação/chaves por conta própria.


## Entrega de23/10/2026 — conferência por pedido e painel agrupado de recebimentos

Base confirmada `3556a2cdb86f1b338c2a4d631dd9ad5d3d26b259`, branch `feat/backend-checkout-authority`, árvore inicialmente limpa em `/home/danillo-malta/Downloads/zzeus-frontend-backend`. Caminho real, hashes de LocalOrders/registro e escrita efetiva fsync/leitura em arquivo temporário exclusivo conferidos antes da aplicação. Não executou aplicadores antigos nem recriou o117. Backend e Electron não alterados.

### Contratos, arquivos e comportamento

Referência somente leitura no backend separado: `receiving-overview.openapi.json` e `receiving-board.openapi.json`, versão2026-10-22.1; SHA256 respectivamente `fa122d1c23f68705da0f21a7ccceb1571544536a0253e63ac5fb2ff9748919a9` e `fc8d1f8b104b158efa8da39786dcd44f9fea464014d80e3e908b4ea64b2f8702`. Implementações receiving_overview/receiving_board/receiving_trace e snapshots de docas consultadas; API existente no código não prova serviço atualizado.

Novos em `frontend-web/src/core/receivingOverview/`: schemas.mjs (projeção somente leitura), contracts.ts, model.mjs/model.d.mts, source.mjs/source.d.mts e words.mjs/words.d.mts. Novos `frontend-web/src/modules/catalog/pages/ReceivingOverview.tsx`, receivingOverview.css e `frontend-web/tests/receivingOverview.test.mjs`. Integração localizada em LocalOrders.tsx, na rota existente /local/orders. Este é o único registro de continuidade.

- GET `/local/v1/purchase-orders/{id}/receiving-overview` mostra previsto, entregue/aceito efetivos, recusado parcial, restante, recusas integrais e aceito estornado históricos separados; autorização humana original, revisão por avisos e agenda paginada. Entradas estornadas não retornam ao saldo efetivo. Histórico não representa reembolso ou nova entrada. Até1.000evidências por pedido; paginação50 na agenda. IDs legados com espaços internos e escape de URL preservados.
- GET `/local/v1/management/purchases/receiving-board` consulta COORTE POR CRIAÇÃO de pedido [início,fim exclusivo), até366dias; fornecedor/produto antes dos limites e estado após validação integral. Até200pedidos/2.000evidências totais/1.000por pedido;422recusa a consulta inteira. Grupos cobrem TODOS os pedidos correspondentes; a tabela de pedidos mostra uma página50. Contadores de coorte/evidência anteriores ao filtro de estado são distintos de correspondentes/página. Pedidos cancelados mantêm restante histórico, excluído da quantidade pendente ativa. Nunca somar unidades diferentes.
- Quantidades de grupo são strings decimais exatas validadas/calculadas com BigInt, inclusive acima de MAX_SAFE_INTEGER. Por pedido, inteiros seguros em milésimos com aritmética e estado conferidos. Ordenação UTF8/nanos e identidade/paginação/filtros/avisos coerentes, sem converter texto em fórmula.
- Consultas exigem view_orders+manage_stock e aparelho/contexto autorizados; leitura histórica não exige renovação de licença. Contexto/token/capacidades/expiração revalidados antes/depois da resposta/corpo. Canais independentes para painel e detalhe; consulta mais recente do mesmo canal prevalece mesmo com transporte que ignore abort. Resultado ligado à assinatura de pedido/filtro/página/atualização; resposta antiga não reaparece sob uma seleção nova.
- Nenhum POST, fila ou comando novo: não apaga tentativas incertas das outras fontes, não dá entrada, paga, confirma fornecedor ou libera doca. Conferir pedido no painel NÃO muda a seleção operacional de Orders115; ajuda orienta abrir o mesmo pedido na lista para receber/agendar. Atualização explícita após operação, medida do snapshot visível; não promete leitura em tempo real.
- CSV da consulta validada: grupos completos + pedidos da PÁGINA, ou resumo + agenda da PÁGINA. record_type e aviso evitam somar tipos entre si; quantidades exatas em milésimos e proteção contra prefixos de fórmula. Não é histórico completo/exportação do banco. Downloads revalidam sessão. Cinco idiomas, temas existentes, controles44px, foco e rolagem de painel/tabela; revisão visual completa ainda pendente.

### Evidências e limites de validação

47testes novos aprovados: GETs reais do adaptador com transporte descartável; quatro mudanças de identidade/logout/revogação/expiração/permissão; resposta fora de ordem/erro antigo/canais independentes/dispose;422/limites e filtros UTC366dias; aritmética, autorização, estornos/recusas separados, contagem/paginação de51agendamentos, duplicidade e inconsistência de agenda; grupos globais versus página2, unidades separadas, total acima do inteiro seguro, nanos/intervalo exclusivo, CSV e cobertura dos5dicionários. Suíte completa final após as4regressões adicionais:1619aprovados,0falhas,0ignorados, incluindo financeiro116 e conferência117. As47da unidade também foram executadas separadamente e passaram. TypeScript exit0. Build isolado Vite/React em20,646s, configFile:false/envFile:false, dist do usuário preservado. Aviso de chunk grande da planilha permanece; não é redução de tamanho do instalador.

Prévia EXPLÍCITA sem backend/banco: conferência ORDER renderizada com5,000kg previstos,3,000entregues,2,000aceitos,1,000recusado parcial,3,000restantes, autorização e avisos locais, temas pelas variáveis existentes. Captura `outputs/recebimento-conferencia-previa-2026-10-23.png` nesta tarefa. Fixture inicialmente sem remaining_days foi recusada pela validação real de capacidades e corrigida somente no laboratório. O bloqueio de cliques pela confirmação nativa anterior continua; consultar painel não produziu mudança, portanto NÃO comprovados visualmente consulta interativa/grupos, paginação/download, alternância dos5idiomas/temas ou circuito da seleção real. Não houve sucesso fictício nem backend iniciado para contornar isso.

Preparação/backup/hashes/contratos/logs em `work/receiving-overview-20261023/` desta tarefa; aplicação restrita a12arquivos de frontend e este registro. Reversão localizada por hashes/diff, sem restauração geral/exclusão. Próximas unidades continuam tratativas e indicadores/utilização de docas, depois faturamento/contas a receber e PDV/fechamento. Não afirmar conclusão das17frentes.

Ambiente integrado ainda pendente: commit/versão compatível com ambos os contratos, URL/perfil explícitos, banco NOVO descartável, identidades fictícias/permissões mínimas e autorização do responsável. Validar autorização→entrada parcial/recusa/estorno→overview, remanescente/estados/grupos, filtro de criação versus datas de entrega, limites200/2000/1000, mudanças concorrentes entre páginas e cancelado com agenda ativa. Conferir recibos sem reenviar operação de resultado incerto. Não iniciar serviço/aplicar ZIP de backend ou escolher banco/estação/chaves nesta tarefa.

Prévia de recebimentos4343 encerrada com Ctrl+C/exit130 ao finalizar. Prévias próprias de catálogo4341 e docas4342 também encerradas com exit130; a sessão da planilha4340 já retornou concluída com exit0. Não encerrou servidor de origem desconhecida.


## Entrega de23/10/2026 — integração do pacote de frontend Frota/Manutenção

Pedido do proprietário: adicionar os dois ZIPs recebidos; após CHECK do complemento de backend recusar a base, orientou seguir com frontend, revisar, corrigir problemas e testar. Base principal3013a4f0ed04fbbc5cb3164c3fc9de58e8ae639c, branchfeat/backend-checkout-authority, árvore inicialmente limpa. Arquivos/payloads e preimagens comparados antes de escrever; caminho real/acesso efetivo conferidos. Não foi criada interface no Electron.

### Origem, compatibilidade e preservação

- Frontend recebido: `TitanSystem_Frontend_Frota_Manutencao_20261023.zip`, SHA256 `9815847718085a297ab4daa66dbc69d6453e1ea5e944a17773b19e31e9ccb5c5`. Complemento backend recebido: `TitanSystem_Backend_Conferencia_Tecnica_Manutencao_20261023.zip`, SHA256 `da5f221c25ce81baf255feaa3823b8063eea39679e5ccb9f80743566a29feef4`. Ambos CRC e SHA256SUMS internos aprovados, caminhos/duplicidades/links conferidos; extração somente em diretório próprio de trabalho. Evidências incluídas no ZIP são relatos de sua origem; resultados abaixo foram executados nesta integração.
- Manifesto frontend com12arquivos, base2ed977b. Tipos/Fleet85 coincidiram com hashes anteriores; novos caminhos ausentes. `git apply --check` falhou em `frontend-web/src/modules/catalog/pages/LocalOrders.tsx:1` e `docs/architecture/CONTINUIDADE_TITANSYSTEM.md:757`. O payload antigo REMOVERIA DockAgenda/ReceivingOverview/diálogo atual. Patch não forçado: só imports/link de manutenção incorporados em LocalOrders atual, preservando116/117, docas, recebimentos, bloqueio de navegação e filas. Registro antigo do pacote não foi copiado por cima deste documento.
- Backup/preimagens/hashes e fontes recebidas em `work/fleet-packages-20261023-uflxpvk3/` desta tarefa. Arquivos exatos: `frontend-web/src/core/business85/types.ts`; novos `src/core/fleetMaintenance/{model.mjs,model.d.mts,words.ts}`; `src/modules/tenant/pages/business85/Fleet85.tsx`; novos `src/modules/tenant/pages/fleetMaintenance/{FleetMaintenance.tsx,fleetMaintenance.css}`; `src/modules/catalog/pages/LocalOrders.tsx`; testesfleetMaintenance/fleetMaintenanceWords; `docs/architecture/FRONTEND_FROTA_MANUTENCAO_20261023.md` e este registro.11fontes/testes/documento de módulo + atualização deste registro. Não incluir ZIPs, dependências, caches, build ou laboratório no Git.

### Funções efetivamente adicionadas

Quatro abas internas na área existente `/local/fleet`: Pedidos, Mecânica, Equipamentos e Custos por bem; link autorizado em /local/orders. Reutiliza arquivo local IndexedDBbusiness85, campo maintenance opcional; arquivos antigos permanecem compatíveis. Sem novo endpoint, envio a outro aparelho, push ou som remoto. O botão de áudio só testa o aparelho atual e não foi ouvido/validado fisicamente.

Pedidos têm bem, problema/local/prioridade, impedimento de uso e referências manuais. Avaliação assume responsável; gerente/dono autoriza reparo/liberação; reparo/espera/conferência e cancelamento/reabertura mantêm revisões/ator/histórico locais. Isso não é inspeção técnica do backend, assinatura qualificada ou prova inviolável. Bloqueio participa do despacho LOCAL; liberar um chamado não apaga outro impedimento. Não cancela viagem em andamento.

Equipamentos incluem máquina/empilhadeira/outros com identificação/local/horímetro. Custos e parâmetros gerenciais, depreciação linear/resultado parcial/impacto estimado e CSV exigem permissão financeira; escrita gerencial exige gerente/dono. Não há pagamento, consumo de peça, folha ou integração fiscal/contábil. Parâmetros ausentes permanecem ausentes. Campo/salvamento de rascunho não prova persistência no backend.

Correções encontradas nesta revisão: (1) total de custos fora do limite inteiro exato poderia lançar erro durante renderização; agora a linha mostra aviso em5idiomas, sem arredondar/zerar ou apagar registros, e exportação continua recusando cálculo inválido; (2) cabeçalho misto de próxima revisão usava horas também para veículo; agora tabela informa km no veículo e h no equipamento, com cabeçalho neutro nos5idiomas. Formulário de equipamento permanece em horas. Fonte recebida preservada para comparação.

Limites legados permanecem explícitos: workspace por empresa/loja/usuário/aparelho, sem compartilhamento interidentidades; sem transação/coautoria confiável entre abas. Revisão esperada é local. Não usar esta fila para anunciar monitor do mecânico compartilhado, nem aplicar automaticamente regras de checklist do backend ainda ausente. Não ampliou papel/permissão.

### Resultados executados

-43testes do pacote aprovados. Suíte completa final após correções:1662aprovados,0falhas,0ignorados, incluindo116/117/docas/recebimentos. TypeScript exit0. Build Vite/React existentes isolado em21.584s, configFile:false/envFile:false, dist do usuário preservado; aviso de chunk grande da planilha permanece.
-27verificações SSR com componente real e fixtures descartáveis:20composições (4abas×5idiomas),5erros de limite numérico sem queda da aba,1perfil financeiro negado e1tabela com km/h separados. SSR não substitui cliques/persistência/serviço real.
- Ensaio interativo com LocalBusiness85 real, origem própria127.0.0.1:4344, identidades fictícias/IndexedDB separado, sem proxy/backend. Abriu pedido bloqueante; avaliou→autorizou→iniciou reparo→encaminhou à conferência→liberou, preservando6eventos. Reabriu página/prévia e recuperou Liberado+6eventos. Não foi reinício completo do sistema operacional/app desktop.
- Registrou custo fictício10,01 com resumo/histórico10,01; cadastrou máquina100h/revisão120h e conferiu tabela junto do veículo1000km. Percorreu cabeçalhos de equipamento nos5idiomas e temas claro/escuro. Perfil fictício employee sem view_accounting viu aviso de permissão, sem tabela/campos de custo; Mecânica estava vazia nesse outro usuário, sem copiar a fila do gerente. Não testou quatro trocas de contexto individualmente na interface, atendimento real entre aparelhos, todos os atalhos/leitores de tela, CSV baixado ou som físico.
- Ensaio estreito configurou390×844; DOM retornou clientWidth375/scrollWidth375. Captura do bridge saiu reduzida e não é aceite completo de smartphone/touch. Viewport restaurado. Capturas nos outputs desta tarefa: `frota-manutencao-fluxo-local-2026-10-23.png`, `frota-equipamentos-unidades-2026-10-23.png` e captura limitada mobile. Nenhuma contém dados comerciais.
- Preparação SSR via Vite encontrou `listen EPERM ...24678` e `ReferenceError: module is not defined` em react/jsx-dev-runtime. Resolvido no laboratório por bundle SSR CommonJS com esbuild JÁ instalado;27casos executados, sem dependências ou alteração de runtime do produto. Browser getByLabel não encontrou controle, DOM confirmou rótulo e getByRole operou corretamente. Processo da primeira prévia encerrou e recarga retornou ERR_CONNECTION_REFUSED; após reiniciar prévia própria, reabertura confirmou persistência. Não contornou alerta de certificado nem iniciou serviço comercial.

### Backend recebido: não aplicado

`aplicar.py --check /home/danillo-malta/Downloads/zzeus-frontend-backend-b02a` saiu1com `Pare: Aplique primeiro Frota e Manutencao de23/10/2026`. Ausentes `backend/internal/localdb/fleet/`, `backend/internal/localapi/fleet.go` e `docs/architecture/BACKEND_FROTA_MANUTENCAO_APLICADO.json`. A referência tem58migrações, incluindo0057_purchase_supply_cycle.sql e0058_supplier_returns.sql; o complemento espera predecessor48ou57e adiciona49ou58. Não renumerar SQL, fabricar marcador ou reconstruir predecessor a partir de testdata. É necessária entrega compatível do responsável com a base atual, seguida de CHECK antes de qualquer aplicação.

Backend branchfeat/backend-b02a-compat/HEAD58a26bbbdf08903d62f6dd1f0542417171b460dd e todas suas alterações locais preservados; nenhum arquivo de backend foi escrito, Go test/migração/banco/serviço não executados nesta integração. O proprietário respondeu para concluir o frontend. Contratos de manutenção técnica recebidos ficam disponíveis no pacote para revisão futura; este frontend AINDA NÃO os chama. Integração/ambiente descartável seguem pendentes de base/versão/perfil autorizados. Não anunciar ambas as partes operacionais.

Salvamento na branch de frontend existente segue autorização humana anterior para GitHub; sem merge, force-push ou mudanças na main. Reversão localizada por hashes/diff preservando alterações posteriores. Esta entrega adiciona a frente recebida, não conclui todas as17frentes do ERP nem substitui a ordem de logística→faturamento/receber→PDV/fechamento.


## Conferência de23/10/2026 — novo consolidado de backend recusado sem escrita

Recebido `TitanSystem_Backend_Consolidado_Frota_20261023.zip`, SHA256 `e2b166608071a6abb3b1719931376902e37753a1638d941ddab1bd292eb23ac6`. Extraído com caminhos/duplicidades/links conferidos em `work/fleet-consolidated-20261023-kde19g05/` desta tarefa; CRC aprovado e433arquivos conferidos por SHA256SUMS. Aplicador e sequência lidos antes de executar. Não executar recuperação de fontes para tentar adaptar a árvore a esta entrega.

Alvo conferido `/home/danillo-malta/Downloads/zzeus-frontend-backend-b02a`, branchfeat/backend-b02a-compat, HEAD58a26bbbdf08903d62f6dd1f0542417171b460dd e alterações locais preservadas. `aplicar.py --diagnostico ALVO` saiu1com erro exato:

> Pare: Fonte de pacote anterior mudou ou falta: backend/internal/localdb/localdb_test.go. Nenhum arquivo sera forcado

O diagnóstico não autorizou prosseguir. Não executados --check/testes Go/vet/race/build/aplicação normal, porque a conferência dos fontes já recusou a base antes da candidata. Nenhum serviço, banco, migração, restauração, instalador ou executável iniciado. O Go encontrado no PATH é `/usr/local/go/bin/go`, versão go1.24.3 linux/amd64; o pacote pede Go1.25. Não instalou/baixou toolchain; isso é requisito adicional a resolver depois da compatibilidade dos fontes.

A cadeia contém10entregas e há8registros presentes. Entretanto, a referência também tem `ciclo-compras-20261022-1` (0057_purchase_supply_cycle.sql) e `coletas-devolucoes-20261022-1` (0058_supplier_returns.sql), que NÃO constam nessa cadeia. Os37arquivos finais combinados dessas duas entregas conferiram com seus marcadores, sem divergência. Portanto, não tratar as diferenças como corrupção ou trabalho descartável.

Em relação ao último estado dos8pacotes da cadeia, divergem7arquivos: localdb_test.go, migrate_safety_test.go, journal_migration_test.go, backup/backup.go, backup/compatibility_test.go e docs/api/{route-inventory.json,routes.openapi.json}. O teste atual exige58migrações. O plano09-fleet só aceita47ou56, após receiving_cases, e pretende criar48ou57_fleet_maintenance.sql; o10-workshop cria49ou58_fleet_workshop.sql. A referência tem58SQLs e números57/58legitimamente ocupados pelas entregas posteriores. Não forçar hashes, fabricar marcadores, retirar SQL, renumerar manualmente ou reconstruir predecessor de testdata.

Próximo requisito ao responsável pelo backend: consolidado/atualização compatível com a base REAL de58migrações, incluindo Ciclo de Compras e Coletas/Devoluções e preservando todas as alterações locais/contratos/rotas. O plano deve conferir preimagens e tratar novas migrações de forma incremental pelo responsável, com candidata/testes antes da escrita. Depois disso confirmar toolchain compatível e repetir diagnóstico/CHECK. Esse diagnóstico não exige restaurar o projeto inteiro.

Status Git do backend após a inspeção permaneceu idêntico ao registrado antes dela. Frontend/Electron não alterados por este pacote; abas de manutenção continuam locais conforme revisão22, sem integração oficial ou notificação remota. O salvamento deste registro na branch de frontend não significa aplicação do backend, compilação de novo binário ou serviço atualizado em execução.


## Entrega de23/10/2026 — incompatibilidade de Frota resolvida na base58

O proprietário autorizou expressamente fazer as mudanças necessárias e entregar no GitHub. A recusa anterior do consolidado não foi contornada por restaurar fontes, reescrever hashes históricos ou fabricar predecessor. Foi preparada/testada uma adaptação real sobre a base58 existente, preservando as duas entregas posteriores Ciclo de Compras e Coletas/Devoluções.

### Estado aplicado

Backend correto `/home/danillo-malta/Downloads/zzeus-frontend-backend-b02a`, branchfeat/backend-b02a-compat. Base original58a26bbb mais122fontes/documentos locais foram preservados em commit SEPARADO `29871fbba6866aa5e34d518a532404b214a6b69f`; esses trabalhos preexistiam e não são atribuídos à adaptação. Nova integração em `857febb588b522fdac45d3773e1b772094b19816`. Não houve checkout de main, reset/clean, merge, force-push, ZIP novo, instalação de dependências, banco comercial ou serviço iniciado.

- SQL1–58mantidas byte a byte; novas `0059_fleet_maintenance.sql` e `0060_fleet_workshop.sql`, com o conteúdo SQL original dos pacotes09/10. Não renomeou uma migração já instalada. Schema de código agora60; isso não afirma migração de banco operacional.
-33arquivos planejados mais registro `docs/architecture/BACKEND_FROTA_BASE58_APLICADO.json`: fontes/testes/contratos/guia dos dois módulos, duas atualizações guardadas em ledger/mutate, montagem Fleet/Workshop no ponto de Tratativas, erro422,80rotas novas e mapa/OpenAPI; testes de schema/interrupção e compatibilidade de backup ampliados até60.397rotas anteriores e contratos de compras/devoluções preservados. Guia específico `docs/architecture/BACKEND_FROTA_BASE58_20261023.md` explica origem, compatibilidade e limites.
- Regressão própria58→60em SQLite temporário com FK ativas: preserva os checksums1–58, definições das8tabelas de compras/devoluções, recibo e evento anteriores, gatilho de imutabilidade; repete migração e confirma as4tabelas centrais Fleet/Workshop e60versões. Testes originais de atualização e restauração também preservados; backup inclui58e59na matriz de origens.
- Candidata isolada e todas as fontes-base foram comparadas antes de aplicar; arquivo temporário exclusivo com fsync/leitura confirmou acesso efetivo. Backup privado em `work/fleet-base58-20261023/backup-source/` desta tarefa, com preimagens e manifesto; gravações relidas e fontes não pertencentes à alteração conferidas. Registro de aplicação descreve esta adaptação; não faz o consolidado antigo reconhecer versões futuras.

### Verificações reais

Toolchain existente em cache: Go1.25.8. A constatação anterior de Go1.24.3no PATH continua correta, mas não impedia usar essa versão já instalada. GOTOOLCHAIN=local/GOPROXY=off/GOSUMDB=off, cache próprio e ambiente sem credenciais. Nenhum download/instalação. `go test -json ./...`: exit0, 2559casos/subtestes aprovados,0falhas e3ignorados condicionais; 51pacotes aprovados. Dois skips requerem PostgreSQL de teste explicitamente configurado; o outro é helper de processo-filho, exercido pelo teste pai. Não anunciar cobertura online PostgreSQL integrada.

`go vet ./...`:exit0. `go test -race ./internal/localdb/fleet ./internal/localapi -run 'Workshop|Fleet'`:47casos aprovados,0falhas/ignorados. Regressão58→60também executada separadamente:1passou. Binário titan-local compilado em artefato isolado, sem execução/instalação; `-buildvcs=false` evita obter VCS da cópia candidata, e o registro guarda hash do binário e fontes/base conhecidos. Não equivale a serviço em execução.

Primeira suíte encontrou1falha porque o filtro source-only do aplicador original omitia `docs/api/examples/payables-flow.csv`. Incluído na candidata SOMENTE o exemplo já versionado, conferido contra a base; não foi alterado o teste ou seu resultado esperado. Caso financeiro repetido passou, depois suíte inteira final passou. Erro inicial de build ao obter VCS da candidata resolvido pela flag de artefato isolado. Logs/primeiro resultado/plano/candidata preservados em `work/fleet-base58-20261023/`. Frontend funcional não mudou; não foi repetida sua suíte1662por alteração exclusiva de backend/documentação.

### Limites e próxima continuidade

Frota/Conferência Técnica agora estão nos FONTES e no binário validado: isso supera o impedimento registrado na revisão23. Ainda não há serviço comercial/disponível autorizado comprovado, banco/estação/chaves escolhidos ou teste entre aparelhos. As abas do frontend continuam business85/IndexedDB e NÃO passam automaticamente a consumir a API. Não importar aprovação local como autorização do servidor. Checklist obrigatório/inspeção válida, revisão gerencial de horas, recuperação de operação e ACK têm os limites do contrato; não são folha, push ou som remoto.

Próxima integração deve criar adaptadores tipados para os contratos Fleet/Workshop e usar um ambiente NOVO descartável explicitamente autorizado. Separar fonte/binário/serviço/tela. Continuar a ordem de fechamento do frontend e preservar116/117/docas/recebimentos. Esta entrega resolve a aplicação compatível do backend recebido; não anuncia todo o ERP ou desktop homologado.


## Entrega de23/10/2026 — consultas oficiais de Frota e Conferência Técnica

### Escopo aplicado e origem

Continuidade do frontend autorizada pelo proprietário. Alvo confirmado `/home/danillo-malta/Downloads/zzeus-frontend-backend`, branchfeat/backend-checkout-authority, baseadaca8e limpa. Regras lidas; nenhum AGENTS.md aplicável encontrado. Backend/Electron não alterados nesta unidade. Não interpreta o pedido de concluir tudo como autorização para inventar integrações, escolher banco ou anunciar aceite integral.

O painel **Registros oficiais de frota e manutenção** foi inserido antes dos rascunhos em `/local/fleet`, com abertura explícita. Os rascunhos IndexedDB de Pedidos/Mecânica/Equipamentos/Custos foram preservados e passam a ter título explícito de rascunhos locais. O adaptador oficial é exclusivamente GET: não migra dados, não envia aprovação local, não cria chamado, não executa manutenção e não marca aviso como lido.

Contratos lidos da referência real backend857febb: `docs/api/fleet-maintenance.openapi.json`, SHA256 `b708bcf570f048fca5c6de4c5b20c71c910d4d50924a46af8c8caddf08e1d6d7`; `fleet-workshop.openapi.json`, SHA256 `402120af9d4bc456c0abb0f4d275bd371358ac37bc6abc70b0c4808eb65b1ef6`. Snapshots somente dos schemas de leitura e dependências, sem código/backend/instaladores. Autenticação e semântica também conferidas em localapi/fleet*.go e localdb/fleet/{types,read,report,notices,workshop_reads}.go.

Arquivos exatos (além deste registro):
- `frontend-web/src/core/fleetOfficial/{fleet-maintenance.schemas.mjs,fleet-workshop.schemas.mjs,model.mjs,model.d.mts,source.mjs,source.d.mts,words.mjs,words.d.mts}`.
- `frontend-web/src/modules/tenant/pages/fleetOfficial/{FleetOfficial.tsx,fleetOfficial.css}`.
- Ponto de montagem delimitado em `frontend-web/src/modules/tenant/pages/business85/Fleet85.tsx`.
- `frontend-web/tests/fleetOfficial.test.mjs`.

### Comportamento e proteções

Consulta paginada de50registros dos14recursos (bens, pedidos, viagens, planos, prestadores, orçamentos, despesas, receitas, impactos, perfis, modelos, exigências, inspeções e horas). Filtros enviados ao servidor; total do filtro separado da página. Não soma recortes como total. Detalhes/histórico consultados por ID/revisão; recibo histórico mostra operação/ator/aparelho/motivo. Disponibilidade derivada, vencimentos preventivos com indicação de data/medidor, custos em período explícito UTC de até366dias, avisos com cursor durável e resumo técnico de um pedido. Avisos são consulta manual, sem alegação de push/som/ACK.

Guarda token/sessão e quatro identificadores antes/depois de fetch e leitura do corpo; expiração e permissões revalidadas. Canais de leitura independentes; somente a última consulta de cada canal pode publicar, mesmo se o transporte ignorar abort. Dispose/timeout/sinal externo impedem publicação tardia. Respostas passam por validação de schema, tipo específico de recurso, ID, revisão, data, paginação, duplicidades, filtro retornado e limites. Erros401/403/404/409/422/5xx continuam erros; sem resultado de demonstração na rota real.

Financeiro exige view_accounting além de view_orders; recursos financeiros ocultos no perfil restrito. Servidor continua responsável pela autorização. Histórico no adaptador não depende de renovação da licença; o ponto de montagem conserva a guarda preexistente de business85, que pode exigir logística ativa para abrir a área. Não expandiu permissões nem licença para exibir módulos.

Centavos e totais exatos usam strings/BigInt, sem conversão de grandes valores para Number. Contribuição parcial, depreciação e impacto estimado têm rótulos distintos; zero no denominador é explicitamente indisponibilidade de custo unitário, não custo zero. Custo não confirma pagamento/lucro líquido; horas não são folha. Inspeção pendente não é liberação. Cinco dicionários, claro/escuro, controles de44px, tabelas com rolagem interna e foco visível. Campos menos usuais mantêm seus códigos sob título explícito de campos do contrato; não significa revisão completa da ergonomia de todos os módulos.

### Evidências reais

-67testes novos, com transporte/fixtures isolados. Suíte completa `node --test tests/*.test.mjs`:1729aprovados,0falhas/ignorados, incluindo financeiro116/fornecedor117. TypeScript `node_modules/.bin/tsc --noEmit`:exit0. Build final isolado Vite/React com configFile:false/envFile:false:exit0,20.900877465s; não substituiu dist nem leu .env. Aviso preexistente de chunk grande da planilha permanece (~6.09MB de JS minificado para sheetRuntime70), além dos avisos de chunks/Tailwind da verificação isolada. Nenhuma instalação ou ZIP.
- Prévia explícita descartável em4345, sem proxy/backend/banco, identidades sintéticas e transporte que só aceita GET. Percorridos lista/detalhe/histórico de bem e pedido, disponibilidade bloqueada, custo9007199254740993centavos exibido90.071.992.547.409,93BRL, checklist obrigatório não atendido,3600segundos declarados/0confirmados, erro503visível sem manter tabela antiga e perfil sem view_accounting com as5categorias financeiras ocultas. Títulos/controles nos5idiomas e claro/escuro conferidos. Não é ensaio de toda combinação de ação, perfil, tela, teclado, acessibilidade ou tamanho de aparelho.
- A prévia encontrou colisão entre tradução do botão abrir e estadoopen; nomes separados e regressão acrescentada. Primeiro teste de idioma falhou por exigir tamanho artificial de frase chinesa; corrigido esse critério inadequado, sem alterar conteúdo nem relaxar a regra comercial. Depois repetidas suíte inteira, TypeScript e build finais.
- Prova visual em `outputs/frota-oficial-conferencia-tecnica-2026-10-23.png` desta tarefa. Preparação, preimagens/hashes, backup, contratos, logs e prévia em `work/fleet-official-20261023/`. Alteração localizada, sem cópia geral do projeto, backend, bancos, segredos, node_modules, caches ou distribuições no commit. Reversão por diff/preimagens, preservando alterações posteriores; não executar o aplicador preparado novamente sobre arquivos evoluídos.

### Pendências reais e sequência

ESTA ENTREGA NÃO CONCLUI TODO O FRONTEND. Frota agora consulta o serviço oficial, mas criar/editar bens, abrir e tramitar chamados, orçamentos/aprovações, viagens/retornos, despesas/perfis, checklist/inspeções, apontamento/revisão de horas e ACK ainda precisam de telas/comandos oficiais com fila durável, IDs originais, revisão esperada, recuperação por recibo e histórico e permissões adequadas. Os rascunhos continuam úteis somente como rascunhos. Não apresentar sua aprovação como aprovação no backend.

Continuar pelas unidades funcionais de escrita e pelas frentes já priorizadas de logística/recebimento (tratativas/indicadores), faturamento/receber e PDV/fechamento. As demais17frentes continuam conforme matriz e entregas específicas; não transformar telas existentes em promessa de operação homologada. Não há porcentagem fundamentada de conclusão integral.

Validação integrada requer NOVO ambiente descartável autorizado com versão/binário de backend correspondente, schema60aplicado nesse banco de teste, tenant/loja/aparelhos/usuários descartáveis aprovados, perfis com e sem view_orders/view_accounting, licença logística e dependências assinadas válidas e endereço/perfil de serviço explicitamente fornecidos. Exercitar autorização/isolamento, paginação, histórico, bloqueios/inspeção vigente, erros/limites e recuperação/reinício; sem dados comerciais. Nenhum desses parâmetros foi escolhido automaticamente. Ainda não demonstra serviço atualizado, integração real, banco persistido no produto ou pacote desktop Windows/Linux homologado.


## Entrega de23/10/2026 — abertura e tramitação oficial de chamados

### Escopo incremental sobre a revisão25

Base limpa `e4d84897e7bc84b56e622459150152477597473f`, publicada na mesma branchfeat/backend-checkout-authority. Consultas da revisão25 preservadas. Backend857febb e Electron sem alterações. Regras de sessão/empresa/loja/usuário/aparelho, cinco idiomas, licença e permissões permanecem. O serviço real continua sem ambiente autorizado; transportes de teste são explicitamente sintéticos.

Arquivos: novos `frontend-web/src/core/fleetOfficial/{commands.mjs,commands.d.mts}`, `frontend-web/src/modules/tenant/pages/fleetOfficial/FleetCommandControls.tsx`, `frontend-web/tests/fleetCommands.test.mjs`; extensões localizadas de `{fleet-maintenance.schemas.mjs,words.mjs}`, `FleetOfficial.tsx` e este registro. Acrescentados somente os schemas oficiais de cadastro/ação de bens/chamados e dependências, extraídos dos mesmos contratos SHA da revisão25. Conferidos também mutate.go e ledger.go do backend real, sem alterá-los. Preimagens/hashes e logs em `work/fleet-commands-20261023/` desta tarefa.

### Fluxos disponíveis no código do frontend

- Cadastro de veículo/equipamento no serviço: referência, nome, categoria, placa/local, unidade e leitura exata em milésimos, observação e motivo; somente proprietário/gerente com contrato válido. Não importa rascunhos automaticamente e não implementa edição/baixa/reativação de bens nesta unidade.
- Abertura de chamado vinculada a bem oficial: problema, prioridade, local, bloqueio de uso, impacto, referência e motivo. Ator solicitante vem da sessão no backend. Formulário → revisão explícita → confirmação de envio → recibo/histórico. IDs gerados antes do primeiro envio e preservados em toda recuperação.
- Tramitação por revisão esperada e permissão: assumir/triagem, atribuir responsável, aprovar com orçamento já existente (view_accounting obrigatório), iniciar/retomar, aguardar peças, encaminhar à inspeção, liberar, cancelar preservando bloqueio quando aplicável, liberar bloqueio de cancelamento, reabrir e registrar observação. Só aparecem ações pertinentes ao estado/ator/perfil; backend revalida tudo. Aprovação não cria orçamento nem confirma pagamento. Cancelamento depois da aprovação pode mantercancelled_blocked; não libera silenciosamente o bem.
- Inspeção/liberação consultam resumo técnico vigente antes do POST: revisão do pedido tem que ser a mesma; checklist obrigatório não satisfeito bloqueia envio. O backend continua responsável pela época/versão/evidência e concorrência. Esta tela AINDA NÃO registra checklist/inspeção/horas; evidências precisam existir no serviço para passar pelo gate.
- Formulários no produto usam os forms de business85 para preservar rascunho por contexto e a proteção de navegação existente. Campos não enviados não são operação oficial. Na prévia isolada, formulários usam estado descartável. Nenhuma ampliação de acesso para mostrar ações.

### Recuperação e isolamento

Fila `titan-fleet-commands:v1:` por quatro identificadores, sem token/senha. Web Locks obrigatório entre abas, persistência relida antes do POST, schema e tamanho validados; storage inválido/indisponível ou lock ausente bloqueia envio. Antes do POST salva uncertain=true. Perda de resposta, erro, troca de sessão e fechamento de tela não apagam a tentativa. Nenhuma recuperação ou reenvio automático ao montar.

Consulta manual de `/fleet/operations/{operation_id}` tem de conferir ID, tipo, revisão, dados/estado esperados e histórico do registro na página da revisão, com o mesmo operation_id, ação, motivo, ator e aparelho originais. Ausência do histórico NÃO é ausência do recibo; bloqueia, sem novo POST. Resultado confirmado limpa somente a mesma tentativa ainda presente. Reenvio explicitamente solicitado preserva exatamente IDs e corpo, e faz nova conferência de estado/revisão antes de enviar; idempotência efetiva continua no backend. Tentativa incerta não pode ser descartada. Somente tentativa ainda não enviada pode ser descartada explicitamente após consulta indicar ausência do recibo. Mesmo erros de POST considerados definitivos por outros clientes permanecem conservadoramente incertos aqui até conferência; não introduziu exclusão automática.

Contexto/token/sessão/capacidades conferidos a cada fronteira assíncrona; fonte antiga não pode enviar, confirmar ou limpar nova sessão/fila. Consulta de recuperação pode continuar após expirar contrato, se sessão/permissões atuais autorizarem; novo envio continua bloqueado. Aprovação exige permissão financeira também na recuperação. Cadastro/ação confronta estado original e resultado derivado, preservando solicitante/dados/identificadores. Estado stale409 não é substituído por cópia inteira ou nova operação.

### Verificações reais

35testes novos de comandos; somados aos67de consulta,102regressões desta frente. `node --test tests/*.test.mjs` final:1764aprovados,0falhas/ignorados. TypeScript sem diagnósticos, exit0. Build isolado sem .env e sem sobrescreverdist:exit0,19.968448991s, saída `work/catalog-management-20261022/build-check-oVA2Ud`. Avisos preexistentes de chunk da planilha/Tailwind continuam; não afirma melhoria de desempenho ou pacote desktop final.

Casos: cadastro de bem, chamado com recibo/histórico, perda de resposta e recuperação sem segundoPOST, ausência de recibo mantendo incerteza, reenvio explícito com corpo idêntico, quatro trocas de contexto, logout durante preflight/leitura do corpo/apósPOST, preservação de fila de outra origem, ator/aparelho divergentes, histórico404, corpo de resultado divergente, lock/quota/storage corrupto, concorrência entre abas, segunda tentativa bloqueada, contrato expirado na recuperação, descarte explícito somente não enviado, revisão stale, triagem, gate de checklist, operador restrito, revogação gerencial e dependência de módulo ausente. Primeiro teste de triagem encontrou fixture de histórico comtotal2mas somente1evento; corrigida a fixture para incluir a criação anterior, sem relaxar o validador.

Prévia explícita4346 com GET/POST SOMENTE em memória de teste, sem proxy/backend/banco: revisão de chamado novo, reabertura da tela com tentativa não enviada preservada, reenvio explícito com resposta perdida, resultado incerto/descarte bloqueado, consulta de recibo/histórico para confirmar, chamado visível com mesmosIDs; triagem explícita posterior → revisão2e2eventos, com motivo/ator/aparelho. Fixture inicial devolvia página para GET de ID inexistente; frontend rejeitou e NÃO enviou POST. Corrigida fixture para404antes de repetir o ensaio. Não anunciou sucesso desse primeiro ensaio inválido. Prova `outputs/frota-chamado-recuperado-triagem-2026-10-23.png` desta tarefa. Dicionários dos comandos incluídos na paridade dos5idiomas; ensaio visual deste circuito foi pt-BR/escuro, não todas as combinações de perfil/tema/idioma/aparelho.

### O que continua pendente

Esta revisão supera a ausência de abertura/tramitação de chamados registrada na revisão25, mas NÃO conclui todas as operações de Frota nem as17frentes do ERP. Faltam comandos oficiais de checklist/inspeções/apontamento e revisão de horas/ACK, cadastro de prestadores e orçamentos, viagens e retornos, planos, custos/perfis e edição operacional dos bens, além de validação integrada autorizada. Aprovação de orçamento e liberação só funcionam se suas referências/evidências já existirem no serviço. Os requisitos de ambiente descartável da revisão25 permanecem; nenhum servidor foi iniciado e nenhum banco, chave, estação ou credencial foi escolhido. Continuar também logística/recebimento, faturamento/receber e PDV/fechamento conforme ordem aceita, sem refazer117.

Salvamento humano autorizado na branch existente, com commits delimitados, sem main/merge/force-push/ZIP/instalação. Para reverter, conferir hashes/preimagens e aplicar diff específico, preservando qualquer alteração posterior; não apagar filas incertas nem executar aplicadores antigos sobre a árvore evoluída.


## Entrega de23/10/2026 — checklist, inspeções e tempo de serviço oficiais

### Base, arquivos e escopo

Base limpa6331d41609527bcd164bd83106e0a6cafcf50f75 em `/home/danillo-malta/Downloads/zzeus-frontend-backend`, branchfeat/backend-checkout-authority. Regras relidas, sem AGENTS.md aplicável encontrado. Frontend116/117, rascunhos, contratos/arquivos anteriores e Electron preservados. Backend de referência857febb/schema60somente lido; nenhum código, banco ou serviço de backend alterado/iniciado. Sem checkout de main, reset/clean, instalação ou ZIP.

Novos `frontend-web/src/core/fleetOfficial/{workshop.mjs,workshop.d.mts}`, `frontend-web/src/modules/tenant/pages/fleetOfficial/WorkshopControls.tsx`, `frontend-web/tests/workshopCommands.test.mjs`. Extensões localizadas de core/fleetOfficial/{commands.mjs,fleet-workshop.schemas.mjs,model.mjs,words.mjs,words.d.mts}, pages/fleetOfficial/{FleetOfficial.tsx,FleetCommandControls.tsx,fleetOfficial.css} e este registro. Nenhum router novo, nenhuma cópia do ERP. Snapshots de escrita e dependências transitivas extraídos de docs/api/fleet-workshop.openapi.json com o mesmo SHA402120af9d4bc456c0abb0f4d275bd371358ac37bc6abc70b0c4808eb65b1ef6 da revisão25. Regras também comparadas com localdb/fleet/{workshop_types,workshop_change,workshop_ledger,workshop_reads}.go e localapi/fleet_workshop.go.

### Fluxos implementados

- Modelos de checklist:1–50itens com IDs estáveis, texto e obrigatoriedade, nome/observação/motivo; criação e inativação para novos vínculos, com revisão e histórico preservados. Inativar o original não invalida silenciosamente sua cópia já vinculada.
- Exigência do chamado: consulta da revisão atual do chamado, modelo ativo e vínculo existente; criar/substituir com revisão esperada nos estados permitidos(open/triaged/approved). A política é copiada, não referência a futura edição. Formulário exibe itens e avisa sobre impacto da substituição em inspeções anteriores. Proprietário/gerente; o servidor revalida.
- Inspeção humana: consulta da revisão do chamado, da política copiada e da etapa vigente (último start/assign/reopen). Considera TODAS as páginas do histórico, até1000revisões; excedente422sem aceitar etapa parcial. Nenhuma resposta pré-aprovada: cada item exige escolha, não conforme/não se aplica exige justificativa, item obrigatório não se aplica produz inspeção NÃO conforme. Registra também falha de inspeção; não transforma falha em impedimento para registrar a evidência. Revisão humana antes do POST e verificação do resultado passado/falho contra respostas/política/etapa originais. Troca de responsável/reabertura/serviço invalida evidência antiga conforme contrato. Inspeção não libera bem automaticamente; liberação continua outra ação gerencial do chamado.
- Horas: declaração pelo ator autenticado em chamado autorizado, início/fim reais, descrição/motivo, até16horas por apontamento, segundos inteiros e sem horário futuro no preparo/envio. Entrada em hora local do aparelho; revisão/histórico em UTC, com rótulos distintos. O serviço continua responsável por sobreposição. Revisão gerencial: confirmar/recusar; anulação autorizada mantém duração, trabalhador e histórico. Declaração não é confirmação; nenhuma folha/pagamento/estoque/fiscal gerado.
- Detalhes legíveis de políticas, respostas e duração, mantendo campos do contrato opcionais. Respostas de inspeção identificadas pelos IDs da política original; não usa rótulos de uma revisão posterior. Estados de horas/modelos têm nomes próprios, sem confundir mensagem de confirmação do recibo com estado confirmado.

### Filas e recuperação

Nova fila `titan-workshop-commands:v1:` por empresa/loja/usuário/aparelho. Usa o MESMO Web Lock de Frota; ambas as fontes impedem nova escrita enquanto a outra fila tiver tentativa pendente (inclusive formato desconhecido/corrompido). Não converte nem remove filas antigas. Token e credenciais nunca entram na fila. Persistência é relida antes do POST; sem storage/lock a escrita é bloqueada.

Operação torna-se incerta ANTES do POST e só é limpa após recibo no namespace `/fleet/workshop/operations/{id}` e histórico do registro concordarem em ID, tipo, revisão, ator, aparelho, ação, motivo, dados e resultado esperados. Recibo404não equivale a falha definitiva; histórico404não equivale a recibo ausente. Consulta não reenvia. Reenvio só explícito, com IDs/corpo originais, depois de conferir referências/revisões atuais; incerteza nunca autoriza descarte. Apenas tentativa ainda não enviada pode ser descartada explicitamente após ausência de recibo. Sessão/contexto/permissões são conferidos a cada fronteira assíncrona; fonte antiga não envia nem confirma/apaga operação em outra origem.

Resultado recuperado correspondente remove erro/revisão antigos do formulário ainda associado à mesma tentativa, sem limpar outro rascunho/sessão. Formulários usam forms de business85 no produto, por contexto; sem esse provider, somente prévia descartável usa estado local. Histórico passou a validar sequência de revisões e IDs de operação únicos na página, para não confundir páginas, eventos ou revisões.

Mantém a limitação de acesso externo documentada na revisão25: os adaptadores podem consultar recibos após expirar contrato se sessão/permissões autorizarem, mas guardas de rota/área business85 ainda podem impedir abrir Frota nesse estado. NÃO anunciar que esse caso já foi percorrido na interface de produção. Resolver o acesso histórico explicitamente, preservando a proibição de escrita sem contrato; não afrouxar permissões gerais para demonstrar telas.

### Verificações reais e correções encontradas

57regressões novas, incluindo políticas imutáveis, itens obrigatórios/opcionais, nenhuma aprovação padrão, falhas com justificativas, troca de requisição/política, etapa vigente na segunda página(até revisão51), limite1000, cadastro/inativação/substituição, declaração/confirmar/recusar/anular horas,16horas/futuro/UTC, ator/aparelho/IDs divergentes, resultado incompatível, perda de resposta, recuperação sem novoPOST, reenvio idêntico explícito, sessão nos4identificadores, logout antes/depois do envio, lock/quota/storage corrompido, proteção cruzada Frota/Oficina, contrato expirado, operador não responsável, detalhes/traduções e fonte encerrada. Somadas aos102testes de Frota das revisões25/26:159testes da frente.

Suíte completa final `node --test tests/*.test.mjs`:1821aprovados,0falhas/ignorados; financeiro116 e fornecedor117 preservados. TypeScript `tsc --noEmit`:sem diagnósticos, exit0. Build Vite/React com configFile:false/envFile:false, sem ler .env nem substituir dist:exit0,19.918197888s, saída isolada `work/catalog-management-20261022/build-check-Ft0lTw`. Avisos preexistentes de chunks grandes/planilha e Tailwind permanecem; não anunciou ganho de desempenho, serviço ativo ou desktop empacotado.

Testes detectaram extração inicial incompleta das dependências dos schemas de escrita, exigência de required ausente no schema de objeto vazio e autorização indevida que bloqueava substituição gerencial do checklist. Corrigidos os fontes/validador, sem relaxar os contratos, e repetidas regressões. A inspeção valida TODAS as respostas, inclusive após uma primeira falha; evita curto-circuito que deixaria notas posteriores sem validação. Hora futura é conferida no preparo/envio, não torna inválida a leitura de uma operação histórica por mera mudança do relógio.

Prévia explícita4347, transportes GET/POST sintéticos somente em memória, sem proxy/backend/banco/dados comerciais: inspeção com obrigatório conforme e opcional não aplicável justificado; perda de resposta, descarte bloqueado, recuperação mantendo contador de1POST; horas08–09locais revisadas11–12UTC,3600segundos declarados e depois confirmados em revisão2com2eventos; recuperação removeu erro/revisão antigos e manteve contador em1antes da confirmação gerencial; vínculo de T-QA em R-BIND/revisão3; criação de modelo e inativação em revisão2, item obrigatório/histórico preservados. Controle nativo de data conferido também pelo teclado; a tentativa de preenchimento automatizado sem confirmar edição não foi anunciada como sucesso. Não significa aceite de calendário touch ou todos os controles nativos Windows/Linux.

Títulos/controles percorridos nos5idiomas; claro/escuro; operador sem permissão financeira não viu categorias financeiras e teve criação de política bloqueada. Não é ensaio de toda combinação de idioma/perfil/tamanho/teclado/acessibilidade. Prova em `outputs/oficina-checklist-historico-2026-10-23.png` desta tarefa. Preparação, backups, hashes, logs e fixture em `work/workshop-20261023/`; dados sintéticos não entram na rota real. Não versionar dependências/caches/builds/fixtures externas ao conjunto de teste.

### Continuidade e limites

Supera as pendências de comandos oficiais de checklist/inspeção/horas das revisões25/26. NÃO conclui todo o frontend/ERP. Frota ainda precisa de prestadores/orçamentos, viagens/retornos, planos, despesas/receitas/impactos/perfis, ações operacionais de bens e ACK de avisos, além da validação integrada. Aprovação do chamado depende de orçamento oficial já existente. Continuam também as unidades de logística/recebimento, faturamento/contas a receber e PDV/fechamento e demais frentes da matriz, conforme ordem aceita. Não refazer117 nem confundir expansão de segmentos com conclusão do público inicial.

Validação integrada mantém requisitos da revisão25: serviço/versionamento correto, NOVO banco/tenant/loja/aparelhos/usuários descartáveis e licença assinada explícita, endpoint e perfil autorizados. Nenhum desses parâmetros foi escolhido automaticamente. Não há prova de serviço real, persistência comercial, teste entre aparelhos, leitor/câmera/impressora, fiscal, folha ou desktopWindows/Linux nesta unidade. Os limites não impedem continuar os trabalhos independentes de frontend com contratos existentes.

Salvamento na branch existente autorizado pelo proprietário; sem merge/force-push/main. Reversão por diff/preimagens específicos, preservando alterações posteriores e filas incertas; não executar aplicador preparado outra vez sobre árvore evoluída.


## Prestadores e orçamentos oficiais — 23/10/2026, revisão28

Base incremental e4748245e5768a5562a041bbe2f4f9d9e19ba10e, mesma árvore e branch feat/backend-checkout-authority. Preservados116/117, rascunhos, backend e Electron. Nenhuma instalação, ZIP, banco ou serviço de backend usado. Preimagens, hashes e logs em work/fleet-quotes-20261023 desta tarefa. Alterações dos arquivos existentes comparadas à base; nenhuma substituição de árvore ou aplicador antigo.

### Arquivos e contratos

Novos frontend-web/src/core/fleetOfficial/{providerQuote.mjs,providerQuote.d.mts}, frontend-web/src/modules/tenant/pages/fleetOfficial/ProviderQuoteControls.tsx, frontend-web/tests/fleetProviderQuote.test.mjs. Extensões localizadas de core/fleetOfficial/{commands.mjs,commands.d.mts,fleet-maintenance.schemas.mjs,words.mjs}, pages/fleetOfficial/FleetOfficial.tsx e este registro. Nenhum router novo.

Contrato Fleet de docs/api/fleet-maintenance.openapi.json na referência857febb, SHA256 b708bcf570f048fca5c6de4c5b20c71c910d4d50924a46af8c8caddf08e1d6d7; ProvidersCreate/ProvidersAction/QuotesCreate e dependências transitivas adicionados ao snapshot fechado. Regras conferidas em localdb/fleet/{types,mutate}.go. Não foram inventadas ações de edição/anulação de orçamento: contrato permite somente criação imutável.

### Comportamento entregue

- Prestadores: cadastrar/atualizar contato e serviços, inativar e reativar por gestor/proprietário. Revisão esperada, motivo e histórico; inativação mantém cadastro e vínculos anteriores. Não contrata nem envia convite/comunicação externa.
- Orçamento: consultar chamado triado, revisar identidade, bem e revisão original; prestador ativo opcional ou origem interna documentada. Três componentes explícitos (peças, mão de obra, outros), aceitando zero informado para garantia etc. Conversão decimal estrita via BigInt para centavos inteiros e total seguro, sem arredondamento binário, agrupamento, expoente ou corte silencioso. Total em BRL. Valores em branco não viram zero implicitamente.
- Proposta imutável não aprova chamado, gera compra/conta a pagar ou move dinheiro. Aprovação explícita do chamado usa identificador do orçamento, em ação separada já existente e sujeita à permissão financeira. Custos realizados continuam separados das estimativas.
- Gestor/proprietário e licença ativa com dependências para escrever; orçamento exige view_accounting também para recuperar recibo. Operador sem essa permissão não vê categorias financeiras, não recebe autorização ampliada para demonstrar telas.
- Reconsulta antes do POST: chamado exatamente igual ao snapshot revisado, bem existente e prestador ativo. Mudanças detectadas bloqueiam envio. O contrato de criação de orçamento não transporta revisão esperada do chamado: essa reconsulta não promete trava atômica entre GET e POST; backend deve revalidar o estado de seu contrato. Não alterar backend nesta unidade.
- Mesma fila Fleetv1 e trava compartilhada com oficina: compatibilidade dos pendentes antigos de bens/chamados/prestadores mantida; orçamento acrescenta snapshot quoteRequest estrito. Tentativa salva antes do envio, IDs preservados, resultado incerto não apagado; consulta GET de recibo e histórico antes de reenvio explícito idêntico. Sem reenvio em troca de sessão ou contexto; tenant/loja/usuário/aparelho continuam separados. Nenhuma migração ou limpeza de filas existentes.
- Formulários vinculados à proteção de edição business85; rascunho não é registro oficial. Revisão humana, erros visíveis, recuperação limpa somente o formulário do operation_id correspondente. Dicionários nos5idiomas, temas herdados e controles de teclado/touch existentes.

### Evidências executadas

33novos testes de providerQuote passaram: centavos exatos e limite seguro, valores inválidos, três ações do prestador, criação de proposta, aprovação separada, garantia zero explícito, origem interna, imutabilidade, chamado alterado, prestador inativo, papel/permissão, logout e troca dos4IDs, perda de resposta, resultado adulterado, recibo ausente e reenvio idêntico. Compatibilidade anterior92casos (Fleet35+Oficina57) passou. Suíte completa node --test tests/*.test.mjs:1854aprovados,0falhas/ignorados. TypeScript tsc --noEmit:exit0, sem diagnósticos. Build isolado configFile:false/envFile:false:exit0,19.876656432s, work/catalog-management-20261022/build-check-jNEyzk. Não leu .env nem substituiu dist. Avisos preexistentes de chunks grandes/planilha e Tailwind permanecem, sem alegar correção de desempenho.

Ensaio visual explícito4348, GET/POST em memória, sem backend/proxy/banco: proposta10+20+0centavos=BRL0,30; perda de resposta deixou tentativa incerta e descarte bloqueado, recuperação por GET confirmou mesmos IDs com contador ainda1POST; formulário/erro de envio limpos após confirmação. Cadastro de prestador e inativação em revisão2 preservaram campos e dois eventos. Cinco idiomas percorridos, claro/escuro; operador teve criação bloqueada com motivo visível e categorias financeiras ausentes. Prova: outputs/orcamento-recuperado-2026-10-23.png nesta tarefa. Não é comprovação do serviço real nem de todas as combinações de acessibilidade/teclado/touch.

### Pendências preservadas e próxima unidade

Supera a dependência da revisão27 de orçamento previamente cadastrado fora da tela. Não conclui frota nem todo frontend. Permanecem viagens/retornos, planos, despesas/receitas/impactos/perfis, ações operacionais de bens e ACK, além da integração real. Não reenquadrar expansão de frota como conclusão do público inicial. Retomar prioridades da matriz: tratativas/indicadores de logística e recebimento, faturamento/contas a receber, PDV/fechamento e demais frentes, revisando código e contratos existentes antes de acrescentar funções.

A limitação de recuperação com licença expirada na guarda externa da rota da revisão27 continua não resolvida; não afirmar que foi demonstrada na UI real. Integração depende de versão correta do serviço e NOVO banco, empresa/loja, usuários/aparelhos descartáveis, licença assinada e perfil/endpoint explicitamente autorizados. Nenhum parâmetro escolhido automaticamente. Backend e Electron intactos. Reversão localizada pelas preimagens/diff preservando evolução posterior e filas incertas. Salvamento na branch autorizada pelo proprietário, sem merge/main/force-push.


## Tratativas de recebimento e ciclo de vida dos adaptadores — 23/10/2026, revisão29

Base incremental174d86f9718203929dcecc398fc4fb0040292084, branch feat/backend-checkout-authority, limpa antes desta unidade. Esse commit de prestadores/orçamentos foi enviado e hash remoto conferido. A tentativa inicial com credential.helper desabilitado retornou fatal: could not read Username for https://github.com: No such device or address; repetição usando a autenticação já configurada concluiu, sem exibir ou alterar credenciais.

### Arquivos exatos e contrato

Novos frontend-web/src/core/receivingCases/{model.mjs,model.d.mts,schemas.mjs,source.mjs,source.d.mts,words.mjs,words.d.mts}, frontend-web/src/modules/catalog/pages/ReceivingCases.tsx, frontend-web/tests/receivingCases.test.mjs; novos frontend-web/src/core/local/{retainDisposable.mjs,retainDisposable.d.mts}, frontend-web/tests/retainDisposable.test.mjs. Alterações localizadas em pages/LocalOrders.tsx (catálogo), pages/fleetOfficial/FleetOfficial.tsx (tenant) e este registro. Nenhum router, backend, Electron, dependência, ZIP ou banco alterado. Preimagens, hashes e logs em work/receiving-cases-20261023 desta tarefa.

Contrato docs/api/receiving-cases.openapi.json da referência857febb/schema60, SHA2562c076466b9d6c6bd08eb2fafc8af501aaabd9df6a9a26304c08cbd08ae06ef29, comparado com localdb/purchases/receiving_cases.go e receiving_case_reads.go. Snapshot fechado de esquemas sem inventar campos/ações. Datas e decimais exatos reutilizam utilitários existentes de docas/recebimento; não se cria outra interface em desktop/.

### Fluxo funcional

- Na rota /local/orders, selecionar o pedido existente e abrir tratativa: falta, avaria, produto divergente ou outro; resultado SOLICITADO (revisão, reposição, crédito, reembolso, coleta ou outro), referência, responsável opcional, prazo opcional e motivo. Gestor pode atribuir usuário autorizado; operador pode deixar sem responsável ou atribuir a si. Serviço revalida vínculo/usuário/aparelho/contrato e copia as evidências em transação.
- Abrir não envia acordo, mensagem ou pedido ao fornecedor, não efetua coleta, pagamento ou ajuste de estoque. Resolução/cancelamento são administrativos LOCAIS; não representam reembolso aceito/pago. Não registra compensação fictícia.
- Consultas paginadas por pedido/responsável/estado, detalhe, evidência original de abertura (produto/unidade/previsto/aceito/restante/alertas/instante), responsável, prazo e histórico de revisões. Evidência imutável é distinguida de saldo atual. Valida quantidades seguras e soma exata, estados conhecidos, campos, filtros, ordem de IDs e paginação. Atraso derivado pelo instante da consulta e estados ativos; nunca notificação ou alteração automática.
- Operador autorizado: iniciar/registrar nota em caso próprio sem atribuição ou atribuído a si. Gestor: atribuir, definir/remover prazo, resolver/cancelar ativo, reabrir fechado. Iniciar sem responsável atribui o ator. Revisão esperada e motivo em cada ação, reconsulta do caso completo antes do POST; conflito não força substituição. Preserva evidência/identidades de origem. Seletores exibem só ações permitidas; ausência de ação mostra explicação. Frontend não substitui autorização no serviço.
- Leitura exige view_orders+manage_stock no contexto; escrita também exige contrato Orders ativo (core/orders). Histórico/recibo no adaptador dispensam renovar contrato. Não se comprovou o acesso real através de TODAS as guardas externas com licença expirada; teste dessa condição foi isolado no adaptador.
- Fila própria titan-receiving-cases:v1 vinculada a empresa, loja, usuário e aparelho; tentativa salva antes do POST, IDs estáveis, comparação exata de resultado+recibo+evento com ator/aparelho. Recibo de revisão antiga é recuperável mesmo que o caso tenha avançado. Consulta de recuperação não reenvia. Resultado incerto não pode ser apagado; repetição somente explícita e idêntica. Logout/troca de contexto invalidam consultas e novos envios antigos, sem apagar a operação. Bloqueio de armazenamento/locks impede novo POST; nenhuma limpeza automática de fila.
- Abertura/ação/revisão/pendência entram na guarda de edição de LocalOrders e beforeunload. Pedidos, recebimento e docas reconhecem a pendência da tratativa e vice-versa. Filtros/páginas não substituem o contexto de uma edição ativa. Erros são visíveis; sem mocks nas rotas reais. Prazo no formulário em hora local do aparelho, revisão/registro/histórico em UTC; campos irrelevantes não aparecem como alterações na revisão.

### Correção encontrada na revisão

O frontend inicia em React.StrictMode. Efeitos antigos de FleetOfficial descartavam os adaptadores memoizados na limpeza de ensaio e depois reutilizavam instâncias já descartadas, bloqueando comandos no desenvolvimento. Novo retainDisposable mantém uma concessão por consumidor e adia descarte final somente até a próxima microtarefa: remontagem síncrona de ensaio cancela descarte, desmontagem real descarta uma única vez. Aplicado aos3adaptadores Fleet e ao novo módulo de tratativas. Não desativa StrictMode nem remove controles de sessão/contexto.5regressões cobrem ensaio, desmontagem real, consumidor compartilhado, limpeza repetida e adaptador opcional.

### Verificações executadas

37regressões de tratativas: abertura sem efeito comercial, ciclo start/note/assign/set_due/resolve/reopen/cancel, imutabilidade de evidência e criador, atribuição ao iniciar, gestor/operador/terceiro, campos irrelevantes, prazos passados/canônicos/inválidos, atraso derivado, quantidades inseguras, revisão alterada, perda de resposta e1POST, recibo ausente, reenvio idêntico, recibo histórico, logout/troca dos4IDs, contrato expirado no adaptador, permissões, resultado adulterado e evento de outro aparelho, histórico descontínuo, páginas/filtros/IDs, consultas simultâneas fora de ordem, dispose, armazenamento/locks e5idiomas.37+5regressões novas passaram.

Suíte final node --test tests/*.test.mjs:1896aprovados,0falhas/ignorados. TypeScript tsc --noEmit:exit0, sem diagnósticos. Build final isolado configFile:false/envFile:false:exit0,20.324112262s, work/catalog-management-20261022/build-check-A9l5kc. Não leu .env nem substituiu dist. Avisos preexistentes de Tailwind/chunks grandes/planilha permanecem; nenhum ganho de desempenho alegado.

Ensaio visual4349 com StrictMode ativo e API sintética em memória, sem backend/banco/proxy: pedido ORDER de10kg,6kg aceitos,4kg restantes; abertura de solicitação de reembolso apenas como intenção, perda da resposta, fila incerta/descarte bloqueado, recuperação mantendo1POST e mesmos identificadores; resolver localmente passou a revisão2 com2eventos e evidência10/6/4 preservada. Nenhum reembolso foi executado.5idiomas e temas claro/escuro percorridos. Operador diferente do responsável viu ação desabilitada e explicação. Prova em outputs/tratativa-historico-2026-10-23.png. Uma recarga posterior retornou ERR_CONNECTION_REFUSED porque a prévia havia encerrado; retomada com sessão própria e nova aba confirmou a explicação do bloqueio. Não foi anunciado sucesso daquela recarga falha. Prévia de frota4348 também refeita em StrictMode: consultas e carregamento de chamado triado para preparar orçamento funcionaram após a correção. Não equivale a teste de toda a rota LocalOrders, aparelho físico, persistência real ou todas as combinações de teclado/touch/acessibilidade.

### Continuidade

Supera a pendência de abertura/acompanhamento administrativo de tratativas. Não conclui o circuito comercial de trocas, coleta, chegada ao fornecedor, crédito/reembolso autorizado ou seus efeitos fiscais/financeiros. Continuam indicadores/utilização de docas, confirmação externa e expedição, faturamento/contas a receber, PDV/fechamento, demais fontes/planilha/desempenho e módulos da matriz. Não declarar o frontend inteiro terminado nem atribuir percentual sem aceites medidos. Frontend116/117 preservados e incluídos na suíte.

Integração real continua dependente de versão correta de backend, NOVO banco/tenant/loja, usuários/aparelhos descartáveis, licença assinada e perfil/endpoint expressamente autorizados; nada foi escolhido automaticamente. Sem alteração do backend ou do Electron. Reversão localizada por diffs/preimagens, preservando alterações posteriores e filas incertas. Publicação autorizada na mesma branch, sem merge/main/force-push.


## Indicadores de docas e utilização histórica — 24/10/2026, revisão30

Unidade iniciada em23/10 e concluída em24/10, mantendo laboratório work/dock-insights-20261023 desta tarefa. Base163268bb31867c5730a7fa0e5ddcfcc8960ebb3a, mesma pasta e branch feat/backend-checkout-authority. Árvore limpa na abertura, regras relidas, sem AGENTS.md aplicável encontrado. Preimagens/hashes dos arquivos existentes conferidos antes da escrita. Nenhuma restauração geral, instalação, alteração de backend/Electron/banco ou ZIP.

### Arquivos e fontes

Novos frontend-web/src/core/dockInsights/{model.mjs,model.d.mts,source.mjs,source.d.mts,words.mjs,words.d.mts,contracts.ts,schemas.mjs}, frontend-web/src/modules/catalog/pages/{DockInsights.tsx,dockInsights.css}, frontend-web/tests/dockInsights.test.mjs. Extensões localizadas de pages/LocalOrders.tsx e este documento. Painel na rota existente /local/orders, sem novo menu/router ou interface paralela no Electron. Tipos gerados dos contratos exatos, validação de esquema e semântica em runtime; contratos não são evidência de serviço em execução.

Backend somente lido na referência857febb/schema60:
- docs/api/dock-insights.openapi.json, SHA256285597b8d733bb5729abcf0ead0076f9b1a65c401f7a45b59cddd341adad8ce2.
- docs/api/dock-utilization.openapi.json, SHA2565db293c0c780efe12d160f87c0ebe9a0356f90a0ed348fd0cd231f99e76a358b.
- Implementação localdb/dockagenda/{insights,utilization}.go e contratos de Dock/Appointment existentes comparados. Consulta de purchase-supply-cycle identificou também contratos de decisão manual/entrega; isso ainda não é integração frontend nem confirmação externa verificada.

### Quatro consultas entregues

1. Janelas planejadas: doca, intervalo até7dias, duração inteira5–480minutos, lacunas e primeiro término que comporta a duração, janelas planejadas ocupadas, fila atual e descarga observada. Doca desabilitada não sugere lacunas. confirmed_available obrigatoriamente false: não promete capacidade, previsão de liberação ou compromisso; reserva continua ação distinta da agenda atômica existente.
2. Métricas por fornecedor: agrupamento por ID, período pelo início PLANEJADO [início,fim) até366dias, estado atual de cada agendamento, amostras/atrasos e médias exatas de espera/descarga. Cancelados fora de chegada; atraso a partir de1segundo inteiro. Sem ranking ou atribuição automática de culpa. Totais de todo o conjunto separados dos fornecedores da página. Número de amostras deve corresponder às evidências dos estados; página completa também reconcilia médias ponderadas exatas.
3. Pendências: ativos independentemente da data prevista, prioridade local1pedido cancelado,2descarga,3chegada em espera,4fim previsto passado. Exclui reservas futuras comuns; estado/razão/prioridade/filtros/ordem validados. Consulta não cancela, marca ausência, confirma leitura ou envia notificação.
4. Utilização histórica: doca inclusive desabilitada, período histórico até31dias, fim não posterior à medição do serviço; duração por calendário, união da última agenda não cancelada/não ausente, descarga real, tempo sem registro de descarga, frações, segmentos e dias UTC. Não é jornada de trabalho, capacidade da equipe ou prova de ociosidade/capacidade livre. Descarga em andamento aparece como observação; não projeta término.

### Exatidão, escopo e sessão

Todos os endpoints são GET em /local/v1/logistics/insights. Exigem view_orders+manage_stock e contexto autorizado; adaptador não exige renovar licença para consultas históricas, mas aceite através de todas as guardas externas com licença expirada continua não demonstrado. Nenhuma permissão ampliada.

Frações usam strings e BigInt: sem conversão para ponto flutuante, denominador positivo e numerador não negativo. Durações e proporções reconciliadas por multiplicação cruzada, soma diária exata, calendário UTC contíguo, recortes nas fronteiras do período, intervalos reais sem sobreposição; página completa confronta segmentos e distribuição diária. Decimais de exibição têm3casas e marca≈quando aproximados; numerador/denominador exatos permanecem visíveis e exportáveis. Média sem amostras mostra Sem amostra; ausência de horário mostra Não registrado. Não inventa zero quando fonte não foi consultada/está indisponível.

Limite5000na origem é explícito;422recusa o resumo, sem corte silencioso. Página50com contagem total distinta; página vazia não zera totais ou recortes diários. Exportação inclui metadados, parâmetros, medição, resumo completo aplicável, dias e somente detalhes da página. CSV textual protegido contra fórmulas, aspas e separadores; frações em campos separados de numerador/denominador.

Consulta somente após ação explícita; filtros aplicados/medição permanecem junto ao resultado, distintos dos campos ainda em edição. Novo pedido de consulta limpa visualização anterior; erro/limite não deixa resultado antigo aparentar atual. Controle por canal, AbortController e conferência também após leitura do corpo impedem retorno antigo mesmo se transporte ignora cancelamento. Logout/troca de empresa/loja/usuário/aparelho e descarte invalidam resposta antiga. Ciclo de vida usa retainDisposable para StrictMode, sem remover proteção de sessão. Formulários usam hora local com fuso efetivo visível; resultado/exportação em UTC. Tabelas com rolagem própria, foco e controles44px, datas sem quebra e frações legíveis; cinco idiomas e temas herdados.

### Evidências reais desta entrega

50testes específicos passaram: quatro consultas GET; histórico em perfil sem papel gerencial/licença ativa no adaptador; duas permissões obrigatórias; troca dos4IDs/logout/corpo tardio; consultas concorrentes e canais independentes; aborto/dispose; limites/períodos/duração; frações inválidas e inteiro acima de2^53preservado; promessa de disponibilidade recusada; doca desabilitada; lacunas coerentes; nulo distinto de média zero; totais/páginas/IDs; prioridade de pendências; período futuro recusado; durações/frações/dias/segmentos reconciliados; limites na origem; nanossegundo exato; médias ponderadas; distribuição diária; amostras coerentes com estado; CSV e5idiomas.

Suíte completa final node --test tests/*.test.mjs:1946aprovados,0falhas/ignorados. TypeScript tsc --noEmit:exit0, sem diagnósticos. Build final configFile:false/envFile:false, isolado sem substituir dist:exit0,23.380186861s, saída work/catalog-management-20261022/build-check-HvGhjS. Avisos preexistentes de chunks grandes/planilha e Tailwind permanecem; nenhum ganho de desempenho foi anunciado. Não leu .env/segredos ou dados comerciais.

Prévia explícita4350, StrictMode ativo, fetch sintético em memória, sem proxy/backend/banco: pendência de chegada em espera, lacuna planejada com fim inicial de30minutos e aviso de NÃO confirmação, métricas de4agendamentos com média1/3segundo, utilização1800/7200segundos=25% com fração1/4 e dia UTC. Entrada22/10/2026,21–23h em America/Sao_Paulo consultou23/10,00–02hUTC, conferida na tela. Erros503/422ocultaram resultado e exportação antigos; retomada funcionou.5idiomas percorridos, claro/escuro; contador dePOSTpermaneceu0. Prova outputs/docas-utilizacao-2026-10-24.png nesta tarefa.

Download real /home/danillo-malta/Downloads/Titan_Docas_utilization_Escopo_e_Pagina.csv,6876bytes,47linhas de dados: reaberto com csv.DictReader UTF8BOM/semicolon; parâmetros/período, resumo1800/1 e1/4, segmentoA e dia23/10conferidos. Contém apenas dados sintéticos, não arquivos do produto, credenciais ou banco. O retorno do download demorou; não foi declarado sucesso antes da confirmação/cotejo do arquivo.

### Continuidade e limites

Resolve a pendência de indicadores de docas/utilização da matriz. Não conclui o frontend inteiro ou circuito interempresa. Próxima unidade disponível: docs/api/purchase-supply-cycle.openapi.json contempla registro de decisão externa MANUAL com external_verified=false, planejamento e ações de entrega, recebimento/vínculo documental. Revisar permissões, recibos e diferenças antes de implementar; não tratar contrato existente como API ausente, nem registro manual como confirmação recebida de outro sistema. Depois faturamento/contas a receber e PDV/fechamento, mantendo as demais lacunas da matriz.

Integração real continua exigindo versão conhecida do serviço, NOVO banco/empresa/loja e usuários/aparelhos descartáveis, licença/perfil/endpoint expressamente autorizados. Nenhum serviço, banco, estação ou chave escolhido automaticamente. Frontend116/117, backend e Electron preservados. Sem main/merge/force-push. Salvamento na mesma branch autorizado pelo proprietário. Reversão por diff/preimagens locais, sem apagar evolução posterior ou filas de operações.


## Decisão manual do fornecedor e consulta do ciclo — 24/10/2026, revisão31

Base a3b0f4d5fd011c2768eb12cda20141d27c015b0e, árvore limpa na abertura, pasta principal /home/danillo-malta/Downloads/zzeus-frontend-backend, branch feat/backend-checkout-authority. Regras relidas; nenhum AGENTS.md aplicável encontrado. Preimagens e SHA256 de LocalOrders.tsx e deste registro conferidos antes das respectivas escritas. Laboratório isolado work/supply-cycle-20261024 contém preparação, preimagens, hashes e logs, sem substituir o produto por cópias. Somente frontend e este registro alterados; sem instalação, ZIP, restauração geral, banco ou alteração de backend/Electron.

### Unidade aplicada

Novos frontend-web/src/core/supplyDecisions/{model.mjs,model.d.mts,source.mjs,source.d.mts,contracts.ts,schemas.mjs,words.mjs,words.d.mts}, pages/{SupplierDecisions.tsx,supplierDecisions.css} e tests/supplyDecisions.test.mjs; integração localizada em pages/LocalOrders.tsx. Sem novo router/menu ou ERP no Electron.

Em /local/orders, ao selecionar um pedido de um produto: consulta do ciclo, última decisão, quantidades do pedido inteiro (planejada, aceita efetiva, restante físico e alocada), entregas paginadas50 e estados registrados. Total de entregas é distinto do recorte visível; até1000,422não vira resumo parcial. Validação de IDs/ordenação/duplicidades, unidade/produto/fornecedor, soma exata dos totais e mínimo de alocação das entregas ativas da página. Contrato não suporta pedido de vários produtos: interface explica e não escolhe o primeiro silenciosamente.

Formulário de atendimento/recusa relatados MANUALMENTE: contato, referência da evidência, momento da comunicação, motivo, quantidade exata e prazo quando confirmado. Revisão explícita com operação original, revisão esperada, unidade e horários UTC antes do POST. Hora local/fuso efetivo visíveis; até3casas decimais em quantidade, sem ponto flutuante para converter; recusa exige exatamente0 e prazo vazio. Não permite atendimento maior que o pedido nem menor que alocação/aceite existente. Registro persistido continua evidence_mode=manual_external e external_verified=false: não é resposta autenticada do fornecedor, reserva, entrada de estoque ou pagamento.

Recebimentos legados podem avançar além do compromisso manual previamente registrado: essa divergência permanece consultável e recebe aviso, sem correção automática nem falso bloqueio de toda a fonte. Uma nova decisão precisa respeitar a quantidade já alocada/aceita. Pedido local cancelado bloqueia preparo; mudança posterior é revalidada no backend. Consulta prévia não substitui transação/autorização atômica no servidor.

### Contrato e proteção das operações

Referência somente lida em backend857febb/schema60: docs/api/purchase-supply-cycle.openapi.json, SHA2563dd081806200ecf44d214a002b65a148d9c59393f5b7c980984c79b27e247025. Conferidos backend/internal/localdb/supplycycle/{model,read,write}.go. Subconjunto tipado e validado em runtime: GET purchase-orders/{id}/supply-cycle, POST purchase-orders/{id}/supplier-decisions e GET purchase-cycle-operations/{operation_id}. Nenhum mock na rota real. Contrato presente não prova serviço atualizado em execução.

Leitura exige view_orders; gravação exige também manage_replenishment e licença Orders ativa, além de bloqueio entre abas disponível. Recuperação histórica no adaptador não exige renovar licença; acesso através de todas as guardas externas com licença expirada continua sem aceite demonstrado. Sem ampliar papel ou permissões. Recibo conferido contra corpo original, pedido, revisão e autor; escopo de aparelho/autor do GET original é responsabilidade verificada no contrato/implementação do backend, pois o DTO não inclui esses campos separadamente. Não inventa evidência de aparelho no retorno.

Fila durável titan-supplier-decisions:v1 por empresa/loja/usuário/aparelho. Cópia imutável da revisão e totais conhecidos, armazenamento verificado antes do envio, lock entre abas e marca incerta antes do POST. Reconsulta o recibo original antes de qualquer reenvio e confronta o ciclo revisado antes do POST. Reenvio somente explícito, preservando todos os identificadores e corpo. Descarte apenas da preparação comprovadamente não enviada, após consulta do recibo; resultado incerto nunca apagado automaticamente. Recibo original válido recupera mesmo após outra decisão mais recente. Token/sessão/capacidades e4IDs conferidos antes/depois do transporte e corpo; troca de sessão não autoriza POST antigo nem apaga fila. Outro aparelho consulta sua própria fila; renovação autorizada da mesma identidade/contexto pode recuperar a original.

Controle de consulta por canal, aborto e conferência após corpo impedem resposta antiga de substituir a nova. Prévia de envio usa canal distinto da consulta visual. Draft e resultado revistos prendem pedido/origem original, não os campos correntes de outra seleção. LocalOrders inclui novo draft/fila no bloqueio de navegação e nos bloqueios cruzados de mutação dos painéis existentes. beforeunload e retainDisposable preservam aviso e ciclo de vida StrictMode. Recibo confirmado é identificado como histórico e mostra pedido/unidade originais, mesmo se o estado atual mudou. Erro de consulta retira totais/decisão atuais anteriores; não inventa zero.

### Verificações executadas

57testes específicos aprovados: confirmação manual/recusa, revisões, milésimos exatos e limites UTF8, datas inválidas/futuras, alocação e recebimento legado, pedido cancelado/múltiplos produtos, divergência na prévia, perda de resposta, recuperação após avanço/renovação de sessão, reenvio explícito idêntico, descarte bloqueado de incerta, troca dos4IDs/logout/corpo tardio, permissões/licença, recibo adulterado/autor/escopo, concorrência, dispose, persistência/lock/fila duplicada, URL de ID preservado, erros403/409/422/503, páginas50+1, identidade/unidade/ordenação e5idiomas.

Suíte final node --test tests/*.test.mjs:2003aprovados,0falhas/ignorados, incluindo116/117 e demais entregas. TypeScript tsc --noEmit exit0, sem diagnósticos. Build final configFile:false/envFile:false exit0,23.64379541s, saída isolada work/catalog-management-20261022/build-check-sg9mQ2. Avisos preexistentes de Tailwind/chunks grandes permanecem; planilha aproximadamente6.09MB, sem promessa de ganho de desempenho. Primeira checagem TypeScript detectou fechamento JSX incorreto no novo painel (TS2657/TS1005); corrigido antes destas verificações finais.

Prévia explícita4351 com StrictMode, fetch sintético em memória e sem backend/proxy/banco: pedido10kg, atendimento manual8,125kg, conversão25/10,09h SãoPaulo→12hUTC; perda após registro preservou operação incerta, bloqueou descarte e recuperou recibo com1POST. Em seguida recusa manual válida passou a revisão2, quantidade0 e prazo vazio. Cinco idiomas percorridos, claro/escuro,503retirou resultado atual mas manteve recibo como histórico. Após ajustes finais de pedido/unidade, prévia recarregada e confirmação8,125kg revisada novamente. Prova outputs/fornecedor-decisao-manual-2026-10-24.png nesta tarefa. Isso não homologa serviço real, toda a rota sob todos os perfis, periféricos ou todas as combinações de acessibilidade.

### Próxima continuidade e limites reais

Conclui o registro manual e consulta descritos; NÃO conclui confirmação autenticada interempresa, frontend inteiro ou entrega desktop. Próxima unidade disponível no mesmo contrato: planejamento nominal de entregas e ações load/arrive/cancel, depois recebimento/obrigação documental vinculados. Este painel mostra essas entregas somente em consulta; não anuncia seus comandos como implementados. Preparação completa experimental do modelo no laboratório não é aplicação desses outros comandos no produto.

Pendência técnica identificada para coordenação do backend: Receive da entrega nominal exige accepted_milli>=1 em Go, apesar do mínimo0 no OpenAPI. Não existe ação terminal nominal de recusa integral nesse contrato; usar a recusa legada da mesma referência pode também impedir cancelamento nominal. Não simular fechamento/estoque/efeito financeiro nesses casos. Revisar e coordenar antes de conectar esse ramo; backend não alterado nesta unidade.

Permanecem faturamento/contas a receber, PDV/fechamento e demais pendências da matriz, além do aceite integrado. Ambiente necessário: versão conhecida do serviço, NOVO banco/empresa/loja, usuários e aparelhos descartáveis, licença assinada, perfil e endpoint expressamente autorizados. Nada escolhido automaticamente. Preservados116/117 e backend/Electron. Publicação autorizada na mesma branch, sem main/merge/force-push. Reversão por diffs/preimagens, sem apagar evolução posterior ou filas incertas.


## Ciclo de entrega identificada, recebimento e origem financeira — 24/10/2026, revisão32

Base498f5cce4df7899012d7054f43ba978a4b3779c8, árvore limpa na abertura na pasta principal e branch feat/backend-checkout-authority. Regras relidas; arquivos locais anteriores preservados,116/117 incluídos na regressão. Backend de referência permanece857febb588b522fdac45d3773e1b772094b19816/schema60 e limpo, somente leitura. Nenhuma instalação, banco, serviço real, ZIP, Electron ou mudança da main. Trabalho/laboratório em work/delivery-cycle-20261024, com preimagens/hashes dos arquivos preexistentes conferidos antes de escrever; sem copiar o produto inteiro.

### Arquivos e aplicação

Novos frontend-web/src/core/deliveryCycle/{model.mjs,model.d.mts,source.mjs,source.d.mts,contracts.ts,schemas.mjs,words.mjs,words.d.mts}, frontend-web/src/modules/catalog/pages/{DeliveryCycle.tsx,deliveryCycle.css} e frontend-web/tests/deliveryCycle.test.mjs. Alterações localizadas em pages/{LocalOrders.tsx,SupplierDecisions.tsx,Receiving.tsx,ReceivingOverview.tsx} e este registro. Painel adicional na rota existente /local/orders, sem router/menu paralelo. Atualização compartilhada por versão, sem remontar formulários, faz decisão, entrega, recebimento e conferência consultarem novamente após operação confirmada. Não usa cópia local como novo saldo oficial.

A fila legada de recebimento passa a compor o bloqueio de outras mutações mesmo sem campo editado; formulário aberto, erro de persistência e operação incerta também contam. Navegação/beforeunload e bloqueios cruzados incluem o novo painel. ReceivingOverview usa a retenção compartilhada do cliente, compatível com StrictMode. Conferência do código mostrou que o dispose antigo apenas cancelava consultas; NÃO foi comprovada falha permanente de acesso. A alteração padroniza o ciclo de vida e foi ensaiada como consulta/atualização em StrictMode, sem atribuir correção de bloqueio permanente inexistente.

### Fluxo funcional conectado

1. Planejar entrega com ID próprio e referência, quantidade exata, revisão da decisão manual, agendamento/evidência de janela opcionais e tratativa opcional de reposição. Somente com atendimento manual confirmado e quantidade ainda não alocada; não ultrapassa compromisso, restante físico ou1000entregas. Agendamento/evidência precisam vir juntos. O servidor revalida pedido, estado ativo da agenda, fornecedor e tratativa de replacement. Não cria outra reserva nem confirma comunicação externa.
2. Conferência de carga na saída, chegada e cancelamento com referência de evidência, momento observado e motivo. Transições planned→loaded→arrived; cancel permitido somente nos estados contratuais. Revisão e snapshot originais confrontados antes do POST, e backend revalida atomicamente pedido cancelado/estado/evidência legada. Não equivale a GPS, prova de entrega autenticada ou mudança automática da agenda.
3. Recebimento ligado à entrega chegada: local autorizado consultado, quantidades entregue/aceita, unidade e motivo. Milésimos exatos; aceita>=1, aceita<=entregue<=planejado da entrega. Local é reconsultado antes do POST; desaparecimento/incompatibilidade bloqueia a entrada. Nova consulta de locais conserva os outros campos do formulário e exige reseleção. O backend exige autorização prévia do pedido, permissão/licença de estoque e executa a entrada atômica. Recibo original liga entrega, pedido, produto, unidade, operador, aparelho e IDs de estoque. Aceite parcial libera o restante do compromisso sem contar a parte recusada como entrada.
4. Obrigação documental BRL ligada ao recibo: referência documental e de origem, emissão, descrição, total e1–120parcelas ordenadas com soma exata em centavos. Cria documento PENDENTE para revisão independente; NÃO paga, liquida ou autoriza documento fiscal. Criação financeira não incrementa revisão da entrega nem entra no histórico de etapas físicas. IDs originais das parcelas, documento e operação permanecem na revisão/fila/recibo.
5. Consulta da entrega e histórico50porpágina, origens imutáveis/estados/revisões/quantidades validados. Consulta financeira por ID da obrigação usa finance/receiving-origins e diferencia review_required da revisão própria do documento. Origem histórica da criação não é consulta vigente; fiscal_authorization_verified permanecefalse. Um estorno posterior pode exigir revisão da origem atual sem adulterar o recibo original histórico.

### Contratos, autorização e recuperação

Mesmo purchase-supply-cycle.openapi.json da revisão31, SHA2563dd081806200ecf44d214a002b65a148d9c59393f5b7c980984c79b27e247025, comparado a backend/internal/localdb/supplycycle/{model,read,write}.go e localapi/supply_cycle.go. Endpoints: POST purchase-orders/{id}/deliveries; GET purchase-deliveries/{id}; GET history; POST actions/receive/payable; GET purchase-cycle-operations/{id}; GET finance/receiving-origins/{id}. GET locations usa formato já consumido pelo cliente oficial de recebimento; array, IDs únicos, nomes e tipos conferidos. Fonte indisponível/limite é erro explícito, nunca fonte demonstrativa ou total zero inventado na rota real.

Leitura de entregas exige view_orders. Plan/etapas exigem manage_replenishment+Orders ativa; recebimento também manage_stock+Core/Inventory. Financeiro exige view_accounting+manage_cash e Finance ativa. A rota de Pedidos ainda exige seu acesso externo e o adaptador mantém view_orders; não foi criada permissão especial para operador financeiro sem acesso a Pedidos. Histórico/recuperação no adaptador não exigem renovar licença, mas aceite através de todas as guardas com licença expirada permanece pendente. Backend continua autoridade das permissões/transações; nenhum perfil ampliado.

Fila titan-delivery-cycle:v1 separada por empresa/loja/usuário/aparelho, armazenamento verificado e lock entre abas. Copia origem/revisão/corpo, grava antes do POST e marca incerteza antes do transporte. Procura recibo original antes de reenvio; prévia reconsulta compromisso/ID livre ou snapshot da entrega e, no recebimento, local. Confere recibo contra o comando original e histórico imutável da etapa; financeiro confere documento/origem, sem exigir que o estado atual seja igual ao resultado antigo. GET original é vinculado no backend ao autor/aparelho; esse escopo não é inventado no DTO que não o contém.

Reenvio somente explícito com mesmo corpo e IDs. Primeira recusa HTTP comprovada400/403/404/409/413/422pode conservar preparação não enviada para descarte explícito após consulta do recibo. Erro de sessão/permissão gerado no cliente depois de POST não é tratado como recusa do servidor. Uma recusa posterior jamais reduz a incerteza de tentativa anterior. Falha de resposta, recibo divergente, histórico ausente ou mudança de contexto não apagam fila. Snapshot diferente em storage bloqueia limpeza. Renovação autorizada da mesma origem recupera a original; outro aparelho encontra fila própria. Contexto/token/capacidades conferidos antes/depois do transporte/corpo; canais de consulta e recuperação separados, resposta antiga perde mesmo se o transporte ignorar aborto.

Interface preserva5idiomas, claro/escuro, controles44px, rolagem interna, foco no primeiro campo e na revisão, campos/IDs/horários/unidades explícitos. Mudança de formulário remove revisão anterior, não altera tentativa incerta. Renderização textual React, sem interpretar conteúdo documental como HTML. Dados usados nos ensaios são exclusivamente sintéticos.

### Verificação real desta unidade

75testes específicos aprovados: percurso completo planejada/carga/chegada/parcial/obrigação pendente; cancelamento e alocação disjunta; revisão e ID concorrentes; janela/tratativa; milésimos e centavos atéMAX_SAFE_INTEGER; parcelas/datas; recebimento integralmente recusado bloqueado; perda de resposta por tipo, recuperação histórica e origem com revisão posterior; IDs/reenvio idênticos; quatro contextos/logout/corpo tardio; licenças/permissões separadas; recusa HTTP inicial versus incerteza anterior; ator/aparelho e recibos adulterados; histórico, concorrência, fila, storage/locks, renovação/outro aparelho, locais desaparecidos/duplicados e5idiomas. Primeiro cenário de licença expirou após já alocar6kg; o teste foi corrigido para tentar somente4kg disponíveis, isolando a condição de autorização sem mudar a regra comercial.

Suíte final node --test tests/*.test.mjs:2078aprovados,0falhas/ignorados, incluindo116/117. TypeScript tsc --noEmit exit0 sem diagnósticos. Build final configFile:false/envFile:false exit0,24.545066895s, saída isolada work/catalog-management-20261022/build-check-uIjZvg. Avisos anteriores de Tailwind/chunks grandes permanecem; planilha aproximadamente6.09MB. Nenhum ganho de desempenho ou CI remoto verde foi presumido.

Ensaio de navegador4352, StrictMode e fetch em memória, sem backend/proxy/banco: planejamento6kg, carga, chegada com perda da resposta e recuperação mantendo3POST, aceite5kg sobre6entregues, totais do pedido10/5/5 e alocação5atualizados também no painel de decisão; documento31,50BRL com parcelas15,00+16,50,5POSTao final, origem ligada ao recibo e entrega, sem pagamento/fiscal e revisão da entrega preservada4. Cinco idiomas percorridos e temas claro/escuro inspecionados. Após finalizar seleção/reconsulta de locais e foco/rolagem, o recebimento foi refeito na versão final: reconsulta conservou entregue6/aceito5, exigiu selecionar novamente LOCATION e confirmou com4POSTincluindo as etapas da fixture. A fixture de preparação descartável foi claramente separada das ações do produto.

Prévia separada guards.html: fila legada incerta tornou outra mutação desabilitada; alternar somente a fixture entre ausente/presente liberou/bloqueou o controle sem POST. Consulta independente5kg/2aceitos permaneceu funcionando em StrictMode, contador2na montagem e3após versão externa. Erros iniciais dos preparadores (closure apontando para adaptador descartado e ausência de MemoryRouter no ensaio de guardas) foram corrigidos somente no laboratório antes dos resultados acima; não foram apresentados como falhas do produto ou sucesso prévio.

Provas outputs/entrega-origem-financeira-2026-10-24.png e outputs/entrega-recebimento-2026-10-24.png nesta tarefa. Ensaios não equivalem a integração real, persistência de servidor, isolamento entre bancos, hardware, toda a navegação ou todas as combinações de acessibilidade/perfis.

### Pendências e continuidade

Mantida a divergência contratual identificada na revisão31: Receive nominal exige aceita>=1no Go e não existe transição terminal de recusa integral; o OpenAPI declara mínimo0. Formulário explica e bloqueia a simulação por aceite zero/cancelamento. Não aplicar a recusa legada à mesma referência presumindo que encerra a entrega, pois pode impedir cancelamento nominal. Coordenar esse ramo com o responsável pelo backend, sem alterar seu código nesta tarefa.

O fluxo manual/local descrito está integrado no frontend, mas confirmação autenticada interempresa, circuito de coleta/troca, liquidação e fiscal não foram concluídos por ele. Faturamento/contas a receber e PDV/fechamento continuam próximos no roteiro, além dos demais módulos da matriz. Identificados também contratos existentes cash-journal/cash-bags/cash-vaults e supplier-returns na referência: presença ainda não significa integração ou ausência de lacunas; revisar código/contratos/testes antes de implementar. Não anunciar frontend/sistema completo nem percentual sem aceite medido.

Integração real exige versão conhecida, NOVO banco/empresa/loja, usuários/aparelhos e dados descartáveis, licença assinada, perfil/endpoint expressamente autorizados. Nenhum serviço, banco, estação ou chave escolhido automaticamente. Frontend116/117 e backend/Electron preservados. Salvamento na mesma branch autorizado pelo proprietário, sem main/merge/force-push. Reversão localizada por preimagens/diff, sem apagar evolução posterior nem filas incertas.


## Backend autorizado e correção do escopo financeiro — 24/10/2026, revisão33

### Mudança de responsabilidade e base real

O proprietário pediu explicitamente desenvolver o backend por completo. As orientações antigas para este chat não alterar backend deixam de limitar o desenvolvimento; continuam válidas a preservação dos trabalhos locais, proibição de banco comercial, segredos e migrações destrutivas, e a necessidade de informar limites reais. Não pausar implementações independentes por falta de serviço comercial. Não interpretar autorização de desenvolvimento como identificação de uma estação/banco/licença operacional.

Frontend principal permanece em ~/Downloads/zzeus-frontend-backend/frontend-web, branch feat/backend-checkout-authority; backend trabalhado em ~/Downloads/zzeus-frontend-backend-b02a/backend, branch feat/backend-b02a-compat. Ao abrir esta unidade, backend já estava em51cbf3cb4c20d3954202bd5fc9b534a74b1c9597, um commit local à frente de857febb, com persistência/recuperação de carrinhos e fechamento em dinheiro feitos anteriormente. Esse trabalho não é atribuído a esta unidade. Também havia alterações legítimas de manutenção interna: API/fleet, backup, testes, inventário de rotas, migração0062, cadastros/fila e documentação; frontend tinha router/menu/frota e novos módulos de manutenção em edição. Todos preservados, sem sobrescrever nem incluí-los no commit da correção financeira.

### Falha reproduzida e mudança aplicada

CreatePayable chamava ReceiptEvidenceTx, que exigia view_orders+manage_stock para ler a evidência. O contrato financeiro exige view_accounting+manage_cash e Finance ativa, sem conceder movimento de estoque. Um funcionário com exatamente os dois direitos financeiros recebeu403ao criar uma obrigação válida; até um recibo estornado retornou403antes de permitir avaliar a origem. Esses dois casos falharam antes da correção, registrados em work/backend-finance-scope-20261024/reproduction.jsonl.

Nova leitura ReceiptForPayableTx exige autorização financeira de leitura e contexto/aparelho válidos; usa a MESMA verificação de recibo, movimento físico, auditoria e estorno dentro da transação. O leitor de estoque original mantém seus requisitos. CreatePayable continua exigindo contrato Finance assinado e manage_cash; cria obrigação pendente e preserva revisão independente. Não amplia permissão de estoque, não paga e não liquida. Estorno, evidência danificada, escopo estrangeiro e falta de cada direito financeiro continuam bloqueando a operação, sem escrita parcial. Recuperação e reenvio mantêm o autor, dispositivo e IDs originais.

Arquivos backend desta unidade: backend/internal/localdb/purchases/receipt_evidence.go, backend/internal/localdb/supplycycle/write.go, novo backend/internal/localapi/supply_cycle_finance_scope_test.go e docs/api/purchase-supply-cycle.openapi.json. Sem nova migração ou alteração da manutenção em andamento. Preimagens/hashes conferidos antes de escrever. Commit publicado e hash remoto conferido:9d9f2817a5afaf6bfe5272d6d9a1b1750cf7bfe3 na branch feat/backend-b02a-compat. A publicação preservou também a base51cbf3cjá existente; não representa reimplementação desse trabalho.

Contrato de ReceiveInput alinhado ao Go: accepted_milli tem mínimo1. Isso corrige a especificação, NÃO implementa recusa integral nominal. Novo SHA256de purchase-supply-cycle.openapi.json:f5e54c6fa79c5e841037629fa381dc7589f67d60a5496751156473aee225b597. Snapshot frontend core/deliveryCycle/schemas.mjs alinhado somente nesse input; interface já exigia aceite positivo. Demais telas/rotas de manutenção locais não foram alteradas por esta correção.

### Evidências e limites dos testes

Go do sistema era1.24.3, abaixo do go.mod1.25. Foi localizado e usado o Go1.25.8 já em cache, sem instalar nem baixar toolchain/dependências: GOTOOLCHAIN=local, GOPROXY=off e GOSUMDB=off. A primeira tentativa de redirecionamento falhou antes de executar testes porque o caminho de logs ainda não existia; corrigido e executado de fato depois. Baseline do ciclo:29eventos de aprovação, sem falhas. Cinco funções de regressão novas, incluindo dois subcasos, verificam funcionário financeiro sem direitos de estoque/reposição/pedidos, idempotência, autor da recuperação, ausência de movimento de estoque/dinheiro, falta de cada direito, estorno,4limites de contexto e contrato/API de aceite zero.

Suíte completa real go test ./... -count=1 -json, na árvore backend incluindo a manutenção local preservada:exit0;51pacotes aprovados,2.374casos finais aprovados,2.611eventos de aprovação contando agrupadores/subtestes,0falhas. Quatro skips registrados: dois cenários PostgreSQL sem URL descartável configurada; TestMigrationCrashChild, auxiliar de subprocesso que só roda pelo pai; e ponte opcional de manutenção. A ponte foi executada separadamente com sua fixture Go revisada e passou: cliente frontend real, sessão e licença sintéticas, API HTTP em127.0.0.1/porta efêmera, SQLite emt.TempDir, quatro cadastros, candidatos, fila, histórico e idempotência. Nenhum serviço/banco comercial, segredo ou licença real usado. Esse teste local não publica o trabalho de manutenção que permanece em edição.

Para comprovar que o commit financeiro não depende dessas alterações não commitadas, seus arquivos versionados backend/docs foram exportados para work/backend-finance-scope-20261024/verified-source, excluindo frontend, .env*, chaves e bancos. Nenhum ZIP ou pacote de entrega foi gerado. Nesse snapshot limpo do commit9d9f281,36eventos de aprovação do ciclo passaram sem falhas. Binários cmd/titan-local e cmd/api compilaram em saída isolada work/backend-finance-scope-20261024/build. Primeira compilação da fonte exportada falhou com “error obtaining VCS status: exit status128”; buildvcs=false removeu apenas a identificação Git automática dessa cópia, mantendo o hash registrado separadamente. Ambos builds finais exit0; binários não foram executados nem anunciados como instalador desktop.

Clientes frontend:132testes de deliveryCycle e supplyDecisions aprovados,0falhas após alinhamento do snapshot. Não se atribui nova suíte global/TypeScript/build frontend nesta unidade, nem CI remoto verde. As2.078regressões globais e TS/build da revisão32 são evidência anterior, não reexecução das alterações de manutenção feitas por outra frente.

### Continuidade obrigatória

Backend inteiro NÃO está concluído por esta correção. Recusa integral da entrega nominal continua exigindo operação terminal atômica, histórico/idempotência, compatibilidade de migração e atualização do cliente; não substituir por aceita0 ou cancelamento fictício. A migração0062de manutenção está em andamento e precisa ser preservada/compatibilizada antes de escolher numeração de outra entrega. Faturamento/contas a receber, pagamentos/fechamento e demais contratos da matriz ainda precisam de revisão factual por código/testes, sem reimplementar o que já está presente. Ponte de manutenção validada é evidência de um subfluxo, não aceite de toda a interface ou módulo.

A nova responsabilidade autoriza continuar backend e frontend onde necessário, com alterações revisadas e unidades testadas. Persistem as pendências de validação PostgreSQL, serviço operacional/ambiente descartável identificado, integrações externas e empacotamento conhecido Windows/Linux. Não iniciar banco real, escolher credenciais/estação por conta própria, fazer merge/main/reset/clean ou atribuir percentual de conclusão sem aceite medido. Registro único continua sendo este arquivo.


## Consolidação de manutenção e recusa integral identificada — 24/10/2026, revisão34

### Bases e preservação

Frontend base16fd22946d441ef05828ab098e7820c914e17aa3, feat/backend-checkout-authority; backend base9d9f2817a5afaf6bfe5272d6d9a1b1750cf7bfe3, feat/backend-b02a-compat, nas mesmas duas pastas Downloads. A autorização do proprietário inclui desenvolvimento do backend e publicação normal nas branches. Não houve checkout/main, merge, force-push, instalação, ZIP, Electron, serviço operacional ou banco comercial. Continuam proibidos segredos, alterações destrutivas e resultados comerciais simulados. Nenhuma URL PostgreSQL descartável configurada foi escolhida; os dois testes opcionais correspondentes continuam sem executar.

Os18arquivos de manutenção já preparados localmente foram revisados; hashes foram conferidos imediatamente antes do stage. Consolidação publicada306005bb7fd04327d1bfae4b0224378186388f28, com hash remoto conferido. Essa consolidação não atribui a esta unidade a autoria de todo o código anterior. Inclui quatro cadastros (especialidades, técnicos, perfis de bens e classificação), fila/candidatos, histórico/recuperação,23operações e SQL0062. Bytes da migração0062preservados, SHA2562b12d9a69d49a4e35d84bbcac1198b3a7b576132f03ef5134f548349e5964bb8. Nenhuma migração SQL histórica alterada. A descrição de origem MANUTENCAO_INTERNA_20261024.md é datada do pacote e menciona posição61; a posição REAL aplicada nos fontes é62, após carrinhos61. A referência de estado atual é este registro.

As alterações frontend preexistentes em router/menu/Fleet85/FleetOfficial e novos core/internalMaintenance, modules/maintenance e testes permanecem separadas, não incluídas no commit desta compatibilidade. Esse catálogo local usa pt/en/es/fr/de: falta zh-CN para cumprir os cinco idiomas obrigatórios do produto. A ponte passando não homologa toda essa interface, sua acessibilidade, notificações móveis ou idiomas. Não apagar nem substituir o material para resolver essa pendência.

### Operação entregue no backend

POST /local/v1/purchase-deliveries/{id}/reject. Aceita somente entrega arrived, revisão exata, quantidade entregue positiva até a planejada, evidência, momento observado e motivo. Exige view_orders+manage_replenishment+manage_stock e módulos Orders/Inventory válidos, sempre no contexto autorizado da sessão. existing_rejection_id vazio cria a recusa legada e o encerramento na MESMA transação. Um ID explícito vincula apenas recusa original conferida com mesmo pedido, referência, produto, unidade e quantidade, preservando autor/aparelho/motivo originais. O vínculo tem seu próprio autor/aparelho/evidência; não duplica registro físico nem outbox da recusa antiga.

Estado lógico rejected, revisão seguinte, histórico e recibo da operação com IDs originais. Libera a alocação da entrega para futura decisão humana, sem reduzir o restante físico do pedido. Não cancela o pedido, não cria reposição automática, entrada de estoque, pagamento, crédito ou documento fiscal. Entrega encerrada não pode ser recebida/cancelada ficticiamente nem usada para retirar devoluções. Concorrência com recebimento produz um vencedor e um conflito, nunca dois efeitos. Todos os inserts verificados por RowsAffected; falha até no evento final reverte recusa, vínculo, operações e outbox.

Migração NOVA0063_purchase_delivery_full_rejection.sql, sem reconstruir tabelas nem reescrever0057/0062. Ledger terminal imutável com chaves compostas, referência diferida à operação e trigger que confere a origem. A linha histórica purchase_deliveries continua arrived e congelada: consumidores DEVEM usar a leitura que verifica e sobrepõe o ledger. O leitor confronta snapshot, operação/evento e evidência legada. O consumidor SQL de coletas foi revisado e bloqueia o ledger. Não consultar a linha física isolada como estado atual. Não usar binário antigo para operar schema63; reversão deve preservar o banco e usar combinação de fontes compatível, nunca apagar ledger ou migração.

Arquivos: localapi/supply_cycle.go e novo supply_cycle_full_rejection_test.go; localdb/supplycycle/{model,read,reject}.go; purchases/{rejections,supplier_return_write}.go; nova SQL0063; novo full_rejection_migration_test.go; guards de versão em localdb_test/migrate_safety_test/journal_migration_test e backup/{backup.go,compatibility_test.go}; docs/api/purchase-supply-cycle.openapi.json, route-inventory.json e routes.openapi.json. Recusa legada foi extraída para helper de transação mantendo o endpoint antigo, licenças, autorização de recebimento, auditoria e idempotência. Commit backend desta unidade:7b3796662c9e455c446c3756b19807c486ff694c. Contrato SHA256:9fab8a47c0f7d4237fc07b9273ccd3a64e7def14a798aefcdeb632f4dbf3eab9.

### Compatibilidade do frontend

Atualizados snapshots/tipos/modelos de deliveryCycle e supplyDecisions, labels nos cinco idiomas pt-BR/en/es/fr/zh-CN, histórico e testes de deliveryCycle. A leitura reconhece rejected, não conta essa entrega como alocação ativa, não oferece ações posteriores e confere as duas autorias/evidências. Histórico rejeita origem/quantidade/revisão alterada ou efeito misturado e valida página sem revisões anteriores carregadas. A tela mostra referência/momento/operador e recusa original, sem tratar como entrada ou pagamento.

Nesta unidade NÃO foi habilitado o comando reject no formulário/fila persistente de frontend: a orientação na tela informa isso. O endpoint está funcional/testado no backend; o operador ainda não tem esse envio pelo painel. Próxima integração precisa acrescentar comando, revisão humana, fila/recuperação e reenvio explícito com proteção de sessão/contexto já existente, incluindo vínculo legado deliberado. Não anunciar o ramo inteiro como pronto ao operador somente pela existência da API. Nenhuma fila incerta existente foi reescrita ou descartada.

### Verificação real

Go1.25.8 previamente em cache, GOTOOLCHAIN=local/GOPROXY=off/GOSUMDB=off, sem instalação. Baseline do ciclo com a nova estrutura passou. Novos cenários completos: 20eventos de aprovação, 17casos finais,0falhas. Incluem recusa nova/legada com autores diferentes, permissões individuais, isolamento de4IDs, dados inválidos, revisão/quantidade, duplicidade, concorrência, rollback nos6pontos, integridade, backup, liberação de alocação e bloqueio de coleta. Evidência legada corrompida bloqueia leitura corrente409, mas preserva o recibo histórico original recuperável. Esse último teste foi acrescentado após iniciar a suíte completa e passou na rodada específica final; não incluí-lo implicitamente na contagem da execução completa anterior.

Suíte completa go test ./... -count=1 -json: 51pacotes aprovados, 2394casos finais, 2634eventos de aprovação contando agrupadores,0falhas. Skips de testes: TestPostgresCatalogCreationAndDiscountAtomicity, TestMigrationCrashChild, TestPostgresDurableRotationConcurrencyAndRollback, TestMaintenanceFrontendBridge. TestMigrationCrashChild é auxiliar executado pelo pai; PostgreSQL permanece sem banco descartável autorizado. TestMaintenanceFrontendBridge foi executado SEPARADAMENTE após schema63 e passou: cliente frontend real, API HTTP loopback efêmera, SQLite/licença/sessão/dados sintéticos em temporários. Processo encerrado pela fixture. Não equivale a serviço comercial implantado.

Rodada migração/backup: 55casos finais aprovados,0falhas; restaura snapshots históricos25..63, confirma que a origem não mudou, migração repetida e dado sentinela preservado. cmd/titan-local e cmd/api compilaram exit0 em work/backend-full-rejection-20261024/build; binários NÃO executados ou instalados. Logs completos/results.json/preimagens/hashes no mesmo laboratório da tarefa.

Frontend:137testes deliveryCycle+supplyDecisions aprovados,0falhas; suíte global2.117aprovados,0falhas/ignorados, incluindo116/117 e a manutenção local preservada. TypeScript final exit0. Build isolado configFile:false/envFile:false exit0,24.913815645s em work/catalog-management-20261022/build-check-vICIy6. A união TypeScript de Delivery foi completada na revisão final e o typecheck repetido; tipos não alteram JS gerado. Avisos preexistentes Tailwind/chunks grandes permanecem; chunk da planilha6.093.356bytes. Não é medição em computador limitado ou homologação visual/operacional de todos os fluxos.

Primeira rodada específica encontrou erro da FIXTURE: instalação de licença Core+Orders retornou400, pois dependências eram inválidas; corrigida a fixture para licença válida sem Orders e verificados também os3direitos individuais. Uma chamada gofmt usou caminho duplicado backend/ e falhou; foi corrigida/executada. A preparação de teste frontend usou caminho relativo de laboratório inexistente e falhou antes de acrescentar os testes; os5casos novos foram depois aplicados no caminho certo e executados de fato. Não apresentar essas tentativas como sucesso.

### Continuidade

Backend inteiro/ERP inteiro NÃO concluído por essas unidades. Permanecem formulário de recusa integral no frontend, homologação de manutenção/locales, revisão factual de faturamento/recebíveis/PDV-fechamento e demais contratos da matriz, além de validação PostgreSQL e integrações externas. Não recriar117de fornecedores nem aplicar os rascunhos workflows117/production118. Combinação operacional deve identificar os DOIS commits de frontend/backend, licença, empresa/loja/usuários/aparelhos descartáveis e perfil/endpoint autorizado. Nenhum serviço atual foi identificado ou atualizado por compilar fontes. Electron, empacotamento Windows/Linux, móvel e periféricos seguem aceites próprios. Manter este registro único, sem porcentagem inventada.


## Tela preta na inicialização local — 24/10/2026, revisão35

Base frontend35060a022694e3d70680029d0f63cacd86abaa89, branch feat/backend-checkout-authority. Investigada a instalação efetivamente servida em http://127.0.0.1:3002/local/login. O navegador reproduziu página vazia e ReferenceError: Cannot access 'React' before initialization, em src/core/router/index.tsx:18:26 transformado pelo Vite. A declaração LocalMaintenance = React.lazy estava na primeira linha do fonte, antes do import de React, na integração local de manutenção ainda não publicada. Não era ausência de backend nem problema das cores do tema.

Correção localizada somente em frontend-web/src/core/router/index.tsx: mover essa declaração para o grupo de páginas lazy, depois dos imports. Preservados o componente, a rota maintenance e todas as outras alterações locais. Preimagem/hash e diff em work/login-black-screen-20261024 nesta tarefa; sem restauração do arquivo inteiro, instalação, backend, banco ou Electron. O arquivo continua compondo a integração local de manutenção; não publicar esse router isoladamente com referência a módulo ainda não versionado. Nenhum commit/push nesta correção local.

Verificação real: recarga no navegador da URL original passou de tela vazia para formulário Bem-vindo ao Titan, usuário/senha e Entrar. Uma segunda aba nova da MESMA URL confirmou inicialização limpa, com zero erros de console; não confundir o erro histórico retido na primeira aba com nova falha. TypeScript tsc --noEmit exit0. git diff --check exit0. Evidência outputs/login-corrigido-2026-10-24.png nesta tarefa. Não submetido login, não usados dados/credenciais ou sessão comercial. Isso comprova a recuperação da renderização de login, não homologação dos fluxos autenticados nem de toda manutenção.


## Recusa integral operável e manutenção consolidada — 24/10/2026, revisão36

### Bases, preservação e publicação

Frontend principal ~/Downloads/zzeus-frontend-backend, branch feat/backend-checkout-authority, base35060a022694e3d70680029d0f63cacd86abaa89. Backend de referência ~/Downloads/zzeus-frontend-backend-b02a, feat/backend-b02a-compat, regra/API de recusa no7b3796662c9e455c446c3756b19807c486ff694c e teste integrado adicional publicado73fec1f7fa4254ea26ede78099f0111a56edb4e8. Hash remoto conferido. Publicação normal autorizada pelo proprietário; sem main, merge, force-push, instalação, ZIP, serviço comercial, banco real ou Electron.

A manutenção frontend já estava preparada e não publicada: core/internalMaintenance, modules/maintenance, router/menu/Fleet85/FleetOfficial e dois testes. Foi lida/revisada e preservada, sem reimplementar o pacote nem executar aplicadores antigos. Esta unidade corrige suas lacunas de idiomas, sessão/recuperação e navegação e a consolida com os fontes. A declaração LocalMaintenance permanece depois dos imports: preserva a correção REAL da tela preta documentada na revisão35, agora incluindo o componente dependente na mesma publicação. Não usar o router de forma isolada. Preimagens/hashes específicos nos laboratórios work/delivery-refusal-ui-20261024, work/maintenance-ui-review-20261024 e work/login-black-screen-20261024 desta tarefa. Nenhuma restauração geral ou exclusão de trabalho local.

### Recusa integral concluída neste fluxo

Commit frontend publicado e hash remoto conferido:7db4cf8d2c65fc5515e853a1a42401f502f2d424. Arquivos core/deliveryCycle/{model.mjs,source.mjs,source.d.mts,words.mjs}, modules/catalog/pages/DeliveryCycle.tsx, tests/deliveryCycle.test.mjs e novo tests/deliveryCycle.bridge.mjs.

A entrega arrived oferece Registrar recusa integral, separada de receber e cancelar. Formulário pede quantidade entregue positiva até a planejada, referência de evidência, momento local convertido a UTC, motivo e ID de recusa legada opcional/explícito. Revisão humana mostra quantidade/unidade, origem, revisão e IDs originais antes do POST. Campo vazio cria recusa; ID informado vincula a evidência existente, cuja identidade/quantidade/unidade/origem são conferidas pelo backend. Não cria estoque, obrigação, pagamento, cancelamento do pedido, nota fiscal ou reposição automática. Novo estado rejected não oferece mutações posteriores; alocação é liberada para nova decisão humana. Backend versão63 é necessário; endpoint ausente não vira sucesso nem recusa simulada.

Comando reject integra a fila v1 JÁ existente, sem reescrever preparações/incertezas anteriores. Captura empresa/loja/usuário/aparelho, contexto e revisão, preserva antes do transporte, consulta recibo original antes de enviar e revalida snapshot na prévia. Exige direitos de pedidos/reposição/estoque e módulos Core/Orders/Inventory. Apenas primeira recusa definitiva de servidor permite marcar não enviada; perda/timeout, corpo tardio, troca de sessão/permissão e erro de cliente não apagam nem desclassificam incerteza. Reenvio é explícito, com mesmo corpo/IDs. Recibo deve concordar com comando, autor/aparelho e histórico imutável, inclusive evidência/rejeição; autor de recusa legada pode ser diferente do autor do novo vínculo, mas somente mediante ID explícito. Sessão encerrada ou qualquer um dos4IDs alterado impede POST/limpeza tardios.

113testes de deliveryCycle aprovados,0falhas. Incluem recusa nova/legada, campos inválidos, estados, cada direito/módulo, perda antes/depois, recuperação após renovar cliente, reenvio idêntico, corpo tardio,4IDs, revisão alterada, recibo/histórico adulterados e licença expirada permitindo só histórico. A ponte Go TestDeliveryFrontendBridge foi adicionada no backend e executada DE FATO com Node/cliente frontend real contra Fiber HTTP loopback/porta efêmera, SQLite t.TempDir, sessão e licença assinadas sintéticas. Perda artificial APÓS resposta real do servidor preservou incerteza; recuperação confirmou rejected e histórico4,0alocação/entrada financeira/física e somente1POST. Não é banco/serviço operacional ou teste de hardware.

Prévia4352, StrictMode e dados explicitamente sintéticos: formulário de6kg, revisão3,15h SãoPaulo convertido18hUTC; perda após registro manteve descarte e novas ações bloqueados. Consultar recibo recuperou revisão4 e estado recusado com POSTs totais4 (3de preparação e1de recusa), sem reenvio. Interface percorreu5idiomas e temas. Prova outputs/recusa-integral-recuperada-2026-10-24.png. Esses dados da prévia não são persistência oficial do servidor; a ponte HTTP acima é evidência separada.

### Manutenção revisada e consolidada

Arquivos: core/internalMaintenance/{client.mjs,client.d.mts,words.mjs,words.d.mts}, modules/maintenance/pages/{LocalMaintenance.tsx,maintenance.css}, router/index.tsx, tenant/pages/LocalMainMenu.tsx, tenant/pages/business85/Fleet85.tsx, tenant/pages/fleetOfficial/FleetOfficial.tsx e tests/{internalMaintenance.test.mjs,internalMaintenance.bridge.mjs}. A publicação inclui a implementação anterior revisada, com sua origem preservada; não atribuir toda a manutenção a uma nova reimplementação desta revisão.

/local/maintenance e o acesso no menu/frota estão conectados aos4cadastros oficiais: especialidades, técnicos já autorizados, perfil dos bens e classificação dos pedidos. Fila por especialidade/técnico, total/paginação, histórico e recuperação usam o contrato existente. Execução e conferência técnica reutilizam FleetOfficial; não há interface comercial duplicada. Pedido e classificação permanecem DUAS operações explícitas; não prometer atomicidade entre elas. Diretório escrito pela gerência não concede permissão nem certificação profissional. Cadastro técnico não revoga login quando inativado. Licenças/perfis continuam revalidados no backend.

Incluído zh para resolver zh-CN, mantendo pt/en/es/fr e o alemão anterior:5idiomas obrigatórios MAIS a tradução alemã preservada. Revisão e histórico têm rótulos legíveis em vez de JSON de transporte; IDs/revisões continuam visíveis por serem necessários à recuperação e decisão. Foco vai ao primeiro controle e à revisão; controles touch/foco e rolagem interna preservados. Isso não homologa todas as combinações de acessibilidade, telefones e teclado de toda frota.

Cliente normaliza mudança de contexto como401, confere sessão antes/depois da resposta e do corpo, abort/timeout, tamanho de corpo e dados persistidos. Consulta antiga não pode autorizar POST. GET de recibo segue primeiro, erro de consulta não envia. Novo tratamento de recusa definitiva diferencia a primeira4xx REAL da incerteza já anterior; erro local pós-POST não a apaga. subscribe notifica o painel após persistência/limpeza e permite hidratar pendência já existente; evento storage considera outra aba. Pendência/erro de armazenamento participa do bloqueio de saída e impede voltar à execução para iniciar novas ações enquanto a recuperação do diretório está pendente. Formulário usa latch para evitar dois envios simultâneos; locks e checagem da fila continuam obrigatórios.

Histórico verifica ID, recurso, sequência de revisões/página, ações e estados; listas rejeitam IDs duplicados e páginas silenciosamente incompletas. Resultado parcial/indisponível não vira total oficial.48testes específicos de manutenção aprovados,0falhas. A ponte TestMaintenanceFrontendBridge passou novamente DEPOIS da revisão do cliente:4recursos, candidatos, fila, histórico e idempotência no backend real descartável. Não foi necessário selecionar banco, aparelho, chave ou perfil comercial.

Prévia4353, componente real e sessão sintética em memória, sem proxy/backend: especialidade revisada, perda de resposta preservou tentativa, desabilitou execução e descarte; consulta de recibo recuperou cadastro, linha/revisão1e liberou novo formulário. Idiomas pt-BR/en/es/fr/zh-CN percorridos, chinês verificado sem fallback para inglês, tema real do produto. A área de execução permanece explicitamente indisponível na prévia porque não é mockada como sucesso. Revisão final mostrada com campos humanos, prova outputs/manutencao-revisao-2026-10-24.png. Nenhuma credencial real digitada e nenhum login comercial submetido.

### Qualidade e limites reais

Suíte global final node --test tests/*.test.mjs:2.164aprovados,0falhas/cancelados/ignorados, incluindo116/117 e as demais entregas. TypeScript final tsc --noEmit exit0. Build final configFile:false/envFile:false exit0, 23.075366991s, saída isolada /home/danillo-malta/Documents/Codex/2026-10-20/github-plugin-github-openai-curated-remote-2/work/catalog-management-20261022/build-check-kTnR9B. Não copiado para dist do usuário. Avisos preexistentes Tailwind/chunks grandes continuam; planilha6.093.356bytes, sem alegar otimização ou ensaio em hardware limitado. git diff --check passou. Não se atribui nova suíte completa Go de2.394casos a esta unidade: esse resultado é da revisão34. Agora foram executadas as DUAS pontes específicas; backend de negócio não foi alterado aqui, apenas teste opcional novo.

Falhas de preparação foram corrigidas antes dos resultados: primeira ponte executada fora do módulo retornou exatamente “go: go.mod file not found in current directory or any parent directory; see 'go help modules'”; a primeira execução Node da ponte falhou com “SyntaxError: Unexpected token ';'” no ensaio, corrigida sua declaração de objeto. Quatro regressões novas de contexto encontraram exceção genérica “Error: context”; o cliente foi corrigido para negar com401, mantendo armazenamento e número de POSTs. Leituras auxiliares que usaram caminho relativo de laboratório na raiz do produto falharam “No such file or directory”; os resultados foram depois lidos nos caminhos corretos. Controle de data da ferramenta visual inicialmente expirou a espera; valor foi conferido e preenchido pelo controle acessível antes da revisão. Não atribuir essas falhas ao backend comercial nem anunciar tentativa falha como aprovação.

### Próxima continuidade

Esta revisão fecha os fluxos de recusa integral e a integração de diretório/fila de manutenção descritos; NÃO significa ERP/frontend/backend inteiros concluídos. Confirmar versões do serviço efetivo e ambiente integrado descartável permanece necessário antes de uso operacional. Não foram implantados binários, renovados contratos reais, usados PostgreSQL comercial, processados pagamentos externos/fiscal, criados apps móveis ou configurado push. Faturamento/contas a receber e PDV/fechamento continuam próximos no roteiro, comparando código/contratos antes de criar algo. Preservar117de fornecedores e os rascunhos não homologados sem misturá-los. Registrar evidências reais e atualizar somente este arquivo.


## Livro oficial de caixa e exportação íntegra — 24/10/2026, revisão37

### Base e escopo entregue

Frontend principal na branch feat/backend-checkout-authority, base1c681fccd86a0c67c381f651b02de18040a76943. Backend separado feat/backend-b02a-compat: ponte publicada28c4d6c3ea36810508a1e223f49b8d4339bcfe10 e correção publicada beaebbe6d85af07bd17bb5b02b11753df4f9ef96; hash remoto conferido. Preservados116/117, rascunhos locais e a manutenção consolidada. Sem main, reset/clean, migração, alteração de schema63, instalação, serviço comercial, Electron ou ZIP.

Novo núcleo core/cashJournal (contracts.ts, model.mjs/model.d.mts, source.mjs/source.d.mts, words.mjs/words.d.mts), componente pos/pages/OfficialCashJournal.tsx e CSS próprio. LocalPointOfSale oferece consulta na página de caixa e preenche o identificador original depois do fechamento confirmado. operations84/Journal84 acrescenta o painel oficial e distingue explicitamente os lançamentos locais de preparação. Não substitui nem promove rascunhos existentes a registros oficiais. Novos testes cashJournal.test.mjs e cashJournal.bridge.mjs.

Contratos existentes reutilizados: docs/api/cash-journal.openapi.json SHA256 f2cbdeb658b5496c79a6745afe895dd1823762440c473e63a06917feb2268412 e cash-journal-export.openapi.json SHA256 6fbc01c64f8afaaa0a71886b24ffc020acd9440eb603044f2a71adc28821aa8b. O texto histórico de ausência de frontend nesses contratos antecede esta integração; esta revisão registra a atualização. GET journal e journal.csv apenas para caixa fechado, operador original e empresa/loja/aparelho autorizados, permissão Sell; histórico não exige licença de PDV ainda ativa. Não ampliar a permissão do contador. O contrato consultado não oferece índice geral dos caixas do próprio operador: consulta por ID e preenchimento a partir do fechamento, sem inventar uma listagem completa.

Resumo oficial é do caixa inteiro, independente da página de50 lançamentos; sinal source_complete descreve cobertura do caixa local, não carregamento de todas as páginas. A interface mostra total/paginação, dinheiro esperado/declarado/diferença, débitos/créditos de controle e não classificados. Origem, autor e operação originais permanecem visíveis; legado desconhecido não recebe autor atual fabricado. Caixa não é livro global da empresa, liquidação bancária, DRE ou escrituração fiscal.

Validação estrita de DTO, origem, sinais, duas pernas balanceadas, inteiros seguros e reconciliação BigInt; formatação preserva centavos negativos e valores grandes. Consulta e exportação invalidam respostas antigas por contexto, sessão, cancelamento e geração, inclusive mudança de caixa durante a leitura do corpo. GET sem corpo, no-store, credenciais somente pelo token explícito. Falha remove a apresentação anterior; não exibir saldo antigo como nova consulta nem gerar zero oficial. Cinco idiomas pt-BR/en/es/fr/zh-CN, temas, foco, controles touch e rolagem interna preservados.

CSV vem inteiro de um único snapshot do backend, nunca concatenação de páginas. Limites do contrato:10.000 lançamentos/16MiB, BOM UTF-8, ponto e vírgula, CRLF e22colunas. Frontend confere cabeçalhos HTTP, codificação, estrutura, contagem, células seguras e reconciliação antes do download, conservando os bytes recebidos. Valores exportados são CENTAVOS inteiros; identificadores/datas devem ser importados como texto. Conteúdo que não pode circular com segurança é recusado, sem transformar dado em fórmula ou prefixar silenciosamente outro valor. Não foi homologado abrir/salvar/reabrir em Excel ou LibreOffice.

### Falha real identificada e corrigida no backend

A ponte HTTP com identificador turn-Á QA reproduziu404: Fiber entregava o parâmetro ainda codificado e o handler consultava o banco com esse valor. Correção localizada em internal/localapi/cash_journal.go e cash_journal_export.go: url.PathUnescape exatamente uma vez no ID das duas consultas,400 para escape inválido. Sem alteração de escopo/autorização/persistência. Regressão também passa com turn-%literal, impedindo dupla decodificação. Preimagens em work/cash-journal-ui-20261024 nesta tarefa.

TestCashJournalFrontendBridge executa Node/cliente frontend real contra Fiber HTTP loopback efêmero e SQLite temporário, sessão/licença/dados sintéticos. Venda3198, suprimento500, sangria300, cancelamento da venda e fundo1000 resultam em saldo1200 e controles8196; consulta além do fim mantém resumo, exportação contém5lançamentos e resumo. Os IDs Unicode e com percentual literal passam em JSON e CSV. Nenhum serviço ou banco comercial foi iniciado. Fixture termina o servidor e descarta a base temporária.

### Evidências executadas

66testes específicos aprovados; suíte global frontend2.230aprovados,0falhas/cancelados/ignorados. TypeScript final tsc --noEmit exit0; primeiro typecheck encontrou incompatibilidade BlobPart/Uint8Array, corrigida com cópia tipada antes de repetir a verificação. Build final isolado configFile:false/envFile:false exit0 em21.343667302s, work/catalog-management-20261022/build-check-ReQ0FD. Avisos preexistentes de Tailwind e chunks grandes permanecem; não declarar otimização de planilha.

Regressões Go do livro/exportação e ponte HTTP:32eventos de teste aprovados,0falhas, pacote localapi aprovado em9.829s. cmd/titan-local e cmd/api compilaram exit0, binários em work/cash-journal-ui-20261024/build, NÃO executados. Não repetir como atual o resultado da suíte Go inteira da revisão34: agora foi a rodada específica. PostgreSQL e integrações externas permanecem pendentes de ambiente descartável autorizado.

Prévia4354 executou o componente real em StrictMode com dados explicitamente sintéticos e transporte de prévia isolado, sem proxy para backend. Verificados tabela/resumo, erro409 removendo o resultado antigo, exportação, cinco idiomas e temas. Prova outputs/livro-caixa-oficial-previa-2026-10-24.png. Download sintético em /home/danillo-malta/Downloads/titan-caixa-lancamentos.csv:1152bytes, BOM,5lançamentos+resumo, saldo1200/declarado1100, SHA256 4a36c99c7231f36981966a7515178a1723f0ba07193904f56a5a0a443f337ff8. Não contém dados comerciais. Essa prévia não é prova de persistência; a ponte Go independente é a evidência integrada. Logs/preimagens/lista exata em work/cash-journal-ui-20261024 da tarefa.

A consulta remota feita sem rede autorizada falhou “Could not resolve host: github.com”; a conferência posterior com acesso de rede autorizado confirmou o hash beaebbe no GitHub. Não atribuir essa falha ao aplicativo.

### Continuidade e limites

Esta unidade conclui consulta e exportação do livro oficial de um caixa fechado no contrato disponível. Malote/cadeia de custódia, homologação completa do fechamento/PDV, faturamento/recebíveis, demais fluxos da matriz, periféricos, fiscal/pagamentos externos, desempenho e empacotamento Windows/Linux não se tornam concluídos por esta entrega. Não declarar ERP inteiro pronto nem porcentagem sem critérios medidos. Próxima unidade deve comparar contrato e código, preservar as recuperações e continuar neste registro único. Não renovar/selecionar licença, banco, estação ou perfil reais para demonstrar o produto.


## Malotes oficiais e estado mensurável — 24/10/2026, revisão38

### Estado confirmado antes do desenvolvimento

Frontend aaba539c8bdcfb7fb1c7ca3e099e041169bb19e0, feat/backend-checkout-authority; backend beaebbe6d85af07bd17bb5b02b11753df4f9ef96, feat/backend-b02a-compat. Ambas árvores limpas no início. Nenhuma alteração ou commit do backend nesta unidade. O ZIP TitanSystem_Backend_Devolucoes_Base7b37966_20261024_INTEGRO.zip tem SHA2567219facef9247701182b849e0300bb42e5ee6bcb5b1bf80122582b670c447330; tentativa anterior parou ANTES da candidata/escrita: “Pare: Dependencia mudou; preservar e adaptar: backend/internal/localapi/cash_journal.go”. Comparação confirmou somente as dependências cash_journal.go e cash_journal_export.go divergentes pela correção de ID do beaebbe. Teste cash_journal_frontend_bridge_test.go preservado, SHA256ed927a0593f349e0c0ed909253b57e9afeec2cd3d4873948a2a10cb52633b7fa. Não alterar hashes do manifesto nem remover a correção para forçar o pacote. Extração validada preservada em ~/Downloads/titan-devolucoes64-retomada.u4apsohb/TitanSystem_Backend_Devolucoes_Base7b37966_20261024. SQL64 e marcador continuam ausentes; não anunciar a devolução parcial aplicada.

O proprietário pediu porcentagens de conclusão do frontend e backend antes de continuar. Registro, contratos e fontes foram conferidos. NÃO existe denominador de requisitos aceitos, pesos ou homologação integral por módulo que permita declarar porcentagem total confiável. Quantidade de testes, arquivos, revisões e rotas não mede conclusão comercial. A lista de17frentes do proprietário continua referência de escopo; subfluxos entregues não encerram cada frente inteira. Não atribuir0% à implementação por falta de aceite nem80/90% com base em quantidade de código.

Frontend com entregas confirmadas inclui116/117, estoque/inventário, documentos/agenda a pagar, receitas/ordens/etapas/resultado/qualidade, planilha e fontes oficiais, docas/recebimento/tratativas/indicadores, ciclo manual de entregas/recusa, frota/oficina/diretório de manutenção, login/contexto e livro oficial. Há lacunas em PDV/pagamentos/fechamento completos, malotes/cofres (malotes avançam nesta revisão), faturamento/recebíveis/DRE/conciliação, circuito interempresa/trocas, fiscal, portal do contador, RH, CRM, demais fontes/desempenho e homologação geral. Não declarar ausente toda API de uma área por faltar seu frontend. Backend possui contratos/persistência/testes de muitos desses subfluxos; pagamento eletrônico efetivo, fiscal, financeiro/RH completos, aparelho independente/offline e provas de implantação continuam sem aceite global. Backend/schema64 NÃO pode ser contado como aplicado.

Para apurar percentual global futuramente, decompor os requisitos aceitos em critérios verificáveis por frente, registrar fonte/contrato/teste e resultado dos aceites de integração, reinício, permissão, isolamento, auditoria e restauração. Distinguir implementado, parcial, não implementado e não verificado; não converter não verificado em concluído. Nesta revisão somente métricas objetivas abaixo foram calculadas:44/44 novos casos (100% de aprovação desta bateria),2.274/2.274 casos do frontend (100% de aprovação da suíte executada),5/5 idiomas observados (100% dos idiomas obrigatórios desta prévia). Esses100% NÃO são porcentagem de conclusão do frontend/backend.

### Unidade funcional aplicada no frontend

Novos core/cashBags/{model.mjs,model.d.mts,source.mjs,source.d.mts,words.mjs,words.d.mts}, pos/pages/OfficialCashBags.tsx e officialCashBags.css. LocalPointOfSale acrescenta o painel somente na página de caixa, preservando fechamento, PDV, rascunhos84/99/105/109 e livro da revisão37. Não transportar os malotes locais do navegador para o servidor nem converter sua cadeia em confirmação oficial. Contrato reutilizado docs/api/cash-bags.openapi.json do backend/schema63.

Fluxo completo deste contrato: criar malote a partir do ID de sangria auditada e lacre; revisar/confirmar preparação; saída e cancelamento pelo remetente; recebimento contado ou rejeição por pessoa distinta do remetente E operador da gaveta. Histórico validado, filtro por estado/caixa e paginação50 com total independente do recorte. Valor declarado nulo para outros gestores até recebimento: resposta que vaze valor antes da contagem é recusada, não apenas escondida via CSS. Declaração/origem vêm da sangria, não são digitadas como novo dinheiro. Conferente informa lacre e contagem em centavos exatos; diferença só aparece depois do registro confirmado. Cadastro de malote não move caixa/estoque/journal nem liquida banco; não comprova transporte remoto ou depósito no cofre.

Custódia do caixa encerrado consulta somente o operador original com ManageCash. Valida soma exata das destinações contra sangrias, recebido contado/declarado/diferença e pagamentos a fornecedor. supplier_returned_cents é informativo, nunca subtraído novamente da sangria nem somado ao caixa. Caixa aberto/inconsistente não vira zero/total parcial. Não calcular resumo empresarial a partir de uma página de malotes. IDs do caminho de malote/operação seguem o contrato ASCII; cobertura de caixas com ID legado não simples depende da leitura atual do backend, que pode recusá-los. Não alterar esse backend durante o trabalho do outro aplicador.

Fila própria titan-cash-bags:v1 por empresa/loja/usuário/aparelho; sem tokens persistidos. Salva corpo/IDs/snapshot antes de qualquer POST; Web Lock obrigatório e releitura do armazenamento. Antes do envio consulta recibo original, e em transição ainda não enviada reconsulta o malote para detectar mudança desde a revisão. Perda de resposta, timeout, leitura de corpo tardia ou revogação após POST deixam resultado incerto. Recuperação somente por recibo original ou reenvio EXPLÍCITO byte a byte; nunca apaga/reenvia automaticamente por trocar sessão/contexto. Primeira recusa definitiva real pode marcar não enviada; incerteza anterior nunca é rebaixada. Nova operação e descarte ficam bloqueados durante incerteza; armazenamento ilegível/indisponível bloqueia escrita. Consulta histórica não exige licença POS ativa; POST exige contrato/permissão e contexto atuais. A autoridade final continua no servidor.

Formulário/revisão e pendência protegem saída de rota/recarregamento, com foco na revisão, controles44px e rolagem interna. Não guardar imagem, credencial ou dados bancários. Cinco traduções, temas claro/escuro; motivos humanos multilinha/Unicode preservados no contrato, sem executar texto como código.

### Testes, integração e limites reais

44novos testes cashBags.test.mjs: cadeia e estado, remetente/conferente, ocultação de declarado, diferenças em centavos, cobertura/fornecedor, paginação/filtros, recuperação/mesmo corpo, contextos4IDs/logout, licença histórica, recusa definitiva versus incerteza anterior, storage/lock, snapshot alterado, recibo adulterado, consulta concorrente, dispose/corpo tardio, cancelamento/rejeição e5dicionários. Primeira rodada42/43: fixture tentou ler uma resposta cega do conferente como se fosse o remetente; modelo recusou corretamente. Teste corrigido para esperar a recusa e verificar o snapshot próprio do remetente; nova regressão de motivo multilinha acrescentada. Sem relaxar a proteção.

Suíte completa final: node --test --test-isolation=none --test-reporter=tap frontend-web/tests/*.test.mjs,2.274aprovados,0falhas/cancelados/ignorados, incluindo116/117. O executor isolado padrão deste ambiente apresentou107unidades por ARQUIVO; não se usou esse número como contagem de casos. A rodada sem isolamento exibiu explicitamente os2.274casos. TypeScript tsc --noEmit -p frontend-web/tsconfig.json exit0. Build final isolado configFile:false/envFile:false exit0 em21.985079229s, work/catalog-management-20261022/build-check-AnQCvC, sem modificar dist do usuário. Avisos Tailwind e chunks grandes preexistentes permanecem.

Integração HTTP real: cashBags.bridge.mjs e fixture Go versionada NO FRONTEND em tests/fixtures/cash_bags_frontend_test.go. Executor explícito tests/cashBags.backend.mjs recebe raiz do backend e caminho do Go; usa overlay temporário para que o teste entre na compilação sem gravar na worktree do backend. Não instala, inicia serviço comercial, lê .env ou abre banco real. Fixture cria SQLite/licença/sessões sintéticas, HTTP loopback efêmero e encerra tudo ao terminar. Comando reproduzível: node frontend-web/tests/cashBags.backend.mjs /home/danillo-malta/Downloads/zzeus-frontend-backend-b02a /home/danillo-malta/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.8.linux-amd64/bin/go. Executado e aprovado. Não é novo resultado da suíte Go inteira.

Ponte comprovou perda APÓS commit real de preparação, recuperação sem segundo POST, remetente e receiver independentes, valor oculto antes da contagem,300declarado/290contado/-10diferença, histórico3, lista/total e cobertura reconciliados. Exatamente3POSTs no ciclo. Go conferiu que cash_movements/cash_journal_entries/stock_movements não cresceram por custódia. Sessão financeira/comercial real não selecionada nem ampliada. Primeira chamada sem acesso de cache falhou “read-only file system” em ~/.cache/go-build; repetida com permissão de execução autorizada, sem mudar os fontes.

Prévia4355 com componente real/StrictMode e dados explicitamente sintéticos em memória, sem backend/proxy: revisão/preparação, perda de resposta, descarte e nova ação bloqueados, recuperação, saída, mudança para conferente, contagem cega e recebimento. Cinco idiomas observados e temas claro/escuro; zero erros de console. Prova outputs/malote-recebido-previa-2026-10-24.png. Isso não substitui a ponte HTTP acima. Primeira tentativa de abrir4355 em sandbox falhou “listen EPERM”; prévia autorizada depois. Preimagem do LocalPointOfSale, hashes, fontes preparados, logs e overlay em work/cash-bags-20261024 desta tarefa. Não recriar ou reaplicar esse diretório sobre alterações posteriores.

### Próxima continuidade

Malotes descritos passam de rascunho a fluxo oficial ligado à tela de caixa. Cofre/depósito/transferência, contagem de cofre e conciliação correspondente continuam separados e não implementados por este painel. Permanecem PDV/fechamento completos, recebíveis/faturamento, demais frentes listadas acima e homologação integrada/externa/hardware/desempenho/desktop Windows/Linux. Não afirmar “frontend completo” por aprovar a suíte. Não há percentual global calculado nesta revisão. O pacote de devoluções deve ser compatibilizado pelo responsável sem remover o beaebbe; não houve novo commit/push ou aplicação dele nesta unidade.


## Cofres, suprimento e conferência independente — 24/10/2026, revisão39

Base frontend01ffb090c787eca4373cd63cb69f9553c302be58, mesma pasta/branch oficial; backend beaebbe6d85af07bd17bb5b02b11753df4f9ef96/schema63 preservado e limpo. Dono solicitou continuar após malotes sem tratar uma unidade como conclusão de todo ERP. Nenhuma alteração de fonte/commit/backend, pacote de devoluções, banco comercial, Electron, instalação ou ZIP nesta unidade. Referências cash-vaults.openapi.json e cash-vault-counts.openapi.json lidas junto com cashvault/cashcount e testes reais. As descrições antigas de ausência de frontend nos contratos antecedem esta revisão.

### Arquivos e comportamento entregue

Novos core/cashVaults/{model.mjs,model.d.mts,export.mjs,source.mjs,source.d.mts,words.mjs,words.d.mts}, pos/pages/{OfficialCashVaults.tsx,officialCashVaults.css,CashCustodyGuard.tsx}, testes cashVaults.test.mjs/cashVaults.bridge.mjs/cashVaults.backend.mjs e tests/fixtures/cash_vaults_frontend_test.go. LocalPointOfSale mostra o painel na página de caixa junto de malotes e livro; OfficialCashBags altera apenas a proteção de saída para compartilhar o provedor. Preimagens e hashes dos2arquivos anteriores em work/cash-vaults-20261024 desta tarefa. Não migra nem apaga rascunhos99/105/109 ou o117 de fornecedores.

Cofres iniciam em zero, sem crédito manual arbitrário. Depósito exige malote recebido/contado e o mesmo conferente original; valor vem do contado, não do declarado nem de campo editável. Malotes sem depósito são consultados por conferente com total/páginas. Transferência exige origem/destino diferentes e ativos, valor em centavos e motivo; valida duas pernas iguais e opostas. Suprimento usa caixa aberto explicitamente identificado e uma operação atômica do backend: débito de cofre, movimento positivo do caixa, mesma origem/ID. Não chamar novamente o endpoint de suprimento de caixa para duplicar essa entrada. Arquivar exige saldo zero e mantém histórico. Lista, saldo atual e extrato paginado são consultas distintas dos snapshots históricos dos recibos.

Conferência: responsável escolhe cofre e ID de outro usuário já autorizado, com motivo; backend valida identidade/permissão e bloqueia movimentos durante counting/submitted. O registro devolve expected_cents=null antes da contagem, inclusive em recibos de abertura/cancelamento. Somente conferente designado envia o contado; depois outro usuário com ManageCash aceita/rejeita com motivo. Criador pode cancelar somente antes do envio. Revisão não ajusta dinheiro nem apaga diferença. Consultas de saldo já autorizadas continuam disponíveis: ocultação do registro de contagem não é promessa de impedir acesso ao saldo por outras consultas permitidas.

Relatório completo do contexto local físico, até1000cofres, valida soma com BigInt, estados e última conferência de cada origem, sem totalizar páginas. Histórico encerrado conserva valor esperado da sua época mesmo que o saldo atual mude. CSV de snapshot único, até2MiB/1000cofres,21colunas,UTF8/BOM/CRLF/ponto e vírgula e centavos inteiros. Frontend valida esquema/células/IDs/estados/contagens/somas/codificação/cabeçalhos e conserva bytes originais para download. Texto com fórmula/controle inseguro é recusado, sem modificar o JSON nem gerar arquivo parcial. Não é saldo bancário, malote em transporte, escrituração fiscal ou homologação abrir/salvar/reabrir Excel/LibreOffice.

Fila titan-cash-vaults:v1 por empresa/loja/usuário/aparelho, compartilhada pelos comandos de cofre E conferência deste painel. Locks e armazenamento durável obrigatórios; corpo/IDs/snapshot preservados antes do POST. Consulta de recibo original precede todo envio/recuperação; revisão reconsulta origens antes do primeiro envio. Mudança já observada impede POST. Contratos não têm compare-and-swap do saldo: concorrência entre consulta e transação pode mudar saldo legitimamente; recibo valida efeitos/IDs/contrapartidas originais, sem exigir falso saldo final calculado do snapshot velho. O backend continua autoridade transacional. UI explica snapshot consultado versus recibo histórico versus saldo atual.

Contexto/sessão/permissão conferidos antes/depois do transporte e corpo; escrita exige POS ativo, histórico não exige licença atual. Resultado incerto não é apagado nem reenviado por trocar sessão, usuário, empresa, loja ou aparelho. Reenvio explícito conserva corpo/IDs. Primeira recusa definitiva REAL pode ser descartável; erro local ou incerteza anterior não são desclassificados. Corpo/recibo adulterado, storage corrompido e lock indisponível bloqueiam escrita. Nenhum token vai para fila.

### Proteção de navegação e correção visual encontrada

Dois useBlocker na mesma página poderiam se sobrepor. CashCustodyGuard mantém UM bloqueador e a união das proteções registradas por malotes e cofres; cada painel remove só a própria inscrição. Guardas de beforeunload e diálogo único preservam operação enviada e avisam sobre formulário não enviado. Nenhuma alteração no motor/fila de malotes ou desconto de seus testes. Proteções de permissão/contexto no servidor permanecem obrigatórias.

Ensaio visual encontrou por um instante linhas de cofre sob o novo título Conferências após trocar de aba. Corrigido com assinatura da consulta incluindo contexto/aba/página/revisão/expansão; página anterior não renderiza sob nova seleção. Mesma proteção no extrato. Nova inspeção imediatamente após trocar aba mostrou ausência das linhas antigas antes de carregar o resultado correto. Rótulos da tabela passam a corresponder ao recurso. Quebra de linha dos rótulos/evidências impede transbordamento; documento verificado1280px de largura para viewport1280px. Formulários/revisão têm foco, controles44px, cinco idiomas e estilos de temas existentes.

### Evidências executadas

65testes específicos aprovados: estados, trilha/autor/revisor, ocultação antes de contar, soma exata, cobertura de páginas versus total, origens, valores negativos/overflow, CSV/UTF8/fórmula/truncamento, recibo histórico, concorrência permitida de saldo, loss/recovery/mesmosIDs,4IDs/contexto/logout/expiração, storage/lock, recusa definitiva, snapshot alterado, consulta tardia, dispose e cinco dicionários. Suíte completa final node --test --test-isolation=none --test-reporter=tap tests/*.test.mjs:2339aprovados,0falhas/cancelados/ignorados, incluindo116/117 e44casos de malotes. TypeScript final exit0. Build final configFile:false/envFile:false exit0,20.937689300s, saída isolada work/catalog-management-20261022/build-check-oYfAOU; avisos preexistentes Tailwind/chunks grandes permanecem.

Ponte real executada via node frontend-web/tests/cashVaults.backend.mjs /home/danillo-malta/Downloads/zzeus-frontend-backend-b02a /home/danillo-malta/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.8.linux-amd64/bin/go. Fixture Go versionada NO FRONTEND entra por overlay temporário, sem criar/modificar arquivo na worktree de backend. HTTP loopback efêmero, SQLite/licença/sessões sintéticas e encerramento pela fixture. Nenhuma configuração comercial escolhida. Resultado final0.639s,1ponte aprovada; NÃO é nova execução da suíte Go inteira.

Percurso integrado: cofre2zero, depósito290do malote recebido, perda APÓS commit e recuperação sem novoPOST; transferência100, suprimento50, cofre1saldo140; abertura de conferência e tentativa nova de suprimento1recusada409; descarte só dessa recusa comprovada, contagem130pelo receiver, esperado140/diferença-10, autor da contagem impedido de autoaprovar, aceite pelo responsável mantendo saldo140. Transferência dos140restantes, arquivamento a zero, cofre2saldo240. Extrato/relatório/CSV completos concordam. Exatamente10POSTs incluindo1recusado; Go conferiu apenas1novo cash_movement/journal, nenhum stock_movement de conferência. Aceite não reparou dinheiro, recuperação não duplicou depósito/suprimento.

Prévia4356 explícita, componente real/StrictMode com transporte sintético em memória, sem backend/proxy: abrir, enviar2,80, outro operador aceitar, esperado2,90/diferença-0,10e relatório mantendo2,90. Cinco idiomas percorridos, temas claro/escuro, zero erros de console. Avisos da prévia: i18next não inicializado no harness e flag futura do ReactRouter; produto usa seu provedor existente. Não afirmar que a prévia é persistência oficial: prova real é a ponte acima. Guardas: rascunho de malote bloqueou saída com cofres montados; após editar cofre e desmontar malotes, a proteção permaneceu; exatamente1diálogo em ambos. Botão Permanecer preservou o formulário. Prova outputs/cofre-conferencia-aceita-final-2026-10-24.png (a primeira captura cofre-conferencia-aceita-previa antecede o ajuste de largura). Logs/preparação/preimagens no laboratório desta unidade.

### Continuidade sem encerrar o restante

Concluídos os fluxos de cofres/conferência descritos no contrato local; dependências de banco/provedor/piloto real continuam explícitas. Próxima frente independente: carrinhos persistidos já contratados em suspended-carts.openapi.json, comparando ao rascunho Till94/111 e protegendo finalização/recuperação para não duplicar venda. Pagamentos eletrônicos reais, fiscal, faturamento/recebíveis amplos, demais áreas e desktop/hardware não se tornam concluídos por este incremento. Não iniciar ou aplicar o pacote backend64 enquanto o aplicador não reconhecer a base financeira corrigida. Continuar no mesmo registro único, sem porcentagem global inventada nem novo ZIP.


## Proteção única de saída em todo PDV — 24/10/2026, revisão40

Base465742f32e72160bb2b4c94f2583d71139e5ff07 publicada/hash remoto conferido, mesma pasta/branch frontend. A revisão39 unificou malotes/cofres, mas a inspeção para carrinhos encontrou useBlocker também em CashClosing92, Desk95 e Till94, que podem coexistir. A proteção foi ampliada ANTES de acrescentar carrinhos, sem afrouxar filas ou mudar endpoints. Backend continua beaebbe limpo; nenhuma aplicação de64 ou commit/backend.

Arquivos alterados: LocalPointOfSale.tsx, CashClosing92.tsx, Desk95.tsx, Till94.tsx e CashCustodyGuard.tsx em modules/pos/pages; este registro. O provedor fica agora ao redor de TODO POSWorkspace dentro do provedor da fonte de fechamento, removendo o provedor aninhado exclusivo de malotes/cofres. Os três componentes legados registram sua condição no mesmo provedor em vez de criar bloqueadores concorrentes; seus usos efetivos foram encontrados apenas no LocalPointOfSale. Mantidos onDirty, modelos, IndexedDB, filas, transporte, autorização e fechamento. Operação pendente e ocupação de caixa/venda também participam da proteção; Desk95 inclui estado pendente além de formulário/telefone alterado. Sair explicitamente não apaga nem reenvia operação. Limpeza de câmera continua pelo cleanup existente de CashClosing92, sem iniciar câmera nos testes.

O aviso compartilhado passou a dialog nativo com showModal, rótulo acessível, foco inicial no botão Permanecer, contenção de foco pelo navegador e Escape que cancela navegação. Não fica escondido no fim de uma página rolada. O registro do painel removido apaga somente sua própria condição; outros formulários continuam protegidos.

Prévia4357 explícita usou CashClosing92/Count107 REAIS ao lado dos componentes reais de malote/cofre, fonte de vendas sintética vazia/complete, IndexedDB somente da origem de laboratório, sem backend/proxy/câmera/fechamento de caixa. Adicionar cédula de200 tornou a contagem não salva e bloqueou navegação apesar dos outros painéis limpos. Rascunho de malote permaneceu protegido após desmontar o fechamento. Com ambos alterados, exatamente1diálogo. Escape preservou o valor do formulário e não navegou. Botão Sair foi testado e levou ao destino de teste. Uma tentativa de clicar denominação enquanto o fechamento estava oculto expirou; inventário visual completo identificou o painel desmontado, que foi exibido antes da repetição correta. Não apresentar a tentativa expirada como aprovação.

A edição do harness durante HMR provocou aviso createRoot duplicado na prévia antiga. Foi encerrada e repetido teste em aba limpa:1diálogo, :modal=true, focoPermanecer,0erros de console. Prova outputs/pdv-protecao-saida-modal-final-2026-10-24.png; primeira captura anterior ao ensaio limpo permanece separada. Nenhuma alteração global do ReactRouter ou de seu modo de segurança.

Verificações finais:2339testes frontend aprovados,0falhas/cancelados/ignorados; TypeScript exit0; build isolado configFile:false/envFile:false exit0 em20.844351857s, work/catalog-management-20261022/build-check-QQDEg7, sem mudar dist. Avisos de chunks grandes/Tailwind preexistentes permanecem. git diff --check passou. Não houve nova suíte Go: mudança somente de navegação, sem serviço/banco comercial. Preimagens dos5fontes, hashes, scripts e logs em work/pos-navigation-guard-20261024 desta tarefa.

Continuidade: carrinhos de backend foram lidos em sale/cart_* e suspended-carts.openapi.json. São escopo de usuário/aparelho, não reservam estoque ao suspender, conferem preço/estoque/caixa ao assumir/finalizar e exigem IDs/revisão estáveis. Rascunhos Till94/111 e recuperação atual de venda não podem ser adotados, apagados ou finalizados automaticamente. Próxima unidade deve integrar com revisão e impedir duas finalizações por caminhos distintos. Demais pendências da matriz e conectores externos permanecem; não declarar ERP completo nem encerrar o trabalho por esta correção.

Observação de publicação40: o primeiro git diff --cached --check detectou3linhas vazias com espaços nos componentes legados. Removidos somente esses espaços, repetida a conferência antes do commit; nenhuma mudança comercial ou de transporte.


## Cotação antiga, sessão atual e incerteza durável do PDV — 24/10/2026, revisão41

Base frontend c1f6c57f83270fdc21b2ed3b5efceb6854720d45 publicada/hash remoto conferido, mesma pasta/branch. Backend beaebbe/schema63 preservado e sem alterações. Revisão de pré-requisitos dos carrinhos encontrou uma lacuna no caminho de venda em dinheiro existente; corrigida ANTES de conectar outra finalização. Não reimplementar116/117, não aplicar backend64 nem gerar pacote.

### Reprodução e correção

Reprodução sintética executada em work/pos-session-guard-20261024/baseline.mjs: createLocalClient+confirmCashQuote+createPOSOperations reais, transporte somente em memória. A fixture encerrou a sessão enquanto respondia a cotação; o código anterior ainda executou1POST de venda depois disso (baseline.json). Não foi venda comercial nem serviço real. O efeito é possibilidade efetivamente reproduzida no caminho antigo, não simples hipótese.

Novo scopedPOSClient.mjs/.d.mts vincula métodos usados pelo PDV ao token, referência da sessão e4IDs, validade temporal e capacidades atuais. Verifica antes/depois de resposta E erro, inclusive leitura do corpo aguardada pelo cliente base. Mudança de empresa/loja/usuário/aparelho, logout, renovação ou revogação impede usar cotação antiga para confirmar. Escrita/cotação exige Sell e módulos Core/Inventory/POS ativos; histórico não exige licença atual. LocalPointOfSale usa esse cliente para cotação, fila de operações e consultas próprias; cliente global permanece disponível para autenticação e outros módulos, sem alegar proteção automática de chamadas fora desse vínculo.

Novo posViewIdentity.mjs/.d.mts dá uma identidade de interface em memória por objeto de sessão/token, usando WeakMap/contador, sem pôr tokens em chaves React ou persistir credenciais. Login renovado remonta o espaço de venda para separar callbacks/latches antigos. Resposta401 antiga só pode invalidar a sessão se token E objeto ainda forem os mesmos; não derruba login posterior. IDs comerciais não são gerados por esse contador.

posOperations mantém chave titansystem-pos-v1, formato/version1 e todos os IDs/corpos pendentes anteriores. Incerteza passa a ser persistida E relida ANTES de POST de venda/abertura/fechamento. Falha/silêncio de storage impede transmissão. Antes de remover verifica a mesma pendência e a sessão atual; outra pendência não é apagada. Retomada consulta comprovante original e não cria nova venda/ID. Qualquer retry legado é tratado como potencialmente já enviado, mesmo que o marcador antigo seja false: não inferir ausência de envio após reinício de uma versão que só marcava incerteza depois do POST.

localClient marca serverRefusal=true somente em HTTP não2xx REAL. Apenas primeira recusa definitiva do próprio POST pode permitir o descarte explícito já existente. Erro local de sessão/permissão, falha de parse/transporte, consulta de comprovante posterior ou recusa após incerteza anterior não autorizam apagar. A primeira rodada específica flagrou uma regressão da preparação: POST aceito seguido de403 na consulta estava sendo classificado como recusa do POST; corrigido com identificação da etapa e repetida a regressão legada SEM alterar seu teste. As28regressões antigas de POS/cotação passaram junto dos novos casos.

### Arquivos e verificação

Alterados core/local/{localClient.mjs,posOperations.mjs,posOperations.d.mts} e modules/pos/pages/LocalPointOfSale.tsx. Novos scopedPOSClient/posViewIdentity com declarações, tests/scopedPOSClient.test.mjs/.bridge.mjs/.backend.mjs e tests/fixtures/scoped_pos_frontend_test.go. Este registro atualizado. Preimagens dos4fontes anteriores, hashes, reprodução e logs em work/pos-session-guard-20261024 nesta tarefa. Nenhuma migração/descarte do armazenamento, senha, token ou alteração de banco real.

25novos testes mais28legados pertinentes:53casos aprovados; suíte completa2364aprovados,0falhas/cancelados/ignorados. Inclui4IDs durante cotação, logout/renovação/expiração durante JSON, perda de permissão/licença, espera de lock, retorno tardio pós-POST, diferenciação de recusa real/local/consulta, retomada version1 antiga, recibo original após novo login, storage/lock/pendência concorrente e identidade de interface sem token exposto. TypeScript final exit0. Build final configFile:false/envFile:false exit0 em20.472605453s, saída isolada work/catalog-management-20261022/build-check-gmA1VB. Snapshot SHA256 de1117fontes/testes conferido sem mudança durante essa validação. git diff --check passou. Avisos conhecidos de Tailwind/chunks grandes permanecem.

Ponte real via node frontend-web/tests/scopedPOSClient.backend.mjs /home/danillo-malta/Downloads/zzeus-frontend-backend-b02a /home/danillo-malta/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.8.linux-amd64/bin/go: aprovada0.441s. Fixture Go no FRONTEND compilada com overlay temporário, Fiber loopback efêmero/SQLite/licença/sessão sintéticas; nenhuma escrita na worktree de backend. Cotação cuja sessão foi alterada gerou0POSTs comerciais. Novo cliente fez venda3198com perda APÓS resposta real do servidor; preservou input/uncertain e novo cliente recuperou recibo pos-guard-sale/pos-guard-op, com1únicoPOST de venda. Go conferiu1venda persistida com os IDs/total originais. Não é teste de adquirente, fiscal, equipamento ou serviço comercial implantado; não repetir como resultado da suíte Go inteira.

Continuidade: carrinhos persistidos continuam a próxima integração funcional independente, agora com o pré-requisito de sessão do caminho antigo fechado. Fechamento integral misto, eletrônico/fiscal, contabilidade/recebíveis e demais frentes permanecem nos aceites registrados. Esta correção não autoriza banco comercial nem conclui o produto inteiro. Manter trabalho incremental sem paradas artificiais entre unidades; não anunciar porcentagens globais sem critérios medidos.


## Recuperação de venda cancelada com pagamento estornado — 24/10/2026, revisão42

Base frontend6f6d719c56fbea6ce867fe865942a881de665546 publicada/hash remoto conferido. Backend beaebbe/schema63 permanece sem alterações. Continuidade do pré-requisito de carrinhos: código de cancelamento do servidor marca sale_payments como reversed; leitor legado e receiptMatches aceitavam somente confirmed, embora a venda cancelada e a recuperação já fossem previstas.

Ponte HTTP real foi estendida para cancelar a venda sintética ENTRE perda da resposta e recuperação. Antes da correção falhou de fato com LocalAPIError status0 “A API local retornou dados incompatíveis.” (before.jsonl). Não era nova venda, ausência de backend nem dado comercial; a fila original permaneceu preservada.

Correção localizada em core/local/localClient.mjs e posModel.mjs: comprovante de venda cancelada pode conservar pagamento reversed; venda committed continua exigindo confirmed. O legado cancelled/confirmed já aceito permanece compatível, com o estado original preservado; não reescrever o histórico nem interpretar confirmed histórico como nova venda ativa. Status pending/refunding/settled continuam recusados nesse contrato em dinheiro. IDs, total, itens, meio, soma e fiscal_authorized=false continuam validados. O cálculo de troco confirmado existente só opera com statuscommitted, portanto comprovante cancelado não produz novo troco confirmado.

Novos7testes localCancelledRecovery.test.mjs; ponte scopedPOSClient.bridge.mjs agora exige statuscancelled/pagamentoreversed depois do cancelamento real e recupera os IDs originais. Suíte completa2371aprovados,0falhas/cancelados/ignorados. TypeScript exit0, build isolado configFile:false/envFile:false exit0 em21.091443412s, work/catalog-management-20261022/build-check-K0Rxw4. git diff --check passou. Ponte final aprovada0.422s:1POST de venda e1cancelamento explícito de preparação, recuperação com0POST adicional. Não confundir “1POST de venda” com ausência do cancelamento de teste. Fixture temporária via overlay, sem gravar fonte no backend ou tocar banco/serviço comercial. Não é nova suíte Go inteira ou estorno eletrônico/fiscal.

Preimagens dos3fontes anteriores, hashes, testes e logs em work/pos-cancelled-recovery-20261024. Uma primeira tentativa de registrar o log usou diretório inexistente e o shell recusou “No such file or directory” antes de executar a ponte; repetida no caminho correto com resultado real registrado. Nenhuma dependência instalada ou pacote reconstruído. Esta correção é cancelamento integral já contratado; NÃO aplica devoluções parciais/schema64 do ZIP bloqueado. Carrinhos persistidos permanecem em preparação separada, ainda não apresentados como entregues.


## Carrinhos persistidos no serviço local — 24/10/2026, revisão43

Base frontend7f259018cb2c3e44fc4b27b00e6beb5062829c2b confirmada limpa e publicada por ls-remote. Backend beaebbe/schema63 permanece sem alterações. Lido contrato suspended-carts.openapi.json e implementação atual da worktree separada. Não aplicar ZIP64; não usar banco comercial, escolher estação/licença/chaves ou iniciar serviço comercial.

Integração em /local/pos com core/officialCarts/{model,source,words}.mjs e declarações, OfficialCarts.tsx/CSS. Usuário identifica o carrinho atual; confirmação de salvamento só limpa rascunho com MESMO ID original E MESMOS itens/locais/quantidades. Histórico não limpa carrinho substituto com itens iguais. Rascunhos Till94/suspended104/carts111 permanecem separados e não são convertidos automaticamente. Atualização de carrinho suspenso permite quantidade/remoção, rótulo e substituição explícita pelos itens atuais; preserva sale_id/payment_id. Releitura exige igualdade da revisão selecionada antes de preparar: nunca adotar versão nova para sobrescrever o formulário antigo.

Retomada liga ao caixa aberto; liberação permite atendimento posterior; cancelamento do carrinho não cancela venda nem estorna pagamento. Conclusão em dinheiro exige carrinho de venda atual vazio, motivo humano, valores inteiros, conferência de preço/unidade/estoque e revisão antes de POST. Caminho oficial fica bloqueado por tentativa legada, exposição a pagamento/desconto, restauração não concluída ou outras ações incompatíveis do checkout existente. Sem ampliar permissões, sem aprovar desconto/cartão/Pix, sem reserva implícita. Revisão mostra troco PREVISTO; somente comprovante real da venda permite conclusão e troco confirmado. A inspeção visual identificou e corrigiu o rótulo anterior “Troco confirmado” na prévia antes de homologar a unidade.

Cada tentativa salva corpo/ID/revisão/origem/incerteza e relê armazenamento ANTES do POST. Chave nova titan-pos-carts:v1 por tenant/store/identity/device; sem credenciais. Usa o MESMO WebLock da fila POS legada; posOperations recusa novo envio se houver tentativa de carrinho, incluindo armazenamento inválido. Reciprocamente carrinhos recusam envio com fila POS antiga. Consultas de recibos originais continuam possíveis nas duas filas; não há descarte/reenvio automático por logout/troca de contexto. Se ambas as filas antigas coexistirem e nenhum recibo existir, a interface não contorna o bloqueio: exige conferência das tentativas preservadas. Falta de locks/storage/permissão impede escrita.

Reenvio é explícito com MESMOS dados/IDs e consulta prévia de recibo. Somente recusa HTTP definitiva inicial pode tornar tentativa descartável explicitamente; erro local, parse, rede ou incerteza anterior não permitem apagar. Finalização do carrinho consulta também comprovante da venda real e confere sale/operation/payment/cash IDs, total, itens e meio. Cart.state completed é histórico mesmo se venda tiver sido cancelada posteriormente: status cancelled/reversed é preservado, sem novo troco confirmado. Sessão atual é conferida antes/depois de resposta e leitura; consultas paginadas concorrentes só aceitam a mais recente. Histórico até4096revisões; listas50por página, sem apresentar recorte como total.

37novos testes, suíte completa2408aprovados,0falhas/cancelados/ignorados. Testes incluem snapshot/centavos/UTF8, revisão/estado, identidade dos recibos, logout e4contextos, armazenamento, incerteza, retomada, concorrência e preservação do rascunho substituto; cinco dicionários com mesmas chaves. TypeScript exit0. Build final configFile:false/envFile:false exit0 em21.434976296s, saída isolada work/catalog-management-20261022/build-check-5FWdZS. Snapshot SHA256 de1138fontes/testes inalterado durante a validação. Avisos conhecidos de Tailwind/chunks de planilha permanecem, sem ocultá-los ou aumentar limites para simular otimização.

Ponte real officialCarts.backend.mjs usa fixture Go no FRONTEND via overlay temporário sobre backend separado; Fiber em porta efêmera, SQLite/licença/identidades sintéticas. Aprovada0.413s: salvar, retomar, liberar, retomar novamente e finalizar,5POSTs de carrinho. Perda da resposta de salvamento e finalização recuperada por recibos, total3198/recebido4000/troco802,1venda. Depois1cancelamento explícito de teste e restauração deliberada da pendência original: recuperação preserva cancelled/reversed sem sextoPOST de carrinho. Go confirma1venda e estoque de volta5000 após cancelamento. Não é suíte Go inteira nem teste de serviço implantado, fiscal ou pagamento eletrônico.

Prévia explícita do componente real em4358, transporte sintético em memória, sem backend/proxy: salvar limpou somente seu rascunho; retomar e concluir mantiveram IDs;3POSTs (salvar/retomar/finalizar), total31,98/recebido40,00/troco8,02 pelo comprovante. Histórico das3revisões, cinco idiomas, temas claro/escuro, sem undefined,0erros de console e largura1280/scrollWidth1280. Evidências nesta tarefa em outputs/carrinho-concluido-contexto-2026-10-24.png e carrinho-concluido-2026-10-24.png. Interface inspecionada isoladamente; não afirmar piloto integrado completo do PDV em perfil real.

Preimagens, hashes, fontes de preparação e logs em work/official-carts-20261024. Falhas da preparação corrigidas antes da entrega: teste concorrente acessava promise antes de iniciar (ajustado sincronismo); fixture reutilizava operation_id de claim em outra revisão (IDs separados, servidor recusou corretamente); TypeScript detectou import cartItems não usado (removido). Prévia sem elevação falhou “listen EPERM: operation not permitted 127.0.0.1:4358”; repetida com acesso aprovado. Leitura de tradução em cwd errado falhou FileNotFoundError antes da escrita, refeita no caminho correto. Atalho de captura em h1 não focável expirou; enquadramento por rolagem e screenshot real concluído. Nenhuma dessas falhas apagou dados ou alterou backend.

Pendências desta área: relatório temporal/CSV de carrinhos ainda precisa de integração própria; filtros de lista e histórico desta unidade já funcionam. Pagamentos mistos/adquirentes/fiscal e validação em ambiente integrado autorizado permanecem separados. Próxima continuidade independente: relatório temporal dos carrinhos pelo contrato existente, sem somar preço salvo como contas a receber ou venda. Não anunciar frontend inteiro ou ERP completo por concluir esta unidade.


## Organização desktop, serviço e site — 25/10/2026, revisão44

### Mudança de prioridade e preservação

Pedido do proprietário: pausar funções de frontend e organizar o produto como software desktop; comunicação local e pela internet conforme configuração da empresa, confirmado na resposta sobre os dois modos. Base principal864743974e5993d6e0bf6857d3b3b0c25446e170, branch feat/backend-checkout-authority. Backend de referência beaebbe6d85af07bd17bb5b02b11753df4f9ef96/feat/backend-b02a-compat, árvore separada limpa e sem alteração de fonte nesta etapa. Nenhum banco, instalação/estação/chave comercial, serviço ou aplicador ZIP foi iniciado. ZIP64 continua bloqueado/preservado.

A preparação de relatório de carrinhos foi PAUSADA, não entregue como outra função:8arquivos locais de core/officialCarts/source.mjs/.d.mts/words.mjs/report.mjs/.d.mts, OfficialCarts.tsx, OfficialCartReport.tsx e cartReport.test.mjs. Permanecem fora do commit desta organização, na árvore original, com cópia exata e hashes conferidos em work/desktop-organization-20261025/paused-source e paused-frontend-hashes.json. Os44testes isolados dessa preparação e TypeScript tinham passado; não equivale a conclusão/homologação da função. A compilação local usa a árvore atual, incluindo essa preparação preservada; não foi convertida em instalador ou pacote homologado.

Lido relatório de66páginas, capítulos2/14/15/17/21 relevantes: a UI é frontend-web; Electron não deve duplicar o ERP; implantações local, loja e hospedada têm autoridades e limites distintos; origem/SPA/API e TLS precisam de validação. Conferido docs/architecture/PROJECT_RULES.md e contrato81. Não renomear/mover diretórios às cegas: preservados116/117, rotas, scripts e histórico.

### Organização aplicada

Frontend-web é a fonte das telas do SOFTWARE. Desktop é sua janela e infraestrutura nativa; backend é serviço separado com autoridade comercial, no computador/servidor autorizado, não conteúdo de página. Website/README.md reserva o futuro site público, sem copiar ERP. README raiz, frontend-web/README.md, backend/README.md e desktop/{README,src/README,src/pos/README} corrigidos: antigos documentos apontavam para cópias TitanSystem/ e propunham outro PDV/offline sem autoridade. Nenhuma fonte Go/migração foi modificada. As árvores antigas foram preservadas.

Desktop/main.cjs substitui a entrada inválida: package.json antigo apontava main.js ausente, referenciava preload.js ausente e main.ts carregava localhost3000/pos. Agora main.cjs/preload.cjs existem, main.ts delega como compatibilidade, origem interna estável titan://app e entrada /local/login. O protocolo entrega UI compilada/rotas SPA e separa recursos ausentes de rotas; não abre servidor Vite/porta HTTP para servir a UI. ContextIsolation/sandbox/webSecurity ativos, NodeIntegration/webview/insecureContent desativados. Só arquivos internos são código; não carregar HTML/JS retornado por backend como interface. Links externos HTTPS exigem escolha explícita no navegador do sistema. Câmera limitada a vídeo da origem própria com autorização; não foi testado equipamento físico. Downloads usam diálogo do sistema, sem caminho arbitrário por IPC.

Desktop/src/config.cjs define configuração plana version1: mode, operational_url e online_url. Local exige literal loopback; remoto exige HTTPS; híbrido usa operacional local e destino online HTTPS. Caminhos /local/v1 e /api/v1 explícitos e não intercambiáveis; sem credenciais, query ou fragmento na URL. Campos desconhecidos/duplicados ou inválidos são recusados. Configuração em connection-v1.json fora da instalação, escrita temporária/sync/rename e cópia anterior; não altera .env. Não há servidor padrão escolhido/iniciado automaticamente. Janela de configuração em setup/ tem cinco idiomas e segue tema claro/escuro do sistema.

Partição persistente por configuração/destinos evita transportar tokens/cookies/IndexedDB/pendências para outro servidor; a partição anterior não é apagada. Retornar à configuração original reabre seu perfil. Browser de desenvolvimento e instalações antigas continuam separados; não migrar suas pendências por conveniência. Alteração com janela operacional requer confirmação e respeita beforeunload; não forçar fechamento para contornar proteção de formulários/operações.

Desktop/src/transport.cjs encaminha apenas as duas APIs para destinos fixos da configuração, preservando método/corpo e codificação dos IDs sem decodificar novamente. Não aceita URL arbitrária do renderer, Cookie/Host fornecidos pelo renderer ou redirect que possa levar bearer a terceiro. TLS permanece validado pelo Electron, sem exceção de certificado. Limites explícitos e erro indisponível, sem fallback entre autoridades. Correção encontrada na revisão do transporte: estouro do limite da RESPOSTA após envio precisa ser502incerto, nunca413descartável. Nova regressão usa cliente/fila POS reais: servidor sintético aceita venda e devolve resposta excessiva; tentativa permanece uncertain com IDs originais, não descartável, recuperação consulta recibo e mantém1POST. Limite do corpo ANTES do envio pode dar413sem transmitir. Não permitir que falha da ponte autorize apagar venda potencialmente registrada.

Frontend-web/src/core/api/api.ts muda somente seleção de transporte: no protocolo titan usa /api/v1 e identifica plataforma desktop; navegador de desenvolvimento conserva endereço anterior. Não altera login, modelo de sessão ou regra comercial. Operações locais continuam usando os clientes /local/v1 existentes. Internet configurada NÃO comprova sincronização comercial, modo cloud/hybrid no núcleo Go, cotas offline, compatibilidade de servidor ou autenticação online por cookies; esses aceites continuam pendentes.

### Build, execução e evidências

Scripts build/start/shortcut/smoke em desktop/scripts. Build usa dependências EXISTENTES, sem instalar, sem .env/config de Vite e sem devserver. Compila UI uma vez por conteúdo, verifica fonte estável durante a execução, hashes dos257arquivos e reutiliza estágio íntegro se nada mudou. Ajustado cwd da compilação para frontend-web para resolver os content globs do Tailwind; o aviso de fontes vazias deixou de ocorrer, sem alterar o frontend/tailwind.config.js. Aviso de chunk grande da planilha continua e não foi escondido.

Estágio final: desktop/.build/app-69f765b6-afb8-4182-8ff5-3b9167356232, source_hash aae5b7503301d4bda7a68629ca62f393bbfd883417ac04ccc166303868d36514. Vite build20.18s, TypeScript do build exit0. Segunda execução confirmou reused:true, sem outra cópia. Não inclui backend, banco, dependências, cache ou árvore inteira de builds em pacote. Preparações intermediárias de QA preservadas/ignoradas pelo Git; não são entregas comerciais. Electron44.9.2 e builder26.15.3 já estavam no laboratório; package-lock preserva metadados exatos das dependências existentes. Não houve npm install/ci, atualização automática ou download de runtime.

35testes desktop aprovados,0falhas/skips, incluindo configuração/ambiguidades, namespace/URLs, isolamento de destino, arquivos/SPA/symlink, headers/redirecionamento, corpo/abort, HTTP descartável real, idiomas, incerteza após envio e atalho sem sobrescrever. Suíte frontend2452aprovados,0falhas/skips; inclui os44testes LOCAIS da preparação pausada, não anunciar esses44como entrega nova publicada. TypeScript independente também exit0. Sem nova suíte Go inteira: backend permaneceu sem alteração.

Dois testes nativos repetíveis em Electron44.9.2 Linux headless, com perfil temporário separado: configuração e login real da UI compilada. URLs titan://app/desktop-setup/index.html e titan://app/local/login, sandbox/contextIsolation true, NodeIntegration false,0erros observados de preload/carregamento/console. Ses.fetch verificou assets/SPA/API,1requisição HTTP real somente ao stub loopback descartável; online não configurado retornou503, asset ausente404. Sem Vite, processo backend ou banco comercial. Rodar TITAN_ELECTRON_BINARY=CAMINHO npm run test:native após build; não confundir smoke de inicialização com venda comercial/hardware/Internet homologada.

Atalho local criado sem sobrescrever em /home/danillo-malta/Downloads/TitanSystem.desktop,538bytes,modo755; desktop-file-validate exit0. Aponta ao runtime já existente e ao estágio identificado. Não é instalador Windows/Linux assinado. Ubuntu poderá exigir “Permitir lançamento”. Não abrir durante uso de versão anterior; o controle de instância única preserva a janela já aberta. O usuário ainda escolhe os endereços autorizados na primeira configuração, sem banco escolhido pelo agente.

Prévia de setup em4359, com banner explícito e gravação APENAS em memória: modo híbrido/URLs sintéticas aceitos, remotoHTTPrecusado e remotoHTTPSaceito sem tentar conectar tais endereços. Cinco idiomas sem undefined/0erros de console. Screenshot real em outputs/configuracao-desktop-2026-10-25.png é da PRÉVIA, não de servidor comercial. A evidência nativa são os dois testes acima. Fontes/preimagens/hashes/logs em work/desktop-organization-20261025.

Falhas de ferramentas/fixture, corrigidas sem alterar dados: primeira fixture nativa interpretou o argumento --ozone no índice errado (“Cannot find module …/native-smoke.cjs/main.cjs”); passou após localizar argumentos relativos ao próprio script. Leitura auxiliar sem elevação do stub4359 retornou “connect EPERM127.0.0.1:4359”; repetida com acesso aprovado. Captura fullPage retornou “Unable to capture screenshot”; captura de viewport real concluída e inspecionada. Build inicial apontou Tailwind content ausente, corrigido pelo cwd, repetido. Não houve rejeição de aprovação automática.

PENDENTE: endereços/serviços/perfil/banco descartável aprovados, compatibilidade/identidade de versão operacional, autenticação online e cookies/CSRF contra HTTPS real, reconciliação/híbrido comercial, transporte entre empresas/aparelhos, hardware, atualização/restauração, instaladores/assinatura e teste Windows. O site público está apenas reservado. A organização desktop está implementada/testada nos limites acima; não representa encerramento do ERP completo. Funções de frontend continuam pausadas conforme pedido.

Referências técnicas consultadas: https://www.electronjs.org/docs/latest/api/protocol/ ; https://www.electronjs.org/docs/latest/api/session/ ; https://www.electronjs.org/docs/latest/tutorial/security/ . O runtime efetivamente testado foi o44.9.2existente, não uma versão deduzida da documentação atual.

Prévia4359 e sua aba encerradas após validação; nenhum processo/aba preexistente do usuário foi encerrado. O atalho foi validado sem executá-lo no perfil real.

Atalho adicional registrado no menu de aplicativos do usuário: /home/danillo-malta/.local/share/applications/titansystem-erp-atual.desktop, nome “TitanSystem ERP”,644,desktop-file-validate exit0. Arquivo novo, sem substituir registros anteriores; usa o mesmo runtime/estágio validado. Não foi executado no perfil real nem instala dependências. Pode ser removido individualmente para desfazer apenas esse registro.

## Revisão45 — piloto backend/API em dinheiro, 25/10/2026

O proprietário pausou as novas funções do frontend e delimitou um circuito útil para um cliente piloto: venda no PDV → caixa → estoque → reposição → pedido ao fornecedor → recebimento → financeiro, somente dinheiro. Critérios: tela consumindo API real, persistência ao reiniciar, backup/restauração e teste automatizado do circuito inteiro. Esta revisão encerra o aceite técnico desse recorte em base descartável, não a implantação comercial nem todo o ERP.

### Base e arquivos entregues

- Backend `~/Downloads/zzeus-frontend-backend-b02a`, branch `feat/backend-b02a-compat`, base `beaebbe6d85af07bd17bb5b02b11753df4f9ef96`, limpa no início. Commit de entrega `cf87809688a339fed0fcc99b748df08924a93db6`.
- Novo `backend/internal/localapi/pilot_cash_acceptance_test.go`: teste de aceitação que executa handlers e transações reais por TCP/HTTP, com subprocessos independentes.
- Novo `docs/architecture/PILOTO_DINHEIRO.md`: recorte, rotas, comandos de teste e backup, resultados e requisitos da instalação piloto. Documento específico de operação; o registro global de continuidade continua sendo somente este arquivo.
- Nenhum fonte Go de produção, migração, frontend ou Electron alterado: os contratos existentes cobrem este recorte. O trabalho acrescentou prova integrada, sem recriar as funções.
- Backend permanece no schema63; ZIP separado de devoluções/schema64 continua sem aplicação.

### Resultado reproduzível

1. Estoque inicial5unidades; venda2aR$15,99: totalR$31,98, recebidoR$40,00, trocoR$8,02. Uma venda/uma parcela; caixa registra valor líquido.
2. API encerrada abruptamente após a venda. Outro processo no mesmo SQLite recupera o recibo original sem novo POST. Estoque3unidades.
3. Política mínimo4/alvo5 sugere2; aprovação humana, fornecedor externo e pedido preservam as decisões. Confirmação do fornecedor registrada manualmente, sem envio eletrônico ou falsa autenticação entre empresas.
4. Viagem identificada, carga/chegada e recebimento conferido de2unidades; uma entrada, estoque5.
5. ObrigaçãoR$20,50 vinculada ao recebimento; criador não pode aprovar, outro gerente aprova. Retirada em espécie vinculada sem segunda movimentação. Saldo documentalR$0,00.
6. Fechamento oficial:1000+3198−2050=2148centavos, diferença zero. Novo reinício mantém16registros/rastreios/recibos, incluindo política/sugestão/aprovação e fornecedor.
7. Backup cifrado consistente com API ativa, integridade verificada, chave errada recusada. Restaura em caminho novo; recusa sobrescrever destino. Quarto processo abre a restauração, sessão antiga recebe401 e novo login recupera os mesmos IDs/valores. Repetições explícitas não duplicam venda, estoque ou pagamento.

### Verificações observadas

- Teste final:101requisições HTTP reais, quatro processos, aprovado em0,834s, sem skip. Detector de corridas na versão final: aprovado em13,555s. Não usa `app.Test` nem mocks nas operações do circuito; só provisionamento e licença são fixtures sintéticas.
- Suíte completa backend:1229testes principais aprovados;2636casos contando subtestes; zero falhas.51pacotes com testes aprovados;18sem testes. Sete testes ignorados:duas integrações PostgreSQL sem banco autorizado, três pontes opcionais, dois auxiliares de subprocessos. O auxiliar do piloto foi executado pelo teste principal.
- A suíte completa precedeu a inclusão de cinco consultas extras na comparação do teste novo; essa versão final ampliada foi reexecutada isoladamente e com detector de corridas. Nenhum fonte de produção mudou entre as execuções.
- Build real dos binários `titan-local` e `titan-backup`: aprovado, Go1.25.8 já disponível, sem instalar dependências. Binários Linux isolados no laboratório, aproximadamente31MiB no total, sem distribuir Electron/node_modules.
- Navegador com frontend compilado existente e proxy exclusivo para a API restaurada: login/contexto, histórico/comprovanteR$31,98, pedido/recebimento2unidades, financeiroR$20,50 aplicado/saldozero. Fonte real, sem respostas simuladas. Capturas locais identificam o piloto descartável. Não foi automação de cliques do circuito inteiro; a automação cobre HTTP e persistência. A leitura visual da tela de caixa não foi concluída após encerramento143 do proxy (`net::ERR_CONNECTION_REFUSED`); seu fechamento foi comprovado por HTTP/SQL.
- Frontend pausado:8/8hashes idênticos aos da revisão44. Não se executou aplicador, instalação, ZIP, alteração de banco comercial ou configuração pessoal do Electron. Nenhum TypeScript/build frontend reexecutado nesta unidade porque não houve alteração desses fontes.
- Laboratório/evidências: `Documents/Codex/2026-10-20/github-plugin-github-openai-curated-remote-2/work/pilot-cash-20261025/`. Logs não publicam tokens/senhas. Capturas em `outputs/piloto-api-venda-restaurada-2026-10-25.png`, `piloto-api-recebimento-restaurada-2026-10-25.png` e `piloto-api-financeiro-restaurado-2026-10-25.png`.

### Encerramento e próxima base

Aceite técnico local em dinheiro concluído. Próxima ação: identificar a instalação de homologação do cliente piloto e a combinação exata de versões, configurar empresa/loja/aparelho/usuários/licença/serviço e custódia da chave de backup, e homologar com o operador. Não iniciar instalação comercial ou escolher banco/chaves por conta própria. Aguardar a nova base do proprietário antes de outra unidade funcional.

Backup atual tem limite de snapshot128MiB; arquivo da estação e chave de cifragem precisam de guarda separada. Internet/TLS comercial, Windows, hardware, fiscal e operação offline multiaparelho não foram homologados por este teste. Demais módulos continuam no estado registrado anteriormente; não anunciar o backend inteiro como concluído.


## Revisão46 — complemento do piloto em dinheiro; aceite visual então aberto (resolvido na revisão47)

A revisão45 encerrou cedo o aceite: comprovou o circuito por API e consultas de telas, mas não a operação inteira originada nas telas. Esta revisão corrige esse registro. Naquele momento, a entrega não podia ser anunciada como completa; a revisão47 registra a conclusão posterior do encerramento visual e da restauração desses mesmos registros.

### Alterações restritas ao piloto

- `frontend-web/src/core/local/cashMovement.{mjs,d.mts}`, `cashMovementWords.{mjs,d.mts}` e `modules/pos/pages/OfficialCashMovement.tsx`: sangria/suprimento oficiais com revisão explícita, centavos inteiros, POST real e consulta do recibo original. As solicitações locais anteriores continuam distintas. A retirada não paga o documento; o vínculo financeiro existente não retira dinheiro novamente.
- `LocalPointOfSale.tsx` monta esse painel e bloqueia a operação concorrente da mesma tela enquanto há edição/tentativa pendente. Web Lock compartilhado com o PDV; pendências de venda/carrinho impedem o novo POST. Guarda comando/contexto, sem token ou senha. Consulta antiga após logout/troca de empresa, loja, usuário ou aparelho não pode enviar POST. Operação incerta permanece guardada; consulta não reenvia e descarte fica bloqueado enquanto incerta.
- `FinalClose108.tsx`: confirmação final em diálogo acessível da própria tela, preservando contagem, revisão, confirmação explícita e nova validação da assinatura antes do encerramento. Substitui a confirmação nativa que bloqueou o navegador do ensaio. Precisa concluir a verificação visual descrita abaixo.
- `LocalMainMenu.tsx`: financeiro visível para quem já possui `view_accounting`; não amplia permissões. O gerente revisor já podia usar a API e o link financeiro nos pedidos, mas o menu escondia esse destino.
- `desktop/tests/pilot-api-client.cjs` e backend `pilot_native_client_test.go`: ponte exclusivamente de testes pelo transporte de produção `titan://app`/Electron, sem scripting do renderer e sem API falsa. Os testes iniciam instâncias isoladas com dados temporários; não mudam o perfil pessoal do aplicativo.
- Backend `pilot_screen_flow_test.go`: fixture opcional que parte sem venda/pedido/recebimento/pagamento e aguarda as operações das telas. Depois confere SQL/HTTP, reinício e backup/restauração dos mesmos IDs. Retomada explícita aceita somente snapshot descartável privado preservado após timeout; renova somente a licença sintética, sem reconstruir negócios. Nenhum código Go de produção ou migração alterado.

### Resultados observados nesta revisão

- Circuito automatizado completo via Electron real:101requisições, quatro execuções isoladas do aplicativo e quatro processos API; aprovado em2,125s. A versão com detector de corridas aprovou em14,914s (teste13,89s). Venda3198, troco802, estoque5000→3000→5000, retirada2050, caixa2148/diferença0, obrigação aplicada2050/saldo0; reinícios, backup cifrado verificado/restaurado, sessão antiga401 e IDs preservados. Sem mock nas operações.
- Suíte backend completa:1229testes principais/2636incluindo subtestes aprovados; zero falhas. Oito skips: dois PostgreSQL sem banco autorizado, três pontes opcionais, dois auxiliares de subprocessos e a fixture visual opcional. O circuito nativo acima foi executado explicitamente, sem skip.
- Frontend completo:2462testes aprovados. Depois, seis casos adicionais de proteção foram incluídos e aprovados com os dez anteriores de movimentação; regressão final direcionada84/84 de financeiro116, fornecedor117 e fechamento108, incluindo16da nova fonte. A execução restrita que mostrava113arquivos sem casos internos não foi aceita como evidência; a suíte foi executada com subprocessos efetivos. Desktop35/35; TypeScript e build final aprovados, Vite21,82s. O aviso do chunk grande de planilha continua; não foi removido ou escondido.
- Hashes dos oito arquivos pausados do relatório de carrinhos:8/8inalterados. Não foram incluídos nesta entrega. Sem instalação, ZIP, banco comercial ou configuração pessoal do Electron.

### Ensaio de telas: avanço real e bloqueio exato

As telas criaram a venda de2unidades/R$31,98, fundoR$10,00, reposição mínimo4/alvo5, aprovação humana, fornecedor externo, pedido, confirmação manual, autorização local, viagem identificada/carga/chegada, recebimento2unidades, obrigaçãoR$20,50 e aprovação por outro gerente. O operador registrou a retirada oficialR$20,50 (POST200 e reciboGET200), depois vinculou a parcela: aplicadoR$20,50/saldoR$0,00. Nenhum desses negócios foi criado pelo driver do ensaio.

A contagem física deR$21,48 foi salva e revisada. Ao solicitar encerramento, a confirmação nativa bloqueou o navegador interno: `Input.dispatchMouseEvent`/`Emulation.setFocusEmulationEnabled` excederam o tempo e a consulta de diálogo retornou `undefined`. Não se observou POST de encerramento no log. O primeiro ensaio expirou após1800,53s e está registrado como FAIL, não PASS. Antes da expiração foi preservado snapshot consistente somente do banco descartável, em diretório privado. A retomada usa os mesmos negócios; não cria outra venda/pedido/pagamento. A nova confirmação interna compila, mas o navegador antigo ainda precisa ser liberado para verificá-la.

Pendência então registrada, resolvida na revisão47: cancelar/fechar a confirmação antiga da aba do piloto na porta4361; retomar as telas na versão final, confirmar o encerramento e verificar saldo2148/diferença0. Só então sinalizar `DONE` à fixture, conferir reinício/restauração e reler o resultado pela tela antes de `FINISH`. Não marcar esses passos como executados antecipadamente. O teste automatizado completo independente já passou; não substitui essa pendência visual assumida nesta revisão.

Evidências locais: `Documents/Codex/2026-10-20/github-plugin-github-openai-curated-remote-2/work/pilot-complete-20261025/evidence/` (`native-final.log`, `native-race.log`, `backend-full.jsonl`, `frontend-full-effective.tap`, `focused-final.tap`, `desktop-tests.log`, `build-final.log`, `browser-http.jsonl`, `screen-flow.log`, `screen-resume.log`). Snapshot/descritor sintéticos em `screens/` privado; não publicar. Captura da venda em `outputs/piloto-completo-venda-2026-10-25.png`.

Novas funções do frontend permanecem pausadas. Schema63 preservado; ZIP64 não aplicado. Não iniciar instalação comercial nem escolher banco, estação, chaves ou serviço do cliente. Aguardar a nova base somente depois de concluir e comunicar o aceite final desta unidade.


## Revisão47 — piloto em dinheiro concluído com aceite das telas e restauração

25/10/2026, conclusão observada às04:09 no Ubuntu. Escopo fechado: uma empresa/loja/aparelho, PDV → caixa → estoque → reposição aprovada → pedido a fornecedor externo → recebimento identificado → obrigação aprovada por outro responsável → pagamento em dinheiro e fechamento oficial. Não representa a conclusão de todos os módulos do ERP.

### Liberação e resultado final

O proprietário informou ter fechado as abas4361 do navegador interno do Codex. A nova aba66 respondeu normalmente. Foi retomada a MESMA base descartável preservada: venda `0b57c82e-82fc-401c-b84b-ba1b67c9a7e5`, pedido `aff41294-2f79-4d21-b2aa-7ac1d2db890c`, viagem `3600f64c-640f-4b9b-972e-11dccb8e85cc` e obrigação `c1541c9a-761e-46d3-bc2d-3f3bdd811dcb`. Não se refez venda, retirada, pedido, recebimento ou pagamento.

A contagem local anterior reapareceu confirmada emR$21,48. A revisão final mostrou uma venda/R$31,98 e nenhuma pendência eletrônica. A nova confirmação interna acessível foi apresentada com valor e limites; somente seu botão explícito enviou o encerramento real. Resposta da tela: **caixa fechado, esperadoR$21,48, diferençaR$0,00**. Livro oficial consultado: esperado/contado2148, diferença0, três lançamentos e nenhuma classificação ausente. É livro operacional, sem promessa de contabilidade fiscal.

O marcador `DONE` foi criado apenas DEPOIS dessa confirmação. A fixture verificou quantidades/somas por SQL e sete recursos originais por HTTP; reiniciou a API no mesmo banco; criou, verificou e restaurou backup criptografado em outro caminho; abriu nova API e recusou a sessão antiga com401. Novo login recuperou os mesmos recursos, IDs e valores, sem POST de negócio do driver.

Na tela conectada ao BANCO RESTAURADO foram confirmados: histórico e recibo original da venda de2×R$15,99/R$31,98, dinheiro confirmado e caixa fechado; estoque físico5,000/reservado0,000/livre5,000 no local original; obrigação aprovadaR$20,50, aplicado líquidoR$20,50, saldoR$0,00 e comprovante `PAGAMENTO-PILOTO-001` preservado. Um novo login do gerente `pilot-reviewer` confirmou o cartão financeiro no menu e a consulta real do documento/saldozero, sem nova permissão. Só então foi criado `FINISH`.

**Resultado efetivo:** `TestPilotCashScreenFlow` PASS em1647,621s na retomada, incluindo espera humana. Log `screen-resume.log`: `PASS: original UI-originated sale/order/receipt/payment/closure preserved across restart and encrypted backup restoration; no workflow POST from acceptance driver`. A tentativa inicial expirada permanece registrada como FAIL; não foi apagada nem contada como sucesso. A retomada não reconstruiu os negócios: renovou apenas a licença sintética do ensaio, como descrito na revisão46.

### Critérios encerrados e entrega

- Telas chamam API Go real: operações de todo o circuito observadas por cliques, POST/recibos oficiais, sem mock; nenhum banco comercial utilizado.
- Reinício não perde o circuito: comprovado tanto com os registros das telas quanto no teste automatizado independente.
- Backup/restauração: snapshot consistente criptografado, verificado e restaurado; comparação de IDs/valores e releitura pelas telas aprovadas. Chave errada e sobrescrita são recusadas no teste automatizado.
- Teste automatizado inteiro:101requisições pelo transporte de produção Electron, quatro execuções do app/API, aprovado normal e com detector de corridas; não é automação de cliques Electron. O ensaio visual complementar utiliza os mesmos assets React compilados e proxy loopback para a API real.
- Validações da revisão46 mantidas:2462casos na suíte frontend completa e seis adicionais aprovados depois; regressão final84/84,16da nova fonte;35/35desktop;1229principais/2636com subtestes backend, zero falhas; TypeScript/build final21,82s. Nenhum código de produção mudou depois dessas verificações: esta revisão só atualiza documentos.
- Entregas já publicadas e hashes remotos conferidos: frontend `62df646d8130717631f0dc52694cca6f3d1c0627`; backend `f0a126b2cb6fd12433c9bed48568a7373cc59e56`. As atualizações documentais desta revisão vêm depois desses commits nas MESMAS branches, sem merge/main/force-push. Oito arquivos pausados continuam preservados e fora dos commits. Nenhum pacote ZIP novo, instalação, banco comercial ou migração.

Capturas reais: `outputs/piloto-caixa-encerrado-2026-10-25.jpg` (livro oficial com esperado/contadoR$21,48 e diferença zero) e `outputs/piloto-financeiro-restaurado-final-2026-10-25.jpg`. Logs completos no laboratório da revisão46. Os descritores e snapshots sintéticos privados não são publicados.

**Encerramento:** esta unidade piloto em dinheiro está concluída no ambiente descartável validado. Não iniciar outra função. O proprietário pode fornecer a nova base. Para implantação em cliente, a próxima etapa deve identificar instalação, versões, empresa/loja/aparelho, perfis/licença, serviço e guarda das chaves de backup explicitamente; não escolher dados ou credenciais pelo usuário. Fornecedor externo tem confirmação manual registrada; o teste não enviou pedido comercial para terceiro. Windows, Internet comercial, fiscal, cartões/Pix e offline multiaparelho permanecem fora deste recorte. O limite atual de snapshot128MiB continua documentado.


## Revisão48 — provas publicadas e correção do alcance do aceite

O proprietário solicitou os logs e confrontou a declaração de conclusão com o relatório datado de21/10. A prova posterior existe, mas a resposta “terminei o que pediu” foi ampla demais sem informar seus limites. Revisões45–47 comprovam um circuito LOCAL em dinheiro, fornecedor externo com confirmação MANUAL registrada pelo operador e recebimento INTEGRAL sem divergência. Não comprovam envio/aceite autenticado entre empresas nem recebimento parcial com divergência nesse mesmo circuito. A automação percorre as APIs reais/transporte Electron; os cliques do ensaio visual foram assistidos, não um E2E de UI autônomo em CI.

Publicados no backend, branch `feat/backend-b02a-compat`, em `docs/architecture/evidence/pilot-cash-20261025/`: `native-final.log`, `native-race.log`, `screen-flow.log` (primeira tentativa FAIL), `screen-resume.log` (retomada PASS), `README.md` com assertivas de reinício/restauração e limites, `ci-scope.json` e `SHA256SUMS`. Logs originais copiados sem reescrita; nenhum banco, token, senha, chave ou dado comercial incluído. Fontes dos testes e reprodução estão no guia específico.

A API GitHub Actions retornou `total_count:0` nas duas branches entregues. O workflow atual dispara push somente em main/develop e PR destinado a elas. Não há log CI verde para apresentar. Os logs publicados são execuções locais durante desenvolvimento antes dos commits, não comprovação de runner remoto executando o SHA atual. O README da branch padrão não descreve automaticamente o estado dessas branches; usar os links diretos de evidências/fontes para análise.

Aceite ampliado solicitado agora: confirmação autenticada do fornecedor, recebimento parcial com divergência, continuidade após reinício e restauração nesse cenário, além de log CI publicado. **Esse aceite ampliado NÃO está concluído com as provas atuais.** Não esconder isso por trás do PASS do circuito integral/manual. Esta revisão organiza e publica evidências; não altera código, não configura/dispara CI, não gera pacote e não inicia outra função. Os oito arquivos pausados permanecem intactos. Não há matriz medida que sustente porcentagem global de25–30%ou50%; não repetir estimativas sem requisitos e critérios contados.


Esclarecimento da revisão48 após nova consulta e autorização explícita do proprietário: o repositório tem32execuções Actions; o run32 da main (024dd3f50d2280c9c1ae6e2ba3f829053bbe84c7) foi aprovado: https://github.com/danilomalta/zzeussss/actions/runs/39305932267. “Zero execuções” continua correto somente para feat/backend-b02a-compat e feat/backend-checkout-authority. Não significa inexistência de CI no repositório. As URLs REST exatas e os escopos estão em ci-scope.json da evidência backend. O run da main anterior não comprova o piloto novo nas branches. Publicação autorizada SOMENTE de evidências e documentos nessas branches; sem alteração de código/workflow, branch nova ou merge.


## Revisão49 — backend: recebimento parcial, backup executável e CI verde

25/10/2026. Continuidade autorizada pelo proprietário para uma nova unidade de backend. Base backend `8d8d91cce137cce670937ea3936df7cdb1970725`; entrega publicada `55fdec11843dfd782679767edb58939176ad4c92`, branch `feat/backend-b02a-compat`, sem merge/main. Frontend permanece na base `644932bef25c7bdbc68fcf428539acb4dad984f5`; só este registro é atualizado. Oito arquivos pausados conferidos por SHA-256:8/8 intactos. Schema63 preservado; ZIP64 não aplicado.

A nova unidade contém12arquivos: workflow dedicado, README da branch, CLI de backup e testes, inspeção autenticada e testes/compatibilidade, piloto parcial e execução de backup pelo processo compilado, guia `BACKEND_PILOTO_AMPLIADO_20261025.md` e contrato JSON da inspeção. Não altera telas/Electron nem inicia serviço comercial. O recebimento parcial utiliza os contratos existentes; a ampliação principal nessa área é prova de integração, não uma nova API de recebimento.

O teste integral conserva101requisições HTTP/quatro processos; o parcial executa119/cinco. No parcial: entregue2000, aceito1000, recusado1000 e restante1000 milésimos; avaria abre tratativa de reposição, preservada em andamento. Estoque5000−2000+1000=4000; documento informado1025centavos somente para o aceito; aprovação exige outro responsável; caixa1000+3198−1025=3173, diferença0, obrigação em aberto0. Reenvio idempotente não duplica; mudar quantidade na mesma operação retorna409. Reinício no meio do recebimento e restauração preservam19recursos, IDs e dados comerciais. Apenas `measured_at` de consulta é excluído da igualdade depois de validar seu formato; datas comerciais permanecem comparadas.

Backup foi executado pelo binário real: key-init/create/verify/inspect/restore em dados sintéticos, com chave errada, escopo errado e sobrescrita recusados. `inspect` autentica o arquivo cifrado e informa hashes/contagens das14tabelas nomeadas do piloto; não imprime registros, chaves ou sessões nem migra/restaura a origem. Compatibilidade histórica25–63 testada; tabelas ausentes são indisponíveis, não zero. Hash comercial e contagens iguais antes/depois da restauração; hash do arquivo cifrado pode diferir. Limite128MiB mantido. Help não lê banco/chaves.

### Evidência remota e local

Run GitHub aprovado no SHA exato55fdec1: https://github.com/danilomalta/zzeussss/actions/runs/40190016452 . Vet, compilação estática Linux, suíte completa e os dois pilotos com CLI real e `-race`: todos success. Suíte CI:1233testes principais/2640incluindo subtestes, zero falhas, oito skips opcionais (PostgreSQL/pontes/auxiliares/fixture visual); pilotos explícitos sem skip,29,980s. Artefato `backend-pilot-55fdec11843dfd782679767edb58939176ad4c92`, retenção14dias, contém log completo JSON, log do piloto e binários. Agora há CI na branch backend; a afirmação de zero execuções da revisão48 é histórica, não o estado atual. Workflow dedicado dispara push dessa branch, PRmain/develop ou manualmente; não publica/deploya nem faz merge.

Local: suíte antes do novo teste help1232principais/2639com subtestes, zero falhas/oito skips; quatro testes CLI finais aprovados depois. Pilotos CLI com race24,894s, estáticos2,212s; pelo transporte de produção Electron e CLI final4,927s,101+119requisições e4+5processos. É transporte nativo de API, não automação de cliques. Nenhum novo ensaio visual Windows ou Electron renderer nesta unidade. Logs em `work/backend-pilot-expanded-20261025/evidence/`; cópia baixada do CI conferida separadamente.

Pacote local concluído: `outputs/TitanSystem_Backend_Piloto_Parcial_BackupCLI_CI_20261025.zip`,17333246bytes (16,53MiB), SHA-256 `95365e19e24331ccf434903f7f146b6b3b526c4aa69ad17ba2bf979680c21ef0`. CRC,25arquivos/hashes do manifesto e verificador após extração aprovados; help do binário extraído executado sem abrir banco. Inclui12fontes exatos, patch entre commits, hashes anteriores/posteriores, dois executáveis Linux amd64 estáticos Go1.25.8 e evidências. Já aplicado na referência local: NÃO reaplicar nem substituir arquivos atuais com payload. Não contém node_modules/cache/banco/chave. Aplicação em outra cópia requer conferir regras/status/base/hashes e `git apply --check`; nenhum aplicador automático.

**Unidade encerrada; ERP inteiro não declarado pronto.** Recebimento parcial com divergência, CLI de backup e CI agora comprovados no circuito LOCAL em dinheiro. Aceite de fornecedor continua MANUAL; confirmação autenticada entre empresas permanece pendente. Tratativa de reposição não é concluída automaticamente. Windows em máquina limpa/instalador e adaptação das permissões de arquivos ainda pendentes; binários entregues somente Linux. Fiscal, TEF/Pix, Internet comercial e offline multiaparelho não homologados por este teste. Próxima unidade deve definir contrato e identidade entre empresas antes de implementar o aceite remoto; implantação exige ambiente e guarda de chaves explicitamente definidos. Sem percentual global sem matriz medida, sem banco comercial, instalação de dependências ou limpeza destrutiva.


## Revisão50 — instalação no Ubuntu e serviços preservados

25/10/2026,15:29 local. O proprietário solicitou instalação direta em vez de instalar o ZIP. Depois escolheu explicitamente: **“Apenas instalar; preservar os dois serviços”**. Instalação concluída por usuário, sem sudo, download de dependências ou criação de empresa/banco/licença.

Executáveis da revisão49 instalados em `~/.local/opt/titansystem/backend/55fdec11843dfd782679767edb58939176ad4c92/`; comandos novos `~/.local/bin/titan-local` e `~/.local/bin/titan-backup` apontam para essa versão. Desktop compilado validado instalado em `~/.local/opt/titansystem/desktop/4e7a50ff66a205baadc9a6693b37e061a0e89596583d8df8c9d950995f358dc7/`. Runtime Electron44.9.2 já disponível foi copiado uma vez para `~/.local/opt/titansystem/runtime/electron-44.9.2-linux-x64/`; não inclui node_modules ou caches do projeto. Atalho do menu `~/.local/share/applications/titansystem-erp-atual.desktop` atualizado para esses caminhos estáveis. Atalho anterior preservado em `.desktop.20261025T182924Z.bak`. Repositórios/estágios originais preservados. Nenhum arquivo de fonte foi alterado por essa instalação; só este registro versionado é atualizado.

Verificação: hashes de todos os arquivos instalados iguais às fontes, comandos resolvem para os binários esperados; help de backup executado sem banco. Electron instalado abriu a janela de conexão em perfil descartável: zero erros, sandbox/contextIsolation ativos, origem `titan://app`, sem Vite nem banco. Dois pilotos reais com o backup instalado passaram em2,171s:101+119requisições, reinício/restauração e recebimento parcial preservados. Evidências locais em `work/backend-install-20261025/installed.json`, `installed-desktop-smoke.log` e `installed-backup-pilots.log` no laboratório Documents/Codex. Inventário SHA-256 da instalação registrado; não contém credenciais/bancos.

Serviços anteriores confirmados intactos depois: PID1362657,porta8182, executável em `titansystem-tests/pdv-zt69por7`; PID3855147,porta8282, em `titansystem-tests/pdv-023hym2n`. Não foram reiniciados, substituídos ou configurados para o novo backend. Instalar executáveis **não significa que os dois serviços passaram a executar55fdec1**. Nenhum banco, estação ou chave desses serviços foi lido. Rede comercial e instalação global do ERP continuam com os limites da revisão49. Para abrir o software instalado, usar **TitanSystem ERP** no menu Ubuntu; a conexão permanece escolha explícita do responsável. Reverter o atalho exige somente restaurar sua cópia anterior; versões novas e antigas não foram apagadas. Oito arquivos frontend pausados seguem8/8intactos.


## Revisão51 — backend: pedidos e decisões autenticados entre empresas

25/10/2026. Continuidade de backend autorizada pelo proprietário, sem pedir instalação manual e preservando a escolha de não trocar os serviços8182/8282. Base `55fdec11843dfd782679767edb58939176ad4c92`;31arquivos publicados em `3adf48088379bef7d25adebe37b699906e53d532`, branch `feat/backend-b02a-compat`, sem main/merge. Frontend não foi implementado nesta unidade; oito arquivos pausados conferidos8/8intactos. Somente este registro de continuidade é atualizado na branch frontend.

Contrato `docs/api/company-trade.openapi.json`:13operações em12caminhos, com vínculo de empresa/loja/aparelho e fingerprint aprovado pelo dono, exportação de pedido previamente aprovado, importação autenticada no fornecedor, decisão explícita assinada por sua estação e aplicação no comércio ligada ao envelope original. Identidade técnica/fingerprint não comprovam CNPJ/identidade legal por si só; a conferência empresarial é responsabilidade explícita dos donos por canal independente. O token de uma instalação não autentica na outra. Novas rotas não aprovam pareamento interno, não reservam estoque/capacidade e não movimentam dinheiro automaticamente. Consultas paginadas de pedidos/mensagens/vínculos e recibos por operação permitem recuperar resultados incertos sem reenvio automático. Fluxo manual anterior preservado; decisão autenticada não pode ser sobrescrita por registro manual.

Migração incremental64 cria as cinco tabelas comerciais/de recuperação e gatilhos, preservando migrações1–63 byte a byte. A revisão local identificou risco de restaurar confiança anterior a uma revogação: cópia restaurada com vínculos passa a exigir revisão explícita do dono antes de novas trocas. Cada restauração gera identificador novo, revisão anterior não o desbloqueia, gerente não pode aprovar, falha de persistência impede publicação do destino. Histórico/IDs/assinaturas e operações incertas continuam legíveis. Não há descoberta automática de revogações remotas. A ferramenta especializada de revisão não estava disponível; houve revisão local e testes, sem alegar auditoria externa.

**Não aplicar o ZIP64 antigo de devoluções sobre esta base.** Ele continua não aplicado; esta migração64 tem outro conteúdo/checksum. Comparar/reconciliar fontes antes de qualquer combinação, sem forçar migração ou sobrescrever banco. Nenhum banco dos dois serviços preservados foi migrado. Backups históricos25–64 foram testados; inspeção `pilot_cash_v1` permanece restrita às14tabelas declaradas e não é usada como prova isolada das novas tabelas. Estas foram relidas pelas APIs depois de reinício/restauração.

### Evidências finais

CI aprovado no SHA exato: https://github.com/danilomalta/zzeussss/actions/runs/40224296905 . Vet, compilação dos seis executáveis, suíte completa e testes de troca/pilotos com CLI real e detector de corridas: todos success. Suíte local E remota:1246testes principais/2665incluindo subtestes, zero falhas, oito skips documentados (PostgreSQL/pontes opcionais/auxiliares/fixture visual). Pilotos explícitos sem skip. Etapa remota com race168,846s; local133,273s. Primeira execução de desenvolvimento detectou inventário de rotas ausente e probes de migração usando a versão64 agora ocupada: inventário corrigido e probes movidos à próxima versão65, preservando provas de rollback; suíte final inteira reexecutada depois das correções e do gate de restauração.

Terceiro piloto percorreu venda→caixa→estoque→reposição→pedido→fornecedor autenticado→recebimento parcial→financeiro. Comércio:131chamadas HTTP/cinco processos; fornecedor independente:8chamadas/três processos, banco/chave/sessão próprios. Pedido importado sobrevive ao reinício do fornecedor; sua decisão assinada sobrevive ao backup/restauração e chega idêntica ao comércio. Comércio mantém estoque5000−2000+1000=4000milésimos, obrigação1025centavos somente para o aceito, caixa3173/diferença0, saldo a pagar0 e tratativa de reposição em andamento. APIs recusam assinatura adulterada, outro destino/empresa/loja/aparelho, quantidades inválidas, mensagens expiradas, divergência/replay e escrita parcial. O mesmo piloto pelo transporte de produção Electron instalado passou em3,800s,131requisições e cinco execuções isoladas do app; fornecedor fez as outras8requisições HTTP. É transporte nativo, não automação de cliques ou UI nova para as APIs de troca.

### Distribuição e instalação realizadas

Instalados os seis binários Linux amd64 estáticos PRODUZIDOS PELO CI: `titan-local`, `titan-backup`, `titan-access`, `titan-peer`, `titan-receive`, `titan-sync`. Destino `~/.local/opt/titansystem/backend/3adf48088379bef7d25adebe37b699906e53d532/`; ponteiro `backend/current` e comandos `~/.local/bin/titan-*` conferidos por hash contra o artefato. Versão55fdec1 mantida. Sem emissor de licença, download de dependências no Ubuntu ou criação de instalação comercial. Validação NO DESTINO com executáveis reais: init descartável, três execuções da API,11assertivas HTTP, login/contexto, reinício, backup/verify/inspect/restore, sessão original recusada no restaurado e mutações sem emissor indisponíveis; PASS. Nenhum banco/chave comercial foi lido.

PIDs1362657 e3855147 conferidos depois da instalação, ainda nos executáveis anteriores de `titansystem-tests/pdv-zt69por7` e `pdv-023hym2n`, portas8182/8282. Não foram reiniciados/atualizados. O atalho desktop da revisão50 permanece preservado. Instalação dos comandos não significa ativação desses serviços antigos na versão nova.

Pacote concluído: `outputs/TitanSystem_Backend_Aceite_Empresas_3adf480_20261025.zip`,47451142bytes (45,25MiB), SHA-256 `4faa7782503469faab3f1840eb9b7ced8a26104fa41214636ca28b902b74011a`. CRC,49arquivos do manifesto e verificador após extração aprovados; patch também passou `git apply --check` contra a base55fdec1 em cópia isolada. Contém fontes exatos, contratos, hashes antes/depois, seis executáveis e evidências. Sem node_modules, caches, Electron duplicado, bancos, estações ou chaves. Já aplicado/instalado: não reaplicar payload/patch sobre a árvore atual. Laboratório `work/backend-trade-20261025/` guarda logs completos, metadados do CI e recibo da instalação. Artefato GitHub `backend-pilot-3adf48088379bef7d25adebe37b699906e53d532`, retenção14dias; fontes permanecem no commit.

**Unidade de backend encerrada com resultados observados.** Aceite assinado entre instalações independentes e proteção de confiança na restauração agora comprovados. Transporte é relay explícito por cliente autorizado; não há worker/relay público ou Internet comercial homologada. Integração das novas rotas nas telas, renegociação/rotação de chave para a mesma identidade, reserva produtiva remota, Windows, fiscal/TEF/Pix e demais módulos não foram declarados concluídos. Sem percentual global sem matriz medida. Próxima unidade deve tratar transporte operacional e integração das novas APIs conforme o ambiente escolhido, mantendo a fronteira de dados e a aprovação humana; não ativar banco comercial por inferência.

## Revisão52 — abertura do desktop corrigida e cinco integrações obrigatórias no CI

25/10/2026. O proprietário relatou que o software instalado não abria. O diagnóstico executou o Electron instalado e distinguiu falha de atalho/configuração da renderização. O atalho em Downloads ainda apontava para um estágio antigo do laboratório. O perfil efetivamente aberto pelo runtime era `~/.config/TitanSystem`, e nele não existia configuração de conexão; o nome antigo de perfil não era a configuração em uso.

### Correção aplicada no Ubuntu

Criado `~/.local/bin/titansystem-desktop`, executável que abre a instalação estável, remove somente `ELECTRON_RUN_AS_NODE` do ambiente e guarda erros de inicialização em `~/.local/state/titansystem/desktop-startup.log`, com umask077. Três atalhos agora apontam para esse lançador: menu `~/.local/share/applications/titansystem-erp-atual.desktop`, `~/Downloads/TitanSystem.desktop` e `~/Área de trabalho/TitanSystem.desktop`. Os dois atalhos preexistentes foram preservados em cópias `.startup-fix-*.bak`; o da área de trabalho foi criado. Os três passaram `desktop-file-validate`. Não foi desativado o sandbox nem instalada dependência.

A conexão nova, criada somente após comprovar ausência de arquivo anterior, aponta `http://127.0.0.1:8182/local/v1`, modo local. Esse destino foi derivado do processo real da interface3002, executando `scripts/demo-vite.mjs 3002 8182`, e das respostas de saúde, não de uma escolha arbitrária de banco. Isso faz o desktop usar o mesmo serviço de teste que o usuário já acessava. Não é seleção de instalação comercial. O Electron abriu `TitanSystem — Local`; janela visível1280×850 e tela de login renderizada foram verificadas por captura da própria janela, sem preencher credenciais. Prova: `outputs/titansystem-desktop-login-corrigido-20261025.png` no laboratório. O aviso de driver MESA observado não impediu a renderização; não foi tratado como erro fatal. A janela pode ser fechada pelo usuário depois dessa verificação.

Inventário da correção em `work/desktop-startup-fix-20261025/installed-fix.json`. Para abrir: **TitanSystem ERP** no menu Ubuntu ou `~/.local/bin/titansystem-desktop`. Não depende de Vite para servir as telas compiladas; depende de a API configurada estar disponível. Serviços8182/8282 mantidos com os PIDs1362657/3855147 e executáveis anteriores, sem reinício, migração ou troca de banco. Instalação dos binários3adf480 não equivale à atualização desses processos. Não foram lidas credenciais ou bancos comerciais. Os oito arquivos frontend pausados continuam8/8iguais aos hashes anteriores.

### Backend: validação que agora falha se omitir integrações

Base limpa `3adf48088379bef7d25adebe37b699906e53d532`; quatro arquivos publicados em `3ab18e1e600930365dd7e4949479570f463948bb`, branch `feat/backend-b02a-compat`. Workflow, verificador Python, sete regressões do verificador e guia `BACKEND_INTEGRACOES_OBRIGATORIAS_20261025.md`. Sem alteração no código comercial, migração ou nos seis binários já instalados.

Antes, a suíte declarava oito testes ignorados. Foram executados e aprovados os dois testes PostgreSQL reais (sessões e catálogo) e as três pontes de adaptadores frontend contra HTTP Go real (livro de caixa, recebimento e manutenção). PostgreSQL17.9 local em cluster novo, diretório temporário privado, senha sintética não publicada, porta livre e bancos novos; somente esse cluster foi parado depois. Sem uso de5432, banco comercial, sudo ou instalação.

O CI cria seu próprio PostgreSQL17 em porta dinâmica, lê adaptadores do frontend fixados no commit `b5b786d900365dfa6288f6273123d4ca8f1a6a7b` e executa os cinco dentro da suíte inteira. Node24 sem npm install. `verify_backend_acceptance.py` exige PASS dos cinco testes E seus pacotes; rejeita falha, teste omitido ou skip inesperado. Só permite os dois auxiliares subprocessados e a fixture visual explícita. Sete regressões do gate aprovadas, incluindo pacote incompleto, JSON inválido, relatório vazio e omissão de cada teste obrigatório. YAML analisado antes da publicação.

Resultados finais locais E remotos:1251testes principais aprovados,2670incluindo subtestes, zero falhas, três skips explícitos (`TestMigrationCrashChild`, `TestPilotCashProcess`, `TestPilotCashScreenFlow`). As cinco integrações obrigatórias passaram dentro da suíte completa, não somente isoladas. Pacote localapi local399,438s; sete regressões Python adicionais aprovadas. O gate novo também foi executado sobre o relatório anterior com oito skips e o recusou corretamente.

CI aprovado no SHA3ab18e1: https://github.com/danilomalta/zzeussss/actions/runs/40236732412 . Vet, seis executáveis, sete regressões do gate, suíte integral e três pilotos com CLI/race: todos success. Etapa dos pilotos168,325s; sem corrida detectada. Relatórios do artefato remoto foram baixados e verificados novamente pelo gate:1251/2670/zero falhas/três skips, iguais ao resultado local. Artefato `backend-pilot-3ab18e1e600930365dd7e4949479570f463948bb`, retenção14dias. Só os três relatórios foram preservados localmente, sem duplicar os seis binários. Logs/SHA256 em `work/backend-postgres-complete-20261025/evidence/` e `ci-artifact/`; resultado remoto em `ci-result.json`. Os binários instalados permanecem3adf480: código comercial idêntico, sem reinstalação fictícia. Nenhum novo ZIP foi necessário.

A ponte HTTP não substitui ensaio visual de todas as telas. Fiscal, TEF/Pix, Windows, hardware, implantação comercial, relay público e demais módulos não são declarados concluídos por essa unidade. O piloto em dinheiro com reinício/restauração e empresas autenticadas tem as provas da revisão51; esta revisão fecha as cinco omissões de validação. Sem percentual global sem matriz medida, sem merge/main, sem alteração dos oito fontes pausados. Esta é a conclusão da unidade de abertura e aceite ampliado, não uma declaração de conclusão de todo o ERP.


## Revisão53 — devoluções65 instaladas, CI integral e medição da abertura de abas

25/10/2026, noite local. Backend base3ab18e1, entrega publicada em `a5bbcdf3420ca4fa6c471fcdd52794f86f84c0dc`,38arquivos na branch `feat/backend-b02a-compat`. Árvore backend limpa depois da publicação. Sem main/merge. Guia exato: `docs/architecture/BACKEND_DEVOLUCOES_CLIENTE65_20261025.md` na árvore backend.

### Aplicação conciliada e funcionamento

Pacote correto anterior: `TitanSystem_Backend_Devolucoes_Base7b37966_20261024_INTEGRO.zip`, SHA256 `7219facef9247701182b849e0300bb42e5ee6bcb5b1bf80122582b670c447330`. Seus31caminhos declarados foram comparados contra o ancestral7b3796662c9e455c446c3756b19807c486ff694c e a árvore atual. Dois conflitos conciliados: lista histórica do backup e inventário de rotas. Aplicador/instalador/scripts de commit antigos NÃO executados; snapshot inteiro, dependências e binários antigos NÃO copiados. Migrações1–64 continuam byte a byte iguais; função usa nova65, probes66. Teste64→65 preserva vínculo revogado, geração de recuperação e checksums.

Quatro operações de devolução: histórico/cotação/criação em `/local/v1/sales/:id/returns` e recuperação em `/local/v1/sale-return-operations/:id`. Reembolso parcial pelos preços originais, arredondamento cumulativo em inteiros, destino de estoque explícito, recibo original, concorrência/idempotência/rollback, livro de caixa/outbox na mesma transação. Venda/pagamento originais imutáveis; cancelamento integral não duplica restituição. Fechamento/CSV separam venda bruta, cancelamento, devolução, reembolso e líquido. Apenas gaveta original ainda aberta e mesmo aparelho, com dinheiro suficiente. Não equivale a devolução fiscal, cartão/Pix, crédito ao cliente ou reembolso em turno posterior.

O cliente antigo `frontend-web/src/core/local/official101.mjs` ainda exige líquido+cancelado=bruto e pagamento registrado=líquido. Relatório com devolução positiva precisa da integração explícita dos novos campos; essa tela NÃO foi atualizada nem homologada por esta unidade de backend. Serviços preservados não foram trocados para65, portanto instalação dos binários não introduziu silenciosamente esse contrato em suas telas existentes.

### Provas automatizadas locais e no GitHub

CI no SHA exato: https://github.com/danilomalta/zzeussss/actions/runs/40239470920 — success. Suíte completa local E remota: **1273testes principais/2714incluindo subtestes aprovados, zero falhas, três skips estruturais** (TestMigrationCrashChild, TestPilotCashProcess e a fixture visual explícita TestPilotCashScreenFlow). As cinco integrações obrigatórias passaram: sessões/catalogo com PostgreSQL17 real e pontes frontend de caixa/recebimento/manutenção contra HTTP Go real. Cluster local descartável parado depois; nenhum banco comercial utilizado.

Quatro pilotos completos passaram, inclusive o novo `TestPilotCashCustomerReturnRestartAndRestore`. Novo circuito: comércio148chamadas HTTP/seis processos + fornecedor independente8chamadas/três processos =156requisições/nove execuções reais. Estoque5000−2000+1000+1000=5000milésimos; caixa1000+3198−1025−1599=1574centavos; bruto3198/devolvido1599/líquido1599; fornecedor em aberto0. Tratativa da avaria permanece em andamento. Queda depois do reembolso, recibo/IDs recuperados, repetição explícita sem duplicação, fechamento e restauração com binário real `titan-backup`. Detector de corridas: etapa remota336,243s, sem corrida detectada. Não é prova de interface de devolução nem de Windows.

Primeira suíte integral revelou corrida na fixture subprocessada: arquivo de prontidão já legível porém vazio. Corrigidos publicação por rename atômico e reconhecimento de URL completa de loopback; log inicial preservado, regressão repetida cinco vezes e suíte integral reexecutada. Nenhuma falha foi convertida em skip. Verificador OpenAPI agora resolve referências/herança e identificadores opcionais corretamente;10regressões novas. Corrigidas ligações documentais de84operações existentes +4novas e parâmetros; mapa completo de527rotas validado no CI. Isso não representa84funções comerciais novas.

Artefato remoto `backend-pilot-a5bbcdf3420ca4fa6c471fcdd52794f86f84c0dc`, retenção14dias. Seis binários +três relatórios baixados, CRC conferido e gate obrigatório reexecutado sobre JSON remoto: mesmos1273/2714/zero falhas. Laboratório `work/backend-customer-returns-20261025/` preserva preimagens, comparação, SQL históricos, relatórios completos, falha inicial e evidência da instalação. Nenhum ZIP novo ou cópia de dependências gerado nesta unidade.

### Instalado no Ubuntu, sem trocar serviços

Seis executáveis Linux amd64 PRODUZIDOS PELO CI instalados em `~/.local/opt/titansystem/backend/a5bbcdf3420ca4fa6c471fcdd52794f86f84c0dc/`: titan-local, titan-backup, titan-access, titan-peer, titan-receive, titan-sync. Todos conferidos por hash; ponteiro `backend/current` trocado atomicamente somente depois da conferência. Comandos `~/.local/bin/titan-*` usam esse ponteiro. Versões anteriores mantidas. Smoke dos binários instalados: três execuções de API,12assertivas HTTP, schema65, identidade preservada depois de reinício/restauração, sessão antiga recusada, mutação sem licença recusada; PASS. `installed.json` e `evidence/installed-smoke.json` no laboratório.

Serviços8182/8282 preservados por instrução explícita anterior: nenhum restart, migração de banco ou leitura de chave/banco comercial. Instalar backend novo não significa que essas duas APIs o estejam executando. Desktop compilado4e7a50ff66a205baadc9a6693b37e061a0e89596583d8df8c9d950995f358dc7 e conexão local8182 preservados. Não declarar toda a instalação comercial atualizada.

### Abertura: separar inicialização de aplicativo e abertura de módulo

O proprietário esclareceu: os três minutos ocorrem **ao abrir uma aba**. Não atribuir essa demora ao login ou declarar resolvida só por otimizar o lançador.

Correção separada publicada em `d246211a8efdc7b8f9380b9ff221615d19638a20`, cinco arquivos desktop/workflow: npm start agora abre estágio já compilado, conferindo inventário e hashes completos; não percorre fontes nem chama build por padrão. `npm start -- --build` é explícito. Ausência/corrupção falha com instrução, sem recompilar/instalar escondido, sem afrouxar sandbox. Atalhos instalados já abrem compilado diretamente.43testes desktop aprovados localmente e no CI: https://github.com/danilomalta/zzeussss/actions/runs/40253211404 . Nenhum fonte funcional do frontend foi alterado; oito arquivos pausados continuam preservados.

Medições nativas no mesmo Ubuntu, perfis novos e conta/API Go descartável, sem mock, sem credenciais comerciais ou Vite: inicialização até controles de login365–415ms; abertura/autenticação até menu515–615ms (tempo de processo inteiro). Abertura de módulos pelo link real do menu, incluindo espera de300ms sem nova resposta HTTP: planilha796,49ms, catálogo409,20ms, pedidos404,59ms, contas a pagar391,87ms, PDV407,12ms. Planilha com ações habilitadas e dois canvas; somente pacote do idioma selecionado carregado. Demais módulos com consultas reais concluídas. PDV mostrou caixa fechado; consulta de funcionários retornou403 no perfil sintético e nenhuma permissão foi ampliada. Essas medidas não comprovam fluidez de grandes bancos/arquivos nem o fechamento de todas as funções.

Uma rodada do PDV atingiu429 após logins repetidos; mantido o rate limit, nova fixture usada no ensaio final. Não foi alteração da API para favorecer o teste. Aviso MESA não impediu renderização. Login público Vite3002, perfil novo sem autenticação:389,23ms/55recursos. Não foi medido módulo autenticado nessa instância existente, nem lida sua sessão. Portanto, **os180segundos relatados ainda não foram reproduzidos**. Perguntado qual aba e se acesso ocorre pelo ícone ou3002; não inventar causa enquanto falta essa localização. Aplicadores antigos1.2/1.3 extraem runtime a cada execução, mas não foi demonstrado que esse seja o caminho usado nem a causa da demora.

Resumo bruto sem credenciais em `docs/evidence/desktop-performance-20261025/module-opening-summary.json`; laboratório `work/desktop-startup-performance-20261025/` contém medições/harnesses. Não guardar contas sintéticas, review.json, senhas ou perfis no Git. Não limpar cache/perfil do proprietário como tentativa indiscriminada.

### Limites e continuidade

Unidade65 aplicada, testada, publicada e instalada. Backend inteiro não declarado concluído. Novas APIs ainda precisam das telas correspondentes; relay operacional/Internet comercial, fiscal/TEF/Pix, turnos alternativos de restituição, Windows e homologação de hardware não são cobertos por estes pilotos. Desempenho lento informado permanece em investigação até reproduzir o caminho real. Não há matriz medida para percentual global. Próximo passo de desempenho: localizar aba/acesso e medir sua sequência de recursos/consultas sem dados comerciais nos logs; preservar os serviços existentes e as filas incertas. Próxima mudança funcional de frontend continua sujeita à pausa vigente, sem incluir os oito fontes locais de carrinhos.


## Revisão54 — esclarecimento: clique para abrir o software

25/10/2026. O proprietário esclareceu novamente: “A do software, quando aperto para entrar no software ele demora, para responder”. A investigação voltou ao lançador, sem assumir que a demora fosse de um módulo. Os tempos de módulos da revisão53 continuam válidos somente para o ensaio descrito.

Executado o caminho real do menu Ubuntu com `gio launch ~/.local/share/applications/titansystem-erp-atual.desktop`, usando o perfil existente e sem preencher login, ler credenciais ou bancos. Retorno do lançador9,04ms; janela real publicada e mapeada em631,63ms (0,63s), estado Normal. WM_CLASS efetivo titansystem-desktop corresponde ao StartupWMClass do atalho. Não foi necessário desativar GPU/sandbox, apagar cache ou matar processo. A janela ficou aberta para o proprietário.

Conferidos os três atalhos do menu, Downloads e área de trabalho: todos apontam ao lançador estável `~/.local/bin/titansystem-desktop`, que abre o runtime/interface já instalados. Nenhum outro atalho TitanSystem em /usr/share/applications ou /usr/local/share/applications; nenhuma entrada Titan/zzeus em favorite-apps do GNOME. Scripts antigos ABRIR_TITAN_UBUNTU.sh dos pacotes1.2/1.3 ainda verificam e extraem a distribuição inteira para pasta temporária em cada execução. Foram apenas lidos, preservados; não há prova de que o proprietário os esteja usando nem de que expliquem180s. Para o caminho medido, usar TitanSystem ERP pelo menu ou o TitanSystem.desktop da área de trabalho/Downloads.

Prova numérica sem credenciais em `docs/evidence/desktop-performance-20261025/real-shortcut-opening.json`. Nenhuma nova alteração de fonte/instalação nesta revisão; não declarar que foi corrigida uma causa ainda não reproduzida. Serviço configurado8182 e serviço8282 preservados. Ainda não foram medidos o clique de login com a conta do proprietário nem o comportamento específico de um eventual atalho diferente.


## Revisão55 — falha REAL no menu Ubuntu: sandbox/AppArmor e entrada antiga

25/10/2026,23h local. O proprietário demonstrou que o ícone não abria. A conclusão anterior baseada no lançamento do Codex era insuficiente: abrir pelo terminal deste agente não comprovava o contexto do GNOME. Não repetir essa equivalência. O journal real da sessão mostrou duas ativações do ícone antigo abortando com FATAL setuid_sandbox_host.cc:166 e apontando chrome-sandbox na distribuição antiga em Documents/Codex/work/titan-desktop-package. A entrada ainda era utilizada pelo menu apesar do arquivo atualizado.

Reprodução independente fora do terminal do agente: `systemd-run --user` executou o lançador instalado na sessão Ubuntu; mesmo erro fatal, agora apontando chrome-sandbox no runtime estável. Kernel mantém unprivileged_userns_clone=1 e apparmor_restrict_unprivileged_userns=1. O runtime copiado tem helper755 do usuário; a sessão real não tinha autorização AppArmor para criar a sandbox por user namespace. O ambiente do agente não expunha essa mesma restrição. Não era demonstrado que houvesse processamento de dez minutos: a ativação observada abortava antes de abrir a janela.

### Correção aplicada e autorizada no Ubuntu

Preparado perfil restrito ao caminho exato do Electron instalado, seguindo o formato dos perfis Chrome/VSCode já presentes no sistema: ABI4.0, flags=(unconfined), userns. Essa concessão permite o mecanismo de namespace do Chromium para o executável nomeado; não desativa a sandbox do Electron, não impõe confinamento AppArmor adicional nem autoriza root ao aplicativo. Parser de sintaxe passou antes da instalação. A ação administrativa foi executada por pkexec/autenticação do próprio Ubuntu, sem ler/receber senha. Criado e carregado `/etc/apparmor.d/titansystem-desktop-userns`,644root:root, SHA256f91aafc1a78777bcdea5eb4cc1510a971014b1964f8a33f0ec07967bc500f680. Não existia arquivo anterior; aplicador recusaria sobrescrevê-lo. Nenhum sysctl global alterado, nenhuma dependência instalada, nenhum helper setuid criado, nenhum --no-sandbox. Conteúdo e limites em DESKTOP_UBUNTU_SANDBOX_20261025.md.

Registro novo `~/.local/share/applications/com.titansystem.ERP.desktop`, Nome TitanSystem ERP, Exec lançador estável, DBusActivatable=false e StartupWMClass correspondente. Entrada antiga titansystem-erp-atual.desktop preservada em backup e ocultada com Hidden/NoDisplay para não duplicar o menu ou reutilizar seu caminho em cache. Atalhos Downloads/TitanSystem.desktop e Área de trabalho/TitanSystem.desktop atualizados; três backups .gnome-fix-*.bak e metadados em work/desktop-gnome-launch-20261025/registered.json. desktop-file-validate e update-desktop-database passaram. Sem alterar launcher, runtime, interface, perfil de conexão ou serviços8182/8282.

### Evidências no contexto corrigido

Serviço temporário de validação do próprio Ubuntu abriu janela mapeada em704,59ms; processo com perfil titansystem_desktop e janela Normal permaneceram ativos em30amostras até6,67s. Ativação GTK pelo novo ID também encontrou janela Normal em684,49ms. A checagem imediata250ms foi prematura e preservada como falha de ensaio, não apresentada como sucesso. A consulta inicial a /proc do ambiente do agente não encontrou os PIDs externos; a prova final foi executada por serviço do próprio Ubuntu.

Conferida instância **lançada pelo GNOME**, escopo app-gnome-com.titansystem.ERP-2396946.scope ativo/running, janela Normal. Verificação no namespace real da sessão: processo principal com perfil titansystem_desktop, três filhos com Seccomp=2/NoNewPrivs=1. Linhas de comando não identificaram tipos internos; não inferir renderer/GPU individual nem tratar cada filho como processo principal. O código instalado mantém app.enableSandbox, webPreferences.sandbox=true e launcher sem --no-sandbox. Prova numérica sem dados em docs/evidence/desktop-gnome-launch-20261025/ubuntu-launch-result.json. Não foi feito login nem lida credencial/banco comercial.

Uma tentativa anterior de automatizar o teclado do overview digitou titansystem nesta conversa; interrompida e comunicada ao proprietário. FocusSearch por DBus havia sido negado; não registrar essa tentativa como clique gráfico bem-sucedido. Validação final se baseia na ativação real/escopo GNOME, janela e contexto de segurança acima.

**Falha de abertura por sandbox corrigida no Ubuntu atual**, com aplicativo encontrado aberto no escopo do GNOME. Isso substitui o diagnóstico anterior que não reproduzia o problema; não é homologação de todas as telas nem da autenticação comercial. Trocar o caminho/versão do runtime exige atualizar o perfil específico por administração; não permitir qualquer executável do usuário com glob. Oito arquivos frontend pausados preservados. Serviços8182/8282 permanecem nos executáveis anteriores e nenhum banco migrado. Esta unidade altera configuração de desktop e documentação, sem novas APIs ou mudanças comerciais.


## Revisão56 — backend: transporte assinado durável instalado e CI aprovado

26/10/2026. Solicitação do proprietário: continuar e concluir o backend. Unidade funcional desta entrega: levar pedidos/decisões já autorizados entre duas instalações pelas APIs reais, substituindo o relay feito apenas pelo driver do piloto. Base limpa a5bbcdf;12arquivos publicados em `1d8b885f9a3d5b280d26968e6f472d9316740cc0`, branch feat/backend-b02a-compat. Nenhuma migração, nova rota ou mudança de regra comercial: schema65 e mapa527 preservados. Não declara todos os módulos do ERP concluídos.

### Arquivos e comportamento

Novo cmd/titan-trade(main.go/teste), pacote internal/traderelay(relay.go, lock_linux.go, lock_other.go, relay_test.go), regressões HTTP/CLI trade_relay_test.go e trade_relay_cli_test.go. Alterados somente piloto autenticado, workflow backend-pilot, README e guia docs/architecture/BACKEND_TRANSPORTE_ASSINADO_20261026.md. Sem frontend funcional, Electron, Go mod/dependências ou SQL.

CLI exige job explícito: empresas/lojas/aparelhos/usuários de origem/destino, fingerprints, mensagem, peer do destinatário e operation_id originais. Consulta a mensagem previamente exportada, confere direção, assinatura canônica/domínio e destino. Identidade técnica não comprova CNPJ. Sem criação/aprovação de pedido, decisão comercial automática ou acesso direto a banco/chave de estação. Serviços usam sessões próprias; token do comércio não autentica no fornecedor. URLs vêm do job autorizado, nunca do envelope. HTTPS/certificado obrigatório fora de loopback canônico; sem proxy herdado, redirecionamento ou skip-verify.

Arquivos job/token0600 e pasta de estado0700. Estado: hashes do job/envelope, chave pública fixada e status, sem token/documento/motivo/produto. Lock flock Linux entre invocações; gravação atômica+fsync de arquivo/pasta antes do único POST. send com tentativa existente não reenvia. recover usa apenas destino e chave pública previamente persistida, funciona com origem desligada e nunca faz POST; valida recibo original. retry é comando explícito, consulta primeiro, envia no máximo uma vez somente após404, com os mesmos IDs.401/troca de contexto não apaga tentativa nem reenvia. Confirmado anteriormente+404 posterior recusa nova mutação e exige investigar restauração. Não há scheduler/retry automático. Contexto é revalidado antes do envio, sem alegar atomicidade de revogação entre empresas independentes.

Antes de transmitir, as duas instalações precisam da revisão de confiança exigida após restauração. O piloto agora registra a ação explícita do dono do fornecedor revisando fingerprints/generation antes de enviar sua decisão guardada. Ler um recibo não remove esse bloqueio. Em plataformas sem lock implementado, a ferramenta recusa executar; Linux é a plataforma comprovada.

### Verificações concluídas

Suíte integral local E remota: **1286testes principais/2743incluindo subtestes aprovados, zero falhas, três skips estruturais** (TestMigrationCrashChild, TestPilotCashProcess e fixture visual explícita TestPilotCashScreenFlow). As cinco integrações obrigatórias passaram: PostgreSQL17 real para sessão/catálogo e pontes HTTP de caixa/recebimento/manutenção. Cluster local novo na43529, parado ao final; nenhum5432/banco comercial. Gate independente reexecutado sobre JSON baixado do CI: mesmas contagens, PASS. Go vet e verificador das527rotas passaram.

Treze testes principais novos, incluindo JSON/arquivos/lock, CLI/help, duas direções reais, perda de resposta após efeito, falha antes de chegar, recuperação com origem desligada, retry explícito depois de404, contexto/fingerprint errados, bloqueio de restauração, sessão inválida sem reenvio/limpeza, redirecionamento sem entrega do token e certificado TLS não confiável. Proxy de falha só interrompe rede diante de backend REAL, não simula resultado comercial. Unidades traderelay/cmd passaram também com-race local; pilotos/HTTP/importações testados com detector de corridas no CI.

Primeira fixture de restauração omitiu updated_at obrigatório; corrigida somente a fixture. Um comando passou flag específica de localapi para outro pacote e foi recusado; log mantido e repetição correta por variável de ambiente passou. Suíte final inteira rodada depois, sem converter falhas em skips. Relatórios/preimagens em work/backend-trade-relay-20261025.

CI aprovado no commit exato: https://github.com/danilomalta/zzeussss/actions/runs/40262463058 . Quatro pilotos completos com CLI de backup e transporte reais/race passaram, etapa356,585s, sem corrida detectada. Circuitos:101requisições/4processos (recebimento integral);119/5(parcial);147+22/5+3(fornecedor autenticado e parcial);164+22/6+3(parcial+devolução). Neste último, total186chamadas/nove execuções: estoque5000−2000+1000+1000=5000milésimos; caixa1000+3198−1025−1599=1574centavos; fornecedor em aberto0, tratativa de avaria em andamento. IDs/assinaturas/recibos iguais após reinício e restauração. Não é aceite em empresa comercial nem UI nova.

### Instalação e validação no destino

Artefato CI backend-pilot-1d8b885f9a3d5b280d26968e6f472d9316740cc0, retenção14dias, contém sete executáveis e três relatórios; arquivos conferidos, sem entradas adicionais. Instalados os sete Linux amd64 no diretório ~/.local/opt/titansystem/backend/1d8b885f9a3d5b280d26968e6f472d9316740cc0: titan-local, titan-backup, titan-access, titan-peer, titan-receive, titan-sync e novo titan-trade. Hashes iguais aos arquivos do runner; current trocado após conferência. Seis comandos existentes continuam pelo ponteiro, novo ~/.local/bin/titan-trade criado. Versãoa5bbcdf e anteriores preservadas. Nenhum ZIP novo, node_modules/cache ou runtime Electron repetido.

No destino: help executado sem arquivos/segredos; smoke dos binários instalados passou com três APIs/12assertivas HTTP, schema65, reinício, CLIbackup/verify/inspect/restore, identidade igual e mutação sem licença recusada. Piloto completo e teste específico da CLI reexecutados com os comandos INSTALADOS: PASS em3,030s,186chamadas/nove processos, transporte/recuperação e backup reais. Evidências/SHAs em docs/evidence/backend-trade-relay-20261026/installed.json e laboratório.

Serviços8182/8282 preservados nos executáveis anteriores, sem restart/migração, conforme escolha anterior do proprietário. Instalação dos novos comandos não atualiza esses processos. Nenhum banco/chave/credencial comercial lido; somente arquivos e contas descartáveis criados para ensaios. Aplicativo desktop e perfil AppArmor da revisão55 preservados. Oito fontes frontend pausados conferidos8/8intactos, sem commit desses arquivos. Registro único é a única alteração na árvore frontend além da evidência documental.

### Limites e próxima continuidade

Transporte explícito entre APIs agora é executável real, não apenas código de teste. Ainda não é relay público, worker de entrega automática, descoberta de empresas, implantação HTTPS comercial ou interface remota homologada. Jobs/estado de transporte ficam em pasta privada separada do SQLite: preservar e incluir na política de backup do operador; a CLI não inventa IDs novos se esses arquivos se perdem.

Permanecem: operação de comunicação integrada ao desktop/serviço autorizado, integrações fiscais/Pix/TEF com provedores reais, Windows e hardware, demais módulos comerciais e ações/telas ainda não homologados. O frontend de relatório de caixa ainda precisa consumir os campos de devolução65 antes de operar esse fluxo visual; pausa funcional mantida. Próxima unidade deve revisar transporte operacional e contratos de renegociação/cancelamento remoto, sem tratar aprovação local como cancelamento comunicado ao fornecedor. Sem percentual global sem matriz medida. Unidade de transporte concluída, publicada, instalada e validada; backend inteiro não declarado concluído por essa prova.

## Frontend consolidado65 (frontend-consolidado-schema65-20261025-1)

Entrega solicitada pelo proprietario para ampliar o frontend e inventariar o trabalho restante. Base de fontesd3d727d; backend de referenciaa5bbcdf/schema65. A autorizacao desta entrega nao substitui os oito arquivos locais pausados de carrinhos: o aplicador escreve somente a lista explicita e preserva outras alteracoes, validando uma copia dos fontes atuais.

Nova area `/local/customer-returns` com consulta, cotacao, revisao, reembolso em dinheiro, fila por contexto, consulta/reenvio do mesmoID, recibo/historico/CSV/impressao. Relatorio101 conciliado com devolucoes65 e metricas antigas explicitamente indisponiveis. Nao altera autenticacao/sessoes/backend/migracoes/servicos/atalhos.

Inventario em `docs/frontend/PENDENCIAS_FRONTEND65.md`/json/html e guia `ENTREGA_CONSOLIDADA65.md`.96itens em32frentes incluem aprofundamento, aceite e dependencias externas; nao sao96funcionalidades necessariamente ausentes. Backend/ERP inteiro nao declarado concluido. Relatorios de testes/build/ponteHTTP e limites acompanham o pacote. Aceite visual desta entrega pendente: download do Chromium falhou. A aplicacao no PC executa novamente testes completos/TypeScript/build em candidata isolada antes de gravar; nao usa banco comercial nem promove desktop automaticamente.

## Revisão57 — fila autorizada instalada; backup cifrado em validação remota

26/10/2026. Continuidade do backend na branch `feat/backend-b02a-compat`, sem mudança funcional do frontend nem reinício dos serviços8182/8282.

### Fila concluída, publicada e instalada

Commit `246691c6d68ff5d64920224c411a11f5d04f7ca5`: `titan-trade queue` captura o job/envelope previamente autorizado, vincula as sessões originais e processa lotes limitados. Logout, substituição de sessão/contexto e divergência de hash pausam sem apagar as provas. Estado incerto força consulta do recibo original;404 exige retry explícito, nunca nova operação automática. Teste mata o worker real depois do commit remoto e comprova recuperação por outro processo com zero POST e um único efeito.

CI aprovado no SHA exato: https://github.com/danilomalta/zzeussss/actions/runs/40267611992 . Suíte completa local e remota: **1.305 testes principais /2.769 incluindo subtestes, zero falhas, três skips estruturais permitidos**. As cinco integrações obrigatórias passaram com PostgreSQL real e pontes frontend contra HTTP Go. Pilotos com detector de corrida passaram em529,698s. Circuito mais completo com fila compilada: comércio171HTTP/seis processos + fornecedor29HTTP/três processos =200HTTP/nove execuções; estoque5000milésimos, caixa1574centavos, obrigação em aberto0, tratativa em andamento. Não é evidência de UI completa ou Internet comercial.

Sete executáveis produzidos pelo CI baixados, gate remoto reexecutado e hashes conferidos; instalados em `~/.local/opt/titansystem/backend/246691c6d68ff5d64920224c411a11f5d04f7ca5/`, ponteiro `backend/current` atualizado atomicamente, versões anteriores preservadas. Smoke instalado: três execuções de API,12assertivas HTTP, schema65 e backup/restauração pelo executável; PASS. Piloto completo usando os comandos instalados + ensaio da fila: PASS em2,963s,200HTTP no circuito completo, recuperação sem novo POST. Nenhum serviço existente foi reiniciado, nenhum banco comercial ou chave real foi aberto. Laboratório `work/backend-trade-queue-20261026/` guarda artefatos, logs e hashes.

### Complemento de recuperação, sem antecipar instalação

Commit `64eff3d27498329e87a0336f47b2d34adcc9409f`: backup AES-256-GCM da fila externa ao SQLite, verificação autenticada e restauração somente em pasta nova, com publicação sem substituição. Não inclui tokens, chaves privadas ou banco. Itens restaurados ficam pausados e permanentemente limitados à consulta de recibos originais, inclusive depois de resume; cópia antiga/404 não autoriza novo POST.

Suíte completa local: **1.311 principais /2.779 incluindo subtestes, zero falhas, cinco integrações obrigatórias aprovadas e os mesmos três skips estruturais**. `go vet`, contratos das527rotas, testes unitários com detector de corrida e quatro cenários com executável real/dados descartáveis aprovados. CI https://github.com/danilomalta/zzeussss/actions/runs/40277261085 ainda em execução nesta revisão; NÃO anunciar este complemento como instalado antes do resultado e das verificações pós-instalação. Guia `docs/architecture/BACKEND_BACKUP_FILA_20261026.md` na branch de backend. Laboratório `work/backend-trade-queue-backup-20261026/`.

### Preservação e limites

Os oito fontes pausados de carrinhos continuam8/8 idênticos à base registrada. Outras mudanças de frontend, inclusive o texto local Frontend consolidado65, pertencem a outra entrega; foram preservadas e não incluídas no commit desta revisão. A publicação deste registro contém somente a atualização desta frente e evidência resumida sem credenciais. Nenhuma porcentagem global ou conclusão do backend inteiro deriva desses testes. Permanecem integração operacional no desktop, Internet/TLS comercial, fiscal/Pix/TEF, Windows/hardware e demais fluxos não homologados; não recriar nem apagar entregas existentes para avançar.

## Revisão58 — backup cifrado da fila publicado, instalado e validado

26/10/2026. Conclui o complemento que estava em validação na revisão57. Backend `64eff3d27498329e87a0336f47b2d34adcc9409f`, branch `feat/backend-b02a-compat`; CI **success** no SHA exato: https://github.com/danilomalta/zzeussss/actions/runs/40277261085 . Fonte e testes publicados, sem alteração das527rotas/schema65 ou migração de banco existente.

### Recuperação efetivamente entregue

`titan-trade queue backup`, `verify-backup` e `restore`: snapshot consistente da fila e suas provas externas ao SQLite, AES-256-GCM com nonce aleatório e domínio próprio, arquivos0600 e chave32bytes explicitamente fornecida. Não abre banco, tokens ou chaves privadas da estação. Captura protegida pelos locks da fila/tentativas; JSON estrito e vínculo de cada prova ao job/envelope/fingerprint originais. Arquivo existente nunca é substituído. Restauração publica somente diretório novo0700 com operação Linux sem substituição; não remove destinos ou preparações antigas.

Todos os itens restaurados ficam pausados e `recovery_only:true`. Mesmo reautorização explícita não habilita POST. Consulta o recibo original;404 mantém pendência e retry é recusado. Cópia anterior a uma entrega não apaga a operação remota nem autoriza duplicação. Mensagem realmente não enviada também pode permanecer bloqueada: requer conciliação humana, não criação automática de outroID. Preservar também o backup oficial SQLite; não existe snapshot transacional conjunto das duas empresas.

### Provas completas e instalação

Suíte completa LOCAL E REMOTA: **1.311 testes principais /2.779 incluindo subtestes aprovados, zero falhas, cinco integrações obrigatórias reais aprovadas e três skips estruturais permitidos**. PostgreSQL17 e SQLite descartáveis; pontes frontend de caixa/recebimento/manutenção contra APIs Go reais. `go vet`, contratos das527rotas e detector de corrida dos componentes aprovados. Rodada remota de pilotos com `-race`: **566,887s**, PASS. Esse tempo é da suíte de testes, não da abertura do software.

Quatro cenários do executável real com duas APIs independentes: mensagem não enviada, entrega posterior ao backup, resposta perdida após commit e entrega previamente confirmada. Restauração mantém IDs/hash, não processa automaticamente; após reautorização só recupera recibo ou pausa. Provas persistidas permitem recuperação com origem desligada. Zero POST na recuperação e um único efeito quando havia entrega. Proxy somente injeta falha de transporte sobre commit real, sem sucesso comercial simulado.

Sete binários Linux amd64 PRODUZIDOS PELO CI instalados em `~/.local/opt/titansystem/backend/64eff3d27498329e87a0336f47b2d34adcc9409f/`: titan-local, titan-backup, titan-access, titan-peer, titan-receive, titan-sync e titan-trade. Hashes de todos conferidos; arquivos/diretórios sincronizados antes da troca atômica de `backend/current`. Releases246691c/1d8b885 e anteriores preservadas.

Comandos EFETIVAMENTE INSTALADOS revalidados: smoke com três execuções de API/12assertivas HTTP, reinício, restauração do banco, IDs preservados e escritas sem licença recusadas; PASS. Piloto completo com venda em dinheiro→caixa→estoque→reposição→pedido/fornecedor autenticado→recebimento parcial→financeiro→devolução→fechamento/reinício/restauração +quatro cenários da fila +envio/consulta da CLI: **PASS em5,811s**. O circuito de venda desse ensaio fez200HTTP/nove execuções de API, estoque5000milésimos, caixa1574centavos e obrigação em aberto0; não confundir com total de chamadas dos demais cenários.

Laboratório `work/backend-trade-queue-backup-20261026/`: preimagens/hashes, suíte integral, logs focados, artefatos do CI, `installed.json`, smoke e resumo final. Evidência resumida sem credenciais em `docs/evidence/backend-queue-backup-20261026/acceptance.json`. Fontes e contrato operacional em `docs/architecture/BACKEND_BACKUP_FILA_20261026.md` da branch de backend. Não gerado outro ZIP, não incluídas dependências ou bancos.

### Serviços e trabalho paralelo preservados

Portas8182/8282 mantêm os PIDs1362657/3855147 e os executáveis originais das instalações de teste; healthHTTP200 após a instalação. Nenhum restart, mudança de configuração, leitura de chave/banco comercial ou promoção automática do desktop. Disponibilizar os novos binários não significa atualizar silenciosamente esses serviços. Os oito arquivos pausados de carrinhos permanecem8/8 com os hashes originais. Alterações da outra frente de frontend, inclusive o registro local de pedidos66, continuam preservadas e fora deste commit.

### Limites reais e próxima unidade

Fila/transporte e recuperação local desta entrega estão concluídos, publicados e instalados. Isso não conclui todo o backend/ERP. Permanecem operação Internet/TLS comercial e coordenação do transporte com o desktop autorizado, contratos de renegociação/cancelamento comunicado entre empresas, provedores reais de fiscal/Pix/TEF, Windows/hardware e demais fluxos sem aceite integral. O binário de transporte compilou para Windows em conferência adicional, mas a operação continua recusada sem lock/publicação equivalentes: não é entrega operacional de Windows. As novas telas de outra frente precisam de seu próprio aceite e não foram homologadas por esta revisão. Nenhuma porcentagem global inferida pelo número de testes, rotas ou linhas.

## Revisão59 — recuperação de recibo com sessão nova e origem desligada

26/10/2026. Backend `6ff0d2f255477aee324c4338c80106bf6734cce2`, branch `feat/backend-b02a-compat`. Unidade concluída, publicada, instalada e revalidada. CI **success** no SHA exato: https://github.com/danilomalta/zzeussss/actions/runs/40280948422 . Sem mudança das527rotas/schema65, de regras comerciais ou de fontes funcionais do frontend.

### Lacuna corrigida e proteção preservada

A consulta de recibo do motor já dispensava a API da origem, mas trocar a sessão do destino exigia ambas as partes online e o worker verificava também o arquivo de credencial da origem. `titan-trade queue recover-session --queue PASTA --id ITEM --target-token ARQUIVO_PRIVADO` resolve isso SOMENTE para leitura. Exige prova original persistida e confere job/envelope/fingerprint/chave pública, além da mesma empresa/loja/aparelho/usuário e fingerprint no destinatário. Não cria uma primeira tentativa, não aceita outra operação e não lê a credencial da origem.

O item mostra `receipt_only_session:true`; o worker só consulta o recibo original e não permite POST/retry.404 pausa; prova desaparecida ou sessão substituída também pausa sem recriar envio ou apagar os IDs. `queue verify --id ITEM` faz a consulta isolada; `run/watch` podem processar OUTROS itens previamente autorizados, cuja autorização não muda. Retomada completa continua exigindo as duas sessões/mesmo envelope e retry separado para operação incerta. A barreira permanente `recovery_only:true` de fila restaurada não é removida por nenhum desses comandos.

### Provas e executáveis instalados

Suíte completa LOCAL E REMOTA: **1.316 testes principais /2.787 incluindo subtestes, zero falhas, cinco integrações obrigatórias reais e três skips permitidos pelo gate (dois helpers de subprocesso e o ensaio visual explícito não executado nesta rodada)**. PostgreSQL17/SQLite descartáveis e pontes frontend contra HTTP Go real. `go vet`, contratos e detector de corrida dos componentes aprovados. Pilotos remotos com `-race`: **640,229s**, PASS. Nenhum tempo de suíte é apresentado como tempo de abertura do software.

Testes reais cobrem entrega incerta, confirmada e fila restaurada com novo login no destino, origem desligada ANTES da reautorização e arquivo de sessão sintético da origem ausente. Recibo original confirmado com zero HTTP na origem e zero POST. Cobertos ainda prova ausente/inválida, sessão de outra empresa, substituição posterior da sessão, perda da prova e404. Proxy somente causa perda de resposta de transporte; os efeitos são transações reais das APIs.

Sete executáveis Linux amd64 PRODUZIDOS PELO CI instalados em `~/.local/opt/titansystem/backend/6ff0d2f255477aee324c4338c80106bf6734cce2/`, hashes conferidos, sync e troca atômica de `backend/current`; versões64eff3d/246691c/1d8b885 e anteriores preservadas. Smoke do `titan-local` instalado: três execuções de API,12assertivas HTTP, reinício/restauração pelo `titan-backup`, IDs preservados, sessão antiga e mutações sem licença recusadas; PASS.

Piloto completo em dinheiro, fila e quatro cenários de backup cifrado usando CLIs de backup/transporte INSTALADAS e servidores Go descartáveis: PASS em **6,785s**; circuito de venda200HTTP/nove execuções de API, estoque5000milésimos, caixa1574centavos, obrigação em aberto0 e tratativa em andamento. Prova adicional com AMBOS os comandos instalados `recover-session` e `verify`: três cenários/origem desligada, PASS em **2,420s**, zero POST. Essa prova adicional usa um overlay externo SOMENTE do arquivo de teste para trocar a chamada de biblioteca por `exec` da CLI; fontes de produto e teste publicados permanecem intactos. O total1316/2787 refere-se à suíte original, sem somar artificialmente esse ensaio extra.

Laboratório `work/backend-trade-recovery-session-20261026/` preserva preimagens, hashes, relatórios local/CI, artefatos, overlay de teste, instalação e resultados reais. Resumo sem credenciais em `docs/evidence/backend-recovery-session-20261026/acceptance.json`. [Guia operacional na branch de backend](https://github.com/danilomalta/zzeussss/blob/6ff0d2f255477aee324c4338c80106bf6734cce2/docs/architecture/BACKEND_RECUPERACAO_SESSAO_DESTINO_20261026.md).

### Preservação e limites

Os serviços8182/8282 não foram reiniciados, migrados ou reconfigurados; permanecem nos executáveis anteriores por escolha expressa do proprietário. Nenhum banco/chave comercial usado. Os oito arquivos pausados de carrinhos seguem com seus hashes originais; demais mudanças de frontend/documentos de outra frente preservadas e fora deste commit. Não criado ZIP nem incluídas dependências ou bancos. Atualizados apenas o estado resumido do BACKEND no início deste registro e esta evidência; matrizes históricas não são recertificadas por esta revisão.

Transporte/fila, backup da fila e recuperação de sessão destas unidades estão entregues. Backend/ERP inteiro não declarado100%: permanecem integração operacional do transporte no desktop, Internet/TLS comercial, contratos de renegociação/cancelamento comunicado entre empresas, fiscal/Pix/TEF com provedores reais, Windows/hardware e demais fluxos ainda sem aceite integral. Nenhuma permissão ampliada ou sucesso comercial simulado para esconder essas dependências. Não usar uma restauração de FILA neste teste como prova nova de restauração do banco remoto; os pilotos de banco continuam sendo provas separadas.


## Revisão60 — devolução em dinheiro em turno posterior escolhido —26/10/2026

### Entrega e autoridade

Backend fonte `49136eab3e53e57d831ad7aae340ab3b6910629d` na branch `feat/backend-b02a-compat`. [CI correspondente aprovado](https://github.com/danilomalta/zzeussss/actions/runs/40286664804); migração66,529 rotas mapeadas,29 arquivos de fonte/teste/contrato/documentação desta unidade. Todos os65 scripts SQL anteriores preservados. Esta revisão atualiza somente o estado do backend e sua evidência; não recertifica matrizes históricas de outras áreas.

Duas rotas novas e explícitas: `POST /local/v1/sales/{id}/returns/cash-session/quote` e `POST /local/v1/sales/{id}/returns/cash-session`, com `refund_cash_session_id` obrigatório. A rota antiga conserva a obrigação de usar o turno original aberto, sem escolher outro silenciosamente.

O turno pagador precisa estar aberto, no mesmo aparelho/empresa/loja e vinculado ao próprio operador. Exige CancelSale e ManageCash e dinheiro suficiente; o turno original diferente precisa de fechamento oficial. Nem o dono debita implicitamente a gaveta de outro operador; o gerente titular pode autorizar a devolução de venda anterior de outro operador. Revogação após cotação impede a escrita.

Devolução parcial, dinheiro, reposição física opcional, proveniência imutável, livro e outbox são atômicos. ID de operação vincula o turno escolhido; reenvio idêntico recupera o mesmo resultado depois de fechar/restaurar, contexto ou destino diferente gera conflito. Não apaga tentativa incerta, não reenvia automaticamente e não simula estorno bancário/fiscal.

### Relatórios e compatibilidade

Venda e fechamento originais permanecem idênticos. `cash_session_id` identifica a gaveta pagadora, `original_cash_session_id` identifica a original quando diferente. `cash.prior_session_refund_cents` separa pagamentos por vendas anteriores, já incluídos em `cash.customer_refund_cents`; não somar outra vez. `returned_sales_cents` e `net_sales_cents` mantêm sua definição operacional de mesmo turno, sem se passar por DRE. CSV contábil autorizado inclui a métrica adicional somente se positiva; formato antigo preservado quando ausente.

Schema65 preenchido com venda, devolução e caixa fechado migra para66 preservando recibo, fechamento e checksums. O executável65 foi recusado contra banco66 descartável sem alterar o arquivo. Não há downgrade automático de banco.

### Verificações observadas

Suíte completa LOCAL e CI: **1.328 testes principais /2.821 incluindo subtestes, zero falhas, cinco integrações reais obrigatórias e três skips explicitamente permitidos pelo gate**. PostgreSQL17/SQLite descartáveis, pontes frontend sobre HTTP Go real, análise estática e contratos aprovados. O teste antigo de coexistência esperava65; sua expectativa foi atualizada para66 e a suíte completa repetida. A primeira falha não foi tratada como sucesso.

Detector de corridas LOCAL de devoluções e quatro pilotos com backup CLI: PASS em **304.209s**. Rodada remota de trocas entre empresas/devoluções/quatro pilotos com backup e transporteCLI: PASS em **504.180s**. Esses tempos medem testes, não abertura do aplicativo.

Testes verificam corpo estrito, ausência de seleção automática, caixa fechado/inexistente/sem saldo/de outro aparelho, vínculo e operador, revogação, idempotência e conflito, disputa de quantidade, devoluções em dois turnos, relatório/CSV, contexto imutável e caixa corrompido. Falhas ABORT/IGNORE em sete pontos não deixam dinheiro, estoque ou histórico parcialmente gravados.

Prova adicional com **`titan-local` e `titan-backup` INSTALADOS, produzidos pelo CI**: 10 assertivas HTTP reais, três processos nativos da API, reinício, backup cifrado e restauração em banco novo; recibo/IDs preservados e repetição sem duplicação; saldo do novo caixa401 centavos, PASS em **0.961s**. Usou overlay EXTERNO somente do helper de teste para executar o servidor nativo, sem alterar fontes publicados. Essa prova não infla o total da suíte original. Smoke separado dos executáveis instalados: três processos da API,12 assertivas HTTP, proteção de licença, restart e backup CLI, schema66, PASS.

Dois circuitos completos adicionais usam APIs NATIVAS e CLIs de backup/transporte INSTALADAS: aceite autenticado entre empresas e circuito com devolução, ambos com recebimento parcial e divergência, persistência e restauração. O circuito com devolução registrou **200 assertivas HTTP em 9 processos nativos de API** (comércio+fornecedor), caixa1574 centavos e obrigação0. Logs integrais sanitizados em `native-cash-circuit.txt` e `native-refund.txt` na mesma pasta da evidência. O overlay externo muda somente o helper de testes; não é uma modificação da API nem um novo mock. Não mede interação visual da tela ou entregaWindows.

### Instalação, preservação e pendências

Sete executáveis Linux amd64 instalados em `/home/danillo-malta/.local/opt/titansystem/backend/49136eab3e53e57d831ad7aae340ab3b6910629d`; hashes dos artefatos conferidos, diretório sincronizado e ponteiro `backend/current` trocado atomicamente; versão6ff0d2f e demais versões preservadas. **Os serviços8182/8282 mantêm seus PIDs/executáveis anteriores e health200**, sem reinício, seleção de banco ou migração. Instalar não equivale a atualizar esses serviços. Oito fontes pausados do frontend com hashes8/8 originais e todas as mudanças locais da outra frente preservadas e fora deste commit.

Evidência sanitizada: `docs/evidence/backend-refund-next-shift66-20261026/acceptance.json`. [Guia e contratos da unidade](https://github.com/danilomalta/zzeussss/blob/49136eab3e53e57d831ad7aae340ab3b6910629d/docs/architecture/BACKEND_DEVOLUCOES_TURNO66_20261026.md). Laboratório `work/backend-refund-next-shift66-20261026/` mantém preimagens, hashes, logs, overlay e artefatos. Nenhum ZIP, dependência, banco ou chave comercial incluídos.

Unidade backend entregue; nova ação ainda precisa ser ligada ao frontend, incluindo leitura dos campos opcionais. Não entrega devolução entre aparelhos/lojas, estornos eletrônicos/fiscais, transporte integrado ao desktop, Internet/TLS comercial, homologação Windows/hardware ou conclusão de todos os módulos do ERP. Backend inteiro não declarado100%; fonte, instalação, serviço ativo e integração de tela são estados distintos.

## Frontend pedidos66 (frontend-pedidos66-20261026-1)

26/10/2026: acrescenta /local/company-trade e menu por view_orders. Interface para 13 rotas existentes: identidade pública/fingerprint, vínculos do dono e revogação, histórico paginado, exportação/importação de envelopes, decisão do fornecedor e aplicação ao pedido, revisão explícita da geração pós-restauração, recibos e fila por contexto. Sem envio automático, assinatura no frontend, backend/migrações/auth/sessões ou mudanças nos oito arquivos de carrinhos. Patch baseado nos pós-estados do frontend65 aplicado no commit019a94f.

Observado: 23 testes novos, 2465 totais, TypeScript e build; ponte Node→dois servidores Go/Fiber/TCP/SQLite temporários com as 13 rotas e sete ações, também com -race. A recuperação pós-restauração foi exercitada com gate semeado no fixture, não com restauração de arquivo. Sem aceite visual/nativo, Windows ou operação comercial. Inventário atualizado em docs/frontend/PENDENCIAS_FRONTEND66.*; guia em ENTREGA_PEDIDOS66.md. Não conclui todo o ERP.

## Frontend producao e desktop67 (frontend-producao-desktop67-20261026-1)

26/10/2026: mantém os módulos de produção/custos existentes e acrescenta ponte de aceite dos clientes reais contra Go/Fiber/SQLite descartável: versões imutáveis, capacidade compartilhada, estimativa/cenário, ordem, aprovação, reserva/consumo, etapas, resultado, perda, lote/qualidade e recuperação após perda da resposta de resultado. Confere resultado único e saldos finais; não comprova cliques, reinício/restore do servidor ou operação comercial.

Desktop ganha diagnóstico de instância existente, carregamento, renderer e timeout; mostra janela quando o carregamento termina e oferece modo software explícito antes do ready. Sandbox/perfis/dados permanecem preservados. O aviso MESA do proprietário motivou esse modo, mas não houve homologação gráfica no PC. Não altera backend, banco, autenticação, sessões, serviços ou carrinhos. Evidências no pacote; pendências em PENDENCIAS_FRONTEND67.*. Não conclui todo o ERP.


## Revisão61 — worker da fila vinculado ao contexto original — 26/10/2026

Backend fonte `a43af76e22b06d2f153e46f99f330b4ddc95dd0a`, branch `feat/backend-b02a-compat`; [CI aprovado](https://github.com/danilomalta/zzeussss/actions/runs/40352341813). Dez arquivos de fonte, testes, schema e documentação. Schema66 e529 rotas preservados, sem alteração dos66 scripts SQL históricos. A entrega anterior de devolução após o fechamento permanece incluída.

### Comportamento e autoridade

`queue status/run/watch --scope ARQUIVO_PRIVADO` limita a leitura e o processamento à origem exata: URL da API, empresa, loja, aparelho, operador e fingerprint da estação. O limite do lote conta somente itens correspondentes. Outros contextos não têm tokens lidos, tentativas incrementadas, registros de erro atribuídos ou agendamento alterado; o status não revela seus IDs ou contagens.

O arquivo não é credencial nem autorização. Job/envelope congelado, hashes de sessão, permissões, aprovação de pares e barreiras de restauração continuam exigidos. Arquivo absoluto regular0600, até4096bytes; JSON estrito rejeita campos desconhecidos, duplicados e nulos. `--scope` explicitamente vazio é erro, sem fallback silencioso. A execução administrativa antiga, sem a opção, conserva seu comportamento autorizado.

O worker fixa seu contexto original durante toda a execução. Mudança, remoção ou invalidação do arquivo encerra o worker em vez de vincular outro operador. Há guard antes de cada tentativa e de cada chamada HTTP, inclusive oPOST. Chamadas impedidas pelo guard não são contadas como HTTP enviado.

Uma requisição que já saiu pode terminar: seu resultado e prova são gravados antes de encerrar o lote. Resposta perdida após efeito remoto mantém a operação incerta e os IDs originais. Voltar ao contexto original recupera o recibo sem novoPOST; não apaga prova nem cria retry automático. O guard não promete cancelar retroativamente um pacote já enviado.

### Verificações reais

LOCAL e CI: **1339 testes principais /2845 incluindo subtestes, zero falhas, cinco integrações obrigatórias reais e três skips auxiliares permitidos**. PostgreSQL17/SQLite descartáveis, pontes frontend contra HTTP Go real, análise estática, contratos e schema JSON conferidos. Nenhuma migração nesta unidade.

Detector de corridas local dos novos cenários HTTP: PASS em **60.070s**; componentes de fila/CLI também aprovados com `-race`. Rodada remota completa de trocas, devoluções e quatro pilotos com CLIs reais: PASS em **852.493s**. Esses tempos não medem abertura do aplicativo.

Prova adicional com `titan-local` NATIVO e `titan-trade` INSTALADOS, produzidos pelo CI: PASS em **6.265s**. Dois contextos reais na mesma fila, seis dimensões de origem, mudança antes doPOST com zero efeito, depois doPOST confirmado e depois do efeito com perda da resposta. `status/run/watch --scope` compilados testados; owatch encerra ao substituir o arquivo mesmo quando a nova origem passaria a corresponder à mensagem pendente. Fila restaurada e origem desligada/arquivo de sessão ausente preservam barreira permanente e permitem apenas recuperar o recibo, com zeroPOST.

O ensaio nativo usa overlay EXTERNO somente na preparação dos testes para iniciar os servidores instalados e desligar a origem nativa no cenário correspondente; fontes publicados não mudam. Não infla o total original da suíte. Log sanitizado `docs/evidence/backend-trade-scope-20261026/native-scope.txt`.

Regressão integral adicional com executáveis instalados: venda → caixa → estoque → reposição → pedido/aceite autenticado → recebimento parcial com divergência → financeiro → devolução, reinício e restauração pelo CLI. **200 assertivas HTTP e 9 processos nativos de API**, caixa1574 centavos, obrigação em aberto0; PASS em **2.288s**. Log `native-cash.txt` na mesma pasta. Smoke separado dos binários instalados: três execuções da API,12 assertivas HTTP, schema66, restart/backup/restauração e proteção de licença, PASS.

### Instalação e preservação

Sete executáveis Linux amd64 do CI instalados em `/home/danillo-malta/.local/opt/titansystem/backend/a43af76e22b06d2f153e46f99f330b4ddc95dd0a`, hashes conferidos e ponteiro `backend/current` trocado atomicamente após sincronizar a nova versão. A versão49136ea e as anteriores ficam preservadas. Os serviços8182/8282 continuam nos PIDs/executáveis anteriores com health200, sem reinício ou seleção de banco; instalar executáveis não os atualiza.

Oito arquivos pausados do frontend seguem com os hashes8/8 originais. As alterações locais de outra frente em frontend, desktop e neste documento foram preservadas e não entram no commit desta revisão. Evidência sanitizada `docs/evidence/backend-trade-scope-20261026/acceptance.json`; laboratório `work/backend-trade-scope-20261026/` preserva preimagens, hashes, logs, overlay e artefatos. Não foram incluídos dependências,ZIP, banco ou credenciais comerciais.

[Guia e formato do contexto na branch backend](https://github.com/danilomalta/zzeussss/blob/a43af76e22b06d2f153e46f99f330b4ddc95dd0a/docs/architecture/BACKEND_FILA_CONTEXTO_20261026.md).

### Limites e continuidade

O filtro do worker está entregue. **Ainda não está ligado ao menu do Electron**. O host deve derivar o contexto de uma sessão e identidade de estação verificadas, invalidar o arquivo antigo e parar o worker ao sair ou trocar sessão/perfil; não ampliar autoridade com dados fornecidos pelo renderer. Esta unidade não altera os arquivos desktop em andamento e não usa sua existência como prova de integração visual.

Continuam sem aceite completo a ligação operacional no desktop, Internet/TLS comercial, fiscal/Pix/TEF com provedores reais, homologação Windows/hardware e outros fluxos sem prova integral. Backend inteiro não declarado100%. Nenhum serviço ou dado comercial foi exposto na Internet. As matrizes históricas de outras áreas não são recertificadas por esta revisão.


## Revisão62 — transporte vinculado à sessão e contexto autenticado — 26/10/2026

Backend `327002939a517a314864cb2d42cfb984ca09f79e`, branch `feat/backend-b02a-compat`; [CI aprovado](https://github.com/danilomalta/zzeussss/actions/runs/40360560172). Onze arquivos de implementação, testes, contrato, guia e workflow. Schema66,529rotas e os66scripts SQL históricos preservados. Os fontes revisados mantiveram seus hashes durante a validação e instalação.

### Entrega

O contexto v2 exige o vínculo exato da sessão original além de URL, empresa, loja, aparelho, usuário e fingerprint. Uma nova entrada do mesmo usuário não seleciona tentativas de sessão anterior. `status/run/watch --scope` omite esses itens e preserva seus IDs, provas de resultado incerto, contadores e agendamento, sem ler suas credenciais ou reenviar mensagens. Trocar somente o vínculo da sessão encerra o worker em curso. Uma chamada já enviada pode concluir, preservando sua prova; não há promessa de revogação retroativa de pacotes.

Novo comando `titan-trade queue context --job ARQUIVO --source-token ARQUIVO --out CONTEXTO_NOVO`: confere `/me` e `/company-trade/identity` reais contra a origem explicitamente escolhida. Cria somente um arquivo v2 novo e privado, após rever o arquivo de sessão; troca durante a verificação é recusada. Não envia mensagens, acessa o destinatário ou substitui contexto existente. O token não vai para o contexto, argumentos ou saída; a opção recebe caminho de arquivo privado. Saída limitada a `context_created:true`.

Formato v1 preservado como filtro administrativo de identidade, sem alegação de vínculo com a sessão atual. Campo de sessão no v1, inclusive nulo/vazio ou alias de maiúsculas no primeiro nível, é recusado. V2 exige hash válido, sem fallback. As verificações de autorização, job congelado, sessões, pares aprovados, recibos e barreira permanente de fila restaurada continuam exigidas.

### Aceite executado

LOCAL e CI: **1347 testes principais /2858 incluindo subtestes, zero falhas, cinco integrações reais obrigatórias e três skips auxiliares previstos**. PostgreSQL17 e SQLite descartáveis; pontes frontend contra HTTP Go real. Análise estática de todo o backend, mapa de contratos e dez regressões do verificador de contratos aprovados. A primeira rodada local precedeu o novo comando; o total informado é da rodada final repetida sobre todos os fontes publicados.

Componentes de fila e CLI aprovados com detector de corridas. Cenários novos contra HTTP real também aprovados com `-race` em **41.369s**: mesmo usuário com outra sessão; tentativa pendente e resultado incerto; contexto existente preservado; ator/fingerprint divergentes; logout; arquivo de sessão trocado durante a verificação. Rodada CI completa de trocas, devoluções e quatro pilotos com CLIs reais aprovada em **567.397s**. Orçamento do CI ampliado para20min no race e40min no job porque a rodada anterior levou852,493s; cobertura mantida. Isso não mede abertura do software.

Prova adicional com API `titan-local` e CLI `titan-trade` NATIVOS INSTALADOS, produzidos pelo CI: PASS em **4.013s**. O comando compilado verificou sessão/estação, recusou sobrescrita e credenciais revogadas, criou contexto0600 sem token e não realizou efeito comercial durante a criação. Nova sessão do mesmo usuário não retomou tentativa antiga. A operação incerta preservou prova e IDs e recuperou recibo sem repetirPOST. O ensaio usa overlay externo somente para iniciar os executáveis em bancos sintéticos; não altera fontes nem simula a API de negócio.

Regressões nativas do filtro anterior também aprovadas: contextos separados, mudança antes/depois doPOST e após perda da resposta, `watch` encerrado, recuperação após restauração com origem desligada. Regressão integral adicional: venda → caixa → estoque → reposição → pedido/aceite autenticado → recebimento parcial com divergência → financeiro → devolução, reinício e restauração peloCLI, **200 assertivas HTTP em 9 processos nativos**, caixa1574centavos e obrigação em aberto0. Smoke nativo de restart/backup/restauração/schema66 e proteção de licença aprovado.

Ensaio adicional HTTPS com os executáveis instalados: duas APIs nativas independentes atrás de terminadores TLS efêmeros em loopback, CA sintética explicitamente confiável, mensagem assinada entregue uma única vez, certificado não confiável recusado antes doHTTP e recuperação com origem desligada sem novoPOST. O código do ensaio externo acompanha a evidência como `native-tls-test.go.txt`; não integra o total do CI nem demonstra instalação comercial na Internet. Não houve `InsecureSkipVerify`, listener público ou dado comercial.

Revisão independente somente de leitura do delta a43af76→3270029, por agente geral de código (a ferramenta especializada da skill não estava disponível): nenhum achado concreto de segurança introduzido nos11arquivos. Testes unitários selecionados de sessão também passaram. O revisor não reexecutou a suíte HTTP e não certifica segurança integral do sistema; registrou a responsabilidade do host pela invalidação no logout.

### Instalação, evidências e limites

Sete executáveis Linux amd64 do CI instalados em `/home/danillo-malta/.local/opt/titansystem/backend/327002939a517a314864cb2d42cfb984ca09f79e`, hashes conferidos e ponteiro `backend/current` atualizado atomicamente; versão anterior a43af76 e demais instalações preservadas. Serviços8182/8282 conservam os mesmos PIDs/executáveis e health200: não foram reiniciados ou atualizados. Nenhum banco/chave comercial lido ou escolhido. Instalar binários não muda os serviços já em execução.

Os oito arquivos pausados do frontend mantêm os hashes originais. Trabalho preexistente do registro único preservado separadamente no working tree; este commit contém somente esta revisão e as evidências sanitizadas em `docs/evidence/backend-trade-session-scope-20261026/`. Laboratório/preimagens/logs/hashes em `work/backend-trade-session-scope-20261026/`. Sem ZIP ou dependências nesta entrega.

O transporte com contexto de sessão e sua geração verificada estão entregues. **A ligação automática ao Electron continua pendente**: o host deve invalidar o contexto ao sair ou mudar perfil e parar o worker; contexto criado não é sessão permanente nem autorização adicional. Esta entrega não altera telas, desktop ou backend online de provedores. O transporte/fila ainda recusa execução fora do Linux (`lock_other.go` retorna `ErrInvalid`): Windows exige implementação e prova de lock durável e arquivos privados, além da homologação do hardware. Fiscal/Pix/TEF reais, operação comercial na Internet e demais fluxos sem aceite integral não estão concluídos por estes testes. Backend inteiro não declarado100%. Guia atualizado na branch backend: `docs/architecture/BACKEND_FILA_CONTEXTO_20261026.md`.


## Integração revisável — 27/10/2026

Worktree isolada `integration/piloto-main-20261027`; originais preservados. Bases
publicadas: backend3270029, frontend3d5abc9, main024dd3f. Conflito somente no
README, resolvido combinando responsabilidades do desktop e do backend.
O incremento fiscal local foi conferido antes da cópia por lista exata de86
arquivos, com workflows reconciliados sem perder os checks de integração.
Alterações frontend posteriores ao head publicado permanecem fora desta revisão.

PRs rascunhos2 e3 registram as branches publicadas. A integração conjunta segue
em PR próprio e não autoriza merge ou implantação em banco real. Proteção de
main exige CI e aprovação independente. Ensaios de instalador devem registrar
SO, pacote, perfil descartável, imagem e tempo; não substituir validação em
equipamento do piloto nem anunciar cliente real sem implantação autorizada.

### Integração: validação de instalação e correções de CI

PR conjunto: https://github.com/danilomalta/zzeussss/pull/4. Os PRs2/3 foram
fechados em favor dele, sem merge nem exclusão das branches. A main tem sete
checks obrigatórios vinculados ao aplicativo GitHub Actions, atualização estrita
da base e aprovação independente, inclusive para administradores. Mudar a
política de aprovação de mantenedor único exige decisão explícita do dono;
revisão automática não é alegada como aprovação formal.

Regressões locais atuais:61 testes desktop aprovados, incluindo encerramento
de processo descartável que ignora SIGTERM. A limpeza encerra apenas a árvore
criada pelo ensaio, por grupo POSIX separado ou PID específico no Windows.
Não altera processos8182/8282 nem a instalação do usuário.

No CI e57d483:2465 testes frontend, TypeScript e build aprovados;85 testes de
ferramentas aprovados. Instalação Linux do backend com binários reais, banco
sintético e reabertura por segundo processo aprovada. Instalador DEB teve suas
dependências resolvidas pelo gerenciador e a janela instalada exibiu5controles
em584ms. Esse job foi cancelado após ficar preso na limpeza; não é registrado
como check aprovado. O ensaio foi corrigido para encerrar a própria árvore.
O NSIS gerado retornou zero sem executável no destino esperado; parâmetros
do ensaio agora usam caminho Windows nativo e escopo explícito de usuário.
A revisão final deve comprovar os dois instaladores e os pilotos no seu próprio
commit antes do merge. Os números acima são resultados identificados, não
uma alegação de CI integral verde da revisão final.

Correções adicionais desta integração: caminhos Windows no build/manifesto,
colisão de nomes FormGuard.tsx/formGuard.mjs corrigida com componente
ProductionFormGuard.tsx, homepage do pacote e nome explícito do executável
Windows. Sandbox preservado em ambos os sistemas. Artefatos de avaliação
ainda não equivalem a instalação comercial assinada nem cliente real operando.

### Decisão do proprietário: aprovação de mantenedor único

Em27/10/2026, o proprietário escolheu explicitamente: CI obrigatório, revisão
automática do Codex quando habilitada e sua autorização final para merge.
Esta decisão substitui a exigência anterior de aprovação por outra conta.
A proteção mantém os sete checks vinculados ao GitHub Actions, base estrita,
aplicação a administradores, resolução de conversas e bloqueio de force-push
e exclusão. A contagem de aprovações externas passa a zero; aprovação da
última atualização por terceiro é desativada. Nenhum merge automático.

A autorização desta decisão não autoriza o merge do PR4. Ele continua
aguardando todos os checks, a revisão automática disponível e a decisão
final do dono. Comentário pedindo revisão não comprova revisão concluída
nem habilitação da integração; o resultado deve ser conferido no GitHub.

Provas da revisão0c1e16d: seis checks verdes, incluindo backend e frontend
(2465 testes frontend, TypeScript/build), runtime desktop e instaladores
Ubuntu/Windows/backend Linux. Abertura instalada:971ms Linux e869ms Windows;
imagens e relatórios conferidos, hashes dos artefatos validados. O check
HTTP integral ainda executava os pilotos com detector de corridas na hora
deste registro. Não é declarado aprovado antes de sua conclusão. A alteração
de política/documentação também precisa dos checks de sua própria revisão.
