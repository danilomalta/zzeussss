# Fase 7F — Catálogo local pela API

As rotas protegidas `/local/v1/products` e `/local/v1/locations` permitem cadastrar e listar dados do SQLite. O `tenant_id`, loja, aparelho e identidade vêm do token verificado pelo processo local. Custo continua oculto conforme papel no serviço de catálogo.

Esta é a primeira leitura e escrita visível da API do dispositivo. O frontend ainda precisa usar estas rotas. As criações de catálogo ainda não geram evento outbox; acrescentar antes de prometer atualização automática para outros caixas.
