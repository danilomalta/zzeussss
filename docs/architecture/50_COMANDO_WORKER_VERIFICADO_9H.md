# Fase 9H — Executável separado para o worker verificado

## Entrega

Novo comando `cmd/titan-sync`, separado de `titan-local` e da API central.
O subcomando `send` executa o worker da 9G com um SQLite EXISTENTE, arquivo de
aparelho EXISTENTE e arquivo de configuração de destino. `--once` executa uma
tentativa da 9F, sem manter worker contínuo. Não modifica o início do PDV,
não instala serviço systemd e não inicia comunicação ao compilar/testar.

Não oferece criação de empresa/aparelho, aprovação de chave ou servidor receptor.
Esses dados precisam ter sido provisionados e aprovados antes do uso. O teste
semeia duas instalações descartáveis explicitamente; isso não é um procedimento
de provisionamento para clientes reais. Listener e configuração completa dos
aparelhos em produção continuam pendentes.

## Entrada

Argumentos: `send --db CAMINHO --station CAMINHO --config CAMINHO [--once]`.
Os nomes CAMINHO são descrição, não comandos prontos para executar. Na revisão
desta fase, executar somente os testes e compilações fornecidos, sem abrir banco
real. Uma orientação operacional posterior precisará dos caminhos/IDs reais e
das aprovações observadas; não inventar instalações para testar em produção.

Configuração JSON, versão 1, com EXATAMENTE os campos:

- `version`: inteiro 1.
- `destination_device_id`: ID público do aparelho aprovado de destino.
- `endpoint`: URL exata do receptor, terminando em `/sync/v1/events`.

Empresa e loja não são escolhidas pela configuração: vêm do aparelho provado.
HTTP só com IP literal de loopback; na rede, HTTPS e certificado confiável para
o sistema operacional. Credenciais/consulta/fragmento na URL são recusados.
O comando não permite desativar verificação TLS nem receber chave do servidor
como substituta da chave pública previamente aprovada no SQLite.

Arquivo station usa os quatro campos existentes de titan-local: `tenant_id`,
`store_id`, `device_id` e `private_key` (Ed25519 em base64 URL sem padding).
Não exibir ou copiar esse arquivo. O worker remetente não precisa carregar a
chave privada X25519 do destinatário; só usa sua chave pública aprovada.

## Inicialização

Configuração e estação são arquivos regulares 0600, sem symlink, até 16 KiB.
Leitura verifica identidade do arquivo aberto, JSON com campos exatos e chave
Ed25519 consistente. Banco inexistente ou não regular/0600 é recusado antes de
`localdb.Open`; o comando não cria banco novo por erro de caminho.

Em execução operacional autorizada, `localdb.Open` aplica as migrações numeradas
normalmente ao banco existente. Planejar backup e migração antes de uso real.
O comando emite e consome um desafio de aparelho no banco, provando posse da
chave privada e pareamento aprovado. Essa prova não autentica funcionário nem
permite criar operação de negócio; só transporta eventos já concluídos.

Após prova, 9F/9G verificam aprovação X25519, pareamentos e dono aprovador antes
de enviar e confirmar. Destino igual à origem é recusado. Um estado de worker
já associado a outro destino exige revisão administrativa, sem troca silenciosa.

## Operação e saída

Sem `--once`, permanece em execução até Ctrl+C/SIGTERM ou erro local impeditivo.
Cancelamento encerra timers/HTTP e fecha o banco. Com `--once`, informa somente
fila vazia ou recepção durável confirmada depois do commit. Não relata estoque
replicado, nota emitida ou pagamento confirmado. Erro do executável usa mensagem
genérica; não imprime chave, conteúdo, URL ou detalhe interno do banco.

A mensagem inicial de worker informa início da tentativa de execução, não
conexão saudável ou sincronização concluída. Telemetria estruturada detalhada
e gestão como serviço ainda serão integradas. Revisão/Linux atual não comprova
ACL/empacotamento e funcionamento em Windows, macOS ou smartphone.

## Verificação

Testes descartáveis: comando `--once` com prova real de aparelho, envio HTTP,
recibo e confirmação da outbox; segunda execução em fila vazia; revogação;
ausência de segredos na saída; configuração malformada sem criar banco; arquivos
inseguros/symlink/chave inválida; comandos e flags desconhecidos.
Os testes do worker contínuo da 9G permanecem parte da suíte.

Sem migração nova. Compilar ambos `titan-local` e `titan-sync`, executar testes e
vet no PC e observar os resultados antes de considerar a fase validada.
