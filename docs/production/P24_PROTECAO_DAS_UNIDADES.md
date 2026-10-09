# P24 — Unidade somente em produto sem dados dependentes

Depende de P23. Sem migracao: SQLite permanece 37.
POST /catalog/products/:id/update permite mudar unidade apenas quando nao houver
movimentos (mesmo saldo zero), contagens, itens de venda, politicas/sugestoes de
reposicao, itens de compra, versoes de receita como resultado/ingrediente ou
itens de reserva de producao, em TODA a empresa, incluindo outras lojas.
Receita inativa/historica tambem bloqueia a troca. Unidades continuam limitadas
a unit/kg/g/liter/ml/meter. Nao converte quantidades nem infere densidade.
Produto sem uso pode ter sua unidade corrigida com revisao, autorizacao,
idempotencia, auditoria e outbox atomicos de P23.

Compartilhado: catalog/edit.go, regra do escritor P23. Nao altera writers de
venda/estoque/compras: verificacao e edicao usam a mesma transacao SQLite.
Novo tipo de dependencia exige ampliar a regra antes de habilitar seu escritor.
Testes: produto sem uso, replay, outra loja com saldo final zero, politica,
receita inativa e edicao de metadados com a unidade anterior.
Backup de P22/P23 segue testado. Nenhuma mudanca de backup ou migracao em P24.

Fechamento: blocos 1 (estado das receitas) e 2 (edicao protegida do catalogo)
do inventario P21 passam a ter implementacao/testes de backend. Restam seis
blocos abertos: inativacao de produtos, manutencao de fornecedores, ciclo de
compras, recebimento conferido, aceite integrado e fechamento conjunto.
As tres interfaces ainda aguardam implementacao/integracao e aceite.
Essa contagem nao e percentual de esforco nem promessa de seis patches.
