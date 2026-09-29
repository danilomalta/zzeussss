# Fase 6A — Contagem física local

`inventory.Count` recebe produto, local, quantidade física e ID estável da operação. Com autorização de estoque, lê o saldo dos movimentos naquele dispositivo, grava a contagem, um ajuste se houver diferença e um evento outbox na mesma transação SQLite. Repetir a mesma operação não altera o saldo. Produto ou local de outra empresa/loja e aparelho revogado são rejeitados.

Uma contagem isolada é **a observação de um dispositivo**, não o saldo global garantido quando PC e celular venderam separadamente sem comunicação. Ainda faltam sessão humana offline autenticada, interface, conciliação entre dispositivos e aprovação de divergências acima do limite configurado. Não aplicar migrações a bancos de usuários sem ensaio em cópia descartável.
