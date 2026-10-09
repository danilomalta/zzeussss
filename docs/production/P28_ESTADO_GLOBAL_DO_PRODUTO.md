# P28 — Estado ativo/inativo do produto por empresa

Base: P27, f5cd65c. SQLite 0040_production_product_state.sql livre nesta
branch; backup passa a admitir explicitamente 40, preservando 25..39,
checksums, ValidateSchema, integrity_check, foreign_key_check e aparelho.
Nenhuma migracao antiga, criptografia ou formato de arquivo foi alterado.

GET /local/v1/catalog/products/:id/state: product_id, status, revision,
repeated=false. Exige view_catalog com membro/aparelho/loja atuais; pode ler
apos expirar contrato. Produto sem estado salvo e active, revision=0.
POST no mesmo caminho: operation_id, expected_revision, status, reason.
Exige manage_stock e contrato Inventory vigente na transacao. IDs 1..128,
revision 0..2147483646, motivo aparado 1..255 bytes sem NUL/CR/LF. JSON
estrito ate 4096 bytes; query, campos extras e nulos rejeitados. Motivo e
revisao obrigatorios; no-op/revisao antiga/replay divergente retornam 409.

Estado e GLOBAL a empresa, como o cadastro de produto. Permissao de
manage_stock na loja autorizada permite altera-lo para todas as lojas.
Revisao do estado e separada da revisao da edicao P23. Replay igual retorna
a decisao original sem reverter o estado atual. Atualiza estado/revisao,
auditoria antes/depois/motivo/ator/aparelho/loja e outbox atomicamente.
Evento catalog.product.state.changed e pendente local, sem importador novo.

Contrato dos escritores compartilhados: verificam estado na MESMA transacao
apos replay e autorizacao. Produto inativo gera 409 em NOVAS vendas, versoes
de receita (ingrediente ou resultado), ordens (ingrediente ou resultado),
pedidos de compra, politicas/sugestoes de reposicao e aprovacao de sugestao.
Rejeitar sugestao inativa permanece permitido. Replay nao reabre operacao.
Nao oculta, exclui, altera snapshots, baixa ou reserva estoque.

Permitidos: consultas historicas, replay autorizado, edicao de metadados
(com a protecao de unidade da P24), ajustes/contagens/transferencias de
estoque e devolucao de venda, sempre com autorizacao e protecao de saldo.
Ordens planejadas ANTES da inativacao podem reservar/consumir/concluir:
o estado nao cancela compromissos existentes. Capacidade segue consulta
historica; capacidade positiva nao autoriza criar ordem com produto inativo.
Produto inativo nao e produto em quarentena: bloqueio fisico/recolhimento
exige outro contrato, nao deve ser inferido deste estado.

Arquivos compartilhados: server.go/catalog.go (rotas e erro 409), sale.go,
production/recipes.go e orders.go, replenishment/policy.go, suggest.go,
review.go e purchases.go. Nova regra foi adicionada, sem mudar auth/sessoes.
Novos escritores comerciais devem usar RequireActiveProductTx apos replay.

Testes cobrem ingrediente/resultado, venda e devolucao, reposicao/aprovacao,
compra, reativacao/replay, permissao/escopo, rollback e backup/restauracao.
