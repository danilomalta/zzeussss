# Piloto local: venda → caixa → estoque → reposição → recebimento → financeiro

25/10/2026. Unidade de aceite do backend sobre `beaebbe6d85af07bd17bb5b02b11753df4f9ef96`, branch `feat/backend-b02a-compat`, núcleo SQLite/schema63. Frontend de referência: branch `feat/backend-checkout-authority`, commit `21ba19714b4f0d1d6f242530e44bbeb58364cb71`. O frontend permanece pausado. Nenhuma API nova, migração ou alteração das regras comerciais foi necessária para conectar este recorte: a entrega acrescenta o teste de aceitação integrado e este procedimento, usando os handlers de produção existentes.

## Recorte comercial

Uma empresa/loja/aparelho, venda e pagamento do fornecedor exclusivamente em dinheiro. Fornecedor externo ao Titan, com confirmação obtida por telefone/outro canal e registrada manualmente pelo responsável. A compra depende de aprovação humana. O teste não envia mensagem ou pedido à empresa fornecedora e não transforma registro manual em confirmação autenticada entre empresas. Cada pedido contém um produto neste contrato. Não inclui fiscal, cartão, Pix, troca/devolução ou sincronização entre aparelhos.

## Critérios e prova repetível

O teste `backend/internal/localapi/pilot_cash_acceptance_test.go`, função `TestPilotCashSaleToSupplierFinanceRestartAndRestore`, realiza o circuito por HTTP/TCP de loopback com porta atribuída pelo sistema, usando `NewWithVerifier`, os serviços transacionais e SQLite real. Não usa servidor falso, substituição dos handlers ou `app.Test` para as operações do circuito. Somente o provisionamento da instalação, pessoas de teste e emissão da licença são fixtures descartáveis.

1. Saldo inicial explícito de 5 unidades e fundo de R$10,00; salva e assume o carrinho no servidor.
2. Vende 2 unidades a R$15,99: total R$31,98, recebido R$40,00, troco R$8,02. Uma venda e uma parcela efetiva em dinheiro, sem autorização fiscal fictícia.
3. Encerra abruptamente o processo da API depois do commit. Inicia outro processo no mesmo banco e consulta o recibo original, sem novo POST de venda. Saldo: 3 unidades.
4. Política mínimo4/alvo5 calcula sugestão de 2 unidades a partir desse saldo persistido. Uma aprovação humana separada habilita a criação do pedido.
5. Registra fornecedor externo, pedido e confirmação manual; autoriza recebimento, planeja viagem identificada, registra carga e chegada. Recebe e aceita 2 unidades, gerando uma única entrada e retornando o saldo a5.
6. Gera obrigação de R$20,50 vinculada ao recibo de recebimento. O próprio criador é impedido de aprovar; outro usuário gerente aprova.
7. Registra uma retirada de R$20,50 e a vincula à parcela, sem movimentar dinheiro novamente. Saldo documental: zero. Fecha caixa: 1000+3198−2050=2148centavos; diferença zero. Não soma os R$40,00 como receita.
8. Compara por HTTP dezesseis registros/rastreios/recibos, incluindo política, sugestão, aprovação e fornecedor antes e depois de outro reinício do processo.
9. Cria backup cifrado consistente com a API ativa, verifica sua integridade, recusa chave errada, restaura em caminho novo e recusa sobrescrever o destino existente.
10. Inicia uma quarta API sobre o banco restaurado. A sessão anterior deve receber401; novo login autorizado recupera os mesmos registros, valores e identificadores. Repetições explícitas com os IDs originais não duplicam venda, estoque ou pagamento. Consulta também as quantidades de linhas e somas oficiais no banco descartável.

O teste normal faz101requisições HTTP. A conferência manual adicional de telas usa outras requisições, contabilizadas separadamente. A contagem não inclui o provisionamento inicial da fixture.

### Executar no terminal

Requer Go compatível com `backend/go.mod` (1.25 ou superior compatível) e dependências já disponíveis. Não instala nada, não usa `.env`, não depende de portas8181/8080 e não abre banco comercial.

```bash
cd "$HOME/Downloads/zzeus-frontend-backend-b02a/backend"
env -u TITAN_SESSION_TEST_DATABASE_URL -u TITAN_CATALOG_TEST_DATABASE_URL \
  -u TITAN_CASH_JOURNAL_BRIDGE -u TITAN_DELIVERY_BRIDGE -u TITAN_MAINTENANCE_BRIDGE \
  GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
  go test ./internal/localapi \
  -run '^TestPilotCashSaleToSupplierFinanceRestartAndRestore$' -count=1 -v
```

Neste Ubuntu foi usado o Go1.25.8 já existente em `~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.8.linux-amd64/bin/go`; o Go padrão1.24 não satisfaz `go.mod`. Substitua somente o executável `go` pelo caminho acima se continuar usando essa instalação. Uma dependência ausente faz o comando falhar com `GOPROXY=off`; não é autorização para instalar.

O teste filho `TestPilotCashProcess` é normalmente ignorado porque só o teste principal o inicia. São quatro processos independentes, não quatro instâncias concorrentes comerciais. Os bancos, chaves e arquivos de contexto são novos e temporários; não são entregues nem versionados.

Para repetição com detector de corridas, use `go test -race ./internal/localapi -run '^TestPilotCashSaleToSupplierFinanceRestartAndRestore$' -count=1`. A suíte existente `go test ./...` também inclui o teste principal. PostgreSQL comercial/integrações opcionais não são necessários para este circuito SQLite.

## Telas existentes e APIs exercitadas

- `/local/pos`: carrinho oficial `/local/v1/pos/carts`, ações e recibos; venda persistida em `/local/v1/sales/:id`; caixa `/local/v1/cash/open`, `/movements`, `/close`, `/sessions/:id/report`.
- `/local/stock`: saldo real `/local/v1/stock/balance` e movimentações `/local/v1/stock/operations`.
- `/local/orders`, aba Reposição: `/local/v1/replenishment/policies`, `/suggestions`, `/reviews`; pedido em `/local/v1/purchase-orders`.
- Detalhes do pedido: `/supplier-decisions`, `/authorize-receiving`, `/deliveries`; `/local/v1/purchase-deliveries/:id/actions`, `/receive`, `/payable`.
- `/local/payables`: obrigação, revisão independente e vínculo em espécie; `/local/v1/finance/payables/:id`, `/approve`, `/cash-payments`; origem `/local/v1/finance/receiving-origins/:id`.

Contratos existentes: `suspended-carts.openapi.json`, `cash-sales.openapi.json`, `cash-closed-report.openapi.json`, `purchase-supply-cycle.openapi.json`, `purchase-receipts.openapi.json`, `supplier-payables.openapi.json`, `payable-cash.openapi.json` e os demais contratos referenciados por essas rotas em `docs/api/`. Autoridade e contexto são derivados da sessão no servidor. Não ampliar permissões de operador em produção para reproduzir o teste.

Foi conferido no navegador o frontend compilado já existente, conectado por proxy de loopback à API Go sobre o banco restaurado, com identificação visível de piloto descartável. Login/contexto, histórico e comprovante da venda, pedido/recebimento e financeiro consultaram handlers reais. Nenhum mock respondeu a essas requisições. Essa inspeção das telas não é um teste automatizado de cliques de ponta a ponta: a automação cobre o circuito HTTP e a persistência. A etapa não edita fontes frontend, não retoma o relatório pausado de carrinhos e não altera a configuração pessoal do Electron.

## Backup operacional e restauração

O mecanismo exercitado é o mesmo de `cmd/titan-backup`: snapshot SQLite com `VACUUM INTO`, AES-GCM, verificação de integridade/FKs/histórico de migrações/contexto do aparelho. Não é cópia direta do arquivo SQLite aberto. Limite atual do snapshot:128MiB; uma base maior exige outra unidade antes de anunciar cobertura de backup para aquele tamanho.

Operação real requer caminhos explicitamente definidos pelo responsável. Estes são modelos de comandos, com variáveis que precisam apontar para a instalação correta; não foram executados contra dados do cliente:

```bash
# No terminal; binário titan-backup já compilado, sem instalar pacotes.
# CHAVE, BANCO, ESTACAO e ARQUIVO são caminhos explícitos da instalação.
titan-backup key-init --key "$CHAVE"                       # criar UMA vez
titan-backup create --db "$BANCO" --station "$ESTACAO" \
  --key "$CHAVE" --out "$ARQUIVO"
titan-backup verify --archive "$ARQUIVO" --key "$CHAVE" \
  --tenant "$EMPRESA" --store "$LOJA" --device "$APARELHO"
titan-backup restore --archive "$ARQUIVO" --key "$CHAVE" \
  --tenant "$EMPRESA" --store "$LOJA" --device "$APARELHO" --out "$BANCO_NOVO"
```

`BANCO_NOVO` precisa ser novo, incluindo ausência de WAL/SHM anteriores. A restauração revoga sessões humanas e exige novo login. O backup não inclui a chave privada da estação nem a chave de cifragem; preserve-as separadamente, fora de Git/ZIP. A troca do serviço para o banco restaurado deve ser feita com parada controlada e aprovação do responsável, preservando a origem. Não há troca automática de banco neste teste.

## Próximo passo para um cliente piloto

O circuito técnico pode ser repetido em uma instalação de homologação. O responsável precisa identificar empresa/loja/aparelho, usuários com suas permissões, licença válida e chaves públicas confiáveis, endereço do serviço, diretório persistente e custódia da chave de backup. A configuração desktop deve apontar explicitamente para essa API. Homologar com o operador e restaurar uma cópia antes de autorizar dados comerciais. A fixture de teste e seu emissor de licença nunca viram credenciais de produção.

Esta entrega comprova o recorte local em dinheiro. Não comprova Internet/TLS, ambiente Windows, equipamento físico, fiscal, todas as APIs do sistema ou segurança de venda offline multiaparelho. O pacote separado de devoluções/schema64 continua sem aplicação. Não foi criado instalador ou ZIP, nem iniciada instalação comercial.

## Resultados observados em25/10/2026

- Teste principal final:101requisições reais, quatro processos, aprovado em0,834s, sem skips. Resultado de caixa2148centavos, estoque5000milésimos e saldo documental zero.
- Suíte completa do backend:1229testes principais aprovados;2636casos incluindo subtestes; zero falhas.51pacotes com testes aprovados e18sem testes. O pacote mais demorado concluiu em381,151s.
- Sete testes principais ignorados:duas integrações PostgreSQL sem banco autorizado, três pontes opcionais por ausência das variáveis explícitas e dois auxiliares de subprocesso. O auxiliar do piloto foi iniciado e executado quatro vezes pelo teste principal.
- A suíte completa foi executada antes de acrescentar cinco leituras extras à comparação de snapshots do teste novo. A versão final ampliada foi reexecutada isoladamente e com detector de corridas. Nenhum fonte de produção foi alterado entre essas execuções.
- Build dos executáveis reais `titan-local` e `titan-backup`: aprovado, Go1.25.8 já instalado. Não houve instalação de dependências.
- Navegador: login e sessão reais; venda recuperada de R$31,98; pedido de2unidades e recebimento integral; documento aprovado de R$20,50, aplicadoR$20,50 e saldoR$0,00. A captura do financeiro e do recebimento identifica expressamente a base restaurada descartável.
- A inspeção visual foi parcial, não uma automação de todas as telas. O proxy temporário encerrou com código143 depois das capturas e uma tentativa de abrir a tela de caixa recebeu `net::ERR_CONNECTION_REFUSED`; isso não foi contado como aprovação visual do caixa. O fechamento oficial foi verificado por HTTP e SQL no teste automatizado. Não houve alteração de firewall ou acesso público.
- Não foram reexecutados TypeScript/build frontend: nenhum fonte frontend foi alterado nesta entrega. Foi usado o build local já validado na revisão44, que também contém o relatório de carrinhos ainda pausado; esse relatório não foi homologado ou publicado aqui. Os oito hashes dos fontes pausados permaneceram iguais.
- A primeira versão do teste consultava uma coluna `kind` inexistente em `cash_movements`. A asserção foi corrigida para usar os valores assinados do esquema real; nenhuma mudança de esquema foi feita para acomodar o teste.

Evidências locais: `Documents/Codex/2026-10-20/github-plugin-github-openai-curated-remote-2/work/pilot-cash-20261025/evidence/`. Capturas em `outputs/piloto-api-{venda,recebimento}-restaurada-2026-10-25.png` e `outputs/piloto-api-financeiro-restaurado-2026-10-25.png`. Logs locais não incluem tokens ou senhas de sessões. Credenciais sintéticas da revisão opcional ficam em arquivo privado temporário separado, nunca nesta documentação/Git.

Os binários Linux compilados estão no laboratório `work/pilot-cash-20261025/bin/`, total aproximado31MiB, sem Electron/node_modules. São executáveis de linha de comando, não instaladores. SHA-256 observado:

```text
titan-local  ccd69baa92a872110f987d7193dac3e8dabf0c85a8261a8cf32643ffe8e0eea3
titan-backup b6f10460c61154da2d30bd1f7bb1de86bd1cfea20017be043bff0912a5ca82d2
```

O aceite técnico deste recorte está concluído em ambiente descartável. A próxima ação é homologação com o operador e configuração explícita da instalação piloto, mantendo este conjunto de versões como referência. Não anunciar o backend inteiro ou a instalação comercial como concluídos com base neste único circuito.


## Complemento de25/10: transporte nativo e operações originadas nas telas

O teste principal aceita três parâmetros opcionais, que devem ser fornecidos juntos: `-pilot-electron` (binário Electron existente), `-pilot-native-script` (arquivo `desktop/tests/pilot-api-client.cjs` do frontend) e `-pilot-desktop-stage` (diretório compilado apontado por `desktop/.build/current.json`). Use-os depois de `-args` no comando Go. Não apontar para um serviço comercial: a API e os bancos continuam sendo criados pelo próprio teste.

Com esses parâmetros, todas as101requisições passam pelo transporte de produção `titan://app`, com sandbox/contextIsolation ligados e nodeIntegration desligado. A API real mantém suas transações e SQLite; o adaptador é somente um transporte de teste, não uma simulação de resposta. Quatro execuções do aplicativo acompanham os quatro processos API. Versão final aprovada em2,125s; com `-race`,14,914s. A suíte backend completa também passou:1229principais/2636incluindo subtestes, zero falhas; oito auxiliares/opcionais ignorados na execução padrão.

A fixture opcional `TestPilotCashScreenFlow -args -pilot-screen-dir=CAMINHO_PRIVADO` inicia um banco sem negócios e publica um descritor sintético privado. Não publica token ou chave. As operações devem vir das telas; o driver somente autentica/consulta e faz a validação de persistência após os marcadores explícitos `DONE` e `FINISH`. Diretório deve ser0700; não reutilizar diretório com marcadores antigos. A opção `-pilot-screen-resume` aceita somente o snapshot descartável privado `preserved-screen.sqlite`0600 do mesmo ensaio, preservando os negócios; não é um importador de banco do cliente.

Neste ensaio as telas efetivamente criaram venda, reposição, pedido, viagem, recebimento, obrigação, aprovação independente, retirada e vínculo financeiro (saldo documental zero). Foi encontrada e integrada a retirada oficial que faltava ao PDV; solicitações locais anteriores não eram dinheiro oficial. O fechamento foi contado/revisado emR$21,48, mas a confirmação nativa bloqueou o navegador antes do POST. O primeiro ensaio expirou e está registrado como falha; snapshot consistente descartável foi preservado. A confirmação foi preparada em diálogo interno acessível e compilada, porém o encerramento visual e a comparação/restauração dos mesmos negócios ainda aguardam a liberação do navegador. Não anunciar esse ensaio visual como aprovado.

O estado global atualizado está somente em `docs/architecture/CONTINUIDADE_TITANSYSTEM.md` do frontend, revisão46. Esta seção atualiza os limites da seção anterior: houve integração frontend mínima necessária ao piloto; não houve alteração Go de produção, nova migração, aplicação do ZIP64, dependência instalada ou uso de banco comercial.
