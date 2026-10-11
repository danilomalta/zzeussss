# Entrega50 — cinco consultas de compras internas

Base PC main caa7c9d. Usa PostgreSQL22 da entrega49; nenhuma nova migração, SQLite ou alteração na frente de produção.

1. Resumo: contagens de compras draft/approved/cancelled, busca literal, produto e fornecedor. Não soma quantidades de produtos diferentes nem inventa saldo financeiro.
2. Lista: filtros, limit1..100 e offset0..1000000000, ordenação determinística por criação desc e IDdesc, total/has_more/next_offset/hash. Dados de produto, fornecedor e condições vêm dos snapshots.
3. Histórico auditado por compra: gestores owner/admin/manager consultam recibos de todos os responsáveis da própria empresa. Valida recibos, sequência decrescente das revisões, continuidade na página e número de operações igual à revisão atual. No offset0 o recibo mais recente deve coincidir com a compra atual. Recibo individual49 continua restrito ao ator.
4. Conferência: compara integralmente proposta, condições, fornecedor e StockPlan atual com os snapshots. Cada comparação tem booleano próprio. Status matches/changed/parent_unavailable; um produto excluído resulta em current_product=null sem apagar histórico. Can_approve somente draft, revisão disponível e quatro comparações iguais. É informativo neste instante; aprovação49 continua verificando locks/revisão/dados. Não promete aceite do fornecedor.
5. CSV: uma página até100 linhas e256KiB, UTF8/ponto e vírgula, textos perigosos protegidos por apóstrofo; quantidade, preço unitário e total em centavos inteiros, source=internal_declaration, motivo e atores. Headers incluem filtros de paginação, hash e escopo online-internal-purchases. Não é pedido transmitido, documento fiscal ou despesa paga.

Todas as consultas revalidam sessão persistente e empresa em transação repeatable-read. Papéis stock/cashier/employee/accountant não autorizados. Sem corpo; queries extras/repetidas/vazias inválidas recusadas. Busca em nome/SKU histórico e motivo atual trata %, _ e barra invertida literalmente. Fornecedor opcional UUID, produto positivo sem zeros à esquerda. Summary não aceita state/limit/offset.

Cada chamada tem seu snapshot; páginas diferentes são independentes e mudanças concorrentes podem deslocar registros. Não se anuncia exportação global imutável. Fonte ausente não é substituída por dados fictícios. Persistência inválida retorna503, acesso externo404. Sem transmissão, reserva, recebimento, financeiro ou atualização dos snapshots.

Testes: paginação/hash/centavos/CSV, continuidade da auditoria, comparações individuais, dados ausentes, corrupção e permissões, parser rigoroso e rotas montadas. Installer executa suite/vet/race/build/contratos/Python e PostgreSQL real obrigatório em banco novo isolado antes do commit. Cenário real50 verifica as cinco funções, filtros, busca literal exclusiva, exclusão lógica, sessão/papéis/empresa e ausência de mutação nas compras/auditoria/outbox e catálogo.
