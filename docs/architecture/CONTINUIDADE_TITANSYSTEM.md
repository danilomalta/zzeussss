# Continuidade do TitanSystem

Revisão 22 — 23/10/2026. Registro único de continuidade a atualizar a cada entrega.
Este arquivo faz parte das fontes do projeto; não implica commit ou publicação.

## Base, localização e responsáveis

- Interface oficial: `/home/danillo-malta/Downloads/zzeus-frontend-backend/frontend-web/`.
- Branch frontend: `feat/backend-checkout-authority`; HEAD auditado `c5579808c62826174b43a2b7a45fbc2ceda1e47b`. Os arquivos locais não publicados também compõem a base; o HEAD sozinho não identifica a entrega.
- Frontend116 presente; Frontend117 CORRETO de conferência por fornecedor aplicado. Confirmados `SupplierReport117.tsx`, `core/payables117`, cinco dicionários, teste e documento193. O usuário informou em21/10: compatibilidade aprovada, backup,136testes sem falhas, TypeScript/build aprovados em20,05s, sem banco/commit/merge/push.
- Frontend passa a ser responsabilidade deste chat/Codex, com preservação do trabalho anterior. Backend continua com responsável separado.
- Backend de referência, somente leitura: `/home/danillo-malta/Downloads/zzeus-frontend-backend-b02a/backend/`; branch `feat/backend-b02a-compat`, HEAD `58a26bbbdf08903d62f6dd1f0542417171b460dd`; árvore limpa na auditoria inicial; a referência recebeu alterações externas posteriores, não aplicadas por este chat. B02B13 está no código e contratos. Processo/versão/banco/estação em execução continuam não identificados.
- `~/Downloads/zzeus-frontend` é uma árvore anterior. `Documents/Codex/.../work` contém cópias de laboratório, não a fonte principal.
- `desktop/` é o invólucro Electron, separado das telas. Não foi alterado nesta continuidade.
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
| Acesso, cadastro e atividade | /local/login; /register; /local/settings | Login/contexto e cadastro112 existem. Conectar criação/recuperação/perfil real e seleção autorizada de loja onde os contratos suportarem; usar atividade central, sem repetir questionário em módulos. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | APIs locais de conta, staff e política existem; não presumir cadastro completo de empresa, licenciamento comercial e filiais. Demonstrar autorização/isolamento. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Sessão financeira | /local/payables; /local/orders | Correção116 preservada; extensão de origem vigente ao115; callbacks consideram token, sessão e capacidades atuais. | Aceite de integração real com perfis descartáveis. | Mesmos contratos, sem mudanças no backend. | Serviço local e perfil a identificar pelo responsável. | Regressões de troca de quatro campos de contexto, logout, corpo tardio, licença revogada e IDs originais; suíte total858 passou. | Implementado e validado em testes isolados; integração real pendente. |
| Venda/PDV | /local/pos | Preservar foco, tela cheia, CPF separado do clube, cotação e revisão de preço, troco após concluir e carrinhos suspensos. Conectar contratos avançados com recuperação sem afrouxar cliente legado. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Cotação/venda local e planos checkout estão no código. Gateway95 inicia desconectado; a existência dos planos não prova PSP ou pagamento misto integrado. Demonstrar venda/reinício/recibo sem duplicação. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Pagamentos, descontos e fiscal | /local/pos | Seletores/drafts não são confirmação. Integrar somente meios e estados reais. Separar cancelamento de carrinho, venda, pagamento e nota. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Pix/TEF/adquirente e fiscal exigem provedor/ambiente e contratos completos. Não considerar papel como liquidação. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Fechamento e comprovantes | /local/cash | 97–108 existem: dinheiro físico, provas, revisão, OCR e relatório oficial. Integrar estados oficiais adicionais sem duplicar parcela por papel+integração; não guardar imagem por padrão. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Relatório oficial e contratos de proofs estão no código. Revisão de papel não é confirmação bancária. Falta demonstrar fluxo combinado com permissões e hardware reais. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Movimentos, livro, malote e cofre | /local/cash | 99/105/109 têm registros/revisões locais. Conectar a cadeia oficial e distinguir o registro local de recebimento remoto. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Há APIs de movimentos, bags, custódia e vaults. Não declarar esses contratos inexistentes. Revisar política do conferente/origem e demonstrar recuperação sem autocustódia/crédito duplicado. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Catálogo, estoque e perdas | /local/catalog; /local/stock; /local/losses | Novo estoque oficial por produto/local com físico/reservado/livre, histórico genérico, entrada/perda/transferência, CSV e cópia de página para planilha. Catálogo anterior preservado; edição cadastral, estado operacional e histórico integrados na revisão17. | Inventário revisado implementado nesta revisão10 (abertura, contagem parcial/zero, aprovação independente, recuperação/histórico/CSV). Gestão, busca, filtros, tabela e criação do produto em cinco idiomas; demais ações de perdas ainda precisam de revisão. Periodicidade da empresa e custo histórico das perdas continuam pendentes. Valoração atual por página e análise estimada de diferenças implementadas na revisão11. | Availability e stock operations/history/receipt existentes; suplemento inventory-costs da referência agora contém contrato e Go do ciclo de inventário revisado. Isso não comprova serviço atualizado. | Homologação em serviço de teste; política de inventário da empresa. |23testes novos do estoque, inclusive preservação da entrada antiga; regressões completas858. | Unidade de movimentos implementada; fluxo amplo de estoque ainda parcial. |
| Reposição e pedido local | /local/orders | 115 aplicado. Preservar mínimo/alvo, sugestão e aceite/recusa humana. Completar conexão entre o pedido local e o circuito interempresa quando houver contrato. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | API local existente não demonstra envio/aceite do fornecedor. Vínculo e condições comerciais não podem ser deduzidos da aprovação local. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Fornecedor, doca, expedição e recebimento | /local/orders | Recebimento oficial integrado aos detalhes do pedido: autorização humana local, aceitação parcial, recusa integral, estorno de entrada e histórico. | Agenda operacional de docas integrada na revisão20: recursos, reserva/reagendamento, chegada/descarga, ocupação e histórico. Painel agrupado e conferência por pedido integrados como consultas na revisão21. Tratativas, indicadores de doca/utilização, confirmação externa, carga/coleta e política de créditos continuam parciais; não foram recriados como sucesso local. | Contratos de receiving authorization/trace/history/operations e receipt/rejection/void usados. | Acordo com fornecedor, motorista e integrações externas. |26testes novos do recebimento; prévia descartável de autorização e entrada parcial; suíte858. | Unidade local implementada; circuito interempresa ainda parcial. |
| Troca, devolução, coleta e reembolso | /local/pos | 100/110 são solicitações e revisão locais. Integrar execução oficial sem apresentar solicitação como reembolso; coleta/chegada são confirmações distintas. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Cancelamento de venda local existe parcialmente. Política do fornecedor, coleta, chegada e crédito/reposição/reembolso exigem contrato e aceite de ponta a ponta. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Documentos e financeiro | /local/payables; /local/payables/agenda |113–117 preservados. Criação documental, parcelas exatas e revisão independente agora integradas aos contratos. | Contas a receber, centros de custo, fluxo de caixa/DRE, OFX e outras fontes continuam a conferir e integrar; não classificados como API inexistente sem inspeção. | Payables e vínculos existentes usados. Demais contratos precisam ser identificados por fluxo. | Bancos/PSP e aceite contábil são separados. | Novos testes de documentos mais regressões113/114/116/117;858 na suíte completa. | Unidade de documentos implementada; módulo financeiro amplo ainda parcial. |
| Contador e exportações | /local/payables/agenda | 117 já entrega CSV/XLSX/impressão/cópia para planilha do escopo consultado; não recriar. Falta acesso externo autorizado, carteira/checklist/entrega quando suportados. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Exportação local não é portal do contador. Compartilhamento revogável, trilha, limites maiores e visão empresarial/histórica exigem contratos próprios. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Receitas, produção e qualidade | /local/recipes; /local/operation | Consulta/capacidade, publicação imutável, ordens, reserva/liberação/consumo, etapas, resultado medido e declarações de perdas/lotes/qualidade integrados em /local/recipes. Planejamento86/90 preservado como rascunho. | Estado da receita, responsáveis autorizados, histórico de etapas e proteção de formulários já implementados na revisão8; planejamento simultâneo/estimativas na revisão9. Permanecem custo real completo, integrações/aceite e demais limites documentados, sem recriar as unidades entregues. | Contratos reais existentes consumidos, incluindo recibos originais e trace; backend separado não alterado. | Serviço/perfis descartáveis ainda a identificar com responsável. |1028testes; TypeScript/build; circuitos visuais em memória e5idiomas. | Subfluxos aplicados; produção ampla ainda parcial, sem homologação real nem instalador novo. |
| RH e equipe | /local/hr; /local/staff | Staff real e painel71 existem. Conectar fontes autorizadas de ponto, tarefas, metas, escalas, férias/documentos/folha e avaliações privadas. Não confundir RH com cadastro de acesso. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Não houve comprovação do ciclo completo de RH/portal móvel; APIs/fonte e permissão por dado devem ser coordenadas. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Planilhas | /local/sheets | Editor e presets preservados. Cinco idiomas carregados sob demanda, com proteção para mudança tardia de idioma/contexto. Cópias117 e estoque permanecem cópias explícitas. | Estoque físico por local e obrigações a pagar por vencimento integrados a contratos existentes; revisar demais fontes, blocos/versões/importação, compartilhamento e homologar editor completo. Separação do motor reduz carga estática inicial, não o pacote total. | Fontes parciais existentes; contratos locais de estoque/financeiro disponíveis para integração posterior. | Medir abertura/memória no hardware alvo; colaboração/compartilhamento dependem de contrato. |Fontes e carregamento: testes incrementais de idioma, contexto, estoque, obrigações e texto literal; suíte1444, TypeScript/build aprovados. Prévia isolada não homologa serviço real. | Otimização implementada e medida em build; planilha ampla ainda parcial. |
| Gestão, comercial, documentos e integrações | /local/enterprise; /local/commercial; /local/documents | Áreas85/89 existem; diferenciar arquivo de trabalho de fonte oficial e conectar cada adaptador disponível. Não criar menu duplicado. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Há fontes parciais/indisponíveis. Cadastro comercial, CRM, integrações e relatórios completos precisam de contrato e aceite delimitados. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Entrega, mapa e frota | /local/delivery; /local/map; /local/fleet | Preservar geografia, clientes externos, janelas negociadas e separação de delivery/cozinha; conectar carga, confirmação e localização autorizada. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Não presumir telemetria/ETA ao vivo, app de motorista ou roteamento comercial completo pela presença do mapa. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Offline e móvel | /local/integrations/diagnostics | Preservar estados de conexão e limites; não prometer venda móvel independente do PC sem prova. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Direitos exclusivos, transferência durável, backup antigo, revogação e reconciliação precisam de protocolo/aceite. P2P, timestamp e reenvio idempotente sozinhos não resolvem saldo global. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Desktop, segurança e desempenho | frontend-web; desktop fora desta tarefa | Telas permanecem em frontend-web. Empacotamento está fora da unidade atual. Medir abertura/operação/consulta/memória no mesmo hardware; preservar cinco idiomas, temas, teclado e touch. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Homologar instalação/reinício/atualização/restauração Windows/Linux sobre combinação conhecida; periféricos e APIs externas não testados nesta entrega. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Empacotamento fora da primeira entrega desta tarefa; desempenho frontend pendente |

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
