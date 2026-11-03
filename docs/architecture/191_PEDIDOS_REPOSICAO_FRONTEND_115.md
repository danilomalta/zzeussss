# Pedidos, fornecedores e reposição — frontend 115

## Escopo e base

114 confirmado no PC: 78 testes selecionados, build 19,92s. Rota existente `/local/orders`, mesmo acesso e clientes de compra/reposição. Apenas frontend e documentação; nenhum backend, SQL, B01, permissão ou dependência nova. O instalador compara a versão anterior de `LocalOrders.tsx`; não substitui conflitos nem altera arquivos do outro trabalho de backend.

## O que funciona nesta entrega

- Abas Pedidos, Fornecedores e Reposição, separando consulta e ação.
- Fornecedores ativos/inativos com busca por nome; cadastro ativo com revisão e confirmação explícita.
- Escolha de fornecedor ativo e reposição aprovada; revisão de nome, produto e quantidade antes de criar pedido local de um produto.
- Histórico paginado, consulta do detalhe do pedido e CSV seguro dos resumos carregados. O endpoint de lista não traz itens: o CSV contém fornecedor, data original e estado, sem inventar quantidade ou total.
- Configuração de mínimo e alvo por produto, cálculo de sugestão, aprovação/recusa com motivo, revisão anterior a cada escrita. Sugestão desatualizada não pode ser aprovada; pode ser recusada.
- Paginação de 50 para cada fonte. Busca, CSV e seletores são da página carregada, não de toda a empresa. Ao mudar página ou atualizar, o resultado antigo é retirado até a resposta; erro não vira zero.
- Falha de fornecedor, aprovação ou reposição não impede a lista independente de pedidos. Atualização é explícita, sem polling contínuo.
- Mesmas filas duráveis e nomes de locks da tela anterior. Grava o comando antes da primeira requisição, mantém os identificadores originais após resposta perdida e oferece consulta ou repetição explícita. Abrir a tela não executa POST.
- Bloqueio entre abas e verificação das duas filas dentro dos locks. Resultado incerto nunca é descartado. Só tentativa de reposição sem incerteza pode ser descartada, após GET confirmar ausência. Cadastro de fornecedor não tem endpoint de consulta por operação: a consulta informa isso; repetir usa o POST idempotente original.
- Aviso de saída para campos não enviados e operação em andamento. Confirmar uma área limpa apenas seus próprios campos, sem apagar formulário de outra aba.
- Cinco idiomas completos; quantidades em milésimos inteiros e formatação exata com BigInt/Intl, inclusive no limite seguro do JavaScript. Sem arredondamento por ponto flutuante.

## Autorização e privacidade

Contexto obtido da sessão e capacidades: empresa, loja, aparelho, identidade e validade. Toda autorização efetiva continua no servidor. A rota mantém suas guardas existentes.

`view_orders` permite consultas; `manage_replenishment` permite consultar aprovações e administrar fornecedor/política/decisão/pedido; `manage_stock` permite calcular. Escritas exigem licença ativa com `core` e `orders`. Consulta de resultado pendente é permitida com a permissão correspondente mesmo após expiração do contrato. Não há concessão automática a operador. Ausência de Web Locks bloqueia gravações e recuperação concorrente, mas permite listas e detalhes.

Listas, detalhes e campos não confirmados ficam apenas em memória e são descartados ao trocar contexto ou sair. A fila já existente conserva o comando mínimo necessário para recuperação por empresa/loja/aparelho/usuário. Não contém token/senha. Ela sobrevive ao logout; apagá-la perderia a possibilidade de resolver uma escrita desconhecida. Não exportar essa fila para logs. A interface nova não registra dados pessoais em console.

## Contratos existentes utilizados

| Método | Caminho local | Uso |
|---|---|---|
| GET | `/local/v1/purchase-suppliers?offset=N` | Fornecedores, página de até 50 |
| POST | `/local/v1/purchase-suppliers` | `operation_id,id,name,status` |
| GET | `/local/v1/purchase-approvals?offset=N` | Aprovações ainda disponíveis |
| GET | `/local/v1/purchase-orders?offset=N` | Resumos de pedidos |
| GET | `/local/v1/purchase-orders/{id}` | Pedido com um item e confirmação original |
| POST | `/local/v1/purchase-orders` | `operation_id,order_id,supplier_id,suggestion_id` |
| GET | `/local/v1/replenishment/products?offset=N` | Saldo e política por produto |
| GET | `/local/v1/replenishment/suggestions?offset=N` | Sugestões e decisões |
| GET | `/local/v1/replenishment/operations/{kind}/{operation_id}` | Recuperação verificando corpo original |
| POST | `/local/v1/replenishment/policies` | Mínimo e alvo em milésimos |
| POST | `/local/v1/replenishment/suggestions` | Cálculo por produto |
| POST | `/local/v1/replenishment/reviews` | Decisão humana e motivo |

Nenhum endpoint novo é necessário para o escopo entregue. Preço, frete, imposto e total não constam no contrato de pedidos: a tela não os fabrica. Não há envio ao fornecedor, confirmação externa, agenda de descarga, recebimento, entrada de estoque, pagamento ou emissão fiscal implícitos.

## O que continua pendente

Futuras integrações precisam de contratos documentados e autorização própria para: envio e confirmação entre empresas; consulta de cadastro por operação; pesquisa global no servidor; pedido com vários itens e condições comerciais; recebimento com divergências, devolução e coleta; confirmação de pagamento, fiscal e agenda de entrega. Esses fluxos não são simulados como ativos. Financeiro113/114 continua separado; pedido local não cria conta a pagar automaticamente. A fila preserva recusas definitivas de compra, conforme cliente anterior; não há botão para apagar uma compra de resultado desconhecido.

## Validação e demonstração

Testes Node do modelo e da fonte utilizam os clientes reais com transporte descartável: permissões/contexto, precisão, CSV seguro, bloqueios, persistência antes de envio, falhas independentes, resposta perdida, consulta sem POST, identidade do recibo e descarte seguro. Regressões existentes de compra e reposição mantidas. Typecheck da aplicação, build e diff-check obrigatórios.

Prévia incluída usa transporte explicitamente fictício, memória isolada e os mesmos componentes/clientes; não chama banco ou API comercial. Permite aprovar, criar pedido, cadastrar fornecedor, calcular, recusar sugestão desatualizada, simular indisponibilidade, resposta perdida, falta de Web Locks e perfil de consulta. Reabrir na prévia conserva fila e servidor fictício em memória, dentro da mesma página; recarregar a página os apaga. No aplicativo real a fila usa o armazenamento já existente.

Testar no sistema: entrar com perfil autorizado, configurar mínimo/alvo, calcular sugestão, conferir e aprovar; selecionar fornecedor e aprovação; revisar e confirmar; consultar detalhes e CSV. Repetir com recusa, perda de resposta e reconexão; não criar outro pedido para a mesma tentativa. Verificar duas abas, perfil sem administração, contrato expirado, ausência de dados, idiomas e temas. Os ensaios com transporte fictício não substituem demonstração contra a instalação local real do usuário.

Chaves novas sem tradução: nenhuma nos cinco arquivos. Traduções estrangeiras precisam de revisão humana. Esta entrega não declara a Parte 1 ou o ERP completos.
