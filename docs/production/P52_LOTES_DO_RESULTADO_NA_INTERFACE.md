# P52 — lotes do resultado na interface

Base confirmada: P51 em 9224ca8. Schema SQLite 44 preservado.

A área de produção consulta GET /local/v1/production/results/{id}/lots?offset=N. Totais produzido, atribuído e não atribuído são globais do resultado. A página possui até 50 lotes; a soma parcial jamais substitui o total global. Lotes anulados ficam visíveis sem integrar atribuições ativas. Uma página cheia oferece próxima consulta; não se presume um total de linhas ausente da API.

Quantidade em milésimos da unidade histórica explícita, com inteiros seguros e equilíbrio conferido em BigInt. Unidade unit exige múltiplos de 1000. Metadados validam datas reais de calendário, fabricação e validade ordenadas, código de até 64 bytes e motivo de até 255 bytes. Validade vazia é desconhecida. Não se converte volume em massa. IDs, produto e unidade da página devem coincidir com o resultado solicitado.

Sem escrita, reserva, consumo ou entrada de estoque; quantidade atribuída não é saldo atual vendável. O servidor existente mantém isolamento por empresa/loja/aparelho e autorização manage_production. Não são alterados backend, rotas compartilhadas, migrações, backup ou autenticação. LocalProduction inclui o painel; package.json registra testes do cliente.

Verificação: testes de calendário, precisão, limites, página parcial, anulados, contextos divergentes e respostas recusadas; suíte frontend e build TypeScript/Vite. Aceitação visual no PC continua separada dos testes automatizados.
