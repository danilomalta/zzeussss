# Gestão do catálogo online — entrega consolidada 24

Base do PC: main 5f2fcff. Escopo exclusivamente PostgreSQL/backend online.
Produção em outra branch, SQLite, banco comercial, frontend e portas não são alterados.

## Entrega funcional

| Função | Contrato | Proteção |
| --- | --- | --- |
| Consulta individual | GET /api/v1/produtos/:id | Empresa da sessão; produto ausente/estrangeiro retorna 404; inclui ativo, versão e preço exato |
| Edição cadastral | POST /api/v1/produtos/:id/details | Nome, descrição e SKU; sem ajuste de saldo; SKU único por empresa |
| Alteração de preço | POST /api/v1/produtos/:id/price | price_cents inteiro entre 0 e 999999999999; decimal exato na persistência |
| Inativação/reativação | POST /api/v1/produtos/:id/active | Booleano explícito; preserva produto e histórico; nenhuma exclusão |
| Histórico | GET /api/v1/produtos/:id/history | Antes/depois, ator, operação e horário; limite 50 padrão/100 máximo; offset até 1000000000 |
| Resultado de operação | GET /api/v1/catalog/operations/:operation_id | Empresa e operador autenticados; continua consultável após novo login do mesmo operador |

Consulta: owner/admin/manager/cashier/stock. Edição cadastral: owner/admin/manager/stock.
Preço, estado ativo e histórico: owner/admin/manager. Consulta de operação: owner/admin/manager/stock,
limitada ao próprio ator. Estes são papéis online existentes; não equivalem à política
departamental ou aos módulos contratados do núcleo local. Não concede permissões ao RH.

## Concorrência e confirmação

Toda alteração exige operation_id UUID canônico não nulo e expected_version inteiro positivo.
JSON duplicado, desconhecido, nulo, número fracionário/exponencial em centavos/versão,
corpo acima de 8192 bytes, campos de empresa/operador e query adicional são recusados.
Nome e SKU são aparados; limites 255/100 caracteres, descrição 2000; controles proibidos.
Na edição, enviar os três campos cadastrais, incluindo descrição vazia quando aplicável.

Cliente deve guardar identidade e conteúdo antes do envio. Consulta individual fornece a versão.
Versão obsoleta retorna 409; duas alterações distintas sobre a mesma versão não se sobrescrevem.
Reenvio explícito da mesma operação/ator/produto/conteúdo canônico devolve o recibo original,
mesmo que a versão atual já tenha avançado. Reutilização com outro conteúdo/ator retorna 409.
Comparação usa SHA-256 do conteúdo normalizado e ação/produto; não armazena tokens/senhas.
Ausência ou falha de consulta não autoriza repetição automática: reconciliar e, quando necessário,
repetir explicitamente a mesma identidade/conteúdo. Nunca criar nova identidade após timeout.

Transação: revalidar sessão/membership/empresa ativa sob bloqueios compartilhados; bloquear
identidade da operação; consultar recibo; bloquear produto; verificar versão; alterar;
inserir snapshot antes/depois e evento de outbox; confirmar. A revogação e mudança de vínculo
concorrem com os bloqueios da transação. Falha de UPDATE, histórico, evento ou commit não
retorna sucesso. Commit sem resposta é incerto; sua conexão pode ter confirmado no servidor.
Não afirmar rollback só porque o HTTP respondeu 503 ou a conexão caiu.

IDs/versões são limitados a 9007199254740991; alteração exige versão abaixo desse limite.
Valores financeiros não são calculados com float; PostgreSQL recebe string decimal vinculada.
Respostas novas usam price_cents. Listagem/cadastro anteriores preservam o formato legado;
esta entrega não migra todos os modelos monetários online.

## Migração e ativação

000008_online_catalog_management.sql é aditiva: catalog_version inicialmente 1,
online_catalog_operations e online_catalog_outbox. FKs compostas impedem vincular
produto/ator de outra empresa. Não existe endpoint para editar/apagar recibos.
Auditoria da criação legada anterior continua pendente; o primeiro recibo novo contém
snapshot do produto encontrado. Não há reconstrução de eventos históricos anteriores.

Histórico online_catalog_migrations próprio, versão 8/checksum; não acrescenta versão
comercial à sequência online_security_migrations (5–7). Migração serializada e DDL/marker
transacionais, rejeitando histórico futuro/adulterado e estado inconsistente. Não existe
downgrade destrutivo automático. Sessões e tabelas products/users/tenants, índices compostos
das migrações 3 e 5 são pré-requisitos; não executar 000001_init.sql para satisfazê-los.

Aplicar o ZIP **não** modifica o banco comercial nem inicia servidor. Depois de revisar
backup e conexão da instalação correta, o operador pode executar, no diretório backend:

```
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online check-catalog
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online migrate-catalog
GOTOOLCHAIN=go1.25.0 go run ./cmd/titan-online check-catalog
```

O comando explícito utiliza a configuração PostgreSQL existente e pode carregar .env;
não publicar seu conteúdo. Falha genérica mantém diagnóstico privado. Sem extensão,
novas consultas retornam 503; operações antigas continuam disponíveis. Reiniciar o
executável com o código atualizado é necessário para montar as novas rotas.
API continua na porta configurada, padrão 8080; nenhum listener é iniciado pelo ZIP.

## Integração e limites

Outbox é intenção durável, uma por operação, sem worker de envio/ack nesta entrega.
Não afirmar preço sincronizado, recebido por terminal ou aplicado em SQLite. Produto
inativo é excluído pelo gerador de sugestões existente; consulta individual continua
permitida. Futuro fluxo de venda online deve validar ativo e gravar preço na venda;
esta extensão não implementa esse fluxo, nem altera vendas locais já registradas.
Fiscal, promoção por período, custo/margem, embalagens, múltiplos códigos, importação,
reajuste em lote/estorno e integrações externas continuam fora deste pacote.

## Verificação e aceite

Testes de biblioteca e HTTP cobrem dados malformados, campos estrangeiros, papéis,
valores exatos, versão obsoleta, reenvio, ator diferente, erros de histórico/outbox,
falha no commit e migração repetida/futura/adulterada. Inventário passa de 69 para 75
rotas, incluindo seis novas operações; contagem não mede conclusão do produto.

Suíte PostgreSQL opt-in ampliada testa migração real repetida, consulta isolada,
edição/preço/estado, SKU duplicado, histórico, recibo por ator, duas concorrências,
revogação e rollback induzido de histórico/outbox. Executada somente em novo banco
*_test loopback pela ferramenta da entrega 21, sem .env nem dados comerciais.
O ambiente de preparação não possui PostgreSQL; o instalador exige sua execução e
aprovação no PC antes do commit. Falha/skip não são aceite. Recursos de teste são
preservados, com credenciais temporárias; relatório privado contém só metadados.

Aplicador executa Go completo, vet, race dos pacotes afetados, contratos, testes da
ferramenta, teste PostgreSQL isolado e build de API/manutenção em /tmp. Commit somente
ao final. Se falhar, preservar mudanças e investigar; não reaplicar ou pular verificação.
