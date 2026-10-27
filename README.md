# TitanSystem

ERP/PDV para desktop Windows/Linux, com telas React compiladas, núcleo operacional local e serviços online separados. O aplicativo se conecta ao serviço autorizado da empresa conforme a configuração; não é o site institucional. Esta branch reúne a integração dos commits publicados de backend e frontend para revisão e CI. Tela, binário e teste automatizado não substituem aceite comercial.

## Arquitetura efetiva

- `backend/cmd/titan-local`: API Go/Fiber com SQLite por instalação, transações, contexto autenticado de empresa/loja/aparelho/usuário e migrações incrementais. Endereço de desenvolvimento normalmente 8181; ensaios usam portas livres de loopback.
- `backend/cmd/titan-backup`: ferramenta operacional separada para criar/verificar/restaurar backups cifrados. O comando `inspect` produz JSON com contagens e resumo criptográfico do perfil do piloto, sem exportar registros ou segredos. A chave e a estação ficam fora do pacote.
- API online em `backend/`: Go/Fiber com PostgreSQL, pgx/GORM e `/api/v1`; não deve ser confundida com a API operacional `/local/v1`.
- `frontend-web/`: telas React/TypeScript do ERP desktop. O nome da pasta não as transforma em site institucional. A versão frontend validada está na branch `feat/backend-checkout-authority`; não copiar a árvore desta branch backend sobre alterações locais da interface.
- `desktop/`: invólucro Electron. A organização do desktop compilado e seu transporte está na branch frontend. Esta entrega backend não é um instalador Windows/Linux.
- `mobile/`: frente própria, sem comprovação de venda independente do PC. `website/` é reservado ao site institucional. A árvore histórica `TitanSystem/` permanece preservada.

## Circuitos do piloto

O teste integral percorre venda em dinheiro, caixa, estoque, reposição aprovada, pedido, registro manual de confirmação externa, recebimento integral, obrigação, aprovação independente, retirada e vínculo financeiro. Reinicia processos e confere restauração em banco novo.

O cenário ampliado recebe 2 unidades, aceita 1 e recusa 1 por avaria. Somente a aceita entra no estoque e gera obrigação de R$ 10,25; a unidade restante mantém tratativa em andamento. O cenário verifica conflito de reenvio, reinício intermediário, preservação de IDs e backup/restauração. A tratativa não é resolvida automaticamente.

A variante `-pilot-backup-cli=/caminho/absoluto/titan-backup` executa o binário real: chave sintética, criação com serviço ativo, inspeção, verificação, recusa de chave/contexto incorretos, restauração e recusa de sobrescrita. Contagens e `business_sha256` do perfil devem coincidir após restauração. Esse resumo cobre as tabelas nomeadas do piloto, não todos os módulos do ERP.

## Aceite autenticado entre empresas

Novo contrato [company-trade.openapi.json](docs/api/company-trade.openapi.json): vínculo aprovado pelo dono com fingerprint conferido, pedido aprovado e decisão do fornecedor assinados por estações distintas, destino exato, idempotência, revogação e recibos persistentes. Consultas paginadas permitem localizar pedidos e recuperar provas. Backup restaurado bloqueia novas trocas até revisão do dono, sem apagar histórico; revisão antiga não desbloqueia nova restauração. Veja [guia operacional e limites](docs/architecture/PEDIDOS_AUTENTICADOS_EMPRESAS_20261025.md).

## Devolução parcial em dinheiro

Integrada a entrega anterior de devolução ao cliente, conciliada com os pedidos autenticados na migração65, sem alterar SQL1–64. Cotação autoritativa, devolução parcial, reposição opcional conferida, dinheiro, livro de caixa, recuperação e relatórios preservam a venda original. O quarto piloto executa o circuito com devolução, queda, reinício e restauração pelo backup real. A rota original continua exigindo a gaveta da venda aberta. A migração66 acrescenta rotas explícitas para devolver em turno posterior do mesmo aparelho: autorização adicional, caixa escolhido pelo operador, saldo conferido e proveniência imutável, preservando o fechamento anterior. Não entrega estorno eletrônico ou fiscal. Veja [turno posterior, contratos e limites](docs/architecture/BACKEND_DEVOLUCOES_TURNO66_20261026.md). Veja [contrato](docs/api/customer-returns.openapi.json) e [integração, provas e limites](docs/architecture/BACKEND_DEVOLUCOES_CLIENTE65_20261025.md).

## Testes e CI

Requer o toolchain fixado em `backend/go.mod` (Go1.26.10 nesta integração), Node24 e um ambiente de teste descartável compatível e dependências disponíveis. Na pasta `backend/`:

```bash
go vet ./...
go test ./... -count=1
go build -o /tmp/titan-backup ./cmd/titan-backup
go test -race ./internal/localapi \
  -run '^TestPilotCash(SaleToSupplierFinanceRestartAndRestore|PartialReceiptDiscrepancyRestartAndRestore|AuthenticatedSupplierRestartAndRestore|CustomerReturnRestartAndRestore)$' \
  -count=1 -v -args -pilot-backup-cli=/tmp/titan-backup
```

O workflow específico [backend-pilot.yml](.github/workflows/backend-pilot.yml) dispara em push desta branch, PR para main/develop ou execução manual. Executa verificação dos contratos, vet, suíte backend com PostgreSQL e três pontes frontend obrigatórios e os quatro circuitos HTTP/SQLite e testes de troca entre empresas com detector de corridas e CLI real; publica logs e binários Linux como artefatos. A aprovação precisa ser conferida no run do SHA correspondente, sem inferir sucesso por esse texto. O workflow histórico `ci.yml` permanece separado.

## Limites e operação

Testes usam somente instalações descartáveis e licenças sintéticas. Nenhum serviço ou banco comercial é selecionado automaticamente. O fluxo manual continua disponível. O novo circuito de aceite autenticado entre empresas usa bancos/chaves/sessões distintos e transporte explícito de envelopes por cliente autorizado; não é relay público nem Internet comercial homologada. Fiscal, TEF/Pix, instalador em máquina Windows limpa e sincronização comercial continuam exigindo entregas próprias. O núcleo local utiliza atualmente proteções de arquivo POSIX 0600/0700; compilar para Windows não basta para homologar essas proteções.

Backup cifrado tem limite de snapshot 128 MiB. Restaurar exige caminho novo, chave e contexto corretos; sessões antigas são revogadas. A inspeção não substitui a verificação criptográfica, que é executada antes dela. Não publicar `.env`, chaves, estações, bancos ou dados comerciais.

Veja [guia do piloto](docs/architecture/PILOTO_DINHEIRO.md), [contratos](docs/api/), [regras](docs/architecture/PROJECT_RULES.md) e [evidências históricas](docs/architecture/evidence/pilot-cash-20261025/README.md). Os resultados desta entrega serão registrados junto do link CI efetivamente observado.


### Transporte explícito entre empresas

`titan-trade` transporta pedidos previamente exportados e decisões assinadas entre duas APIs autorizadas. Persistência da tentativa, recuperação sem POST/origem online e retry somente explícito com os mesmos IDs. Nenhuma aprovação automática, credencial na linha de comando ou abertura de banco pelo cliente. HTTPS validado fora de loopback; Linux com lock durável. [Contrato dos arquivos, comandos e limites](docs/architecture/BACKEND_TRANSPORTE_ASSINADO_20261026.md). Os serviços instalados existentes não são reiniciados automaticamente.


### Fila operacional do transporte assinado

`titan-trade queue` adiciona e processa somente mensagens já autorizadas, com snapshot persistente, vínculo à sessão original e confirmação por recibo. Operação incerta é consultada, sem reenvio automático; mudança de sessão pausa sem apagar prova. Worker cancelável e limites explícitos, com teste de queda após commit real do destinatário. [Comandos, estados e aceites](docs/architecture/BACKEND_FILA_ASSINADA_20261026.md). Não instala daemon nem troca os serviços existentes.


### Backup privado da fila

`titan-trade queue backup`, `verify-backup` e `restore` cifram e validam os arquivos externos de transporte. Restauração exige pasta nova e mantém os itens pausados e limitados à consulta dos recibos originais; uma cópia antiga nunca autoriza reenvio. Não inclui banco, tokens ou chaves privadas. [Formato, comandos, recuperação e limites](docs/architecture/BACKEND_BACKUP_FILA_20261026.md).


### Recuperar recibo com a origem desligada

`titan-trade queue recover-session` reautoriza somente a consulta do recibo original usando sessão nova no destinatário. Exige prova persistida, preserva IDs e bloqueia POST/retry; não depende do arquivo de sessão ou da API da origem. [Regras, comandos e evidências](docs/architecture/BACKEND_RECUPERACAO_SESSAO_DESTINO_20261026.md).


### Worker com contexto explícito

`queue status/run/watch --scope ARQUIVO_PRIVADO` limita a fila ao servidor, empresa, loja, aparelho, operador e fingerprint originais. Mudança do arquivo encerra o worker, preservando tentativas e provas; não amplia permissão nem vincula outro contexto silenciosamente. [Formato, segurança e integração desktop pendente](docs/architecture/BACKEND_FILA_CONTEXTO_20261026.md).


## Desktop, serviços e site

- `frontend-web/`: fonte única das telas internas do software. Vite é ferramenta de desenvolvimento/compilação.
- `desktop/`: executa e distribui essas telas como aplicativo; não duplica os módulos.
- `backend/`: serviços Go, autorização e persistência. O servidor pode ser separado do PC cliente; não é conteúdo de página web.
- `website/`: área reservada ao site público futuro, sem copiar telas, dados ou permissões do ERP.

Não mover as pastas históricas apenas para mudar nomes: contratos, scripts, CI e alterações locais dependem dos caminhos atuais. Consulte [a execução desktop](desktop/README.md) e o [registro único de continuidade](docs/architecture/CONTINUIDADE_TITANSYSTEM.md).


## Integração em revisão e critérios de entrega

Esta integração combina os commits publicados `3270029` do backend e `3d5abc9`
do frontend com a main `024dd3f`, preservando as alterações locais não publicadas
do frontend. Inclui o incremento fiscal local conferido por hashes, descrito em
[FISCAL_COMPONENTES_20261026.md](docs/architecture/FISCAL_COMPONENTES_20261026.md).
Emissão, DANFE e SPED oficiais continuam indisponíveis; geração/assinatura local
de teste não é homologação fiscal.

O CI de PR testa a combinação efetiva de backend e frontend, sem trocar seus
adaptadores por outra revisão. Os checks obrigatórios executam a suíte frontend
completa, regressões Go, o piloto real com reinício/restauração e os testes de
runtime. `desktop-installers.yml` prepara instaladores em runners descartáveis
Ubuntu/Windows, instala em caminho novo e verifica abertura com perfil limpo,
registrando imagem e tempo. Um job separado verifica o instalador Linux do
backend com binários reais e banco sintético. O cliente Windows conecta a um
servidor autorizado; este ensaio não entrega backend local Windows, TEF, fiscal,
equipamentos ou cliente comercial em uso.

Instaladores são artefatos de avaliação sem assinatura comercial ou publicação
automática. Os resultados precisam ser lidos no run do SHA deste PR; a existência
do workflow não prova que ele passou. A main exige aprovação independente,
checks verdes e resolução das conversas; não há merge automático.
