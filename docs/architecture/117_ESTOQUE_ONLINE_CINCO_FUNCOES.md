# Entrega42 — análise do estoque online

Base de aplicação: main em d9f480b, entrega41 validada no PC. Esta entrega consulta campos existentes de products no PostgreSQL. Não cria migração nem modifica a frente de produção.

## Cinco funções

1. `GET /api/v1/catalog/stock/summary`: total de produtos, estoque zero, abaixo do mínimo, baixa cobertura e ausência de demanda positiva.
2. `GET /api/v1/catalog/stock/zero`: página de produtos com estoque exatamente zero.
3. `GET /api/v1/catalog/stock/below-minimum`: página de produtos cujo estoque é menor que o mínimo registrado; inclui quantidade faltante para atingir esse mínimo.
4. `GET /api/v1/catalog/stock/coverage`: página com cobertura estimada, estado e data registrada da previsão. Inclui casos sem demanda positiva, com cobertura nula.
5. `GET /api/v1/catalog/stock/export.csv`: download de uma página com filtros, estado real dos campos e proteção contra fórmulas em textos.

## Fonte e interpretação

A fonte é o catálogo online: estoque, estoque_minimo, ponto_reposicao, demanda_media_diaria e ultima_previsao_demanda_em. Não é leitura dos movimentos do SQLite/PDV, reconciliação entre aparelhos ou saldo de reserva da produção. A API não anuncia esses dados como estoque comercial sincronizado.

Esses campos já existem no schema de produtos. O relatório não gera previsão, não atualiza a demanda, não garante atualização dos campos e não escreve estoque, pedido, reserva, custo ou outbox. A data da previsão é exibida quando registrada; não há garantia de recência nem classificação automática por idade.

`below_minimum` significa estoque menor que mínimo, estritamente. Igualdade não alerta. Mínimo zero não classifica estoque zero como falta. `missing_to_minimum` é max(mínimo−estoque,0), em unidades inteiras. O ponto de reposição é informado como metadado; não dispara pedido.

`coverage_days` é estoque/demanda registrada positiva. Apresentação em string decimal, truncada para quatro casas. A classificação usa comparação exata estoque<horizonte×demanda; não usa o número apresentado. Igualdade ao horizonte não é baixa cobertura.

Sem demanda positiva, coverage_days é null e coverage_state é no_positive_demand. O estoque pode ser zero com cobertura zero quando a demanda é positiva; isso é diferente de cobertura desconhecida. Demanda negativa/NaN/inválida é recusada como indisponibilidade, sem ser convertida silenciosamente para zero.

Os valores NUMERIC(12,4) são lidos em texto e convertidos para inteiros com escala10000. Nenhum float64 participa do cálculo. Os limites do schema INTEGER e NUMERIC mantêm multiplicações dentro de int64. Datas sem valor permanecem null; campos desconhecidos ficam em branco no CSV.

## Contrato e autorização

- Papéis owner, admin, manager e stock; empresa e sessão persistida revalidadas dentro da transação.
- Filtro q literal em nome/SKU, até120 caracteres, sem curingas `%`/`_`. Decodificação de URL uma vez.
- active padrão true, podendo false ou all. Produtos logicamente excluídos sempre ficam de fora.
- horizon_days de1 a365, padrão7. Representa horizonte de análise, não previsão prometida de ruptura ou prazo de entrega.
- limit padrão50, máximo100; offset de0 a1bilhão. Total, itens e generated_at pertencem à mesma transação repeatable-read.
- CSV permite issue all, zero_stock, below_minimum, low_coverage e no_positive_demand. As outras rotas não permitem sobrescrever o issue.
- Resumo não aceita paginação. Parâmetros repetidos, extras, vazios nos campos tipados e corpo em GET são recusados.
- Contagens podem se sobrepor: um produto sem estoque pode estar abaixo do mínimo e ter baixa cobertura. Não somar contagens como categorias exclusivas.
- 401 para sessão ausente/revogada;403 para papel sem acesso;400 para query inválida;503 para banco ou dados inconsistentes. Resultados parciais não são publicados.

## CSV, identificação e limitações

CSV UTF-8 com separador ponto e vírgula, máximo100 linhas/256KiB por página. SKU/nome com risco de fórmula recebem apóstrofo; aspas e separador seguem encoding/csv. Campos de quantidade e demanda são exatos.

Download com no-store/nosniff, nome fixo, total/count/has-more/next-offset, hash SHA256 do conteúdo da página e escopo online-stock-analysis. O hash não é assinatura nem prova fiscal. Cada página seguinte é nova leitura; não há snapshot global garantido entre páginas.

Esta entrega não altera a interface nem implanta o servidor comercial. Cobertura baseada em demanda registrada é estimativa operacional, não promessa de disponibilidade. Ajustar limites/demanda pela interface e sincronizar os saldos com o PDV são trabalhos separados.

## Validação

Testes unitários: cálculo exato em quatro casas, limite máximo do schema, igualdade ao horizonte/mínimo, zero versus desconhecido, demanda inválida, filtros, CSV, resumo, autorização, revogação e rejeição de página parcial.

Testes HTTP: cinco rotas, papéis, queries inválidas, filtros fixos e decodificação literal.

PostgreSQL isolado: cinco produtos com todas as situações, empresa externa, paginação e páginas vazias, SKU com `%` e produto de controle, horizonte diferente, papel stock, CSV, filtros de inativos/excluídos, demanda negativa, sessão revogada e ausência de alterações ao catálogo/custos/recibos.

O instalador roda testes Go completos, vet, race, builds, contratos151 e testes do runner. A integração PostgreSQL real é obrigatória no PC antes do commit. Neste ambiente de criação ela não foi executada por ausência de PostgreSQL.
