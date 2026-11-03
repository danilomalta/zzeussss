# Frontend109 — Malotes e detalhes do livro de caixa

## Base e escopo

Requer108. Instalação108 confirmada no computador do usuário:161 testes,0 falhas, build20,33s. Duas melhorias de frontend nos fluxos existentes: central de malotes e resumo/detalhes do livro de caixa. Não muda Go, SQL, sessão, licença, APIs, contratos, cotação B01, adquirente ou fechamento oficial. Não cria dinheiro, movimento de estoque ou operação de tesouraria. Registros de malotes continuam no armazenamento local existente, por contexto. Nenhuma dependência nova.

## Malotes

Bag99 passa a abrir com uma central de consulta: busca por código/destinatário, filtro por etapa, opção só com diferença efetivamente conferida, indicadores de pendentes/conferidos/com diferença e CSV da lista filtrada. Paginação de10 malotes; limite de500 conforme o modelo anterior. Sem malotes, o formulário de lacre existente continua abaixo. O modelo só permite um malote criado por caixa atualmente; esta entrega não remove esse limite. A lista permite consultar históricos importados que o modelo já aceita, sem prometer criação de múltiplos lacres no mesmo caixa.

Diferença não conferida aparece como “Ainda não conferido”, nunca zero. Diferença negativa/positiva/zero só deriva de malote validado na última etapa, com contagem e justificativa quando necessária. Pesquisa literal sem regex, sem distinção de acentos, com limite de200 caracteres. A validação da cadeia e do contexto acontece antes de exibir ou filtrar. Soma em centavos com BigInt e verificação de limite seguro. Dados inválidos não viram lista vazia nem liberam ações.

“Abrir etapas e conferência” abre o cartão de trabalho existente: lacre, entrega, recebimento, segunda conferência, impressão80mm e cópia do código. A política de duas pessoas, permissões e proibição de conferir o próprio malote continuam sob bagAction99/bagStep94. A seleção não concede acesso nem busca outro contexto. Ao trocar de malote com contagem em edição, confirmação protege contra descarte involuntário; quando confirmada, limpa somente campos transitórios. Mudar contexto ou usuário recria/limpa o estado da seleção e contagem.

Falha booleana de gravação agora exibe erro no fluxo de etapas e preserva a contagem. Trava síncrona evita executar duas mutações pelo mesmo clique. O armazenamento/CAS anterior continua responsável pela concorrência entre instâncias. Não há sincronização nova entre aparelhos nem confirmação remota de entrega. O recibo do malote permanece registro local; não comprova transferência bancária.

CSV usa a função existente csv97 contra aspas, delimitadores e fórmulas. Contém código do malote, destino, valor lacrado, etapa, contagem quando disponível, diferença e data da última etapa, mais aviso local. Download direto; nenhuma busca de diretório pelo aplicativo. Preferência do navegador pode perguntar onde salvar.

## Livro de caixa

Ledger99 conserva consultas autorizadas, atualização existente a cada minuto quando visível, filtros, paginação, CSV e impressão da listagem. O resumo acompanha os filtros aplicados, separando número de parcelas de venda, solicitações e eventos de atividade. Tabela por forma mostra parcelas/valor apenas das vendas consultadas. Uma venda paga em dinheiro100, débito96 e crédito119 tem3 parcelas, total315; não são3 vendas. Solicitações e registros de atividade não entram na soma de recebimentos. Aprovação local não significa execução financeira. Não é saldo da gaveta: abertura/sangria/suprimento oficiais continuam sob consulta própria.

Consulta incompleta mostra traço para valores desconhecidos, mantendo contagens somente dos registros carregados. Erro no período não produz resumo silencioso de zero. O modelo valida tipos, origens, chaves únicas, quantidade, datas e centavos antes de resumir. Não altera lançamentos, solicitações ou documentos.

Clicar no tipo/número da linha abre detalhes com origem, tipo, data, valor, forma, situação, motivo e indicação do usuário atual/outro responsável. Não imprime UUID interno de venda, pagamento ou pessoa. Detalhes são somente leitura e a impressão contém aviso fiscal/bancário ou local conforme a origem. Snapshot do registro protege a impressão: se consulta ou estado mudar, exige conferir dados atualizados. Troca de contexto limpa a seleção; filtro que exclui o registro oculta seus detalhes. Campos são renderizados como texto e o utilitário anterior escapa impressão.

## Integração e limites

Sem novos endpoints nesta entrega. Reutiliza ClosingSourceContext92 para vendas e Till94/IndexedDB existentes para malotes, solicitações e diário local. Não muda ledger99, till94, gate97 ou autorização no servidor. Malote remoto, auditoria inviolável e nome real de outros funcionários exigem futuros contratos no backend; não são simulados como integração ativa. Nenhum dado pessoal novo fica em cache permanente. Não adiciona timers ou requisições; o polling do livro permanece o anterior.

26 chaves novas completas em pt-BR/en/es/fr/zh-CN; datas/números via Intl. Traduções estrangeiras ainda requerem revisão nativa. Tokens neutros existentes, sem azul/dourado novo; texto auxiliar14px, campos16px e alvos48px nas novas áreas. Tabelas com rolagem interna, cartões responsivos e foco visível. Controles da demonstração são identificados separadamente do produto.

## Verificação e aceite

20 testes novos: diferenças desconhecida/negativa/zero, filtros, cadeia inválida, políticas e permissões, duplicidade, overflow, consulta parcial, parcelas divididas, solicitações/atividade fora do total, filtros, snapshot e cobertura de textos em cinco idiomas. Suíte completa520 testes; typecheck e build; navegador Firefox com ambas áreas nos dois temas e três larguras. Relatório completo em VALIDACAO.txt do pacote. Sem teste de backend/banco comercial, notas da maquininha, malote físico, impressora física ou concorrência real entre aparelhos.

Testar /local/pos → Malote: procurar destinatário/código, filtrar etapa/diferenças, baixar CSV, abrir etapas, executar conforme perfil, conferir erro de gravação na prévia. Em política dupla, operador que lacrou não deve registrar a própria segunda conferência. Trocar malote com campos preenchidos deve pedir confirmação. Livro de caixa: filtrar forma/origem/período, observar resumo e ausência de dupla soma, abrir e imprimir detalhes, atualizar consulta, trocar filtros e testar dados parciais na prévia. Encerramento e relatório oficial seguem os contratos anteriores. Esta entrega não declara toda a Parte1 concluída.
