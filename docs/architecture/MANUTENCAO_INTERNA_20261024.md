# Manutenção interna — implementação de frontend e backend (24/10/2026)

## Responsabilidades e origem

Esta entrega não implementa nem substitui carrinhos suspensos, checkout, pagamentos ou os testes desses fluxos do outro chat. Frontend de referência a3b0f4d5fd011c2768eb12cda20141d27c015b0e, incluindo indicadores de docas; backend de referência857febb588b522fdac45d3773e1b772094b19816. Predecessor58 verificado:29871fbba6866aa5e34d518a532404b214a6b69f. Fontes lidos por integração GitHub e materializados em diretório isolado; nenhuma pasta do computador do usuário foi acessada daqui. Sem commit, push, serviço comercial ou alteração de Electron.

## O que existe nesta unidade

- Acesso principal direto em `/local/maintenance`, visível para quem possui `view_orders`; leitura histórica não depende de renovação de contrato. Há também link na frota.
- Não é preciso cadastrar veículo: máquinas, equipamentos e instalações usam o cadastro oficial compartilhado de bens. Uma instalação pode ter categoria `other`; o perfil de manutenção informa o tipo installation, setor e localização. O armazenamento chama-se fleet por reaproveitamento do contrato, e não limita os bens a veículos.
- Pedidos e execução: cadastro de bem, abertura de pedido por gerente/operador autorizado, urgência, bloqueio operacional, triagem, atribuição, orçamento, aprovação, execução, espera por peças, inspeção, liberação, cancelamento e histórico. Usa o fluxo oficial já existente, não converte rascunho em fato.
- Conferência técnica herdada e incluída no backend cumulativo: modelos versionados, checklist obrigatório por pedido, respostas de inspeção, apontamento de períodos de mão de obra e relatório. Liberação permanece do responsável autorizado segundo regras do servidor.
- Especialidades configuráveis (ex.: elétrica, mecânica, refrigeração, hidráulica), técnicos vinculados às pessoas já autorizadas, especialidades de cada técnico, contato interno, perfil setorial dos bens e classificação técnica dos pedidos.
- Fila por especialidade ou técnico, estado atual do pedido, setor, revisão, total completo do filtro, emergências e não atribuídos; paginação independente da soma. Sem classificação continua visível na fila geral. Trabalho atribuído a um técnico permanece em sua consulta mesmo se a especialidade for desativada. Cadastro desativado não revoga login nem apaga serviço.
- Cadastro de técnico é diretório operacional: NÃO concede permissão, não certifica capacitação profissional e não autoriza atividade elétrica. A seleção por especialidade é organização consultiva; não há atribuição automática ou regra nova de certificação técnica.
- Catálogo em cinco idiomas: português, inglês, espanhol, francês e alemão; tema herdado, foco, labels e tabela com rolagem interna. Formulários especializados são uma primeira interface operacional, incluindo referências por ID mostradas nas tabelas oficiais, não uma reformulação visual completa.
- Revisão humana e confirmação antes do POST. Comandos próprios preservados no navegador antes do envio; consulta de recibo primeiro, mesmos IDs no reenvio explícito, proteção de sessão/empresa/loja/usuário/aparelho, bloqueio entre abas e proibição de descarte de resultado incerto. Sem armazenamento disponível ou Web Locks, a escrita fica indisponível.

## Pedido e comunicação

O gerente ou operador abre pedido na área Pedidos e execução, inclusive pela mesma interface responsiva no smartphone quando sessão e rede autorizadas existirem. O pedido é persistido pelo servidor; o monitor do técnico consulta a fila técnica. Avisos oficiais duráveis já existentes em `/fleet/notices` podem ser consultados e confirmados pelo cliente. Não há neste ZIP entrega push, som no sistema operacional, app iOS/Android nativo, serviço remoto configurado, DNS, sincronização P2P ou prova de que o telefone tocou. Essas funções precisam implementação e testes próprios.

Criar pedido e classificá-lo são DUAS operações explícitas. Se a classificação falhar, o pedido continua salvo, sem classificar. Nunca anunciar atomicidade entre elas. Atribuição continua por ação oficial, e classificação não equivale a aceite de um técnico.

## Backend novo

23 operações em `/local/v1/maintenance`: quatro recursos (specialties, technicians, assets, classifications), cada qual com lista, detalhe, histórico, criação e ações; recibo por ID, candidatos autorizados e fila. Novo SQL aditivo: `internal_maintenance.sql`, na posição61 desta base; o aplicador escolhe o próximo número livre real. Tabelas STRICT, chaves compostas por empresa/loja, referências a bens/pedidos/pessoas, controle de revisão, histórico e outbox na mesma transação. Registro operacional, recibo e outbox devem afetar exatamente uma linha; falha ou trigger que ignore escrita causa rollback.

Idempotência do cadastro tem namespace próprio `/maintenance/operations`, vinculado a ator/aparelho. Não confundir com `/fleet/operations` nem com venda. Mesmo ID e corpo canônico retorna fotografia anterior; corpo alterado ou revisão concorrente retorna conflito. Escrita exige contrato Logistics/dependências e permissões atuais. Cadastros exigem owner/manager; classificação inicial também pode ser do solicitante/responsável autorizado. A interface de diretório oferece gravação gerencial; abertura de pedido permanece disponível ao operador autorizado.

Candidatos são nomes/IDs/papéis de membros ativos autorizados na loja; consulta gerencial, com limite explícito1000 membros na empresa. Não há folha ou dados bancários. Fila tem limite explícito5000 pedidos de origem; ultrapassar gera422, sem somar parcialmente e declarar total. Limites, paginação e autorização estão no contrato `docs/api/internal-maintenance.openapi.json`. Nomes únicos de especialidades usam NOCASE do SQLite (ASCII), não equivalência linguística completa de Unicode.

## Efeitos que continuam separados

Orçamento não é pagamento. Custo gerencial/tempo declarado não é folha, lucro líquido ou lançamento bancário. Espera por peça não reserva ou consome almoxarifado. Uma inspeção registrada não prova presença física ou habilitação do trabalhador. Nenhuma liberação decorre de timeout.

## Pacote e convivência com o outro chat

Backend cumulativo inclui os fontes oficiais de Frota e Conferência Técnica adaptados à base58, além da nova unidade. Aceita base58 preservada, base60 compatível ou sequência posterior com migrações adicionais intactas. Não falsifica marcador de pacote antigo. Confere hashes das SQL1..58, contratos de SQL de frota quando existentes e conflitos em cada arquivo/trecho. Não reescreve migração histórica. Migração do outro chat recebe preservação de nome/bytes; novas SQL seguem ao fim.

Frontend aplica pequenos trechos no router/menu/painel/link da frota e arquivos próprios. Arquivo de router final incluído é somente para revisão: não copiá-lo sobre a árvore local. Em conflito, o aplicador para com caminho. Sem escrita forçada. Os dois projetos são independentes; usar --frontend na raiz que contém frontend-web e --backend na raiz que contém backend. Nunca aplicar um na pasta do outro por suposição.

Validação ocorre em cópias de fontes descartáveis: testes, TypeScript/build frontend; Go test/vet/build backend. O pacote não inclui token, senha, estação, chave privada, banco comercial, node_modules ou executável pré-aprovado. O build usa envFile:false e não lê .env. Arquivos .go/sql e fixtures de testes são copiados; dados privados fora dessas áreas não entram na candidata. Na aplicação, hashes são rechecados e backup privado guarda apenas arquivos alterados. Atualização SQL só acontece na inicialização futura autorizada do serviço. Não iniciar automaticamente.

## Validação ainda necessária no destino

- Revisão visual completa e tarefas reais com gerente/operador/técnico, desktop e smartphone.
- Login legítimo e combinação frontend/backend efetivamente em execução, em banco descartável autorizado.
- Rede entre aparelhos, TLS, permissões reais, atualização do binário em uso; copiar fontes não atualiza um serviço já iniciado.
- Custódia/qualificação técnica definida pela empresa, integrações com almoxarifado, compras e financeiro, notificações nativas e telemetria.
- Compatibilidade com as mudanças inéditas de carrinhos do outro chat: preservação por patches/migrações é testada, mas o código ainda não publicado desse chat não foi inspecionado.

Isso conclui esta unidade de implementação, não o ERP inteiro, o aplicativo móvel ou a homologação operacional.
