# TitanSystem — inventário após pedidos66

Atualização dos mesmos 96 itens em 32 frentes. Os arquivos65 permanecem como fotografia da entrega anterior. Esta entrega implementa as três frentes EMP; aceite visual/comercial continua pendente. Não conclui fiscal, Pix/cartão, operação móvel independente ou todos os módulos.

|ID|Frente|Estado|Pendência|
|---|---|---|---|
|DEV-01|Devoluções|entregue_neste_pacote|Aceite visual no desktop instalado: história, cotação, revisão, operação incerta e recuperação.|
|DEV-02|Devoluções|entregue_neste_pacote|Publicar backend65 no serviço autorizado antes de usar as operações novas; preservar serviços antigos até decisão do proprietário.|
|DEV-03|Devoluções|entregue_neste_pacote|Reembolso em outro turno, cartão/Pix ou crédito ao cliente exige contratos de backend próprios.|
|REL-01|Relatório oficial|entregue_neste_pacote|Comparar impressão e CSV com fechamento após devoluções no serviço autorizado.|
|REL-02|Relatório oficial|entregue_neste_pacote|Manter indisponível a métrica de devolução em serviços antigos; nunca apresentar ausência como zero.|
|REL-03|Relatório oficial|entregue_neste_pacote|Não converter confirmação local de parcelas em liquidação bancária.|
|PDV-01|Venda e carrinhos|existente_exige_aceite|Executar aceite pelas telas com quedas, reenvio, concorrência e restauração, preservando as filas incertas.|
|PDV-02|Venda e carrinhos|existente_exige_aceite|Concluir revisão dos oito arquivos locais pausados de carrinhos no projeto do proprietário; este pacote não conhece nem substitui seus bytes.|
|PDV-03|Venda e carrinhos|existente_exige_aceite|Finalização mista exige confirmação e conciliação de provedores no backend; cash-only atual permanece explícito.|
|PAG-01|Pagamentos eletrônicos|bloqueio_backend_provedor|Integrar Pix real com consulta de cobrança, expiração, duplicidade e autorização.|
|PAG-02|Pagamentos eletrônicos|bloqueio_backend_provedor|Integrar adquirente/TEF com recusas, estorno e recuperação após perda de resposta.|
|PAG-03|Pagamentos eletrônicos|bloqueio_backend_provedor|Vincular pagamentos divididos a uma venda transacional e ao fechamento por meio; comprovante não é aprovação.|
|CAI-01|Caixa e livro operacional|existente_exige_aceite|Ensaiar abertura, suprimento, sangria, fechamento e consulta histórica pelos perfis efetivos.|
|CAI-02|Caixa e livro operacional|existente_exige_aceite|Validar caixa original/aparelho e retornos401/403/409 sem importar valores de outro turno.|
|CAI-03|Caixa e livro operacional|existente_exige_aceite|Conciliar o relatório líquido com o journal incluindo devoluções e cancelamentos distintos.|
|MAL-01|Malotes e cofres|existente_exige_aceite|Auditar quais ações usam fonte oficial e quais continuam sendo rascunhos locais.|
|MAL-02|Malotes e cofres|existente_exige_aceite|Conectar recebimento, conferência independente e recuperação às operações correspondentes do backend.|
|MAL-03|Malotes e cofres|existente_exige_aceite|Exercitar duplicação de origem, conferente distinto, envio incerto e restauração.|
|FIN-01|Contas a pagar|existente_exige_aceite|Aceitar documentos, parcelas, revisão independente, vencimentos e pagamento manual com perfis reais.|
|FIN-02|Contas a pagar|existente_exige_aceite|Preservar origem de retirada/suprimento e impedir uso duplicado do mesmo dinheiro.|
|FIN-03|Contas a pagar|existente_exige_aceite|Pagamentos por banco/Pix/transferência exigem novos contratos e conciliação externa.|
|FOR-01|Visão do fornecedor|existente_exige_aceite|Preservar a entrega117 aplicada; comparar totais com a fonte oficial por fornecedor.|
|FOR-02|Visão do fornecedor|existente_exige_aceite|Testar atualização parcial, erro de uma fonte, exportação e datas locais.|
|FOR-03|Visão do fornecedor|existente_exige_aceite|Revisar acessibilidade e idiomas em conjuntos grandes de parcelas.|
|PED-01|Compras e reposição|existente_exige_aceite|Ensaiar sugestão→aprovação→pedido→recebimento parcial→financeiro nas telas atuais.|
|PED-02|Compras e reposição|existente_exige_aceite|Confirmar revisão humana e contexto antes de todoPOST após consulta assíncrona.|
|PED-03|Compras e reposição|existente_exige_aceite|Distinguir sugestão de pedido aprovado e recebimento de entrada física efetiva.|
|EMP-01|Pedidos autenticados entre empresas|implementado_exige_aceite_visual|Interface implementada para identidade pública, conferência do fingerprint, vínculo pelo dono, paginação e revogação. Pendente aceite visual e verificação independente da contraparte real.|
|EMP-02|Pedidos autenticados entre empresas|implementado_exige_aceite_visual|Interface implementada para exportar pedido aprovado, importar envelope, decidir quantidade/prazo e aplicar decisão. Ponte real HTTP com duas empresas passou; pendente aceite pelas telas no serviço autorizado. Transporte entre instalações é manual, sem worker automático.|
|EMP-03|Pedidos autenticados entre empresas|implementado_exige_aceite_visual|Interface implementada para consulta do bloqueio por restauração e revisão explícita da geração pelo dono. Testado contra gate sintético real do backend; não executou restauração completa de arquivo nem revisão comercial de fingerprints.|
|REC-01|Recebimento e divergências|existente_exige_aceite|Conferir entrega, quantidade aceita, avaria, recusa integral e saldo restante com estoque oficial.|
|REC-02|Recebimento e divergências|existente_exige_aceite|Conectar tratativas e decisão independente até resolução, sem encerrar avaria automaticamente.|
|REC-03|Recebimento e divergências|existente_exige_aceite|Ensaiar estorno do recebimento com histórico preservado e sem fingir devolução de dinheiro.|
|EST-01|Estoque|existente_exige_aceite|Validar físico/reservado/livre por produto e local, incluindo reservas produtivas.|
|EST-02|Estoque|existente_exige_aceite|Ensaiar entrada, transferência, perda e recuperação dos recibos com unidade esperada.|
|EST-03|Estoque|existente_exige_aceite|Revisar paginação, exportação de página versus total e desempenho em saldos grandes.|
|CAT-01|Catálogo e fornecedores|existente_exige_aceite|Homologar cadastro, unidade, preço/custo, códigos e alterações já existentes contra a API efetiva.|
|CAT-02|Catálogo e fornecedores|existente_exige_aceite|Confirmar precisão de quantidades, centavos e relatórios nos limites seguros.|
|CAT-03|Catálogo e fornecedores|existente_exige_aceite|Testar scanner físico e importação com erros parciais, sem substituir dados em silêncio.|
|PRO-01|Produção e receitas|existente_exige_aceite|Mapear cada comando visual aos contratos de receitas versionadas, capacidade, ordens e materiais existentes.|
|PRO-02|Produção e receitas|existente_exige_aceite|Demonstrar reserva→consumo→etapas→inspeção→resultado/perdas em uma ordem real descartável.|
|PRO-03|Produção e receitas|existente_exige_aceite|Expor ingrediente limitante, arredondamento, responsável e histórico sem comprometer o mesmo saldo em alternativas.|
|LOT-01|Lotes e qualidade|existente_exige_aceite|Identificar os controles visuais ainda ausentes para rastreio, validade, bloqueio e inspeção.|
|LOT-02|Lotes e qualidade|existente_exige_aceite|Vincular origem de ingredientes e lote produzido com autorização e histórico.|
|LOT-03|Lotes e qualidade|existente_exige_aceite|Devolução do cliente não reconstrói lote que a venda original não registrou; definir política antes de automatizar reposição por lote.|
|CUS-01|Custos e sugestões|existente_exige_aceite|Conferir quais cenários de custo já têm adaptadores visuais e adicionar somente os faltantes.|
|CUS-02|Custos e sugestões|existente_exige_aceite|Explicar preço histórico, unidade, perdas e critério da sugestão antes da aprovação humana.|
|CUS-03|Custos e sugestões|existente_exige_aceite|Aceitar arredondamentos e concorrência sem reservar ou produzir por simples sugestão.|
|DOC-01|Agenda de docas e logística|existente_exige_aceite|Aceitar reserva, chegada, início, conclusão e inspeção com permissões atuais.|
|DOC-02|Agenda de docas e logística|existente_exige_aceite|Ligar ocupação, indicadores e conflitos à agenda efetiva, evitando sobreposição de capacidade.|
|DOC-03|Agenda de docas e logística|existente_exige_aceite|Separar conclusão de descarga, recebimento aprovado e obrigação financeira.|
|MAN-01|Manutenção e frota|existente_exige_aceite|Inventariar ações de bens, chamados, prestadores, orçamentos, oficina, fila e registros existentes.|
|MAN-02|Manutenção e frota|existente_exige_aceite|Concluir os comandos ainda ausentes e demonstrar revisão independente e recuperação.|
|MAN-03|Manutenção e frota|existente_exige_aceite|Viagens, despesas, bloqueios produtivos e planos exigem aceite individual dos contratos suportados.|
|RH-01|RH, ponto e tarefas|existente_exige_aceite|Separar previsões ou prévias das operações persistidas de ponto e tarefas.|
|RH-02|RH, ponto e tarefas|existente_exige_aceite|Aceitar perfis de funcionário/gerente, privacidade de avaliações e histórico.|
|RH-03|RH, ponto e tarefas|existente_exige_aceite|Demonstrar operação de smartphone e recuperação offline antes de anunciar ponto móvel funcional.|
|FIS-01|Fiscal|bloqueio_backend_provedor|Definir provedor/SEFAZ e contratos de emissão, rejeição, contingência e cancelamento.|
|FIS-02|Fiscal|bloqueio_backend_provedor|Conectar interface somente a estados fiscais comprovados; venda não equivale a autorização fiscal.|
|FIS-03|Fiscal|bloqueio_backend_provedor|Homologar certificados, ambiente e cenários legais com responsável fiscal.|
|CON-01|Contador e exportações|existente_exige_aceite|Homologar exportação operacional, período, escopo e valores exatos com o contador.|
|CON-02|Contador e exportações|existente_exige_aceite|Separar livro de caixa de escrituração legal e de liquidação bancária.|
|CON-03|Contador e exportações|existente_exige_aceite|Conferir devoluções65 nas exportações já existentes sem duplicar cancelamentos.|
|ANA-01|Painéis e indicadores|existente_exige_aceite|Aceitar origem, completude e período dos indicadores e estados sem dados.|
|ANA-02|Painéis e indicadores|existente_exige_aceite|Distinguir previsto, registrado, confirmado externamente e parcial.|
|ANA-03|Painéis e indicadores|existente_exige_aceite|Ensaiar consultas grandes, filtros e exportação com dados sintéticos representativos.|
|PLA-01|Planilhas|existente_exige_aceite|Homologar fórmulas, filtros, validação, importação e exportação nos cinco idiomas.|
|PLA-02|Planilhas|existente_exige_aceite|Medir abertura, memória e edição em hardware alvo; o chunk grande continua visível no build.|
|PLA-03|Planilhas|existente_exige_aceite|Evitar exposição de macros/fórmulas e preservar identificação como texto nas importações financeiras.|
|OCR-01|OCR, impressão e câmera|bloqueio_hardware|Validar scanner e câmera físicos com permissão e fallback.|
|OCR-02|OCR, impressão e câmera|bloqueio_hardware|Homologar impressora térmica58/80mm, spool, papel e reimpressão sem repetir venda.|
|OCR-03|OCR, impressão e câmera|bloqueio_hardware|Confirmar revisão humana do OCR antes de vincular comprovante ou valor.|
|ACE-01|Acesso, cadastro e sessão|existente_exige_aceite|Executar aceite de papéis e isolamento empresa/loja/aparelho/usuário nos módulos integrados.|
|ACE-02|Acesso, cadastro e sessão|existente_exige_aceite|Concluir cadastro/recuperação/perfil conforme contratos reais; não alterar autenticação neste pacote.|
|ACE-03|Acesso, cadastro e sessão|existente_exige_aceite|Revalidar contexto em operações tardias e preservar filas da identidade original.|
|LIC-01|Licenciamento e atividade|existente_exige_aceite|Aceitar concessão/revogação de módulos e políticas comerciais com backend de licenças autorizado.|
|LIC-02|Licenciamento e atividade|existente_exige_aceite|Garantir que consulta histórica permaneça separada de autorização para novas mutações.|
|LIC-03|Licenciamento e atividade|existente_exige_aceite|Revisar seleção de atividade e capacidades sem repetir questionário em cada módulo.|
|DES-01|Desktop Linux e Windows|existente_exige_aceite|Compilar explicitamente novo estágio e promover somente após revisão; iniciar não deve recompilar automaticamente.|
|DES-02|Desktop Linux e Windows|existente_exige_aceite|Homologar instaladorWindows, permissões, assinatura, atualização e rollback em máquina limpa.|
|DES-03|Desktop Linux e Windows|existente_exige_aceite|Reproduzir lentidão no caminho efetivo do proprietário; medições isoladas não explicam por si180segundos.|
|MOB-01|Smartphone e vários aparelhos|bloqueio_backend_aplicativo|Implementar venda, persistência própria e recuperação no smartphone com PC desligado.|
|MOB-02|Smartphone e vários aparelhos|bloqueio_backend_aplicativo|Definir autoridade de estoque, conflitos e reconciliação comercial entre aparelhos.|
|MOB-03|Smartphone e vários aparelhos|bloqueio_backend_aplicativo|Aceitar reinstalação, restauração, pareamento e revogação em aparelhos físicos.|
|BKP-01|Backup e operação|existente_exige_aceite|Oferecer interface apenas para operações autorizadas de backup sem expor chaves.|
|BKP-02|Backup e operação|existente_exige_aceite|Demonstrar restauração e revisão de confiança entre empresas antes de novas trocas.|
|BKP-03|Backup e operação|existente_exige_aceite|Promover binários e serviços por decisão explícita; código ou binário instalado não comprova serviço atualizado.|
|IDI-01|Idiomas e acessibilidade|pendente_aceite_transversal|Revisão nativa de textos pt-BR/en/es/fr/zh-CN e terminologia financeira.|
|IDI-02|Idiomas e acessibilidade|pendente_aceite_transversal|Percorrer teclado/leitor de tela, foco, mensagens, contraste, zoom e390/768/1280 em cada fluxo crítico.|
|IDI-03|Idiomas e acessibilidade|pendente_aceite_transversal|Validar quantidades/unidades, datas/fuso, centavos e impressão em cada idioma.|
|CI-01|Contratos e integração contínua|pendente_integracao|Usar a suíte completa test:all; test:local histórico cobre somente uma parte dos testes.|
|CI-02|Contratos e integração contínua|pendente_integracao|Atualizar o snapshotfrontend pinado no CIbackend mediante coordenação, sem alterar a configuração neste pacote.|
|CI-03|Contratos e integração contínua|pendente_integracao|Exigir evidência porSHA e por fluxo; CI de launcher não homologa todo ofrontend.|
|SEG-01|Segurança e dados privados|pendente_aceite_transversal|Revisão externa de segurança, limites de transporte e operações críticas antes de implantação comercial.|
|SEG-02|Segurança e dados privados|pendente_aceite_transversal|Manter.env,senhas,estações,chaves,bancos e perfis fora dos pacotes e logs.|
|SEG-03|Segurança e dados privados|pendente_aceite_transversal|Definir retenção, auditoria, restauração e procedimentos operacionais com o responsável da empresa.|
