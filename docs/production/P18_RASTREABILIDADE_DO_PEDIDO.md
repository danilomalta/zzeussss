# P18 — Rastreabilidade de pedido de compra

Base efetiva: `458164e`, branch `feat/producao-receitas`, pasta
`~/Downloads/zzeus-frontend-producao`. Entrega separada no modulo de compras.

## Contrato

`GET /local/v1/purchase-orders/:id/trace`

Retorna `order`, `creation`, `approval` na mesma transacao SQLite:

- `order`: cabecalho e unico item preservado na criacao; fornecedor ID/nome,
  sugestao, aprovador, datas, estado real `local_not_sent`; produto ID, SKU,
  nome, unidade e quantidade exata `quantity_milli`.
- `creation`: evento `purchase.created`, operacao, aparelho, autor e data.
- `approval`: ID da sugestao, operacao de revisao, aparelho, revisor,
  decisao humana `approved`, motivo registrado e data de decisao.

Nao inclui preco/custo, chave, token, assinatura de aparelho ou payload da
outbox. Fornecedor continua referencia local, nao uma conta externa.
Datas permanecem como registradas. Nao se presume sincronizacao de relogios
entre aparelhos nem se infere envio/confirmacao/recebimento desses registros.

ID exato de 1 a 128 bytes apos decodificacao URL, sem espacos nas bordas,
NUL, CR ou LF. Nao aceita query parameters, incluindo empresa/loja/offset.
400 para entrada invalida; 401 para sessao/aparelho invalido; 403 sem permissao;
404 se nao existir o pedido no escopo; 409 para rastreabilidade inconsistente.

## Vínculos conferidos

Consulta primeiro o pedido da empresa e loja autenticadas, incluindo seu
criador/aparelho. Le o item preservado, sem JOIN com catalogo ou fornecedor atual.
Unidades permitidas sao as seis existentes: unit, kg, g, liter, ml, meter.
Quantidade inteira entre 1 e 9007199254740991 inclusive, na escala milli da unidade:
5000 unit = 5 unidades, sem converter para gramas nem somar unidades distintas.

Deve existir exatamente um evento purchase.created para o pedido nessa loja.
Operacao, autor, aparelho e data do evento devem coincidir com o cabecalho.
A revisao da sugestao deve pertencer a mesma empresa/loja, ter decisao approved
com revisor e data iguais aos campos preservados no pedido. Pedido sem item,
com mais de um item, unidade/quantidade invalida, estado desconhecido, auditoria
faltante/duplicada/divergente ou aprovacao divergente nao gera resposta parcial.

Motivo e aparelho da aprovacao vem de restock_reviews. A tabela conserva a
revisao registrada, mas este endpoint nao adiciona criptografia ou prova contra
alteracao direta de SQLite. A quantidade vem exclusivamente do item da compra,
nao da sugestao ou politica atual; alterar posteriormente nomes, unidades ou
recomendacao de reposicao nao reescreve o pedido. Essa leitura nao valida o
estoque atual e nao libera uma aprovacao para reutilizacao.

## Autorizacao e efeitos

Reutiliza purchases.ReadTx e view_orders. Consulta vinculo, loja, identidade,
aparelho e politica efetiva dentro da mesma transacao das leituras. Nao amplia
papeis nem muda autorizacao/idempotencia dos escritores. O papel supplier e
apenas membro local sujeito ao mesmo escopo, nao acesso entre empresas.

Preserva leitura autorizada apos vencimento do contrato e sem verificador de
emissor configurado, como Get anterior. Nao avanca relogio de licenca.
Nao cria auditoria para GET, evento, outbox, pedido ou aprovacao. Nao reserva,
baixa, recebe ou paga estoque. Replay de criacao continua no escritor antigo;
Trace apenas mostra seu unico registro de auditoria.

## Compartilhados e migracoes

Unico compartilhado alterado: server.go acrescenta mountPurchaseTrace(protected)
ao lado de mountPurchaseSearch. Rota especifica com /trace, sem substituir
/purchase-orders/:id, P17, criacao ou listagem anteriores. Conflito nessa montagem
deve ser revisado na integracao. Nucleo e handler ficam em arquivos novos.

Nenhuma migracao. SQLite permanece 35. Migracoes existentes, whitelist explicita
e todos os arquivos de backup permanecem intactos: checksums, ValidateSchema,
integrity_check, foreign_key_check, aparelho, criptografia e recuperacao.

## Validacao

Testes usam exclusivamente bancos SQLite descartaveis e Fiber App.Test:
campos da aprovacao/auditoria, item congelado apos alteracoes do catalogo,
quantidade maxima/seis unidades, replay e concorrencia, entrada estrita,
view_orders/papeis, escopos de identidade/aparelho/loja/empresa, duas empresas
com IDs iguais no mesmo banco, contrato vencido/leitor sem verificador,
sem escrita/relogio, rejeicao de vinculos quebrados e backup/restauracao da
compra com seu item, aprovacao e auditoria.

Instalador confere branch, base, arvore limpa e checksum; aplica somente P18.
Executa backup/CLI, compras e API P18, suite completa, vet, race e build CGO=0
sem executar servidor. Commit somente dos sete arquivos apos sucesso.
Sem merge, push ou alteracao de bancos reais. API em
`docs/api/purchase-trace.openapi.json`.
