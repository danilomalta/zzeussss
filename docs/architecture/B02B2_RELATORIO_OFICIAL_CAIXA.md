# B02B2 — consulta oficial de um caixa encerrado

## Base e fronteiras

Base no PC: B02B1 `b882ece`, branch `feat/backend-b02a-compat`, worktree
`zzeus-frontend-backend-b02a`. Esquema SQLite permanece 45. Não há migração nova,
alteração de backup, autenticação ou sessões. A produção continua independente.
Frontend97/98/99 em desenvolvimento na pasta original não é editado.

## Contrato implementado

GET `/local/v1/cash/sessions/{id}/report`, sem query string ou corpo.
Autenticação e escopo são derivados da sessão. Exige permissão de venda e o
mesmo operador, empresa, loja e aparelho do caixa. Histórico autorizado pode
ser consultado sem contrato POS vigente ou verificador configurado.

A consulta só funciona depois de o caixa estar encerrado. Retorna 409 para
caixa aberto, sem revelar seu saldo esperado. Outro operador/caixa não acessível
ou ID inexistente retorna 404. Não usa `cash/current` para preencher histórico.

O relatório lê, em uma transação consistente:

- ID da sessão, operação oficial de encerramento, abertura e data de fechamento;
- abertura, soma líquida de todos os movimentos, esperado gravado, declarado e
  diferença; suprimentos e sangrias são obtidos dos movimentos auditados;
- quantidade de vendas, bruto incluindo canceladas, total cancelado e líquido;
- parcelas dos pagamentos persistidos, separadas por meio e status registrado.

Confere abertura + movimentos contra o esperado gravado. Confere soma das
parcelas contra cada venda e compatibilidade dos estados: vendas ativas têm
parcelas confirmadas ou pendentes; canceladas têm parcelas revertidas.
Recusa ausência de parcelas, soma divergente, encerramento ausente/incoerente,
estados contraditórios e movimentos de ajuste divergentes do registro auditado.
O erro 409 não contém totais parciais utilizáveis. Falha de banco retorna erro,
não relatório zerado. Uma consulta completa sem vendas retorna zero conhecido.

Usa inteiros exatos. A soma de movimentos com sinais opostos usa `big.Int`,
evitando overflow do SUM SQLite ou dependência da ordem. Valores finais devem
ser representáveis como inteiros seguros JavaScript; caso contrário retorna
422. Não arredonda números grandes para conseguir mostrar o relatório.

## Significado dos campos

| Campo | Fonte e significado |
|---|---|
| `complete=true` | Toda a fonte local desse caixa foi consultada na mesma transação |
| `cash.expected_cents` | Valor gravado pelo encerramento oficial, conferido com abertura/movimentos |
| `cash.declared_cents` | Contagem enviada e aceita no encerramento |
| `cash.difference_cents` | Declarado menos esperado |
| `cash.movement_net_cents` | Saldo líquido de todos os movimentos, inclusive vendas, estornos e ajustes |
| `cash.supply_cents` / `withdrawal_cents` | Totais positivos dos suprimentos/sangrias auditados; já estão dentro dos movimentos |
| `gross_sales_cents` | Soma original das vendas ativas e canceladas |
| `cancelled_sales_cents` | Total das vendas cujo estado atual é cancelado |
| `net_sales_cents` | Soma das vendas atualmente ativas |
| `recorded_confirmed_cents` | Status `confirmed` persistido, sem afirmar confirmação de adquirente |
| `recorded_pending_cents` | Status `pending` persistido em venda ativa |
| `recorded_reversed_cents` | Status `reversed` persistido em venda cancelada |
| `bank_settlement_available=false` | Não existe fonte de liquidação/conciliação bancária nesta entrega |
| `financial_ledger_available=false` | Este relatório não é razão financeiro de pagar/receber |

Os meios são os efetivamente definidos pelo esquema legado: `cash`, `card`,
`pix`, `other`, sempre nessa ordem. `card` NÃO informa se é débito ou crédito.
Não distribuir esse valor por palpite entre as colunas de débito/crédito da UI.
Também não converter `recorded_confirmed_cents` em `providerConfirmed=true`.
Uma parcela pendente não se torna paga por aparecer no relatório.

`complete` não significa consulta de todos os aparelhos, conciliação de banco,
pagamento verificado, revisão gerencial ou fechamento de diferenças. Não há
comprovantes locais, OCR ou revisão do navegador nesse resultado. Não altera o
contrato da rota de encerramento e não decide se uma revisão deve ser aprovada.

Planos B02B1, inclusive o R$315 distribuído em100+96+119, não são vendas nem
recebíveis. São excluídos de todos os totais por construção da consulta.

## Integração com frontend98

O contrato fornecido do frontend98 separa resposta oficial da conferência97 e
recusa reaproveitar serviço de outro caixa. Esta API fornece a futura consulta
histórica oficial por ID. A ligação do adapter permanece pendente: não foi
criada conversão parcial que fingisse suporte a débito/crédito ou confirmação
eletrônica. Frontend99/malotes/livro local não foram incorporados nem alterados.

## Testes e limites

Testes HTTP com SQLite descartável exercitam encerramento real, ajuste real,
venda em dinheiro e cancelamento pelos serviços existentes, mais consulta após
novo turno e reinício SQLite/servidor, escopo, revogação, ausência de licença,
concorrência de leituras e falhas de histórico induzidas. Respostas reais são
confrontadas com OpenAPI estrito. A matriz geral de isolamento inclui a nova rota.

Fixture de compatibilidade insere explicitamente registros legados de card/PIX
e IDs de parcelas iguais em vendas distintas. Testa leitura/agregação e NÃO
demonstra cobrança, finalização mista, conector de pagamento ou liquidação.

Não foram executados nesta entrega: banco da demonstração, PostgreSQL real,
provedor bancário, fiscal, hardware ou aceite visual do relatório98/99.
Evidências de execução acompanham o pacote; o aplicador repete a regressão
no PC e só faz commit após sucesso. Sem merge ou push.

## Estado e próximos trabalhos

| Entrega | Estado | Pendências |
|---|---|---|
| B02A — produção/saldo livre | Aplicada em b1e3d80 | Integração visual separada |
| B02B1 — plano e distribuição persistida | Aplicada em b882ece; testes do PC aprovados | Solicitação e confirmação de cobranças |
| B02B2 — consulta oficial de caixa encerrado | Código/testes desta entrega | Aplicação no PC; ligação do adapter |
| Venda mista/financeiro/fechamento por meio | Pendente | Débito/crédito próprios, fonte autorizada, lançamentos e reversões |
| R$315 completo | Pendente | Venda, caixa230, parcelas100+96+119 e financeiro conciliados |
