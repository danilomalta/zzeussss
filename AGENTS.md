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
