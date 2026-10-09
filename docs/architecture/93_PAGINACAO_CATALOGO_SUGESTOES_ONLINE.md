# Paginação de produtos e sugestões online — entrega 18

Base de aplicação: main `5019567`. As duas listas online consultavam todos os registros de uma empresa sem limite. A entrega acrescenta LIMIT/OFFSET validados antes da consulta de negócio.

## Contrato

| Entrada | Padrão | Limite |
| --- | --- | --- |
| limit | 50 | 1 a 100 |
| offset | 0 | 0 a 1.000.000.000 |
| status, apenas sugestões | PENDING | PENDING, APPROVED, REJECTED |

limit/offset aceitam apenas dígitos, até dez caracteres. Parâmetros desconhecidos, duplicados, vazios, negativos, fracionários ou exponenciais retornam 400. status vazio mantém o comportamento legado PENDING; repetição é recusada.

GET /api/v1/produtos/ continua retornando array. GET /api/v1/discounts/suggestions continua retornando count e sugestoes; count é o número de registros da página. Coleções vazias são arrays vazios. Ordem por id descendente, sem total global ou snapshot consistente entre páginas.

Mudança de comportamento: clientes que esperavam todos os registros em uma chamada agora precisam buscar páginas sucessivas. Parâmetros antes ignorados agora são recusados. O frontend local não usa essas rotas online; os contratos locais permanecem distintos.

## Autorização e limites do escopo

Sessão e vínculo atual são conferidos antes do handler. A consulta SQL preserva tenant_id derivado da sessão. O cliente não escolhe empresa com query. Papéis de consulta permanecem: produtos admin/owner/manager/cashier/stock; sugestões admin/owner/manager.

Não altera POST de criação de produtos, geração ou aprovação de descontos. Revisão de desconto e outras operações inacabadas continuam bloqueadas. Não implementa sincronização local/nuvem. Modelos online ainda têm campos float64 legados; não são equivalentes ao dinheiro exato em centavos do núcleo local. A migração desses campos exige entrega própria.

Paginação limita registros devolvidos; offsets altos ainda podem ser caros. Não representa proteção DDoS ou índice de desempenho medido. Cursor e política de consultas de maior escala permanecem trabalho futuro.

## Verificação

Testes de rotas com SQL simulado conferem empresa correta, parâmetros LIMIT/OFFSET e status, resposta de página e recusa de consulta inválida antes do SQL de negócio. Testes existentes de sessão revogada, papel negado e isolamento continuam ativos. Não executa migrações nem utiliza PostgreSQL comercial.

Rodar testes das rotas/apicontract/middleware, vet, race e verificador de contratos. Testes SQL simulados verificam a consulta construída pelo GORM; não substituem medição ou plano de execução em PostgreSQL real. Arquivos compartilhados com produção não são alterados por esta entrega.
