# Entrega51 — documentos internos de compras

Base PC main6901434. Migração incremental PostgreSQL23, dependente da22, com ledger próprio/checksum/lock e DDL transacional. Sem nova migração SQLite ou alteração na produção.

1. Preparar documento: compra approved com revisão esperada; confere compra, proposta, condições, fornecedor e produto sob locks. Preserva compra completa e autor/data da preparação. Um documento por compra mesmo após cancelamento. Sem edição/regeneração nesta entrega.
2. Consultar: mostra prepared/cancelled, revisão, file e SHA256, motivo e atores. File é histórico e não é substituído por dados atuais.
3. Cancelar: prepared1 para cancelled2 com motivo, revisão esperada e recibo idempotente; não depende dos pais atuais e não cancela a compra. Mantém file/digest/autor/data originais. Cancelado não volta a prepared.
4. Download JSON: file UTF8 estável e até64KiB, kind=internal_purchase e format_version1, header SHA256 dos bytes exatos, attachment e no-store/nosniff. Prepared pode ser baixado depois de mudanças/cancelamento da compra: é documento histórico, não comprovação da validade atual. Consulte a conferência50 para situação atual. Cancelled bloqueia download409; file continua disponível para auditoria na consulta e no recibo. SHA256 é integridade, não assinatura ou aceite externo.
5. Recuperar operação: recibo do próprio ator da mesma empresa, preservando resultado original após perda da resposta, cancelamento ou alterações nos pais. Gestores podem consultar o documento da própria empresa; recibo individual de outro ator retorna404.

Writes revalidam sessão persistente, usam advisorylock por operação, locks dos pais/revisão, readback de documento/recibo/outbox, RowsAffected e rollback atômico. Eventos purchase.document.prepare/cancel são internos; nenhum worker ou rede os despacha. Concorrência de preparações para a mesma compra resulta em um documento e409 nos demais. Replay divergir em ator/intenção é409.

Only owner/admin/manager. Sem query; GET sem corpo; JSON até8KiB, campos desconhecidos/repetidos/null/trailing recusados. Sessão revogada401/ErrDenied, empresa externa404, falhas internas503 sanitizadas. Não modifica preço/custo/estoque/compra/proposta/fornecedor. Nenhum pedido é transmitido ou aceito pelo fornecedor, sem reserva, doca, recebimento ou financeiro. O documento é preparação persistente para integração posterior.

Testes locais: entrada rigorosa, bytes/hash, snapshots adulterados, cancelamento, replay, estados, permissões, readbacks ignorados/adulterados e atomicidade; ledger/checksum/futuro, rotas montadas. Installer suite/vet/race/build/contratos/Python e PostgreSQL real em banco novo antes do commit. Cenário real obrigatório inclui cinco funções, checksum23, arquivo exato após mudança de estoque, empresa/ator/papel/sessão, concorrência, falha outbox e rollback completo, pais divergentes e compras/catálogo preservados. PostgreSQL não executado no ambiente de preparação.
