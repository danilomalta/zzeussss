- Não escreva relatório nem reavalie a maturidade do projeto. Resposta
  final com no máximo 10 linhas.
- Não reimplemente assinatura XML, XSD, mTLS, numeração, recuperação nem
  cancelamento. Reaproveite fiscalproduction, fiscalcore e localdb/fiscal.
- Não leia o repositório inteiro. Use grep para achar só os arquivos
  necessários.
- Prefira biblioteca padrão do Go. Se precisar de dependência nova, pare e
  pergunte.
- Sempre use go test -timeout 120s -count=1 e rode só os pacotes que
  alterou. Em ./internal/localapi/ use sempre -run; o pacote inteiro é
  lento.
- NUNCA remova ou enfraqueça o bloqueio de emissão em produção.
- Segredos (certificado, CSC, token) nunca em texto puro, em log ou em
  resposta HTTP.
- Se o mesmo erro persistir após 2 tentativas, pare e mostre o erro.
- Ao terminar: commit, push e responder só com branch, hash e resultado
  resumido dos testes.

## Decisões do produto

1. O sistema é ALUGADO a clientes (provedor de ERP). O provedor NÃO tem acesso aos dados nem às chaves dos clientes. Sem backdoor, sem conta mestra do provedor, sem cópia de chave ou segredo no provedor, sem telemetria com dado fiscal ou de cliente.
2. Cada cliente escolhe onde o sistema roda e onde os dados ficam: servidor local, servidor próprio ou nuvem. O código nunca presume o destino.
3. Cada cliente tem as PRÓPRIAS chaves (chave mestra do cofre, tokens, certificados), geradas na instalação dele, sob controle dele. O cliente é responsável pelo backup. Chaves de um cliente nunca abrem dados de outro.
4. Existem DOIS cadastros: (a) PRÉ-CADASTRO, o mínimo para a pessoa começar; (b) CADASTRO COMPLETO, dentro do sistema, onde se registram meios fiscais, funcionários e o resto. Os dados podem ser puxados de uma fonte automática OU digitados à mão; a digitação manual sempre funciona.
5. Produção fiscal continua BLOQUEADA. Nada é simulado como oficial.
