# Fase 5D — Remoção da sincronização simulada

O backend PostgreSQL deixava um worker ativo consultando um endereço externo a cada minuto e registrava no log uma mensagem de sincronização mesmo sem entregar qualquer evento. A inicialização e o código dessa rotina foram removidos. O texto de inicialização da API passa a indicar apenas o endereço da API na rede local, sem anunciar venda móvel pronta.

O outbox e o registro de recibos da Fase 5C continuam armazenados apenas no SQLite do dispositivo. A remoção do worker não liga esse banco ao backend PostgreSQL. A próxima implementação deve estabelecer um destino autorizado, protocolo autenticado e processamento idempotente antes de iniciar um worker real.
