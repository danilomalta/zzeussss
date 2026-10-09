# PostgreSQL isolado: catálogo e sugestões — entrega 21

Base main 4decda6. Acrescenta teste de integração e seleção de suíte na ferramenta existente, sem mudar handlers, migrations SQL, frontend, produção ou banco do cliente.

## Execução e isolamento

`python3 -B tools/test_online_postgres.py --suite catalog` usa o PostgreSQL local, padrão 5432. Cria um papel aleatório restrito com senha válida por uma hora e um banco novo terminado em `_test`. Não carrega `.env`, não usa credenciais da aplicação e não seleciona bancos existentes. As credenciais temporárias viajam por stdin/environment, nunca nos argumentos ou relatório. A seleção padrão `sessions` conserva a validação anterior.

O teste exige loopback e nome `_test`. Cria um schema exclusivo `titan_catalog_<UUID>`, configura search_path e preserva fixtures ao terminar. Usa as migrações aditivas existentes 000002/000003 e o DDL existente de sessões 000005. Referências `public.` da migração 000003 são redirecionadas para esse schema exclusivo; a estrutura de tabelas, índices e constraints permanece a original. Não executa 000001_init.sql, DROP ou TRUNCATE.

Banco, papel e schema de teste ficam preservados para inspeção. Isso não constitui uma instalação comercial ou nova migração na aplicação. O teste não acessa SQLite da demonstração.

## Critérios de aceite

| Verificação | Evidência exigida no banco real |
| --- | --- |
| Cadastro HTTP | 201, ID persistido, tenant derivado da sessão |
| Dinheiro | NUMERIC conserva 2.50 e 9999999999.99; 0.001 retorna 400 |
| SKU | Mesmo SKU na mesma empresa retorna 409; em outra empresa retorna 201 |
| Contexto | FK composta rejeita produto de outra empresa; sugestões da empresa A não aparecem na B |
| Concorrência | Duas gerações simultâneas criam uma única sugestão pendente |
| Reexecução | Sugestão já pendente não entra em items_gerados |
| Atomicidade | Trigger de falha no último produto desfaz sugestão gravada antes dele no mesmo lote |
| Limite | 1001 produtos ativos retornam 409 sem sugestões novas |
| Revogação | Sessão revogada recebe 401 e não cria produto |

O teste identifica exceções da FK pelo SQLSTATE 23503. Usa sessões e tokens somente de fixture, com assinatura de teste. Não testa login do cliente, frontend, sincronização, aplicação de descontos, fiscal ou desempenho com carga real.

## Resultado e falhas

A ferramenta exige eventos JSON de execução e aprovação do teste exato `TestPostgresCatalogCreationAndDiscountAtomicity` e do pacote de rotas. Skip, teste diferente, falha ou ausência de aprovação não valem como sucesso. Relatório privado contém status, etapa, suíte e recursos de teste; não contém senha, URL ou logs brutos.

Quando possível, uma falha registra `failure_location`, apenas nome de arquivo permitido e número da linha. Não copia texto do erro, SQL ou entradas. O instalador aplica o patch, testa Go/vet/race e a ferramenta Python, exige o PostgreSQL real e só então registra o commit. Se falhar, preservar as alterações e não reaplicar.

Nesta preparação, testes Go sem banco, vet, race e testes unitários da ferramenta foram executados. O PostgreSQL real não estava disponível no ambiente de preparação: esta entrega não declara esse aceite aprovado. A execução obrigatória no PC é a evidência final.
