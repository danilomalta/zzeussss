# Fase 9G — Worker com espera progressiva persistente

## Entrega

`incoming.RunDeliveryWorker` chama a entrega 9F repetidamente, fora da interface
do PDV. Faz uma tentativa por vez, aguarda quando não há eventos e persiste a
espera progressiva após falhas. Encerramento por cancelamento interrompe timers
e requisição HTTP; a fila permanece no SQLite. A função é bloqueante e precisa
ser executada pelo processo em goroutine dedicada.

O worker NÃO é iniciado pelo executável atual nesta fase. Instalação, listener,
TLS, configuração dos aparelhos e inicialização no comando ainda serão integrados.
Não confundir biblioteca testada com comunicação automática já instalada no PC.
Não há aplicação dos eventos recebidos ao domínio nem compartilhamento entre empresas.

## Configuração e autorização

`WorkerConfig` recebe banco, aparelho de origem, chave privada de assinatura,
destinatário aprovado, endpoint e cliente HTTP confiável. Não aceitar esses dados
livremente do navegador. Origem/destino continuam da mesma empresa e loja.
Chaves/pareamentos e aprovação X25519 são verificados no início e na rotina 9F.

Defaults: consulta em fila vazia a cada 2 segundos; pausa após falha começa em
1 segundo, dobra e limita a 1 minuto. Durações negativas/inconsistentes e valores
acima de uma hora são rejeitados. Configuração de produção deve manter pausas
adequadas; valores pequenos usados nos testes não são política de produção.

HTTPS obrigatório fora de loopback. Endpoint sem credenciais, consulta ou
fragmento. O cliente da 9E continua recusando redirect e TLS sem verificação.
Erros locais de confiança/configuração/conflito encerram o worker e precisam de
intervenção. Erros de transporte são adiados; rejeição HTTP remota é reportada
pelo transporte como falha e também recebe pausa, até correção do destino.

## Estado durável

Migração 0019 cria `sync_worker_state`, com origem, destino, ID do evento atual,
contagem limitada de falhas e próximo instante permitido em milissegundos UTC.
Não guarda conteúdo de venda, senha, token ou chave privada. As referências de
aparelhos incluem empresa e loja. Estado e contador são gravados juntos.

Reinício respeita a espera pendente para o mesmo evento. Ao mudar o evento,
reinicia sua contagem. Sucesso limpa o estado correspondente; fila vazia também
limpa estado antigo. Falha ao persistir a pausa encerra o worker em vez de entrar
em repetição acelerada. O contador de tentativas da outbox permanece separado.

A fonte atual tem UM destinatário autoritativo. A tabela não permite trocar
silenciosamente esse destino na reinicialização. Troca de destino exige um fluxo
administrativo futuro; não excluir a tabela manualmente para contornar o erro.
Usar um worker por origem. Não há eleição de líder ou lease entre processos:
as entregas concorrentes continuam idempotentes, mas esta fase não oferece
coordenação de múltiplos workers nem distribuição para vários destinatários.

## Tempo e observação

Timers são canceláveis e nenhuma transação SQLite fica aberta durante espera
ou HTTP. Mudança para trás no relógio pode prolongar a espera; a checagem é
repetida em intervalos limitados ao RetryMax. Não é um relógio confiável contra
adulteração de administrador. Avanço do relógio pode antecipar tentativa; não
autoriza operação de negócio nem ignora a validação das chaves.

Callback opcional `Observe` recebe apenas `idle`, `waiting`, `deferred` ou
`delivered`, ID do evento e dados de pausa. É síncrono no worker e deve retornar
rapidamente. A UI deve consumir notificações por canal/fila, sem bloquear o
worker. O callback não recebe conteúdo, segredo ou erro bruto. Não exibir status
de mensagem entregue como estoque central atualizado ou confirmação fiscal.

## Testes e aceite

Bancos descartáveis e servidores de loopback: falhas remotas seguidas de entrega
automática; pausa crescente e limitada; limpeza do estado; reinício com pausa
persistida; cancelamento em fila vazia; configuração/confiança inválida sem envio;
falha de persistência de pausa preservando evento e tentativa.

Executar migrações apenas nos bancos descartáveis dos testes nesta aplicação do
patch. Não iniciar o worker nem abrir o banco real por esses comandos. Observar
os testes Go, vet e compilação antes de considerar esta fase validada.
