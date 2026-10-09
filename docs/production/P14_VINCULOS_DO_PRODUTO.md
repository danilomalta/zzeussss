# P14 — vínculos do produto com receitas e ordens

Base efetiva: **0d62bdf**, branch **feat/producao-receitas**, P13 registrada
com árvore limpa. Início da consulta de vínculos do catálogo com a produção,
sem interface nova e sem alterar cadastro ou escritores de estoque.

**Sem nova migração.** SQLite permanece **35**. Backup, migrações, formato,
criptografia e recuperação não são alterados.

## Consulta e autorização

GET /local/v1/catalog/products/:id/production-usage

Exige sessão, aparelho autorizado, view_catalog e manage_production atuais.
Não amplia papéis do catálogo para permitir leitura de produção. Empresa,
loja, usuário e aparelho vêm da sessão. Produto é da empresa; vínculos são
somente da **loja autorizada**, inclusive quando o catálogo é compartilhado
entre lojas. Resultado não informa custos ou preços.

| Query opcional | Significado |
| --- | --- |
| kind | versions (padrão) ou orders, tipo da página |
| offset | Posição na página desse tipo, padrão 0 |

IDs têm até 128 bytes UTF-8, sem espaços nas bordas, NUL, CR ou LF. offset
aceita somente dígitos decimais 0..9007199254740991, sem sinal ou fração.
Rejeita valores vazios, repetidos, query desconhecida, tipo inválido e IDs
inválidos. Não aceita tenant_id, store_id ou limit. Produto ausente na empresa
autorizada retorna 404; produto existente sem vínculos retorna 200 e items [].

```text
GET /local/v1/catalog/products/flour/production-usage
GET /local/v1/catalog/products/flour/production-usage?kind=orders
GET /local/v1/catalog/products/bread/production-usage?kind=versions&offset=50
```

Contrato autocontido: docs/api/production-product-usage.openapi.json.

## Contagens e página

Resposta contém product (id, nome e unidade atuais), store_id, kind,
recipe_version_count, latest_recipe_version_count, order_count, offset,
limit (50), has_more e items. As três contagens são de todo o escopo, não da
página. Trocar kind ou offset não altera o significado das contagens.

recipe_version_count conta versões em que o produto é saída ou ingrediente.
Cada versão conta uma vez; publicar uma versão nova não apaga vínculos antigos.
latest_recipe_version_count conta apenas a revisão mais recente publicada de
cada receita que ainda referencia o produto. **Mais recente não significa
ativação.** Não fabrica estado ativo/inativo inexistente no contrato atual.

order_count conta ordens vinculadas a uma versão imutável que referencia o
produto, em todos os estados reais: planned, approved, cancelled e completed.
Não exclui ordem cancelada do histórico. completed conserva a semântica P06
de resultado registrado, sem reescrever a coluna física status da ordem.

Versões são ordenadas por recipe_id,revision; ordens por created_at DESC,id
DESC. has_more se refere somente ao tipo selecionado. Página vazia mantém
contagens globais. Não somar contagens de versões e ordens como produções
simultâneas, nem somar quantidades de registros históricos.

## Papel, unidade e quantidade preservados

Cada link tem kind, id, recipe_id, version_id, role (output ou ingredient),
unit, quantity_milli, quantity_basis e unit_matches_catalog.

Para versions, id é version_id, quantity_basis é per_batch: rendimento da
receita quando role=output, quantidade do ingrediente por lote de receita
quando role=ingredient. latest_version informa igualdade com a revisão mais
recente publicada. Não afirma se essa versão pode ser usada em nova produção.

Para orders, id é order_id, quantity_basis é planned_total: quantidade
preservada por lote de receita multiplicada pelo número de lotes planejados.
Inclui order_status efetivo e location_id. Não representa reserva, consumo,
produção boa ou saldo atual. Essas informações continuam nas APIs P05..P13.

Quantidade e unidade vêm da versão histórica ou snapshot da ordem, nunca de
nome/unidade atuais do catálogo. Valida snapshots e limites exatos antes de
multiplicar, evitando overflow. Exemplo: uma ordem preserva 1500000 em g para
farinha; alterar catálogo para kg não transforma a resposta em 1500000 kg.

unit_matches_catalog é igualdade literal entre unidade histórica e atual.
Não é validação de conversão, densidade ou compatibilidade dimensional. false
não autoriza converter nem substituir valores históricos. Não transforma
volume em massa. A leitura não reescreve nenhum snapshot.

## Coerência e limites de decisão

Produto, contagens, cabeçalhos de receitas, versões e ordens usam uma única
transação de leitura SQLite. Cursores são fechados antes das consultas de
detalhes; não chama APIs públicas que abrem outra transação dentro dela.
Revisão mais recente, contagem e página ficam coerentes sob nova publicação
concorrente. Requisições distintas podem observar mudanças: offset não é
cursor imutável para exportação de várias páginas.

Não retorna safe_to_change, não altera unidade, exclui, inativa, bloqueia ou
autoriza edição de produto. **Zero vínculos na loja não prova ausência de uso
na empresa inteira**, em outras lojas ou em estoque, vendas e compras.
Consulta orienta revisão humana; futura operação de escrita deverá validar
seu próprio contrato e concorrência dentro da transação de escrita.

Não escreve auditoria, outbox, reservas, movimentos, relógio do contrato ou
dados comerciais. Histórico autorizado permanece legível após expiração do
contrato, com permissões atuais. 400 query inválida; 401 sem sessão; 403 sem
permissões/aparelho; 404 produto ausente; 409 snapshot/quantidade inconsistente
nas validações; 500 falha interna. Não substitui validação integral do banco.

## Mudança compartilhada e testes

Única mudança compartilhada: server.go monta uma rota GET protegida em
/catalog/products/:id/production-usage. Cadastro/listagem de produtos e APIs
de produção anteriores mantêm contratos. Sem mudança em autenticação, sessões,
infraestrutura, PostgreSQL, backup ou escritores de catálogo/estoque.

Testes em SQLite temporário e Fiber App.Test verificam saída/ingrediente,
versões antigas/mais recentes, ingrediente removido da versão posterior mas
preservado em ordem antiga, quantidade exata e overflow, mudança no catálogo
sem reinterpretar unidades, ausência de preço/custo, produto sem vínculos,
contagens e paginação de 53 versões/ordens, query estrita, autorização/contexto,
expiração sem escrita no relógio, publicação concorrente e backup/restauração
de vínculo de ingrediente com ordem concluída. Nenhum servidor externo é
iniciado e nenhum banco comercial ou demonstração é usado.

aplicar.sh confere base 0d62bdf, branch, árvore limpa e checksum; aplica apenas
P14; executa backup/CLI, testes específicos, suíte completa uma vez, vet, race
e build sem executar o binário. Registra apenas os sete arquivos listados
após sucesso, com visualizador desativado. Sem merge ou push.

Limites: vínculos de produção da loja autorizada, sem painel frontend,
exportação, edição de catálogo, inativação, saldo, custo, dependências de
vendas/compras ou decisão automática de alteração de unidade.
