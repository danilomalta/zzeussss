# Fase 9I — Executável receptor com TLS e encerramento

## Entrega

Novo executável `cmd/titan-receive`, subcomando `serve`. Carrega a instalação
existente, prova o aparelho por desafio, valida a chave privada X25519 da 9C
e inicia o handler HTTP da 9D. Não altera o começo do PDV, não inicia o remetente
e não instala um serviço persistente no sistema operacional.

Compartilhamento permanece restrito à mesma empresa e loja, com aparelhos
pareados e concessões de eventos aprovadas no banco. Não aplica mensagem ao
domínio de estoque/financeiro e não oferece comunicação comercial entre empresas.

## Argumentos e configuração

Argumentos operacionais: `serve`, `--db`, `--station`, `--encryption` e opcional
`--listen`, `--tls-cert`, `--tls-key`. Caminhos devem identificar arquivos reais
previamente preparados. Não fornecer um comando de inicialização com IDs ou
arquivos inventados e não executar contra banco real nos testes desta etapa.

Padrão de escuta: `127.0.0.1:8282`. Endereço deve ter IP literal e porta válida.
Wildcard `0.0.0.0`, host vazio e nomes DNS são recusados. HTTP sem TLS somente
em loopback. Qualquer outro IP explícito exige certificado PEM e chave TLS PEM.
Porta 0 é aceita para testes/diagnósticos, com endereço real mostrado na saída;
produção deve usar uma porta definida.

O certificado é carregado antes da abertura do banco. A chave TLS deve ser
arquivo regular 0600, sem symlink. TLS mínimo 1.2. Certificado deve corresponder
ao nome/IP usado pelo cliente e ser confiável para ele. O teste configura uma
CA de httptest no cliente de teste, sem desligar verificação de certificado.
Não instalar certificados ou chaves de teste na infraestrutura real.

A chave TLS não substitui a chave privada Ed25519 do aparelho nem a X25519:
TLS protege o transporte; Ed25519 autentica mensagem/recibo; X25519 participa
da criptografia do conteúdo. Não copiar uma mesma chave privada entre aparelhos.

## Arquivos e prova

O pacote novo `stationfile` reutiliza o formato da instalação existente e fornece
uma leitura verificada para o receptor. Rejeita arquivos de aparelho não regulares,
symlink, permissão diferente de 0600, tamanho excessivo, JSON ambíguo e chave
inconsistente. Banco deve já existir e ser regular 0600; não cria banco por engano.
Sua abertura operacional aplica migrações numeradas e grava/consome desafio.
Preparar backup e revisão das migrações antes do uso real.

O comando remetente existente mantém seu leitor anterior nesta fase; uma
consolidação dos leitores pode ser feita posteriormente sem reescrever o fluxo.
Não modifica o arquivo station nem cria ou regenera chaves. X25519 ausente,
insegura ou de outro aparelho é erro, sem alternativa em texto puro.

## Limites do servidor

- Cabeçalhos: timeout de 5 segundos e limite configurado de 8 KiB.
- Leitura/escrita: 15 segundos; conexões ociosas: 1 minuto.
- Handler: limite de corpo da 9D e timeout de operação de banco já existente.
- Admissão global: até 8 requisições ativas e 20 inícios por segundo.
- Excesso: 503 por ocupação ou 429 com Retry-After por taxa; sem recibo falso.
- Ctrl+C/SIGTERM: parar aceitações, aguardar até 5 segundos e fechar se necessário.

São limites básicos de requisições, não um limite global de conexões TCP nem
proteção completa contra DoS. Para uma implantação acessível publicamente,
planejar firewall/proxy, limitação de conexões e controles de rede. Este patch
não publica serviço na internet, abre firewall nem instala túnel.

Saída inicial mostra somente o endereço realmente ligado. Erros do comando são
genéricos. Log padrão do net/http é desativado; telemetria estruturada está
pendente. Não declarar serviço saudável ou mensagem aplicada por estar escutando.
Se uma resposta se perder no encerramento, o remetente deve repetir o evento
existente para recuperar o recibo durável, sem criar outra operação.

## Verificação

Testes usam apenas instalações/diretórios descartáveis e portas temporárias:
aparelho provado, recepção real por CLI, recibo verificado, encerramento;
TLS carregado de PEM com cliente verificando certificado; escuta insegura e
chave TLS com permissão inadequada recusadas; flags inválidas sem criar banco;
limite de inícios; prova/revogação e arquivo privado verificado no pacote novo.

Sem migração nova. Compilar `titan-local`, `titan-sync` e `titan-receive`; observar
testes e vet no PC. Concluir isso não comprova provisionamento de dois computadores,
rede LAN real, failover do smartphone ou compatibilidade de ACLs multiplataforma.
