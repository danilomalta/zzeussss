# Entrega43 — planejamento declarado do estoque online

Base de aplicação: main `d744454` (entrega42). A frente de produção continua separada.

| Função | Método e caminho | Resultado |
| --- | --- | --- |
| Consulta atual | GET `/api/v1/produtos/:id/stock-settings` | Produto/version, saldo atual e parâmetros registrados. |
| Prévia | POST `/api/v1/produtos/:id/stock-settings/preview` | Antes/depois projetados, falta para mínimo e cobertura exata em horizonte7; nenhuma escrita. |
| Gravação versionada | POST `/api/v1/produtos/:id/stock-settings` | Parâmetros, versão incrementada, recibo e outbox em uma transação. |
| Recuperação | GET `/api/v1/catalog/stock-settings/operations/:operation_id` | Recibo original do próprio ator, inclusive após alterações posteriores. |
| Histórico | GET `/api/v1/produtos/:id/stock-settings/history` | Snapshots, responsável, motivo, timestamp, total/limit/offset/has_more/next_offset. |

## Escopo e permissões

Owner/admin/manager. Empresa/usuário/sessão vêm do servidor; a persistência revalida sessão, vínculo e papel dentro da transação. Stock continua consultando os relatórios42, mas não recebe permissão para editar parâmetros. Histórico pode ser consultado por outro gestor da mesma empresa; recuperação por operation_id é restrita ao ator original. Empresa alheia e produto inexistente retornam404. Sessão revogada é recusada também em replay.

São declarações manuais em produtos PostgreSQL. Estoque mínimo, ponto de reposição e prazo são independentes; nenhum pedido é emitido automaticamente. Não há reserva, baixa, recebimento, integração SQLite/PDV ou previsão estatística. A outbox registra evento durável; entrega externa do evento não é implementada nesta fase.

## Entrada e prévia

Exatamente sete campos JSON: operation_id, expected_version, minimum_units, reorder_point_units, lead_time_days, daily_demand, reason. JSON até8192 bytes; campos extras, duplicados, faltantes, null, tipos incorretos e corpo inválido são recusados. GET sem corpo. Consulta/histórico só aceitam os parâmetros documentados; extras e repetições são recusados.

Quantidades são inteiros0..2147483647. Prazo0..365 dias. Versão esperada1..9007199254740990, impedindo overflow no incremento. Demanda é string decimal exata sem sinal/expoente, até99999999.9999 com até quatro casas. Motivo aparado, obrigatório, até500 caracteres. Demanda normalizada para quatro casas antes do hash: `1` e `1.0000` representam o mesmo valor.

Prévia exige a versão vigente. Projeta a próxima versão, calcula falta=max(mínimo-saldo,0), reaproveita os cálculos exatos da42 e usa horizonte fixo7. Demanda zero retorna cobertura null, nunca infinita ou fabricada. A prévia é snapshot e não trava/reserva saldo para uma gravação futura. Expected_version protege alterações que respeitam catalog_version; alterações externas diretas sem incremento não são cobertas por esse protocolo.

Se a demanda declarada mudar, ultima_previsao_demanda_em é limpa: declaração manual não é nova previsão. Se a demanda for mantida, o timestamp existente é preservado. Variabilidade da demanda não é alterada. Preço, custo, descrição, nome, SKU, estado ativo e saldo não são alterados.

## Atomicidade, repetição e histórico

A gravação obtém lock transacional por empresa+operation_id, verifica replay antes de consultar a versão atual, bloqueia o produto FOR UPDATE e exige expected_version. Atualiza os parâmetros e catalog_version, verifica o estado persistido, grava recibo before/after e insere evento `product.stock-settings.changed` na outbox. Inserção ignorada/falha ou estado inesperado provoca rollback. Até um envio sem diferença nos parâmetros gera uma versão e uma auditoria; repetir o mesmo operation_id normalizado recupera o recibo anterior sem nova versão.

Mesmo operation_id com payload/produto/ator diferente retorna409. Duas operações novas com a mesma versão têm apenas um vencedor. Replay concorrente gera somente um recibo/evento. Resultado incerto após perda de conexão exige recuperar o próprio recibo ou repetir o mesmo operation_id/payload; não inventar outra chave para o mesmo envio.

Histórico usa transação repeatable-read, ordenação created_at DESC/operation_id DESC, limite1..100 (padrão50), offset0..1000000000, total e próxima página. Snapshots permanecem após exclusão lógica do produto. Consulta atual de excluído retorna404, histórico e recibo persistem. Não há exportação CSV deste histórico nesta entrega.

## Schema e operação

Migração PostgreSQL `000018_online_stock_settings.sql`: tabelas online_stock_setting_operations, online_stock_setting_outbox, índice de histórico e ledger online_stock_setting_migrations com SHA256. Requer custos17 verificados. Versões desconhecidas ou checksum alterado são recusados. Não altera migrações SQLite/backup nem a branch de produção.

Comandos de manutenção explícitos `titan-online migrate-stock-settings` / `check-stock-settings`; não existe DDL no startup HTTP. Aplicação comercial dessa migração exige procedimento próprio e não é executada pelo instalador43.

## Verificação e limites do aceite

Unitários: parser/limites/papéis, demanda exata, prévia sem escrita, preservação/invalidação do timestamp, sucesso atômico, outbox com erro/ignorada, UPDATE ignorado, readback adulterado, replay, conflito, versão antiga e snapshots inválidos. Migração: fresh/repetida/checksum/futura. Rotas montadas: autorização, IDs, corpo e parâmetros. Inventário passa de151 para156 rotas.

Teste PostgreSQL opt-in ampliado: cinco funções HTTP, contexto de empresa, replay/ator/versão, paginação, campos protegidos, previsão mantida, rollback por trigger, outbox ignorada, repetição concorrente, disputa de versão, exclusão lógica, sessão revogada e ledger adulterado/futuro. Fixtures com falha ficam limitadas ao cenário; triggers desabilitados logo após validação. Somente banco novo *_test loopback e dados descartáveis.

O ambiente de criação verifica Go/vet/race/builds, contratos e testes do runner. Não possui PostgreSQL para executar esse teste real aqui. O instalador exige que o teste PostgreSQL passe no PC antes de criar commit; skip não comprova sucesso. Não executa push, merge, servidor ou migração comercial. Se falhar, preserva alterações e registra linha/relatório privado para correção, sem reaplicar automaticamente.
