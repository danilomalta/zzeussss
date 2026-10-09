# P25 — Manutencao de fornecedores locais

Base: P24, 0b71562. SQLite 0038_production_supplier_edits.sql, livre na branch.
Nao altera migracoes anteriores. Backup admite explicitamente schema 38,
mantendo schemas 25..37, ValidateSchema, checksums, integridade, FKs e aparelho.

GET /local/v1/purchase-suppliers/:id/edit-state: cadastro atual e revision.
POST /local/v1/purchase-suppliers/:id/update: operation_id, expected_revision,
name, status (active/inactive), reason. Campos obrigatorios, JSON estrito,
sem tenant/store/supplier_id no corpo. IDs 1..128; nome/motivo 1..255 bytes;
revision inicial 0 e maxima 2147483647; a escrita exige expected_revision
menor que o maximo. Nome e motivo sao aparados; controles NUL/CR/LF rejeitados.
Alteracao sem efeito e revisao desatualizada retornam 409.

Escrita exige manage_replenishment e contrato Orders vigente dentro da
transacao. Leitura exige view_orders, aparelho/membro atuais e permite
contrato expirado. Cadastro pertence a empresa E loja; nao e conta remota.
Replay exige mesmo aparelho, ator, loja e pedido canonico. Retorna a revisao
e valores daquele evento; nao sobrescreve o cadastro atual. Criacao e edicao
nao podem reutilizar operation_id entre si no mesmo aparelho.

Mudanca atualiza cadastro, revisao, antes/depois/motivo auditado e outbox
purchase.supplier.edited em uma transacao. IGNORE/ABORT abortam tudo.
Outbox e pendente local, sem importador remoto contratado nesta entrega.
Inativar bloqueia NOVOS pedidos; ordens existentes, snapshots de nome,
itens, aprovacao e replay da criacao continuam preservados. Reativacao
nao cria pedido nem reaproveita uma aprovacao ja usada.

Contrato compartilhado: purchases.go usa purchase_supplier_originals para
replay da criacao, pois o cadastro agora muda. Migracao captura o cadastro
existente como original e novas criacoes gravam o payload inicial atomicamente.
Nao reconstroi alteracoes manuais anteriores a migracao. server.go apenas
monta as novas rotas; validacao minima de backup atualizada separadamente.
Sem estoque/financeiro, sem envio/confirmacao externa ou alteracao da main.

Testes cobrem revisoes, replay apos mudancas, bloqueio/reativacao, snapshots,
escopo/permissoes, campos, rollback e backup/restauracao dos novos dados.
