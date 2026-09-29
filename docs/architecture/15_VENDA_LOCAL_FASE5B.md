# Fase 5B — Núcleo de venda local

`sale.Complete` só conclui venda em dinheiro depois de validar a sessão humana, o aparelho aprovado, a loja e o turno de caixa aberto. O valor vem do preço no catálogo SQLite. Quantidade em milésimos e preço em centavos são multiplicados com inteiro exato e arredondados uma vez por linha, meio centavo para cima. A baixa de estoque na gôndola, itens, pagamentos registrados pelo operador, movimento de caixa, identidade da operação e evento outbox entram na mesma transação.

O ID de operação é estável em repetição. Repetir exatamente a mesma solicitação retorna o resultado original, mesmo após mudança de preço. Mudar a solicitação com o mesmo ID dá conflito. A venda exige saldo local suficiente e um turno do mesmo operador e aparelho. O payload outbox contém linhas com preço capturado no momento da venda e o valor total para sincronização futura.

Cartão, PIX, troco, descontos, fiscal, cancelamento, confirmação de pagamento e sincronização continuam indisponíveis. O dinheiro fica registrado como confirmado pelo operador, sem conciliação externa. Ainda não existe API local nem tela ligando estes serviços ao PC ou celular; a prova de login humano offline e o worker de outbox devem vir antes da operação real. Não aplicar migrações deste pacote no banco real sem ensaio sobre cópia descartável.
