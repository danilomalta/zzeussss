# Cadastro online de produto — entrega 19

Base de aplicação: main 021c5f1, após paginação online. Esta entrega reforça POST /api/v1/produtos/ existente, sem iniciar servidor, migrar banco, alterar frontend ou incorporar produção.

## Entrada e compatibilidade

Content-Type application/json obrigatório; charset permitido. Um objeto de até 8192 bytes, UTF-8 válido, sem chaves repetidas, campos desconhecidos, null ou conteúdo posterior. Formulários anteriormente aceitos pelo BodyParser deixam de ser aceitos. A rota continua exigindo sessão online persistida, vínculo vigente e papel admin/owner/manager/stock. A empresa vem da sessão; tenant_id enviado no corpo resulta em 400.

| Campo | Regra |
| --- | --- |
| nome | Obrigatório, aparado, 1–255 caracteres Unicode |
| sku | Obrigatório, aparado, 1–100 caracteres Unicode; unicidade por empresa no banco |
| descricao | Opcional, até 2000 caracteres, padrão vazio |
| preco | Opcional, padrão zero, número decimal de 0 a 9999999999.99; até duas casas; sem expoente, sinal ou string |
| estoque | Opcional, padrão zero, inteiro escrito somente com dígitos, de 0 a 2147483647 |

Caracteres de controle são recusados nos três textos, inclusive tabulação, quebra de linha e NUL. Os limites de nome/SKU, preço e estoque correspondem à migração PostgreSQL 000002. Limite de descrição e corpo são limites desta API. Não executamos a migração inicial.

O preço é lido como token decimal, validado em centavos inteiros e só então convertido para o float64 do modelo existente. PostgreSQL continua com NUMERIC(12,2). Isso evita arredondar entradas como 0.001 silenciosamente; não completa a substituição do modelo monetário legado nem muda cálculos de desconto existentes.

## Resposta e limites reais

201 conserva o objeto Product existente: ID/timestamps do GORM, campos de produto, sem tenant_id. Ativo é definido pelo servidor. Erros de entrada retornam 400 genérico, sem refletir dados enviados; falha de gravação permanece 409 genérico. A negociação opcional de erros existente é preservada.

Não há novo protocolo de idempotência, histórico de preço, aprovação, auditoria administrativa do cadastro ou sincronização comercial. Estoque inicial permanece campo legado, não recebimento conferido ou movimento auditado. Em resposta perdida, não repetir automaticamente a criação: esta API não consegue confirmar operação por identificador. Não é correto anunciar esses itens como concluídos por existir documentação.

## Validação

Testes do decoder: valores máximos, centavos, defaults compatíveis, comprimento Unicode, rejeição de arredondamento, overflow, duplicatas, campos extras, null, controles, UTF-8 inválido e media type errado. Testes HTTP com SQL simulado: sessão verificada, campos inválidos recusados antes de INSERT, empresa derivada da sessão, papéis e revogação preservados. Erros privados do banco não são incluídos no corpo de resposta.

Comandos: checker de contratos; go test ./...; go vet ./...; go test -race nos pacotes de catálogo/delivery, rotas e middleware; git diff --check. O script de aplicação só registra commit depois dos checks. Não altera dados reais nem inicia PostgreSQL. Testes SQL simulados não comprovam execução de INSERT em PostgreSQL real; esse aceite permanece separado.
