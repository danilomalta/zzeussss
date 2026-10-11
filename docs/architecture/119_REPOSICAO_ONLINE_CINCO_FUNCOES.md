# Entrega44 — planejamento de reposição online

Base de aplicação: main `4007a91`, entrega43 validada no PC. A frente de produção permanece separada; nenhuma migração nova nesta entrega.

| Função | Método e caminho | Resultado |
| --- | --- | --- |
| Resumo | GET `/api/v1/catalog/replenishment/summary` | Contagens de necessidade, atendimento do alvo, inativos, limites e demanda zero. |
| Lista paginada | GET `/api/v1/catalog/replenishment/items` | Quantidade sugerida, alvo, gap e dados de planejamento por produto. |
| Detalhe individual | GET `/api/v1/catalog/replenishment/products/:id` | Explicação calculável da recomendação, produto/version e parâmetros da43. |
| Simulação | POST `/api/v1/catalog/replenishment/products/:id/simulate` | Estado atual e cenário de saldo hipotético, com `simulated=true`, sem gravar. |
| CSV | GET `/api/v1/catalog/replenishment/export.csv` | Uma página, fórmulas de texto protegidas e headers de paginação/hash. |

## Cálculo e significado

Período de planejamento = prazo declarado de reposição + dias até a próxima revisão. Revisão padrão7 dias, intervalo1..365. Prazo cadastrado0..365; período1..730. Demanda continua sendo declaração do cadastro, não previsão gerada nem consumo confirmado. Um prazo zero não comprova entrega no mesmo dia; o responsável precisa validar a configuração.

Demanda de período = ceil(demanda_diária × período). O código usa inteiro escalado por10000, sem float, mantendo frações até quatro casas. Exemplo: estoque1, demanda0.1429/dia e período7 resultam em demanda1.0003, arredondada para2 unidades, portanto sugestão1. Alvo=max(mínimo, ponto de reposição, demanda de período). Gap=max(alvo-estoque,0).

Estados exclusivos: `needs_replenishment`, `within_target`, `inactive`, `quantity_exceeds_limit`. Produto ativo com gap positivo e alvo dentro do limite recebe `suggested_units=gap` e `requires_approval=true`. Inativo e alvo acima2147483647 recebem sugestão zero e estado explícito, sem esconder o alvo/gap calculado. O alvo pode chegar a73000000000 unidades, representado exatamente em int64/JSON; esses resultados ficam bloqueados para sugestão executável. Demanda zero aparece em `no_positive_demand=true`; mínimos configurados ainda podem justificar sugestão, sempre sujeita à revisão humana.

O cálculo identifica proposta de reposição; não cria ordem, compra, reserva, mensagem a fornecedor ou saldo. `planning_only=true` em todos os itens. Não há total de unidades somado entre produtos com unidades comerciais diferentes. Aprovação/pedido e sincronização com o núcleo SQLite exigem integração posterior.

## Entrada, consulta e segurança

Owner/admin/manager/stock. Empresa, usuário e sessão derivados/revalidados no backend. Sessão expirada ou revogada também é recusada pela biblioteca. Papéis caixa/contabilidade/employee não recebem acesso.

Resumo aceita apenas q, active e review_days. Lista/CSV aceitam também state, limit e offset. active=true padrão, false ou all; state=needs_replenishment padrão, all ou um estado exclusivo. Busca literal por nome/SKU, com %/_ escapados. q até120 caracteres, espaços aparados. GET sem corpo; extras/repetidos/vazios em enums/números são recusados. Paginação1..100 (padrão50), offset0..1000000000; items/total/has_more/next_offset. Ordenação por ID crescente.

Detalhe aceita apenas review_days, encontra o produto na própria empresa e pode apresentar inativo identificado; removido logicamente retorna404. Listas excluem removidos. Cada consulta usa repeatable-read para contagem e linhas consistentes dentro da resposta. Páginas independentes não são um snapshot global; alterações entre páginas podem mudar a seleção. source_hash identifica query/total/itens de uma página, não prova assinatura nem integridade externa.

Simulação exige exatamente expected_version, review_days e stock_units em JSON até4096 bytes. Version1..9007199254740991, revisão1..365 e saldo hipotético0..2147483647. Não aceita extras, campos duplicados, null, número fracionário ou formato incorreto. Rejeita versão antiga com409; não incrementa versão. Resultado contém current e scenario, ambos calculados sobre os parâmetros registrados da mesma leitura. Cenário não representa uma baixa ou entrada já ocorrida.

Demanda negativa/NaN ou prazo fora do limite no banco gera503, mesmo se o filtro de estado escondesse o produto inválido. Não converte valor inválido em demanda zero. Consulta individual/linhas validam também produto, version, preço e demais parâmetros.

## CSV e operação

CSV UTF-8, separador ponto e vírgula, até100 linhas de dados e262144 bytes. Apenas uma página; página vazia mantém cabeçalho. Nome/SKU perigosos recebem apóstrofo; campos são citados pelo encoder. Campos numéricos/calculados não usam formato monetário nem ponto flutuante.

Headers: no-store, nosniff, Content-Disposition, X-Titan-Report-Total/Count/Has-More/Next-Offset, X-Titan-Source-Hash, X-Titan-Report-Scope=online-replenishment-planning e X-Titan-CSV-Text-Escape=apostrophe. Next-Offset só quando houver próxima página.

Nenhuma DDL, alteração de estoque, evento/outbox ou migração comercial é executada. As funções consultam os parâmetros existentes da43 e campos do PostgreSQL; não leem nem escrevem banco SQLite, backup ou branch de produção.

## Verificação e aceite

Unitários cobrem mínimos/reposição, ceil fracionário, demanda zero, limites extremos, inativos, dados inválidos, parser, papéis/sessão, simulação sem SQL de escrita, versão antiga, paginação e CSV adulterado/protegido. Parser de query e rotas montadas verificam listas permitidas, defaults, campos vazios/repetidos e permissões. Inventário de156 para161 rotas.

PostgreSQL opt-in: cinco funções HTTP, parâmetros reais, resumo/estados, quantidade fracionária, bloco de quantidade excessiva, paginação, busca literal com controle, filtros, saldo hipotético, isolamento, CSV, hashes de dados antes/depois, integração de gravação43→consulta44, versão antiga, dado persistido inválido, exclusão lógica e sessão revogada. Fixtures só em banco novo *_test em loopback. Testes de leitura devem provar ausência de mutações, incluindo recibos/outbox da43.

Go/vet/race/builds, contratos e testes do runner são executados no ambiente de criação. PostgreSQL real não está disponível aqui: o instalador testa no PC e só cria commit após passar; skip não equivale a sucesso. Sem push, merge ou servidor comercial. Em falha, preservar alterações e corrigir sem reaplicar.
