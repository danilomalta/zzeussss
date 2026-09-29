# Sincronização ainda sem transporte

O ping externo e o worker simulado foram removidos do `cmd/api`. O processo PostgreSQL não abre o SQLite de um dispositivo e não deve anunciar sincronização concluída.

O armazenamento local de eventos pendentes e recibos verificáveis está em `internal/localdb/outgoing`. Ainda faltam transporte autenticado, persistência idempotente no destinatário, verificação de recibo real, configuração explícita de destino e integração com aplicativos e API local. Até isso existir, nenhum worker deve confirmar eventos apenas por conectividade de rede.
