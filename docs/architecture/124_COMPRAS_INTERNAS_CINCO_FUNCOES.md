# Entrega49 — cinco funções de compra interna

Base main8e9855b, migração PostgreSQL000022. Não altera produção/SQLite.

1.Criar compra interna a partir de proposta45 aprovada e vínculo48: revisões esperadas de ambos, quantidade da proposta, mínimo atendido, fornecedor/condições ativos e produto atual igual ao snapshot aprovado. Um registro por proposta, inclusive após cancelamento; para outra intenção de compra, criar uma nova proposta. Quantidade não é arredondada silenciosamente.
2.Consultar registro por UUID na empresa, com snapshots de proposta, condições e contato. Estado real draft/approved/cancelled e source=internal_declaration.
3.Aprovar internamente com revisão/motivo, revalidando proposta, condições, fornecedor e produto/estoque sob locks. Divergência409. Não troca snapshots para esconder mudanças.
4.Cancelar draft/approved com revisão e motivo, preservando histórico mesmo após alteração/inativação/exclusão lógica de pais. Não há operação para envio externo nesta entrega, portanto este cancelamento é exclusivamente interno.
5.Recuperar recibo histórico de operação do próprio ator. Replay exato retorna before/after original, sem duplicar auditoria/outbox e sem transformar estado atual.

## Valores e significado

Total BRL em centavos = quantidade inteira da proposta × preço unitário declarado48. Limite9007199254740991, multiplicação verificada antes de executar, sem float nem overflow. Mínimo da condição precisa ser atendido; não converte embalagens/unidades. Preço zero é gratuito declarado. Não soma frete/tributos nem inventa desconto/pagamento. Não atualiza preço de venda/custo/estoque.

Approved significa decisão do gestor comprador, nunca aceite do fornecedor. source explicita origem interna. Nenhum vínculo remoto autenticado, cotação confirmada, pedido transmitido, reserva de estoque/doca, recebimento, financeiro ou pagamento foi implementado aqui. Outbox na mesma transação é registro durável, não prova de entrega externa. Uso posterior para comunicação/compras precisa autorização de compartilhamento e aceite do fornecedor.

## Consistência, migração e segurança

Owner/admin/manager, tenant da sessão, revalidação persistente com locks. FKs compostas proposta/condições/atores, par proposta único, operação idempotente, revisões pessimistas, auditoria/outbox verificadas por readback e rollback integral em falha/trigger ignorado. Recibo só do ator; registro consultável por gestores da empresa; empresa alheia404. Aprovação depende de igualdade completa dos snapshots correntes, inclusive estoque e contato, e estado draft. Cancelamento approved é possível porque nada foi enviado externamente.

CLI migrate-purchase-drafts/check-purchase-drafts após supplier-offers21. Ledger22 com SHA256 e lock, rejeição de histórico desconhecido/checksum divergente, DDL transacional. API não migra ao iniciar. Instalador usa banco novo isolado, não banco comercial.

## Verificação

Local: JSON estrito/limites/total exato/overflow/mínimo, estados e snapshots, rotas/papéis, transação/replay/rollback/readback/auditoria/outbox e cancelamento, migração fresh/replay/futuro/checksum, suiteGo/vet/race/builds, contratos186 rotas e14 testes Python. PostgreSQL real obrigatório no PC antes do commit: total6170centavos, fluxo criar/aprovar/cancelar e recuperar original, proposta única/replay por ator, foreignoffer/empresa/papéis, proposta não aprovada, oferta alterada, corrida de decisões, outbox ignorada rollback integral, estoque alterado bloqueia aprovação, cancelamento após exclusão lógica e catálogo/custo/estoque preservados. Preparação sem PostgreSQL disponível; testes reais pendentes até instalador passar.
