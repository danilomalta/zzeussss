# Frontend produção69 — 28/10/2026

## Problema e comportamento final
A tela de produção lia sua fila ao montar e depois das próprias ações. Uma operação enviada ou recuperada em outra aba não atualizava esse aviso visual. A trava transacional do cliente continuava protegendo o envio, mas a segunda aba mostrava um estado desatualizado. O teste de navegador reproduziu a ausência do aviso antes da correção e passou com ela.

O observador novo relê somente a fila do próprio contexto em eventos storage,focus,pageshow e retorno da visibilidade. Ignora chaves de outras identidades, não confia em newValue, não grava dados, não remove fila e não chama API. Corrupção ou contexto expirado mantêm o bloqueio de escrita. A desinscrição remove os listeners. Não existe reenvio automático, polling de servidor ou confirmação fictícia.

O diálogo de revisão recebeu nome acessível ligado ao seu título. Ao abrir outra revisão, mensagens de sucesso/erro de uma ação anterior são retiradas; falhas da revisão atual continuam visíveis. Escape cancela a revisão ainda não enviada preservando os campos; durante o envio o fechamento continua impedido.

## Aceite integrado realizado
Chromium133 real em modo headless, Playwright1.51.1, componentes React reais Recipes/ProductionCommands/ProductionExecution/ProductionAftermath e clientes de produção existentes. API Go/Fiber real com SQLite descartável. As924fontes do backend de referência foram comparadas por ID de blob Git com a5bbcdf3420ca4fa6c471fcdd52794f86f84c0dc: nenhuma divergência. O testeGo adicionado existe só nas evidências do pacote, não no patch de backend.

Todas as mutações abaixo foram feitas pelos controles visuais e seus diálogos:
1.Consultar receita e capacidade por local; publicar versão2 preservando versão1.
2.Cancelar uma revisão por Escape sem POST; conservar campos e então confirmar explicitamente.
3.Criar ordem da versão original com3bateladas, local e responsável; aprovar.
4.Configurar duas etapas; reservar, liberar, reservar novamente e consumir.
5.Completar Misturar e Assar em sequência. Resultado indisponível antes da conclusão.
6.Registrar27unidades boas de30planejadas. O proxy de teste recebeu201do backend e descartou a resposta depois do commit. Não descartou nem alterou a transação.
7.Constatar pendência automaticamente na segunda aba; impedir Escape durante envio; recarregar o primeiro navegador, consultar o recibo original e limpar o aviso da segunda aba sem segundoPOSTde resultado.
8.Classificar3unidades de perda, declarar lote27comdatas e registrar parecer humano de qualidade.
9.Conferir ausência de overflow global em390,768,1280px.

O testeGo conferiu diretamente:1resultado;produto bread27000milli;flour2500000milli;oil200002milli no local production-room. Isso verifica a conciliação de consumo e entrada e que classificação de perda/lote não movimentou estoque outra vez. Lotes/qualidade são as declarações suportadas pelo contrato atual, sem inferir rastreio de lote de ingrediente que a origem não registrou.

Resultado: fluxo passou10.02s;com detector de corrida passou12.22s (processoGo13.234s). Dez testes novos do observador passaram. Suíte total2485frontend,57desktop,TypeScript sem erro. Build desktop268arquivos,27.70s. O bundle grande de planilhas continua avisado.

## Entrega e instalação
Base informada do PC:3d5abc9755e01365134d2bd571d3867ffad37622 na branch feat/backend-checkout-authority,68jáaplicada e ainda não registrada. Esta entrega adiciona2módulos de observação,1teste e3documentos; modifica apenas ProductionCommands.tsx. Não altera autenticação,sessões,backend,migrações,bancos ou serviços.

O pacote contém os fontes consolidados como referência. Somente manifest.changes é aplicado, após gitapply--check e testes em candidata isolada dos fontes atuais do PC. Não copiar fontes/ sobre o projeto. Carrinhos e a continuidade compartilhada não são substituídos. O commit seleciona explicitamente os caminhos conhecidos de68e69, evitando um novo prompt só para registrar a68; se68jáestiver registrada, seus caminhos não geram nova alteração.

O atualizador compila um estágio novo preservando os anteriores e cria atalho novo. Não encerra instâncias antigas nem tenta confirmar operações. Sandbox Electron permanece ativo; as opções do Chromium isolado de CI existem somente no harness de teste e não são transmitidas ao desktop.

## Limites precisos
Este teste monta os componentes reais em uma página isolada autorizada pelo fixture; não opera o shell/login completo do desktop. Não usou conta,estação,banco ou fornecedor comercial. Não reiniciou o servidor, não restaurou arquivo de backup, não homologou Windows,leitor de tela,periféricos,SEFAZ,Pix/TEF ou venda móvel independente. Os96itens do inventário permanecem rastreados sem transformar pendência em aprovação. Esta é uma entrega de código e evidência do fluxo suportado, não uma declaração de ERP comercial inteiramente concluído.
