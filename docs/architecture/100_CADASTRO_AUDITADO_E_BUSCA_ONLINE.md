# Cadastro auditado, recuperação e busca — entrega 25

Base main 249fe51, após catálogo online 24. Exclusivamente PostgreSQL/backend.
Não incorpora produção, nem altera SQLite, frontend, banco comercial ou portas.

## Quatro funções

| Função | Contrato | Resultado |
| --- | --- | --- |
| Cadastro idempotente | POST /api/v1/produtos/ | Corpo anterior, novo Idempotency-Key obrigatório; 201 com produto original inclusive no replay |
| Auditoria da criação | GET /api/v1/produtos/:id/creation | Operador, operação, snapshot persistido (incluindo estoque inicial) e horário |
| Busca e filtros | GET /api/v1/produtos/search | Nome/SKU, q até 120 caracteres, active=all/active/inactive, limit/offset |
| Recuperação do resultado | GET /api/v1/catalog/creations/:operation_id | Recibo da própria empresa/operador, também após novo login |

Empresa, usuário e papel vêm da sessão. Cadastro: owner/admin/manager/stock;
busca: também cashier; auditoria por produto: owner/admin/manager. Consulta
de operação: owner/admin/manager/stock, restrita ao ator original. Não existe
atribuição de empresa/papel pelo corpo ou query; política departamental local
continua separada desses papéis online.

## Mudança de contrato e compatibilidade

O corpo continua nome/sku obrigatórios, descricao opcional, preco decimal
não negativo com até duas casas e estoque inteiro não negativo. A entrega 19
já valida esses campos; agora conserva centavos exatos até a escrita SQL.
O adaptador float legado não participa da persistência do novo cadastro.

Novo requisito: header Idempotency-Key com UUID canônico não nulo. Cliente
deve gerar e guardar chave/conteúdo antes da requisição. Ausência/invalidade
retorna 428, sem iniciar cadastro. Não existe rota antiga sem esse requisito
para contornar a transação. Essa é uma mudança deliberada do contrato online:
o frontend online antigo precisa acrescentar e persistir a chave antes de
voltar a cadastrar; frontend local e APIs locais não são afetados.
CORS admite o novo header apenas nas origens já configuradas.

201 mantém ID/nome/descricao/sku/preco/estoque/ativo/CreatedAt/UpdatedAt reais
e acrescenta price_cents, version e operation_id. Preco sai como número JSON
decimal exato; o consumidor pode preferir price_cents. Campos estatísticos
adicionais do antigo modelo GORM não são prometidos nessa resposta básica.
CreatedAt/UpdatedAt vêm do RETURNING do produto; o recibo possui seu próprio
horário de auditoria. Não fabricar ID, preço, saldo ou confirmação.

## Transação, incerteza e auditoria

Revalidar sessão/vínculo/empresa ativa sob bloqueios compartilhados; bloquear
identidade da criação; consultar recibo; inserir produto com RETURNING;
validar os valores persistidos; inserir auditoria/snapshot; inserir outbox;
confirmar. Falha de escrita, auditoria, evento ou commit não retorna sucesso.
Ignorar INSERT de evento por trigger também causa rollback, via contagem de linhas.

Repetição do mesmo ator/empresa/chave/conteúdo normalizado devolve o resultado
original. Nome/SKU são aparados antes do hash. Mesma chave com conteúdo ou
ator diferentes retorna 409. SKU continua único por empresa. Duas chamadas
concorrentes com a mesma identidade geram um produto, um recibo e um evento.
Recebimento do commit sem resposta HTTP é incerto: consultar o recibo; não
inventar nova chave nem reenviar automaticamente. Uma falha de consulta ou
404 isolado não é autorização para criar outra identidade. Repetição explícita
deve conservar chave/conteúdo. O snapshot de criação não acompanha edições;
GET /produtos/:id devolve o estado atual.

Namespaces explícitos: alterações usam catalog/operations; cadastros usam
catalog/creations. Não interpretar uma chave de edição como recibo de criação.
Auditoria não contém senha, token ou URL de conexão. Criações anteriores à
ativação não ganham histórico retroativo; auditoria desse produto retorna 404.

Estoque inicial é o campo legado online e fica no snapshot; não equivale a
movimento de estoque do núcleo local. Outbox é intenção durável e não tem
worker/ack nesta entrega. Nenhum preço/cadastro é anunciado como recebido
pelos caixas, smartphone ou SQLite.

## Busca

Consulta somente nome/SKU, sem diferenciar maiúsculas; acentos não são
normalizados e não há correção de digitação. %, _ e barra invertida são
escapados como caracteres literais. Valores são vinculados, nunca SQL livre.
Produto eliminado logicamente não entra no resultado; ativo/inativo segue
filtro explícito. Padrão all. Ordenação id descendente, limit 50 padrão/100
máximo e offset até 1000000000. items=[] é estado vazio real. Não confundir
quantidade da página com total do catálogo. Paginação por offset pode mudar
com novas inserções; não representa snapshot de múltiplas páginas.
Query duplicada/desconhecida ou seletor de empresa é recusado. Não promete
fornecedor, marca, código de barras ou busca fiscal inexistentes nesse modelo.

## Migração e ativação

PostgreSQL 000009 cria online_catalog_creations e
online_catalog_creation_outbox, com FKs compostas por empresa e auditoria
única por produto. Pré-requisito: esquema online/catalog management 8 e índices
compostos de produtos/usuários das migrações 3 e 5. Nenhuma migração SQLite.

A entrega 8 tem seu checker/histórico já publicado. A extensão 9 usa
online_catalog_creation_migrations próprio para preservar compatibilidade
do checker anterior. Versão/checksum únicos, rejeição de histórico futuro/
adulterado, lock de manutenção e DDL/marker transacionais. Não modifica os
SQLs antigos nem seus checksums. Nenhuma migração de segurança é acrescentada.
titan-online migrate-catalog agora instala/verifica as duas extensões;
check-catalog verifica ambas. Não há downgrade destrutivo automático.

Aplicador não toca no banco comercial. Ativação é manutenção explícita pelo
operador na conexão correta, após revisar backup/configuração existente:

```
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online migrate-catalog
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online check-catalog
```

Executar no diretório backend. Comando pode carregar .env; não publicar seu
conteúdo. Nunca executar 000001_init.sql. Sem extensão, cadastro com chave
válida falha 503 antes da escrita. Migração não inicia API; servidor precisa
estar executando o código atualizado para montar as três novas rotas.

## Verificação e limites

Biblioteca/HTTP: dinheiro exato, papel/vínculo, header obrigatório, replay,
conflito, auditoria/outbox ignorado ou com falha, commit incerto, busca literal,
paginação, vazio, filtros inválidos, CORS e migração repetida/futura/adulterada.
Suíte PostgreSQL da entrega 21 ampliada: migração 9 real repetida, cadastro,
replay, identidade por ator, SKU, consulta, busca, duas chamadas concorrentes
e rollback após produto/auditoria antes de outbox. Código/testes anteriores
continuam presentes; testes de rota antiga agora exigem proteção, não bypass.

O ambiente de preparação não possui PostgreSQL: aplicação exige teste real
em banco novo *_test loopback no PC antes do commit. sudo -v fora da etapa
temporizada evita o timeout de autenticação anterior. Skip não é aprovação;
recursos de teste são preservados e relatório privado só contém metadados.
Sem push, servidor ou banco comercial automático. Falha: preservar mudanças
e enviar diagnóstico; não reaplicar/pular teste. Fiscal, importação/lote,
custos/margens, fornecedor e integrações comerciais permanecem pendentes.
