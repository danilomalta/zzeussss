# Geração de sugestões online — entrega 20

Base main 94a43bd. Não modifica SQLite, produção, frontend ou dados reais. Sem migração: reutiliza produtos e o índice parcial PostgreSQL da migração 000003.

## Comportamento real

POST /api/v1/discounts/suggest exige sessão persistida, vínculo atual e papel admin/owner/manager. A empresa vem da sessão; corpo/query não configuram empresa ou critérios. A operação lê produtos ativos em ordem de ID, até 1001 registros. Até 1000 são analisados numa única transação. Se houver 1001, retorna 409 e aborta antes de gravar. Não anuncia análise completa de um catálogo truncado.

O limite é uma proteção operacional desta primeira versão síncrona, não um benchmark ou recomendação de tamanho de empresa. Análise paginada em background ainda precisa de um protocolo de execução próprio. Um catálogo maior não pode usar esta rota até esse fluxo existir.

| Regra existente | Sugestão |
| --- | --- |
| Estoque maior que 50 e demanda média diária menor que 0,5 | 15%, faixa textual 10%–20%, PRODUTO_PARADO |
| Caso anterior não atendido; referência demanda × dias de reposição × 1,5 positiva; estoque maior que duas referências | 10%, faixa textual 5%–10%, EXCESSO_ESTOQUE_VS_GIRO |
| Nenhuma regra atendida | Nenhuma sugestão |

Essas regras são heurísticas legadas. Não são previsão validada, não consideram custo/margem/validade/fiscal e não devem aplicar desconto automaticamente. Demanda omitida permanece zero no modelo existente; isso pode produzir sugestão de baixo giro mesmo sem medição. Validação comercial com dados reais continua pendente.

## Transação, repetição e falhas

O lote só é devolvido depois da confirmação do commit. Consulta ou INSERT com erro faz rollback e não devolve lista parcial. Conflito somente na chave (tenant_id, product_id), onde status=PENDING e deleted_at IS NULL, não cria outro registro. Agora ON CONFLICT cita essa chave e condição: não oculta indiscriminadamente qualquer conflito único. Resposta conta somente registros criados nesta transação, não os ignorados.

Validação adicional recusa produto sem ID ou pertencente a outra empresa antes de qualquer INSERT, mesmo se um adaptador retornar contexto incorreto. A consulta já filtra empresa e ativos; essa checagem não substitui o isolamento SQL.

Resposta 200: message, items_gerados e sugestoes, com array vazio quando nada foi criado. Sugestões são PENDING, sem modificar preço ou saldo. Erro 500 é genérico, sem SQL. Erro na confirmação de commit ou perda de resposta pode significar resultado incerto; não garante ausência de gravação. Não repetir automaticamente. O índice de pendentes não é uma identidade idempotente da análise: se houver revisão/remoção entre chamadas, uma reexecução pode criar outras sugestões.

Não acrescenta auditoria de execução, idempotência por operação, aprovação, cálculo monetário da venda, histórico de preço ou integração com caixa local. A revisão online continua bloqueada pela rota existente. Modelo de demanda e desconto continua float64 legado; a entrega não completa a revisão monetária.

## Evidência e aceite

Testes com SQL simulado exigem filtro por empresa, ORDER BY/LIMIT, alvo exato do conflito, omissão de duplicata na contagem, rollback no segundo INSERT, erro de query/commit sem sucesso parcial, recusa em 1001 e aceite em 1000, recusa de contexto estrangeiro e respostas HTTP 200/409/500 sem SQL privado. Testes existentes de autenticação/papel/revogação permanecem ativos.

Checker de contratos, go test ./..., go vet ./..., race dos pacotes usecase e rotas, git diff --check. SQL simulado não comprova concorrência ou sintaxe efetivamente aceita em PostgreSQL real. O índice e o SQL gerado foram conferidos; aceite de integração PostgreSQL para esta geração permanece separado. Nenhum teste toca banco real do cliente.
