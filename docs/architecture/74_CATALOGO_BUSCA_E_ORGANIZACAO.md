# Catálogo: busca e organização — primeira entrega

## Comportamento

A rota de consulta `GET /local/v1/catalog/search` deriva empresa, loja, aparelho e usuário da sessão. Pesquisa nome, SKU e código de barras em todos os produtos da empresa, antes de paginar os resultados em até 50 linhas. A rota antiga `/products` continua atendendo o PDV e a seleção de produtos no estoque.

A busca ignora acentos e diferenças entre maiúsculas e minúsculas. Todos os termos precisam corresponder. Trechos de palavras permitem busca abreviada; não há dicionário de abreviações. Uma inserção, remoção ou substituição pode corresponder a termos não numéricos de pelo menos cinco caracteres, com indicação de correspondência aproximada. Códigos numéricos nunca recebem essa correção. A busca é uma consulta; não escolhe nem registra automaticamente produtos na venda.

Filtros disponíveis: unidade, ausência de código de barras, custo zerado e ausência de política de reposição na loja atual. Custo zerado significa o valor existente igual a zero, não ausência comprovada de informação. A política consultada é a estrutura real de reposição; não representa quantidade disponível ou pedido aprovado.

Custo e o filtro por custo são restritos aos papéis owner, manager e stock, como na consulta existente. O filtro é recusado para os demais papéis, evitando revelar custos por contagens. Sessão e revogação são verificadas no servidor. Parâmetros desconhecidos ou duplicados são recusados.

## Interface

O cadastro real fica recolhido em “Cadastrar produto”, separado da busca e da tabela. Busca e filtros são aplicados ao pressionar Buscar/Enter. Paginação preserva a consulta aplicada. Colunas opcionais: código de barras, unidade, custo autorizado e reposição. Valores monetários são alinhados à direita. As escolhas de colunas permanecem apenas enquanto a tela está aberta.

Uma nova consulta limpa as linhas anteriores e cancela a consulta anterior. Uma falha não aparece como catálogo vazio confirmado. A atualização usa os mesmos filtros. O fluxo de entrada de estoque e as permissões de cadastro permanecem na API existente.

## Limites e próximas entregas

Esta entrega não adiciona marca, fornecedor por produto, categoria, campos fiscais, embalagem, vários códigos, edição direta, importação ou desfazer lotes. Essas funções precisam de estruturas persistentes, autorização e auditoria próprias. Também não declara sincronização comercial concluída ou situação fiscal validada.

A busca normaliza e examina as linhas do catálogo da empresa a cada consulta; não é um índice de busca. A memória de resultados é limitada a uma página, mas o tempo da consulta cresce com o catálogo. Antes de uso com grandes bases, medir latência e concorrência no equipamento real e evoluir a indexação. A contagem e a página pertencem à mesma transação de leitura; mudanças posteriores podem alterar a próxima página.

## Verificação

Testes HTTP usam bancos descartáveis e verificam pesquisa além de 50 linhas, paginação, acentos, aproximações, códigos numéricos, filtros reais, autorização de custos, empresas distintas, sessão e revogação, e rejeição de parâmetros inválidos. Testes do cliente verificam contagem de página, campos obrigatórios, centavos seguros, custo autorizado, cancelamento e falhas sem dados inventados.

Comandos: `npm --prefix frontend-web run test:local`, `npm --prefix frontend-web run build`, `go test -count=1 ./...` e `go vet ./...` em backend, além de `git diff --check`.

Demonstração após reiniciar a API com o código novo: abrir Catálogo, buscar um produto por nome sem acento, aplicar filtros, alternar colunas e abrir o cadastro recolhido. Conferir ambos os temas e uma tela menor. Build/testes não substituem esse aceite no navegador e no equipamento do operador.
