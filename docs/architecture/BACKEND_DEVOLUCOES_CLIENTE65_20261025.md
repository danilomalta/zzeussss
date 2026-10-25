# Devolução parcial de venda em dinheiro — integração65

Base: `3ab18e1e600930365dd7e4949479570f463948bb`, branch `feat/backend-b02a-compat`,25/10/2026. Frontend, serviços8182/8282 e bancos existentes preservados.

## Origem conciliada

A função veio do pacote do responsável anterior `TitanSystem_Backend_Devolucoes_Base7b37966_20261024_INTEGRO.zip`, SHA-256 `7219facef9247701182b849e0300bb42e5ee6bcb5b1bf80122582b670c447330`, base7b3796662c9e455c446c3756b19807c486ff694c. Não foi executado seu aplicador, instalador ou script de commit. Não foram copiadas as centenas de dependências do snapshot, binário antigo, marcadores ou configuração.

Somente os31caminhos de fonte/teste/contrato/documentação declarados foram comparados contra o ancestral real e os fontes atuais. Dois conflitos: histórico de compatibilidade do backup e inventário de rotas. Conciliação preserva as13operações de pedidos autenticados, acrescenta quatro operações de devolução e aceita schemas25–65. A documentação do aplicador antigo ficou somente como referência no laboratório; este guia descreve a integração atual.

A migração64 de pedidos permanece byte a byte igual, assim como os SQL1–63. `0065_customer_returns.sql` acrescenta duas tabelas imutáveis e atualiza transacionalmente a view derivada do livro de caixa; nenhum registro de journal é reescrito. Probes artificiais usam66. Teste64→65 preserva uma contraparte revogada, a geração de bloqueio de recuperação e cada checksum histórico. Não aplicar o pacote antigo sobre essa base. Não há downgrade automático nem alteração nos bancos dos serviços preservados.

## Operação real

- `GET /local/v1/sales/:id/returns`: histórico e quantidades/centavos restantes, sem apresentar recorte como total.
- `POST /local/v1/sales/:id/returns/quote`: cotação pelos preços originais e quantidade ainda disponível; nenhum efeito comercial.
- `POST /local/v1/sales/:id/returns`: responsável com permissão `cancel_sale` e licençaPOS confirma itens, destino `restock` ou `no_restock`, motivo e total esperado. Resposta201; repetição idêntica200 com recibo original; divergência409.
- `GET /local/v1/sale-return-operations/:id`: recuperação pelo identificador original, sem criar reembolso novo.

A operação grava dinheiro, linhas, movimentos de estoque quando autorizados, livro de caixa e outbox na mesma transação. Falha ou escrita ignorada em qualquer etapa causa rollback. A venda e o pagamento originais permanecem registrados; devolução não é cancelamento da venda, reversão eletrônica ou cancelamento fiscal. Cancelamento integral posterior não pode devolver as mesmas quantidades novamente.

Valores em centavos e quantidades em milésimos inteiros; arredondamento proporcional cumulativo faz parcelas fracionadas fecharem no total original. Preço atual não muda o reembolso. Reposição exige conferência humana de aptidão; `no_restock` não devolve mercadoria avariada ao saldo vendável. O backend não realiza inspeção sanitária automática nem reconstrói lotes não registrados na venda original.

O reembolso novo usa somente a gaveta original ainda aberta, no mesmo aparelho, com saldo físico suficiente. Outro turno não altera fechamento já emitido. Recuperação e leitura permanecem após fechamento; operação incerta não é apagada ou reenviada por troca de sessão. Histórico exige permissão vigente e mantém contexto de empresa/loja/aparelho. Recuperação não burla revogação ou licença para criar novos efeitos.

Fechamento e exportação contábil distinguem `returned_sales_cents`, `customer_refund_cents`, venda bruta, cancelamento e venda líquida. O dinheiro sai uma única vez. Interface que calcula líquido apenas pelo status precisa consumir a nova métrica; nenhuma tela foi alterada nesta unidade.

## Aceite automatizado

Novo `TestPilotCashCustomerReturnRestartAndRestore` estende o circuito existente com fornecedor autenticado e recebimento parcial:5000−2000+1000+1000=5000milésimos de estoque; caixa1000+3198−1025−1599=1574centavos; venda bruta3198, devolvido1599, líquido1599, obrigação do fornecedor em aberto0. Uma devolução, uma linha, um débito de caixa. O recibo é recuperado depois de queda do processo; mesmos IDs depois de reinício/restauração e reenvio explícito, sem duplicação. Nova devolução após fechamento é recusada. A tratativa de avaria permanece em andamento.

Comércio:148requisições HTTP em seis processos; fornecedor:8em três processos independentes, banco/chave/sessão próprios. Total156requisições/nove execuções da API. Teste funciona com biblioteca de backup ou `-pilot-backup-cli=/caminho/absoluto/titan-backup`; o CI exige executável compilado e detector de corridas. Não usa mock de negócio, banco comercial ou navegação simulada para declarar persistência.

As18regressões HTTP importadas incluem permissões, campos ambíguos, conflito de IDs/corpo, duas conexões concorrentes, seis pontos de rollback, valor histórico, quantidade excessiva, produto sem reposição, reinício, histórico danificado, licença e coexistência com manutenção/recusa. São testes existentes da entrega anterior, agora conciliados com esta base, além do piloto e da prova ampliada de migração. Resultados finais devem ser lidos no CI e no registro único de continuidade.

O perfil de inspeção `pilot_cash_v1` conserva suas14tabelas históricas; não prova sozinho as duas tabelas novas. O piloto relê ambas por HTTP e confere suas contagens na cópia restaurada. Backup é integral e cifrado; assinatura de pedidos e bloqueio de confiança na restauração continuam ativos.

## Correções de validação encontradas nesta integração

O verificador anterior não resolvia referências reutilizadas e pressupunha `operationId` em toda operação, embora seja opcional. Agora resolve Path Item, operação e parâmetro, preserva herança e precedência, rejeita ciclos, exige identificadores únicos quando presentes e confere o inventário inteiro. Dez regressões Python cobrem esses casos, inclusive parâmetro obrigatório omitido e referência inexistente.

O mapa central tinha84operações existentes sem ligação com os contratos detalhados; somadas às quatro novas, foram acrescentadas88ligações, sem recriar seus esquemas. As referências são reutilizadas como Path Item quando possível; três aliases de nome de parâmetro mantêm seus esquemas originais referenciados. Foram corrigidas100declarações de parâmetros de caminho no mapa anterior e uma no contrato de fechamento. São correções documentais de operações existentes, não84novas funções. O mapa de527rotas e todas as referências JSON passam no verificador, agora obrigatório no CI.

A primeira suíte completa detectou uma corrida no auxiliar de prontidão dos pilotos: arquivo legível ainda vazio produzia URL vazia. O filho agora publica a URL por rename atômico e o pai exige URL completa de loopback/porta válida. Não foi ignorada a falha: ela foi preservada no laboratório, o cenário foi repetido cinco vezes e a suíte inteira reexecutada. Essa correção é do ensaio subprocessado, não de uma operação comercial.

## Limites desta função

Não inclui devolução em outra gaveta/turno/aparelho, crédito ao consumidor, cartão/Pix, documento fiscal, frontend ou implantação comercial. Esses limites são recusas explícitas, não sucessos fictícios. Esta integração resolve uma entrega antes bloqueada por conflito de base, sem declarar todo o backend comercial concluído.
