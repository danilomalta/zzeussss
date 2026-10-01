# Fase 9D — Receptor HTTP de mensagens cifradas

## Entrega e limites

`incoming.NewSealedReceiverHTTP` constrói um `net/http.Handler` reutilizável.
Não abre porta, não modifica a API Fiber de vendas, não monta rota no servidor
existente e não inicia envio automático. O executável atual continua com sua
configuração anterior. Esta etapa testa a fronteira HTTP antes da instalação de
um serviço de comunicação. Não é o sistema completo de sincronização.

Escopo mantido: dois aparelhos previamente aprovados da mesma empresa e loja.
Compartilhamento entre empresas, provisionamento real de aparelhos, instalação
de chave pública, serviço de rede, TLS e rotina de envio continuam pendentes.
Os registros recebidos não são aplicados automaticamente às tabelas de negócio.

## Contrato HTTP

- Caminho exato: `POST /sync/v1/events`, sem parâmetros de consulta.
- Tipo: `application/json`; conteúdo comprimido não é aceito.
- Corpo: envelope cifrado da 9B, com campos exatos, sem nomes duplicados,
  extras, nulos ou documentos JSON adicionais. A validação inclui o destino.
- Limite de corpo: 512 KiB, incluindo base64 do texto cifrado. Requisições com
  tamanho declarado ou transmissão de tamanho desconhecido têm o mesmo limite.
- Destino e chaves privadas vêm do processo de inicialização verificado. A
  requisição não pode escolher uma chave privada ou alterar o aparelho receptor.
- Autorização usa assinatura Ed25519 e concessão de recebimento por tipo de
  evento, verificadas no banco em cada operação; não usa token humano do PDV.
- Não existe entrada alternativa com mensagem em texto puro.

O construtor verifica a chave de assinatura contra o pareamento aprovado e
copia seu conteúdo para impedir alteração pelo chamador. A chave X25519 deve
ser carregada com o escopo validado usando as funções da 9C. O construtor não
substitui essa responsabilidade de inicialização.

## Respostas

- 201: mensagem nova e recibo gravados na mesma transação.
- 200: repetição idêntica, retornando o recibo anteriormente persistido.
- 400: entrada inválida; 403: comunicação não autorizada; 409: conflito.
- 404: caminho/consulta não aceitos; 405: método inválido; 415: formato inválido;
  413: corpo excessivo; 500: falha interna, sem divulgar detalhes do banco.

Sucesso retorna `receipt` (campos atuais de `outgoing.Receipt`, incluindo prova
assinada) e `repeated`. Não retorna conteúdo da venda nem chave privada.
Todas as respostas usam `Cache-Control: no-store`. Não há CORS para navegador.

Um recibo atesta somente recepção durável. A origem deve verificar sua assinatura
com a chave pública aprovada antes de confirmar a outbox. Status HTTP sozinho
não confirma nada. Se a resposta se perder após o commit, repetir o mesmo evento
devolve o recibo estável. Não gerar outro ID de operação para repetir um envio.

## Requisitos para um serviço posterior

Não publicar este handler diretamente na internet. A instalação do listener
precisa definir endereço autorizado, TLS/autenticação de transporte, timeouts de
leitura/escrita/cabeçalhos, limite de conexões e requisições, encerramento e logs
sem conteúdo sensível. O timeout de 10 segundos do contexto da operação não
substitui os timeouts de leitura do servidor. O corpo cifrado protege o conteúdo;
metadados de roteamento e disponibilidade exigem proteção de transporte.

## Verificação

Testes usam bancos/diretórios descartáveis. Um servidor `httptest` abre uma
porta temporária em loopback apenas durante o teste e fecha ao terminar.
Cobrem troca HTTP com recibo verificável e repetição com nova cifra; ausência
de autorização e revogação; JSON ambíguo/texto puro; limites de corpo, método e
tipo; falha de persistência sem recibo nem detalhe interno; conflito sem substituir
evento; e rejeição de chave privada de assinatura estranha ao aparelho.

Resultados Go devem ser observados no PC antes de declarar esta fase validada.
O teste de loopback não comprova funcionamento em rede entre dois computadores,
smartphone independente, nuvem ou comunicação comercial entre empresas.
