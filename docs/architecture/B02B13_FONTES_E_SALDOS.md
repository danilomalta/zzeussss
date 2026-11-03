# B02B13 — fontes elegíveis e saldo por parcela

## Base e escopo
Exige B02B12 `af33023895374d22e7e18e1178943e570c52a7fe`, branch
`feat/backend-b02a-compat`, na worktree `zzeus-frontend-backend-b02a`.
A saída do proprietário confirmou base e árvore limpa em20/10/2026.

Esta entrega resolve uma dependência operacional da integração financeira:
localizar IDs reais de fontes já auditadas, sem digitar IDs por tentativa, e
consultar aplicação/devolução/saldo de cada parcela no mesmo snapshot SQLite.
Não modifica as interfaces113/114 desenvolvidas no outro chat. As novas APIs
precisam ser conectadas a essas telas em entrega própria.

Não cria pagamento, sangria, suprimento ou reserva. Não modifica os comandos
B02B11/B02B12, os recibos originais nem o seu formato. Não cria migração:
o esquema e a compatibilidade de backup permanecem53. Não altera autenticação,
sessões, permissões centrais, PostgreSQL, estoque ou produção.

## APIs reais
Contrato completo: `docs/api/payable-readiness.openapi.json`.
Todas são GET sob sessão local, com `ViewAccounting`, contexto derivado da sessão,
sem corpo e `Cache-Control: no-store`. Não exigem Finance ativo para consultar
registros históricos. A permissão de leitura não concede autorização de escrita.

| Caminho em /local/v1 | Resultado |
| --- | --- |
| /finance/payables/{id}/installment-balances | Todas as parcelas com principal, bruto pago, devolvido, aplicado e saldo |
| /finance/payables/{id}/cash-sources?installment_id={id} | Sangrias do próprio ator elegíveis para uma parcela |
| /finance/payable-cash/{id}/return-sources | Suprimentos do próprio ator elegíveis para devolver integralmente um pagamento |

Consultas de fontes aceitam apenas `limit`1–100 (padrão50), `offset`0–1000000000
e, somente no primeiro caso, `installment_id` obrigatório. Recusam valores
vazios, duplicados, formatos numéricos não decimais e parâmetros extras.
Saldos por parcela não aceitam query. IDs inválidos400; recurso fora do contexto404;
sem permissão403; inconsistência409; limite de fonte422; nenhuma resposta parcial.

## Saldo por parcela
A consulta lê documento, histórico e pagamentos/devoluções numa única transação.
Retorna parcelas na ordem documental, preservando principal e vencimento;
`paid_cents` é bruto histórico, `returned_cents` é devolvido,
`applied_cents=paid_cents-returned_cents`. O saldo é principal menos aplicado
somente em documento aprovado; nos demais estados o saldo ativo é zero.
O resumo é o mesmo conceito de espécie da B02B12 e concilia com as parcelas.
Não somar principal, aplicado e saldo como receitas diferentes.

Valores são inteiros exatos até9007199254740991 centavos. Todos os pagamentos
são validados, incluindo os devolvidos. Nenhum dado de bancos externos é
presumido. Esta consulta não transforma `settled_cents=null` em liquidação
bancária nem marca `payment_execution_available=true`.

## Elegibilidade das fontes
São consultados os movimentos do próprio ator na mesma empresa/loja/aparelho.
Não expõe os movimentos de outro ator como opções utilizáveis. Turno encerrado
pode fornecer origem histórica; a consulta não reabre o caixa.

Sangria: valor positivo não excede o saldo da parcela nem a margem restante do
limite bruto histórico do documento. Exclui qualquer uso histórico em malote ou
pagamento, mesmo depois de cancelamento/rejeição/devolução. Documento não
aprovado, parcela já integralmente aplicada,1000 pagamentos históricos ou limite
bruto esgotado retornam lista vazia e `blocked_reason` explícito.

Suprimento: valor exatamente igual ao pagamento, posterior ou simultâneo à
sangria original, não usado em outra devolução nem financiado por cofre.
Pagamento já devolvido retorna `payment_already_returned`. A data relevante é a
da movimentação original, não a data em que foi classificada como pagamento.

Ordem das fontes: `operation_id` crescente; paginação só depois de validar
integralmente a coleção do ator e tipo. Máximo10000 fontes nesse conjunto,
incluindo fontes antigas/usadas/fora do valor procurado. Acima disso422,
mesmo numa página vazia; não oculta truncamento como lista completa.
Fonte auditada inconsistente fora da página ou fora do valor solicitado também
impede o resultado. Reutiliza a verificação movimento/journal/auditoria B02B12.

`complete=true` significa completude dessa consulta limitada ao ator/tipo/contexto,
não cobertura bancária ou comercial global. `reservation_created=false` e
`write_authorization_checked=false` permanecem explícitos. `amount_limit_cents`
é o máximo permitível naquele snapshot para pagamento, ou o valor exato para
devolução; é zero quando há bloqueio. Lista vazia sem bloqueio significa que
nenhuma fonte desse conjunto atende aos critérios.

## Integração e concorrência
A interface futura escolhe `operation_id` da origem e usa os comandos B02B12
existentes. Deve mostrar dados atuais, valor, turno e motivo do bloqueio,
permitir atualizar e tratar409 na confirmação. A lista não autoriza nem reserva
o movimento. Finance/Core e ManageCash continuam verificados pelo comando.
Duas consultas podem selecionar a mesma origem; só a transação de escrita pode
vinculá-la. Duas sangrias que isoladamente cabem na mesma parcela também não
podem ultrapassar o saldo quando confirmadas em concorrência.

Nenhuma ação automática a partir da consulta. A propriedade de uma fonte não
comprova correspondência física com o fornecedor ou recebimento bancário.

## Arquivos compartilhados e validação
Modificados: uma chamada de montagem em `backend/internal/localapi/server.go`,
inventário/mapa/índice de contratos. Serviços e testes novos em arquivos próprios.
SQL0001–0053, backup e frontend permanecem byte a byte na base conferida.

Dez testes HTTP novos exercitam fonte/saldo, paginação e ausência de reserva,
origens devolvidas, histórico de malote cancelado, suprimento de cofre,
autorização/contexto, licença sem Finance para leitura, parâmetros estritos,
fonte danificada fora da página, concorrência de pagamentos, reinício do banco,
cancelamento, limite10001 e precisão no máximo inteiro seguro.
Schemas são conferidos contra respostas reais do Fiber e inventário contra
as rotas montadas. Testes de isolamento existentes incluem automaticamente
as três novas rotas. Evidências executadas constam no pacote; o aplicador repete
Go/test/vet/race/build e a verificação frontend da worktree do backend.

## Estado e pendências
B02B12: instalada pelo proprietário em af33023, árvore limpa.
B02B13: código e testes desta entrega; aplicação/commit no PC pendentes até log.
Frontend113/114: não alterado; presença de arquivos não comprova integração.
Continuam pendentes execução transacional direta de dinheiro, pagamento direto
de cofre, Pix/TEF/banco, fiscal, devolução parcial, integração visual destas
consultas e sincronização comercial entre bancos de aparelhos independentes.
Não conclui venda mistaR$315, produção P02–P12 ou operação comercial completa.
