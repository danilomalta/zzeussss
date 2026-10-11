# Entrega39 — custo declarado e margem online

Base do PC: main, 58a6ddb. Escopo: backend online PostgreSQL; frente de produção,
SQLite, estoque, caixa, financeiro e interface não são modificados.

| Função | Método e rota |
|---|---|
| Consultar custo unitário declarado | GET /api/v1/produtos/:id/cost |
| Registrar ou atualizar custo com revisão e recibo | POST /api/v1/produtos/:id/cost |
| Histórico paginado de alterações | GET /api/v1/produtos/:id/cost/history |
| Recuperar recibo após perda de resposta | GET /api/v1/catalog/costs/operations/:operation_id |
| Calcular margem com preço e custo atuais | GET /api/v1/produtos/:id/margin |

## Estado e precisão

Cada custo pertence a uma empresa e produto. Antes da primeira gravação:
`known=false`, `revision=0`, `cost_cents=null`, `actor_id=null`, `updated_at=null`.
Zero é um custo conhecido válido somente quando registrado explicitamente.
Valores aceitos: 0 a 999999999999 centavos; revisão 1 a 9007199254740991.
O preço e a versão do produto não são alterados ao registrar custo.

A margem é calculada em uma leitura consistente de produto e custo:
`margin_cents = price_cents - cost_cents`.
Quando preço é positivo, `margin_bps = margin_cents * 10000 / price_cents`,
arredondando metade para longe de zero. Percentual = pontos-base / 100.
`margin_bps` é texto decimal inteiro, pois margem negativa com preço pequeno
pode superar o inteiro seguro do JavaScript. `margin_cents` permanece inteiro
seguro. Os estados são `unknown_cost`, `zero_price`, `nonnegative_margin` e
`negative_margin`. Custo desconhecido não produz margem; preço zero produz
diferença em centavos, mas nenhum percentual.

É custo unitário declarado pelo operador, não custo médio contábil ou apuração
de lucro. Tributos, frete, despesas, perdas e descontos de venda não são
calculados nesta entrega. Nenhum preço é reajustado automaticamente.

## Gravação e recuperação

JSON estrito até 8192 bytes com cinco campos obrigatórios:
`operation_id`, `expected_product_version`, `expected_cost_revision`,
`cost_cents`, `reason`. UUID válido, inteiros exatos, motivo não vazio após
aparar espaços e até 500 caracteres. Extras, duplicações, null e frações são
rejeitados. Queries extras são recusadas; GET com corpo também.

A sessão, papel e vínculo são revalidados sob bloqueio no banco. Somente
owner/admin/manager leem ou alteram custos. Empresa nunca vem do JSON/query.
Uma transação bloqueia a chave da operação e o produto antes de comparar
versão e revisão, inclusive na primeira gravação sem linha de custo.
Revisão divergente produz 409; não existe última gravação vence.

Custo, recibo com antes/depois e snapshot do produto, e outbox durável são
confirmados juntos. Uma falha de auditoria ou outbox desfaz a alteração.
A outbox registra intenção de publicação; não afirma entrega de mensagem.
Replay com o mesmo UUID, ator, produto e conteúdo devolve o recibo original,
mesmo depois de outras mudanças. Reutilização incompatível produz 409.
Recibo individual só é consultável pelo ator original; histórico do produto
é consultável pelos papéis autorizados da empresa. Produto/recibo alheio 404;
sessão revogada 401 na API. Nenhuma resposta expõe erros SQL brutos.

Histórico ordenado por created_at e operation_id decrescentes; limit padrão
50, máximo 100; offset máximo 1000000000. Retorna array vazio real quando não
há alterações. Paginação offset entre pedidos distintos pode mudar se houver
novas gravações; não é um snapshot persistente entre páginas.

## Migração explícita

`000017_online_product_costs.sql` cria custo, operações e outbox. Usa FKs
compostas de empresa/produto e empresa/ator. Não altera migrações anteriores.
Histórico próprio com checksum; rejeita checksum alterado ou versão futura.
Pré-requisito: schema de revisão de descontos 16 verificado.
Comandos de manutenção adicionados: `titan-online migrate-costs` e
`titan-online check-costs`. A inicialização da API não aplica DDL.
O instalador do ZIP aplica schema apenas no banco novo isolado de teste;
não executa manutenção comercial. Sem schema17, operações respondem 503.

## Verificação e aceite

Testes de parser, papéis, revogação, recibo adulterado, rollback tardio,
replay, revisão/versão antiga, arredondamento, limites e migração/checksum.
Rotas montadas conferem autorização e entrada. Inventário passa a 136 rotas.
O cenário PostgreSQL opt-in cobre as cinco funções, migração repetida, empresa
alheia, outro ator, concorrência de replay e conflito, rollback de outbox,
snapshot preservado, preço zero e sessão revogada. Um novo app de teste dá
orçamento HTTP independente ao cenário39; o limite real continua habilitado.

Instalador exige main limpa na base58a6ddb, checksum do patch, contratos,
suite Go, vet, race, compilação e testes Python. Depois exige execução real
de `tools/test_online_postgres.py --suite catalog` em banco novo isolado.
Somente após sucesso cria commit. Falha preserva alterações para diagnóstico;
não reaplicar o ZIP automaticamente. Não inicia servidor comercial ou push.

Validação disponível no ambiente de preparação: suite Go (PostgreSQL opt-in
sem URL é skip), vet, race, builds, contratos e testes Python. PostgreSQL
real depende do PC e não pode ser declarado aprovado antes dessa execução.
