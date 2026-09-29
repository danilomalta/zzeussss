# Fase 5A — Turno de caixa local

`register.Open` e `register.Close` gravam turno, vínculo do operador, fechamento e evento outbox em transações SQLite. Um turno aberto por loja e aparelho; repetição com os mesmos dados é segura. O fechamento registra a contagem declarada antes de retornar diferença e valor esperado ao chamador.

A tabela `cash_sessions` anterior aponta para `users`, enquanto o controle de acesso usa `identities`. Na abertura, a identidade previamente autorizada é projetada para `users` com o mesmo ID, e `cash_session_operators` registra seu vínculo. Essa compatibilidade não autentica ninguém: o chamador deve provar sessão humana e aparelho, usando os IDs apenas depois dessa prova. Aplicativos e APIs ainda não estão ligados a estes serviços; nenhuma venda ou fechamento real está disponível na interface.

A migração 0008 é incremental. O índice de turno único exige que dados legados não contenham dois turnos abertos no mesmo aparelho. Antes de ativar a migração sobre cópias de bancos existentes, auditar essa condição e decidir a regularização, sem apagar dados automaticamente.
