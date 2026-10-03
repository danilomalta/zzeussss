# Pedido local a partir de reposicao aprovada

Base: 7f61048. Esta entrega cria pedidos comerciais locais, sem envio entre empresas.

## Comportamento

- Fornecedor e uma referencia comercial por empresa/loja, nao uma identidade com acesso ao sistema.
- Cadastro de fornecedor ativo/inativo, idempotente por aparelho/operacao. Edicao e inativacao pela tela ficam pendentes.
- Uma aprovacao gera no maximo um pedido nesta entrega. A estrutura existente de reposicao aprova um produto por sugestao; por isso cada pedido tem um item. Agrupamento de varias aprovacoes fica para outra entrega.
- O pedido preserva fornecedor, produto, SKU, unidade, quantidade e identificacao da aprovacao. Mudancas posteriores no catalogo/sugestao nao alteram essa copia.
- Pedido, item e auditoria sao gravados na mesma transacao, incluindo observacao do contrato. Escritas ignoradas por trigger tambem falham.
- Operacoes repetidas exigem os mesmos identificadores, referencias, aparelho e autor, alem de autorizacao e contrato ainda vigentes.
- Nao grava preco, frete, imposto ou total desconhecido. Nao baixa/acrescenta estoque, nao registra pagamento e nao publica evento na outbox.
- Estado unico: `local_not_sent`. Nao e confirmacao do fornecedor.
- A aprovacao continua historicamente `approved`; o vinculo do pedido e consultado por uma chave unica. A limitacao anterior de uma aprovacao pendente por produto permanece ate desenvolver seu encerramento no recebimento.

## Acesso

Escritas exigem ManageReplenishment e contrato Orders ativo (dependencias Core e Inventory). Consultas exigem ViewOrders com vinculo, loja e aparelho validos. Consulta de aprovacoes exige tambem ManageReplenishment.

Assim como catalogo e comprovantes, leituras autorizadas permanecem disponiveis apos expiracao do contrato. Nenhuma lista de modulos ou empresa enviada pelo navegador concede acesso.

O papel local `supplier` continua um perfil interno vinculado a loja; o cadastro comercial nao cria esse perfil. Autenticacao de fornecedor de outra empresa ainda nao esta integrada.

## API

| Metodo | Caminho sob /local/v1 | Uso |
| --- | --- | --- |
| GET/POST | /purchase-suppliers | Listar/cadastrar referencias |
| GET | /purchase-approvals | Listar aprovacoes ainda sem pedido |
| GET/POST | /purchase-orders | Historico/criacao |
| GET | /purchase-orders/:id | Consultar pedido e item preservado |

Listas retornam no maximo 50 itens, com offset >= 0. JSON de escrita rejeita campos extras/duplicados/nulls. IDs e contexto comercial sempre sao conferidos no backend.

## Interface

Tile Pedidos e fornecedores abre /local/orders somente quando capacidades indicam Orders e ViewOrders. Quem tem ManageReplenishment pode cadastrar fornecedor e criar pedido. Demais usuarios autorizados podem consultar historico/detalhe.

Operacao pendente e salva antes do POST, separada por empresa/loja/aparelho/autor. Bloqueio entre abas impede gravacao simultanea no navegador. Resposta perdida mantem IDs e dados originais. Consulta de pedido precede repeticao explicita; falha de consulta nao inicia outro POST. Confirmacao divergente nao limpa a pendencia. Cadastro de fornecedor repete seus IDs originais, com garantia idempotente no servidor.

Pendencias corrompidas ou recusadas definitivamente nao sao apagadas automaticamente. Recuperacao administrativa dessas pendencias ainda precisa de uma interface especifica.

## Limites e proximo trabalho

As sugestoes/aprovacoes reais existentes podem ser usadas, mas sua criacao e revisao ainda nao tem tela/API publica nesta entrega. Uma instalacao nova mostra lista vazia; nao sao inseridas aprovacoes ficticias. O teste HTTP usa os casos de uso reais para demonstrar a sequencia completa em banco descartavel.

O contrato POS sozinho nao contrata Orders. Nao alterar licenca de cliente nem habilitar esse modulo automaticamente. A demonstracao atual do PDV continua com seu perfil anterior.

Antes do envio entre empresas, definir vinculo mercado-fornecedor, identificadores comerciais de ambos, correspondencia de produtos/unidades, permissoes direcionais, versao de pedido, confirmacao parcial/recusa e destino confiavel. Nao adaptar o transporte atual removendo isolamento por tenant/loja.

Proxima entrega funcional: expor politica, sugestao e revisao pela API contratada e tela; depois confirmacao autorizada do fornecedor, agendamento e recebimento.

## Verificacao

Testes HTTP cobrem aprovacao real, copia estavel, repeticao/conflito, concorrencia, autorizacao, contrato ausente/expirado, isolamento, rollback de pedido/item/auditoria, persistencia e ausencia de movimentos/eventos indevidos. Testes Node cobrem entrada/resposta, perda de resposta, consulta antes de POST, referencias divergentes, armazenamento e escopo.

Executar backend go test -count=1 ./... e go vet ./..., frontend npm run test:local e npm run build, git diff --check. Inspecao visual no navegador e teste no PC do usuario continuam necessarios.
