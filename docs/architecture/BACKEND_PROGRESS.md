# Backend e integração — matriz de progresso

## B02B4 — lançamentos operacionais do dinheiro

Base no PC: B02B3 `5dbeb69`, instalação e testes confirmados. Nova migração47
adiciona histórico imutável de abertura, venda, estorno, suprimento e sangria,
capturado na transação original, e consulta paginada do turno encerrado.
Backup25–47. Ajuste compartilhado no executor de migrações para suportar triggers;
nenhuma edição em0001–0046, autenticação, sessões ou frontend.

Escopo, contratos e coordenação:
[B02B4_LANCAMENTOS_CAIXA.md](B02B4_LANCAMENTOS_CAIXA.md).
IDs e autor preservados; fontes antigas sem vínculo permanecem não classificadas,
sem responsável inventado. Resumo do turno inteiro, precisão exata e recusa de
divergência sem reparar passado. Contrapartidas são controles operacionais.
Financeiro completo, confirmação eletrônica, venda mista e adapter visual pendentes.

## B02B3 — finalização real de planos integralmente em dinheiro

Base no PC: B02B2 `5dabd4a`, testada e registrada com árvore limpa. Nova rota
`/local/v1/checkout/plans/{id}/finalize-cash` une plano, pagamentos com IDs
estáveis, venda, consumo de estoque, caixa líquido, outbox e auditoria de
recebido/troco numa transação. Esquema 46, backup compatível 25–46. Migrações
anteriores, autenticação e frontend não foram alterados.

Detalhes e coordenação da 0046:
[B02B3_FINALIZACAO_DINHEIRO.md](B02B3_FINALIZACAO_DINHEIRO.md).
Planos em dinheiro podem informar prontidão; finalização revalida contrato,
turno, preço/unidade e estoque. Planos mistos continuam `can_finalize=false`.
Cash confirmado significa recebimento declarado pelo operador, sem confirmação
de adquirente. Finalizações são recuperáveis após reinício ou estorno da venda.
Financeiro completo, conector eletrônico, R$315 misto e adapter visual pendentes.

## B02B2 — consulta oficial de caixa encerrado

Base no PC: B02B1 `b882ece`, testada e registrada com árvore limpa. Nova API GET
`/local/v1/cash/sessions/{id}/report` lê fechamento histórico, valores oficiais,
vendas/cancelamentos e status dos pagamentos em uma transação. Recusa caixa
aberto e histórico inconsistente; não reaproveita sessão atual. Esquema segue45.

Escopo, campos, precisão e limites:
[B02B2_RELATORIO_OFICIAL_CAIXA.md](B02B2_RELATORIO_OFICIAL_CAIXA.md).
Os status legados e o meio `card` não viram confirmação eletrônica ou distinção
débito/crédito. Planos B02B1 não entram como venda. Não há banco conciliado,
razão financeiro, mudança no encerramento ou integração de frontend97/98/99.
Parcelas comerciais confirmadas, financeiro e aceite completo R$315 pendentes.

## B02B1 — distribuição persistida do checkout

Base no PC: B02A `b1e3d80`, branch isolada `feat/backend-b02a-compat`.
Novo esquema 0045, sem alteração nas migrações anteriores da produção.
Detalhes, arquivos compartilhados e coordenação:
[B02B1_PARCELAS_PERSISTIDAS.md](B02B1_PARCELAS_PERSISTIDAS.md).

Três APIs criam, recuperam e cancelam planos duráveis com IDs estáveis de
checkout/operação/parcela, soma exata, snapshot de preços e histórico.
Recuperação após reinício e repetição não reprecificam nem duplicam o plano.
Produtos ativos e saldo livre são verificados na criação; nenhum saldo é
reservado por um plano. Backup/restauração passam a aceitar também 45.

Todas as parcelas permanecem `not_requested`, `provider_confirmed=false`, e
`can_finalize=false`. O caso R$315 é distribuição persistida, não venda mista
concluída. Confirmação de cobranças, financeiro, fechamento por meio e ligação
ao adapter Checkout96/frontend97/98 continuam pendentes. Eventos da outbox
não possuem aplicação comercial no transporte atual.

Os testes HTTP do pacote verificam reinício, resposta perdida, concorrência no
processo, rollback, precisão, autorização, isolamento e backup/restauração.
Resultados da execução acompanham a entrega; aceite no PC exige o aplicador.
As seções seguintes registram o escopo e as evidências das entregas anteriores.

## B02A — pré-requisito de compatibilidade com produção

Base de aplicação no PC: B01 `c557980` em `feat/backend-checkout-authority`.
Fonte de produção conferida: `ba29cbdf6a0a2081754609de63ce003414c74f79`.
Esta entrega integra no backend as migrações 0028–0044 e os serviços/testes
existentes dessa revisão de produção, preservando os arquivos SQL byte a byte.
Não cria 0045, não renumera migrações e não faz merge de branches/histórico.
Novas mudanças da produção devem ser comparadas com esse snapshot antes de
integrar; não reaplicar patches de produção já incorporados.

A produção acrescenta receitas, ordens, materiais, resultados, perdas, etapas,
lotes, qualidade, edição/inativação de catálogo e operações de compras. O
inventário foi atualizado a partir das rotas Fiber reais; os OpenAPI da fonte
acompanham os serviços. Isso é integração de backend, não aceite das telas.

A regra transacional `stockreservation.FreeTx` agora é usada pelo cálculo
compartilhado de cotação e finalização da B01. Saldo físico reservado não pode
ser vendido, nem por linhas repetidas. Produto inativo não gera nova cotação
ou venda; consulta e replay de venda já confirmada continuam disponíveis.
Perdas, transferências e contagem reutilizam as proteções da produção.

Backup/restauração passam a aceitar os esquemas 25–44 conforme o código da
produção, mantendo ValidateSchema, checksums, integridade, chaves estrangeiras,
conferência de aparelho e revogação de sessões na restauração.

Testes específicos exercitam reserva pelo serviço real, recusa de saldo
comprometido, venda do saldo livre, repetição e liberação de reserva; também
testam inativação sem apagar nem bloquear recuperação de uma venda anterior.
Os testes existentes de migração/restauração cobrem a cadeia incorporada.

B02A NÃO implementa parcelas persistidas, financeiro ou fechamento misto.
Ela resolve o requisito de compatibilidade necessário antes desse esquema:
o backend estava em 27, enquanto produção já ocupava 28–44. A próxima parte
da B02 deve implementar o ciclo financeiro sobre esta base unificada, com
numeração de migração coordenada. O cenário de R$315 permanece pendente.
Frontend, autenticação, sessões e PostgreSQL não foram alterados nesta entrega.

Validação B02A: suíte Go completa e go vet passaram. Race passou nos pacotes
sale, stock, stockreservation, inventory, production e backup e nos testes HTTP
selecionados de compatibilidade/reservas/isolamento de todas as rotas. Os 68
testes Python passaram. O build estático de titan-local passou em uma cópia
com os mesmos arquivos de backend e diretório Git próprio: na worktree deste
ambiente, o Go tentava obter metadados VCS do diretório externo /workspace e
falhava antes de compilar. O frontend desta revisão completa do usuário não
está disponível aqui; seus testes/build serão repetidos pelo aplicador no PC.
Dois testes de contratos falharam na primeira suíte por campos novos não
declarados; os contratos foram estendidos, e a suíte completa passou depois.
Não foram executados PostgreSQL real, hardware, banco comercial, pagamentos
externos, reconciliação entre aparelhos ou validação visual da produção.

As seções seguintes preservam a auditoria e as evidências históricas da B01;
o estado atual é complementado por esta seção e pelos testes da B02A.

Backend auditado: main pública `7140b25c4deb164989a95d1a1b9876e2e46a9a51`.
Base de aplicação desta adaptação: `0f6cd3931ba1ea00bf9401c77786b88e58fe9e9f`.
O proprietário forneceu a lista completa de alterações do commit (frontend e
documentação) e o diff do PDV/service95. Esses dois arquivos foram reconstruídos
e seus hashes Git conferidos: `2a0be8597670f4b52ce21a4e16847c61d3c74cfb`
e `3df20fe3afa93a912b1d43b6cacace713c196645`, respectivamente.
Isso NÃO equivale a possuir a revisão 0f6cd39 inteira neste ambiente.
Branch de trabalho: `feat/backend-checkout-authority`. Não comprova a ausência
de mudanças adicionais no PC ou na branch de produção do proprietário.
Adaptação: 17/10/2026. Nenhum banco comercial foi utilizado.

| Capacidade | Tela | Contrato | Rota | Serviço | Banco/migração | Permissão | Teste | Demonstração | Lacuna |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Sessão/contexto | login local | localauth.Session | /local/v1/login, /me | localauth, identity | SQLite 0015 e revisões de conta/acesso | sessão, empresa/loja/aparelho, políticas | suíte Go e route_isolation_test.go | testes HTTP em banco descartável | aparelho físico/piloto não ensaiado nesta entrega |
| Cotação | confirmação do LocalPointOfSale | sale-quote.openapi.json | POST /local/v1/sales/quote | sale.QuoteWithContract | catálogo/movimentos existentes, sem nova migração | Sell + POS/Core/Inventory via RequireTx | sale_quote_test.go; localQuote.test.mjs | preço/estoque reais nos testes HTTP | snapshot sem reserva; descontos e pontos indisponíveis |
| Venda/pagamento | PDV local | SaleInput, sale.Receipt | POST /local/v1/sales; GET /sales/:id | sale.complete/Read | sales/items/payments/operations + outbox | Sell + contrato POS; leitura autorizada | sales_test.go, sale/*test.go, posOperations | replay e reinício em testes | somente dinheiro confirmado; cartão/Pix sem provedor |
| Estoque da venda | carrinho e comprovante | sale.Item | POST /sales | priceItemsTx/complete | stock_movements + sale_item_stock | mesmo escopo da venda | saldo repetido, concorrência, rollback | banco SQLite descartável | direitos offline/última unidade entre aparelhos não comprovados |
| Suspensão | Till94 | till94.mjs | nenhuma rota de suspensão encontrada | estado do navegador | tillStorage94/IndexedDB | checagens da interface | modelos locais | rascunho local | falta persistência autoritativa e recuperação em outro terminal |
| Cancelamento | histórico/solicitação | sale.CancelInput | POST /sales/:id/cancel | sale.CancelWithContract | SQLite 0020 + compensações | CancelSale + POS | sale_cancel_test.go | testes HTTP | devolução parcial e reembolso eletrônico pendentes |
| Caixa/movimentos | abertura/fechamento/Till94 | register.OpenInput, CloseInput, MovementInput | /cash/open, /cash/close, /cash/movements | register | SQLite 0008 e 0021 | Sell/ManageCash + POS | cash*_test.go | testes de fechamento cego | aprovações de tela não equivalem a autorização de supervisor persistida |
| Fechamento por meio/malote | CashClosing92, Till94 | closing92, till94 | fechamento oficial somente em espécie | register.close; modelos no navegador | cash_closures; rascunhos locais | sessão/caixa no backend existente | fechamento em dinheiro testado | cenário misto não concluído | conciliação por parcela, OCR e cadeia de malote autoritativos pendentes |
| Financeiro | telas financeiras existentes | modelos/adaptadores distintos | não há serviço local completo encontrado | financial contém rascunhos online | cash_movements não é razão financeiro completo | por projetar sobre autorização existente | sem aceite integral | não demonstrado | pagar/receber, baixas, contrapartidas, taxas e liquidação |
| Pagamentos/Parte96 | Checkout96, Desk95 | CheckoutAdapter96 recebido; Gateway95 padrão continua desconectado | B01 usa /sales/quote no onCash existente | dinheiro pelo serviço real de venda; adaptador avançado não implementado | plano96 no navegador, venda oficial SQLite | frontend não concede aprovação eletrônica | testes da ponte de dinheiro e HTTP | nenhum conector real homologado | cotação96 persistida/parcelas e binding completo pendentes |
| Fiscal/SPED | telas e contratos | NFeEmitterUseCase é interface | rotas SPED oficiais retornam Indisponivel | sped_service.go contém simulação com URL fictícia | esquema histórico online | não implementado como emissão operacional | testes de indisponibilidade existentes | sem SEFAZ | definir piloto, documento/UF/regime, provedor e homologação; não ativar simulação |
| Sincronização | ferramentas de pareamento | incoming/outgoing | receptor separado | outbox/inbox cifrados | SQLite 0010, 0017–0019 | aparelho e pares autorizados | incoming/outgoing tests | testes, sem ensaio de 35 aparelhos | recibo de transporte não aplica todo evento comercial |
| Backup/restauração | CLI | backup.Create/Verify/Restore | não confundir com API comercial | localdb/backup | snapshot cifrado; restaura em arquivo novo | operador com chave e dispositivo esperado | backup e cmd/titan-backup | banco descartável | desastre real, PostgreSQL e identidade antirrollback fora deste aceite |

## Incremento B01 — cotação autoritativa ligada ao PDV

Na adaptação 0f6cd39, a ligação ocorre no `submitSale` já chamado por
`Checkout96.onCash` quando o adaptador avançado está desconectado e há uma única
parcela em dinheiro, sem desconto/pontos/fiscal. O ID continua vindo do draft96;
não se gera outro ID na finalização. A cotação B01 não é convertida em Quote96
com ID/expiração fictícios, nem os IDs aleatórios dos pagamentos antigos são
apresentados como IDs de parcelas do contrato96. O adaptador avançado permanece
uma dependência posterior, com persistência e contratos próprios.

- Reutiliza cálculo e validação da venda em dinheiro, sem alterar seu evento v1.
- Consulta preço/unidade no catálogo, exige caixa do operador/aparelho, acumula
  linhas repetidas e aplica arredondamento inteiro por linha.
- O servidor ignora autoridade alegada pelo navegador: JSON com preço, tenant,
  desconto ou aprovação é recusado, assim como duplicatas e valores inexatos.
- Antes de gerar a venda, o PDV consulta a cotação. Preço/unidade diferente
  atualiza o carrinho e interrompe o envio para nova confirmação. Falha não
  autoriza usar o preço em cache. Não troca a recuperação de venda pendente
  por uma nova cotação ou por outro ID.
- Quote não cria reserva, venda, pagamento, movimento ou evento comercial.
  RequireTx pode avançar a referência temporal persistida do contrato.
- A finalização revalida caixa, preço e estoque na transação original. Mudança
  após a cotação pode recusar a venda; quote não é garantia de saldo/preço.
- O cliente limita a resposta a inteiros exatos do JavaScript. Não há migração
  de float64: modelos online com valores float64 foram identificados e precisam
  de inventário/conversão próprios antes de uso financeiro ampliado.

## Evidências e limites

Adaptação 0f6cd39: os 10 testes de cotação/ponte de dinheiro passaram, incluindo
preservação do ID, preço alterado e interrupção antes de confirmar. Os 71 testes
locais anteriores também foram executados sobre os módulos disponíveis.
O build completo e o teste `checkout96.test.mjs` da base nova NÃO foram
executados aqui, pois faltam fontes novas não fornecidas. O aplicador os executa
na worktree do PC e só cria commit após sucesso. Não foram criados mocks para
preencher essas dependências. Os resultados abaixo pertencem à B01 original
na base 7140b25; os arquivos de backend desta adaptação são idênticos àquela B01.

Baseline: 71 testes locais do frontend e build passaram; suíte Go passou.
O primeiro teste Python falhou por Go fora do PATH, sem alterar o código das
ferramentas; reexecução com o toolchain no PATH é registrada na entrega.
O build já possuía chunks grandes (planilhas); não há medição de hardware antigo.

Após o incremento: `go test -count=1 ./...`, `go vet ./...`, build de
`cmd/titan-local` com CGO_ENABLED=0, 78 testes locais do frontend, build do
frontend e 68 testes Python passaram. `go test -race -count=1
./internal/localdb/sale ./internal/localapi` também passou (sale 2,321s;
localapi 457,476s neste ambiente; isso não mede velocidade de venda).
O teste do inventário de rotas inicialmente
falhou por faltar a nova rota; o inventário foi atualizado e a suíte completa
passou, sem desativar a verificação.

Os testes novos cobrem cotação/repetição sem escrita comercial, arredondamento,
preço alterado, estoque acumulado, contrato/permissão, JSON ambíguo e limites
numéricos. Os testes de isolamento de todas as rotas incluem a nova rota.

O caso de R$315 misto é testado como indisponível, sem baixa nem confirmação
falsa. Isso NÃO é aprovação do cenário obrigatório: abertura R$100 + dinheiro
R$100 + suprimento R$50 − sangria R$20 = R$230 continua dependendo dos
conectores, parcelas, conciliação e financeiro. Débito R$96 e crédito R$119
não são convertidos em dinheiro para aparentar sucesso.

Próxima dependência: registro durável de parcelas e conectores de pagamento
com estados e consulta, separando espécie, recebível e liquidação; depois
fechamento por meio e financeiro. Sandbox deverá ter perfil/configuração
explícitos e não conceder aprovação ao ambiente comercial.

Não foram executados nesta entrega: PostgreSQL real (testes opt-in), SEFAZ,
adquirente, hardware, aparelho móvel, 15 PCs/20 smartphones ou ensaio visual
com browser. Código ligado e teste HTTP não são homologação/equipamento.
