# Entrega40 — cinco relatórios de custo e margem online

Base do PC: main limpa no commit a16cbef, entrega39 aprovada em PostgreSQL
real. Esta entrega lê o custo declarado do schema17, sem aplicar novo DDL.

| Função | GET /api/v1/catalog/margins/ |
|---|---|
| Resumo de custos e margens | summary |
| Produtos sem custo informado | missing-cost |
| Produtos com margem negativa | negative |
| Produtos abaixo da meta percentual | below-target |
| Exportar página CSV | export.csv |

## Regras de classificação

Consulta pertence exclusivamente à empresa da sessão. Somente
owner/admin/manager. Sessão e vínculo revalidados no banco em transação de
leitura repetível, que reúne contagem e itens do mesmo estado. Produto
logicamente excluído não participa; ativo/inativo entra conforme filtro.
Ausência de custo significa desconhecido, não custo zero. Não se estimam
custos, receita, lucro ou valores fiscais.

Resumo apresenta total, known_cost, missing_cost, negative_margin,
below_target e zero_price. Known e missing particionam o total. Outras
contagens podem se sobrepor. Zero_price conta produtos de preço zero mesmo
sem custo; negativa exige custo conhecido e preço menor que custo.

Meta default 3000 pontos-base (30%). Aceita 0 a 10000. A lista below-target
exige preço positivo e custo conhecido. Compara a razão exata antes de
arredondar: `(price_cents - cost_cents) * 10000 < target_bps * price_cents`.
Igual à meta não entra. Preço zero não tem margem percentual; preço zero
com custo positivo aparece na lista negativa. Meta zero inclui margens
negativas de produtos com preço positivo.

Os itens reutilizam os contratos de ProductCost/Margin da entrega39, com
snapshot de preço/versão e custo/revisão. Margin_bps é texto decimal para
preservar inteiros maiores que o limite seguro do JavaScript. O valor exibido
arredonda metade para longe de zero; esse arredondamento não altera filtros.

## Filtros, paginação e exportação

`q`: nome ou SKU, pesquisa literal sem interpretar % e _ como curingas,
apara bordas, até 120 caracteres, sem controles. `active`: all, true, false;
padrão all. `target_bps`: inteiro decimal entre 0 e 10000, padrão3000.
Resumo aceita apenas esses três parâmetros. Demais rotas: limit padrão50,
1 a100; offset 0 a1000000000. Query repetida, extra ou corpo GET é400.
Rotas fixas não permitem substituir issue por query. CSV aceita issue all,
missing_cost, negative ou below_target, padrão all.

Página contém total, items, has_more, next_offset quando necessário,
generated_at, filtros e source_hash. Ordem ID crescente. Cada requisição é
consistente, mas não há snapshot persistente entre páginas; novas gravações
podem mudar a posição dos itens. Página vazia é array vazio real.
Source_hash identifica filtros, total e conteúdo da página, não assinatura
de autenticidade nem trava para edições futuras.

CSV é uma página, não download ilimitado do catálogo. Delimitador ;,
UTF-8, máximo100 produtos e262144 bytes. Campos de custo/margem desconhecidos
são vazios. Colunas incluem IDs, nome/SKU, ativo, versão, preço, custo
conhecido, revisão, custo, margem e estado. Nome/SKU são escapados pelo
escritor CSV e recebem apóstrofo quando podem ser fórmula de planilha.
Não é arquivo de importação e não deve ter apóstrofos removidos para executar
fórmulas. Headers informam total, quantidade, has_more, next_offset, hash,
escopo e escape. Cache-Control no-store; download com nome fixo e nosniff.

Nenhuma consulta grava produto, custo, recibo ou outbox. Não há alteração
de estoque, vendas, financeiro, interface ou branch de produção. Sem schema17,
consulta falha503; inicialização não cria tabela automaticamente.

## Verificação

Testes de filtros, papéis, revogação, custo conhecido/desconhecido, metadados,
paginação, recusa de resultado parcial, fórmula CSV, razão não arredondada
e resumo inconsistente. Rotas montadas validam permissões e queries.
Inventário passa de136 para141 rotas.

Cenário PostgreSQL opt-in: cinco estados de produto/custo, contagens,
meta zero/exata, página vazia, preço zero, busca com porcentagem literal,
ativo/inativo, exclusão lógica, tenant alheio, CSV HTTP e sessão revogada.
Hash de estado antes/depois prova que as leituras não alteraram produtos,
custos ou quantidades de recibos/outbox. Usa app de teste independente para
não consumir o orçamento HTTP dos cenários anteriores; limite real permanece.

Instalador: base e SHA256, contratos, suite Go, vet, race, builds, testes
Python e PostgreSQL real em banco novo isolado. Só cria commit após sucesso.
Não faz push, migração comercial ou inicia servidor comercial. Se falhar,
preserve alterações e envie saída; não reaplique automaticamente.

PostgreSQL real não executado no ambiente de preparação por falta de servidor.
Teste obrigatório está integrado ao instalador do PC. Testes locais de Go,
contratos, vet, race, builds e Python não substituem esse aceite.

## Correção do cenário de porcentagem literal

A primeira execução no PC parou na asserção de total da busca `%literal`.
Fixtures anteriores35,37,38 já continham esse texto e não possuíam custo.
A consulta válida incluía esses produtos da mesma empresa, tornando incorreta
a expectativa de apenas um resultado. O cenário40 agora usa prefixo exclusivo
`escape40-%literal`, acompanhado de `escape40-Xliteral` sem custo como decoy.
Busca por `escape40-%literal` deve encontrar somente o primeiro; uma busca
que interprete porcentagem como curinga encontraria os dois. O decoy não
entra no filtro Margin40 dos demais relatórios. A correção não modifica SQL,
handler, dados comerciais ou testes dos cenários anteriores.
