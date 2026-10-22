# Continuidade do TitanSystem

Revisão 11 — 22/10/2026. Registro único de continuidade a atualizar a cada entrega.
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
| Catálogo, estoque e perdas | /local/catalog; /local/stock; /local/losses | Novo estoque oficial por produto/local com físico/reservado/livre, histórico genérico, entrada/perda/transferência, CSV e cópia de página para planilha. Catálogo anterior preservado. | Inventário revisado implementado nesta revisão10 (abertura, contagem parcial/zero, aprovação independente, recuperação/histórico/CSV). Tradução e integração das demais ações do catálogo/perdas, periodicidade da empresa e custo histórico das perdas continuam pendentes. Valoração atual por página e análise estimada de diferenças implementadas na revisão11. | Availability e stock operations/history/receipt existentes; suplemento inventory-costs da referência agora contém contrato e Go do ciclo de inventário revisado. Isso não comprova serviço atualizado. | Homologação em serviço de teste; política de inventário da empresa. |23testes novos do estoque, inclusive preservação da entrada antiga; regressões completas858. | Unidade de movimentos implementada; fluxo amplo de estoque ainda parcial. |
| Reposição e pedido local | /local/orders | 115 aplicado. Preservar mínimo/alvo, sugestão e aceite/recusa humana. Completar conexão entre o pedido local e o circuito interempresa quando houver contrato. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | API local existente não demonstra envio/aceite do fornecedor. Vínculo e condições comerciais não podem ser deduzidos da aprovação local. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Fornecedor, doca, expedição e recebimento | /local/orders | Recebimento oficial integrado aos detalhes do pedido: autorização humana local, aceitação parcial, recusa integral, estorno de entrada e histórico. | Confirmação externa, docas, carga, coleta e política de créditos permanecem parciais; não foram recriadas como sucesso local. | Contratos de receiving authorization/trace/history/operations e receipt/rejection/void usados. | Acordo com fornecedor, motorista e integrações externas. |26testes novos do recebimento; prévia descartável de autorização e entrada parcial; suíte858. | Unidade local implementada; circuito interempresa ainda parcial. |
| Troca, devolução, coleta e reembolso | /local/pos | 100/110 são solicitações e revisão locais. Integrar execução oficial sem apresentar solicitação como reembolso; coleta/chegada são confirmações distintas. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Cancelamento de venda local existe parcialmente. Política do fornecedor, coleta, chegada e crédito/reposição/reembolso exigem contrato e aceite de ponta a ponta. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Documentos e financeiro | /local/payables; /local/payables/agenda |113–117 preservados. Criação documental, parcelas exatas e revisão independente agora integradas aos contratos. | Contas a receber, centros de custo, fluxo de caixa/DRE, OFX e outras fontes continuam a conferir e integrar; não classificados como API inexistente sem inspeção. | Payables e vínculos existentes usados. Demais contratos precisam ser identificados por fluxo. | Bancos/PSP e aceite contábil são separados. | Novos testes de documentos mais regressões113/114/116/117;858 na suíte completa. | Unidade de documentos implementada; módulo financeiro amplo ainda parcial. |
| Contador e exportações | /local/payables/agenda | 117 já entrega CSV/XLSX/impressão/cópia para planilha do escopo consultado; não recriar. Falta acesso externo autorizado, carteira/checklist/entrega quando suportados. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Exportação local não é portal do contador. Compartilhamento revogável, trilha, limites maiores e visão empresarial/histórica exigem contratos próprios. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Receitas, produção e qualidade | /local/recipes; /local/operation | Consulta/capacidade, publicação imutável, ordens, reserva/liberação/consumo, etapas, resultado medido e declarações de perdas/lotes/qualidade integrados em /local/recipes. Planejamento86/90 preservado como rascunho. | Custos completos, atribuir outros responsáveis nos formulários, estado ativo/inativo da receita, histórico próprio de etapas na UI, bloqueio interno de navegação dos formulários e aceite real ainda pendentes. | Contratos reais existentes consumidos, incluindo recibos originais e trace; backend separado não alterado. | Serviço/perfis descartáveis ainda a identificar com responsável. |1028testes; TypeScript/build; circuitos visuais em memória e5idiomas. | Subfluxos aplicados; produção ampla ainda parcial, sem homologação real nem instalador novo. |
| RH e equipe | /local/hr; /local/staff | Staff real e painel71 existem. Conectar fontes autorizadas de ponto, tarefas, metas, escalas, férias/documentos/folha e avaliações privadas. Não confundir RH com cadastro de acesso. | Revisar integração e concluir ações indicadas; não equivale a API ausente. | Não houve comprovação do ciclo completo de RH/portal móvel; APIs/fonte e permissão por dado devem ser coordenadas. | Serviço e perfil reais não confirmados; hardware/provedores conforme fluxo. | 157 regressões 113/114/116/117 na unidade anterior; demais verificações desta etapa pendentes. | Parcial; frontend a concluir |
| Planilhas | /local/sheets | Editor e presets preservados. Cinco idiomas carregados sob demanda, com proteção para mudança tardia de idioma/contexto. Cópias117 e estoque permanecem cópias explícitas. | Integrar fontes reais pendentes de estoque/financeiro, revisar blocos/versões/importação e homologar editor completo; não afirmar conclusão por reduzir chunk. | Fontes parciais existentes; contratos locais de estoque/financeiro disponíveis para integração posterior. | Medir abertura/memória no hardware alvo; colaboração/compartilhamento dependem de contrato. |5testes novos de idioma; regressões68/69/70 na suíte858; TypeScript/build passaram. | Otimização implementada e medida em build; planilha ampla ainda parcial. |
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

`/local/stock` passa a selecionar três painéis: movimentos genéricos anteriores, inventário revisado e posição/valoração. Botões com estado pressionado e rótulos nos cinco idiomas. Trocar de painel conserva os componentes/campos montados; não há envio ou descarte implícito. O inventário116/117 não foi substituído; a navegação de rota mantém a proteção de campos e filas. A consulta física anterior e sua fila continuam independentes.

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
