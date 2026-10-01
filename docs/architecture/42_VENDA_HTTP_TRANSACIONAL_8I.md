# Fase 8I — Venda em dinheiro e consulta pela API local

Base: 0244fb9 (caixa HTTP da 8H). Sem frontend, nova migração,
PostgreSQL, acesso de rede ou transporte entre aparelhos.

## API

Todas as rotas ficam em /local/v1 e exigem sessão humana ativa do aparelho
que inicializou o servidor. Contexto de empresa/loja/usuário/aparelho não
é aceito no corpo da requisição.

POST /sales recebe exatamente:
- operation_id, sale_id, cash_session_id: IDs não vazios até 128 bytes;
- items: 1 a 500 objetos com product_id, location_id e quantity_milli;
- payments: 1 a 8 objetos com method e amount_cents.

IDs devem ser gerados no dispositivo (UUID recomendado) e preservados junto
ao conteúdo do pedido para recuperação de resposta perdida. Não criar um
novo ID para repetir uma venda cuja confirmação não chegou.

Novas vendas retornam 201 com sale_id, total_cents e repeated false.
Repetição idêntica retorna 200 e repeated true. Mudança de dados com IDs
reutilizados retorna 409, sem duplicação ou alteração da venda anterior.
O hash vincula também a ordem das linhas e dos pagamentos: o cliente deve
retransmitir o mesmo conteúdo lógico, sem reordená-lo.

GET /sales/:id retorna o registro operacional persistido, incluindo itens,
valores, pagamentos e horário original. A consulta é restrita à própria
empresa, loja, aparelho e identidade responsável pela venda. Venda ausente
ou fora desse escopo retorna 404. A visão gerencial de outros operadores
e de múltiplos aparelhos ainda não foi implementada.

A consulta não exige licença vigente, mas continua exigindo sessão,
papel Sell e aparelho aprovado. Não retorna token nem custo do produto.

## Dinheiro, estoque e caixa

O preço vem exclusivamente do catálogo local persistido na finalização.
Preço informado pelo cliente HTTP é rejeitado, incluindo campos extras
em itens. Consulta e repetição usam os valores históricos, não o preço atual.

Quantidades usam milésimos de unidade; produtos unit não aceitam frações.
Valores monetários são centavos int64. O arredondamento existente usa
inteiros exatos (meio centavo para cima) por linha, sem float.

Nesta fase só cash é aceito. A soma dos pagamentos deve ser igual ao total.
Não há captura de valor entregue, cálculo de troco, desconto, venda a prazo,
cartão ou confirmação PIX. Um method eletrônico recebe 409 e não grava.
Campos de confirmação enviados pelo cliente não são aceitos.

Para nova venda, o caixa deve estar aberto e pertencer ao mesmo operador
e aparelho. O saldo local da gôndola deve cobrir todas as linhas combinadas.
É rejeitada venda que faria o saldo esperado do caixa ultrapassar int64.
Conflitos de saldo global entre dispositivos offline não são resolvidos aqui.

## Atomicidade e recuperação

CompleteWithContract verifica Sell, aparelho e contrato POS na mesma
transação da venda, itens, pagamentos, baixa de estoque, vínculo dos itens,
movimento de caixa, chave de idempotência e outbox.

Falha em qualquer gravação desfaz todas as partes e a observação de relógio
da licença. Nenhum sucesso é informado antes do commit.

Repetir uma venda já concluída depois de fechar o turno devolve o resultado
anterior; isso não permite criar uma nova venda em caixa fechado.
Repetição pelo POST exige contrato atualmente válido. Com licença ausente,
expirada ou sem POS, a recuperação do histórico ocorre pela consulta GET.
Sem verificador configurado, POST retorna 503.

Complete interno é preservado para compatibilidade. Rotas novas usam
somente CompleteWithContract. A assinatura de licença não impede que
um administrador do dispositivo modifique o binário ou os dados locais.

## Documento operacional e limitações

O resultado contém fiscal_authorized false. Nenhuma autorização da SEFAZ,
nota fiscal ou impressão foi realizada. Itens são identificados por IDs;
nomes e unidades não foram congelados no esquema original e não são
inventados pela consulta. A ordem de apresentação usa IDs persistidos,
sem prometer a ordem original de bipagem.

Não implementa cancelamento, devolução, sangria, suprimento após abertura,
pagamentos eletrônicos, transporte da outbox, frontend ou smartphone.
Não modifica políticas comerciais nem concessões de licença da 8H.

## Verificação

Testes adicionados cobrem:
- abertura, venda, baixa, consulta e fechamento de caixa;
- repetição após fechamento, divergência de operação/venda e mudança de preço;
- estoque insuficiente, item repetido, referência inválida e pagamento rejeitado;
- contrato, verificador, sessão, papel e revogação;
- rollback por falha de pagamento e outbox, incluindo relógio da licença;
- reabertura do arquivo e recuperação sem duplicação;
- JSON ambíguo nos níveis externo e interno, campos nulos e valores inexatos;
- pedidos concorrentes, produto/loja/operador alheios;
- limite de saldo do caixa, arredondamento por peso e consulta após expiração;
- contrato nil sem fallback para caminho interno.

Testes usam apenas bancos descartáveis de t.TempDir. O pacote é preparado
por leitura do código e validação de aplicação do patch. Go está indisponível
no ambiente de preparação; testes, vet e compilação devem passar no PC
antes de considerar esta fase entregue.
