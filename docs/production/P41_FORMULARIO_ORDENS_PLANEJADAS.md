# P41 — Planejamento visual de ordens

/local/production carrega uma versao pelo ID, preserva referencia/revisao,
mostra rendimento e ingredientes planejados antes de criar. Batidas inteiras,
quantidades multiplicadas com BigInt, resultado limitado ao inteiro seguro.
Local e responsavel explicitos; padrao do responsavel e operador da sessao,
sem pressupor assinatura/aprovacao por ele. Backend valida receita ativa, unidades,
permissao atual do responsavel e local. Criar nao reserva nem consome estoque.

Utiliza a fila unica P40, mesmos IDs/payload em consulta/repeticao. Resultado
original de criacao permanece planned, mesmo apos aprovar/concluir. ID da ordem
confirmada exibido no formulario. Consulta lista atual distinta do recibo original.

Sem migracao, schema44 e backup inalterados; backend exatamente P40.
Compartilhados: LocalProduction acrescenta formulario, package.json teste.
Testes frontend100 e build aprovados localmente, preview exato/overflow,
responsavel/referencias, payload de planejamento e confirmacao sem novoPOST.
Sem DB/servidor reais. Aceite visual nao executado. Executar ordem/material/result
permanece fora do formulario de planejamento; proxima etapa altera apenas estado.
