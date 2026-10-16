# Correção91 — caixa real

Esta correção altera `/local/pos`, e não apenas a Central de operação90. O feedback apontou que o caixa integrado continuava só com confirmação em dinheiro e ocultava o resumo em tela cheia. A entrega90 não concluiu toda a parte1.

## Comportamento corrigido

1. **Fechar caixa** aparece no cabeçalho do PDV, inclusive em tela cheia, e leva a `/local/cash/register`. Venda em edição, pagamento em andamento ou operação pendente impedem a saída por esse botão; o operador recebe orientação para resolver a venda.
2. **Pagamento** mostra os produtos e permite escolher Dinheiro, Débito, Crédito e Pix. A API existente aceita somente dinheiro. As outras escolhas mostram a limitação e bloqueiam finalizar; nunca são convertidas em dinheiro silenciosamente.
3. **Quer nota fiscal?** permite não solicitar nesta tela ou informar CPF/CNPJ. O caixa não tem contrato para emissão ou identificação fiscal, portanto solicitar essa opção bloqueia finalizar, sem descartar silenciosamente o pedido. O responsável autorizado tem ligação para a preparação fiscal90. Nenhum documento é anunciado como emitido.
4. **Cadastro do clube de pontos** fica no cabeçalho e no pagamento. O formulário registra uma solicitação local explícita, com nome/CPF, contato opcional e autorização do cliente. Existe exclusão da solicitação após salvar. Não cria associado oficial, saldo, pontos, desconto nem vínculo com a venda.
5. **Compra confirmada** abre um resumo modal, inclusive em tela cheia, com os itens retornados no recibo, total confirmado, valor entregue e troco grande. Não desaparece por temporizador ou atualização do turno; permanece até **Iniciar próxima venda**.
6. **Perda de resposta** preserva o valor recebido pelo mesmo identificador de operação. Ao conferir a operação pendente, o recibo existente recupera o mesmo troco sem reenviar outra venda. Sem metadado confiável, mostra valor desconhecido; não presume dinheiro entregue igual ao total.

## Garantias e limites

- O payload da venda existente permanece inalterado: produtos/quantidades, turno, identificadores e pagamento cash com o valor líquido da compra. O valor entregue e o troco não entram como receita.
- Metadados de dinheiro são limitados às últimas20 operações, separados por empresa/loja/aparelho/identidade. Não contêm produtos, nomes, CPF, senha ou token. Não é possível alterar o valor recebido da mesma operação sem conflito.
- Falha ao preservar esses metadados bloqueia o envio de uma nova venda. Falha em lê-los depois de uma venda já confirmada não transforma a compra em falha nem inventa troco.
- Itens e valores da conclusão vêm do recibo validado pelo cliente existente. Os nomes vêm do carrinho capturado antes da limpeza ou do catálogo carregado; nome não disponível não vira UUID visível.
- O resumo só aparece como compra confirmada após o recibo aceito. Nenhuma abertura física da gaveta foi implementada ou simulada. Ela depende da integração de equipamentos e deve ser acionada somente após confirmação autorizada, com proteção contra repetição.
- A solicitação do clube é um rascunho local por contexto. Dados pessoais são gravados apenas por ação explícita com autorização registrada. Não são enviados, incluídos em logs ou usados para pontuação. O backup89/90 não os inclui automaticamente.
- CPF/CNPJ e adesão não são anexados ao payload cash atual, pois a API não possui esses campos. A identificação fiscal solicitada bloqueia finalizar até existir integração, em vez de prometer emissão futura sem registro no servidor.
- A confirmação em dinheiro, abertura e fechamento oficiais usam os mesmos clientes existentes. Sem mudança no backend, banco real, migração, dependência ou push.

## Contratos necessários para finalizar as integrações

1. Pagamentos: formas habilitadas por instalação, lançamento/autorização Pix/TEF/POS, consulta e confirmação do provedor, pagamentos divididos, estados pendentes e idempotência.
2. Fiscal: identificação e documento ligados à venda; emissão, rejeição, contingência, consulta e impressão autorizada; acesso apropriado para o operador.
3. Clube: política/termos configurados, consulta e cadastro de associado com consentimentos, identidade por empresa, vínculo da venda, crédito/resgate e auditoria autorizados. A solicitação local não deve ser ativada automaticamente só por sincronizar.
4. Gaveta: serviço do aplicativo/driver com comando autorizado e idempotente ligado à venda/recibo confirmado; não depender apenas de um evento arbitrário do navegador.

## Verificação

Testes monetários e de persistência dos metadados, bloqueio de métodos não suportados, isolamento dos quatro contextos, erro de armazenamento, resposta perdida e ausência de duplicação; teste de solicitação/exclusão do clube em IndexedDB simulado; DOM virtual do PDV real, incluindo confirmação em tela cheia, cadastro local e fechamento pelo cliente real com servidor simulado; TypeScript/Vite. Não foi testada gaveta, maquininha, emissão fiscal, câmera ou backend comercial. A prévia usa um servidor simulado separado do código instalado.
