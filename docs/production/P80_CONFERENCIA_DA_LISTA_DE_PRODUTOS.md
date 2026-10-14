# P80 — Cinco funções de conferência de uma lista de produtos

Base `2f2121e`, branch `feat/producao-receitas`. SQLite permanece 44.

1. Informar lista de até dez produtos únicos, com quantidade e unidade explícitas, para um único local.
2. Consultar saldo físico/reservado/livre de cada produto nesse local.
3. Conferir quantas demandas estão cobertas e se todas possuem cobertura matemática.
4. Calcular faltas exatas por produto na unidade do estoque.
5. Calcular sobras estimadas por produto na unidade do estoque, sem misturar produtos.

Lista não é receita, ordem de produção ou pedido de compra. Adicionar/remover linha é edição local em memória, sem gravação em storage. Produtos repetidos são recusados mesmo com unidades distintas, para não contar o mesmo saldo duas vezes. Não soma massas ou volumes de produtos diferentes. Referências precisam existir e estar autorizadas pelo backend em cada consulta.

## Leitura e isolamento

Até dez GETs paralelos existentes `/local/v1/stock/availability?product_id=...&location_id=...`, com Bearer, sem corpo/cookies/cache. Não adiciona endpoint ou método de escrita. Aguarda todos os resultados; qualquer falha recusa a conferência inteira, sem fabricar saldo zero para produto ausente. Uma falha 401 prevalece para invalidar a sessão; permissões e isolamento permanecem conferidos pelo servidor.

Rejeita IDs com mais de 128 bytes UTF-8, entradas/quantidades inválidas, duplicatas e campos inesperados antes da rede. Faz cópia da lista antes de aguardar as respostas, preservando a solicitação original se o chamador alterar os objetos. Resposta precisa corresponder ao produto e local solicitado; saldos e unidades obedecem ao validador existente. Fonte divergente ou conversão incompatível não produz resultado parcial.

Cada GET mantém a coerência do seu produto; a lista inteira não é snapshot transacional. Não há instante único do estoque informado pelo contrato e a interface não inventa data do servidor. Escritas concorrentes entre as leituras ou depois delas permanecem possíveis. Cobertura global é apenas conjunção matemática das demandas consultadas, sem garantir disponibilidade simultânea/futura ou aprovar produção.

## Cálculo e exatidão

Reutiliza `stockNeed`/`stockCoverage`: quantidade positiva com vírgula, até três casas, sem separador de milhares/sinal/expoente; até 20 caracteres e limite 9007199254740991 em milésimos. Unidade igual ou kg/g e liter/ml; sem conversão massa-volume, contagem-massa ou comprimento-massa. Usa saldo livre após reservas, nunca o físico como disponível. Quantidade fracionária de unit é limiar de simulação, não autorização de movimentação física.

Na unidade do estoque, demanda convertida = numerador / denominador em milésimos. Falta = máximo de zero e demanda − livre. Sobra suficiente = livre − demanda, na mesma escala racional. Intermediários BigInt podem superar o limite numérico seguro; numeradores/denominadores permanecem strings. Não arredonda nem acumula frações entre produtos. Ingrediente insuficiente não apresenta zero como sobra disponível.

## Interface e contratos compartilhados

`LocalStockAvailability.tsx` acrescenta painel independente; `package.json` inclui testes. Clientes, API, estoque, reservas e escritores existentes permanecem com seus contratos. Inputs/apagar/adicionar/nova consulta limpam a conferência; troca de sessão limpa também local/lista. `useReadTask` descarta respostas ultrapassadas e remove resultados da sessão anterior. Rascunho não é persistido nem enviado a outro operador.

Testes: limites/duplicatas/quantidades, conversões, faltas/sobras/frações/extremos, uso do saldo livre, fontes divergentes, falha parcial/401, três produtos/unidades sem soma, mutação da lista durante consulta, GETs sem escrita. Frontend completo/build; backend HTTP de disponibilidade, suíte completa uma vez, vet e corrida dos testes específicos. Patch/script verificados sobre base limpa e etapa já registrada/base errada.

Sem migração, ajuste de backup, main, merge/push, banco comercial/demonstração ou aceite visual em navegador.
