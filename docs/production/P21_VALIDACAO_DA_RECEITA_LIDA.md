# P21 — Validacao da receita armazenada

Depende de P20. Branch feat/producao-receitas, sem migracao: SQLite 35.

O leitor compartilhado scanVersion valida os campos, ingredientes, unidades,
quantidades e limites da receita armazenada, alem da relacao expected_revision
com a revisao preservada. GET por ID confere tambem o ID interno do snapshot
contra o ID solicitado. Divergencias retornam 409 em vez de publicar receita
aparentemente valida. JSON sintaticamente ilegivel continua erro de leitura.

Compartilhado: production/recipes.go, usado nas leituras e novos planos.
Isso nao compara snapshots historicos com o catalogo atual: versoes antigas
continuam legiveis apos novas publicacoes ou alteracoes posteriores do catalogo.
Nao e prova criptografica contra alteracao de SQLite. Nao cria auditoria, muda
contrato/autorizacao, estoque, backup ou migração.

Testes: rendimento zero, revisao divergente, unidade desconhecida, duplicidade
de ingrediente, identidade interna divergente, versoes antigas e ausencia de
escrita. Suíte, vet e race acompanham a entrega.
