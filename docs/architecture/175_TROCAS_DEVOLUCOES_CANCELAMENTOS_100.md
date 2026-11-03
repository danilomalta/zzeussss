# Frontend100 — solicitações de trocas, devoluções e cancelamentos

Requer 96+97+98+99. Escopo: interface, estado local, leitura dos clientes existentes e contrato de adaptação. Não altera arquivos da B01-0f6, APIs, package.json, backend ou migrações.

## Fluxo entregue
1. Localizar a venda por lista paginada do turno (20 por página), busca por data/referência ou referência do comprovante. A consulta direta pode encontrar venda de turno anterior quando o serviço existente autorizar. Data ausente fica indisponível, sem inventar data atual.
2. Consultar total e parcelas do comprovante original; escolher item, quantidade e solução: reembolso, troca ou crédito no estabelecimento. Nome/unidade usam o catálogo atual quando disponíveis, pois o comprovante legado não contém esses campos imutáveis; item ausente usa rótulo neutro e aviso. Valor vem do total líquido da linha original, nunca do preço atual do catálogo.
3. Prévia de quantidade original, solicitações locais ativas, quantidade disponível neste registro e valor estimado. Confirmar gera somente uma solicitação persistida localmente.
4. Na confirmação, a venda é consultada novamente. Identidade, itens, pagamentos, total, estado e autorização fiscal devem coincidir com a consulta revisada; se mudarem, a tela mostra a nova resposta e exige nova confirmação. Nenhuma alteração silenciosa.
5. Fila com busca, estado, páginas de 10, impressão/PDF, CSV, retirada e revisão com justificativa. Consultar venda de origem abre os itens correspondentes para conferência, sem executar devolução.

## Quantidade e centavos
- Produtos do catálogo por unidade exigem inteiros; produtos fracionáveis usam milésimos. Sem unidade no catálogo, a consulta original precisa ser conferida pelo responsável. O backend deverá devolver a unidade imutável da venda e validar a quantidade.
- Valor estimado distribui o restante líquido da linha entre a quantidade ainda não solicitada. Não permite exceder valor/quantidade neste registro local.
- Uma linha de 100 centavos e 3 unidades gera estimativas 33,34,33 para solicitações de 1 unidade. Se retirar a intermediária 34, o próximo pedido da unidade restante estima 34; não perde um centavo nem duplica o total.
- Solicitações recusadas/retiradas liberam apenas a margem local de solicitação. A venda, estoque, dinheiro e nota originais não são modificados.
- Pedido integral não pode coexistir com devoluções locais ativas da mesma venda. Estimativa zero, quantidade inválida e resposta inconsistente bloqueiam novo registro.
- As regras de rateio são estimativas do cliente. O serviço responsável deve recalcular valores, tributos, descontos, pagamentos e histórico integral antes de efetivar.

## Revisão e permissões
- Reutiliza sell/manage_cash e os papéis existentes. Interface de devoluções é oculta para perfil sem essas permissões. Isso não substitui autorização no servidor.
- Somente gerente/dono com manage_cash faz revisão local; justificativa mínima de 3 caracteres. Quem solicitou ou gerente pode retirar solicitação pendente/aceita localmente, preservando histórico.
- Gerente pode selecionar a área local de outro operador na mesma empresa/loja/estação pelo seletor já existente; cria novos pedidos na própria área. Consulta por referência continua sujeita à autorização do servidor; o frontend não comprova empresa a partir do código.
- useTill94 agora devolve estado vazio/não pronto durante mudança de contexto até carregar a chave correta, evitando reaproveitar visualmente o registro do operador anterior. Resultado de leitura atrasada é descartado; campos não salvos continuam usando o aviso de navegação existente.
- Controle de revisão/CAS existente protege contra sobrescrita entre abas. Não existe coordenação global de devoluções entre usuários/aparelhos nesta entrega.

## Integração usada
ClosingSourceContext92 para listar vendas; CheckoutAdapter96.receipt quando instalado; senão localClient.receipt existente. ReturnSourceContext100 é um adaptador tipado opcional, sem URLs adivinhadas; a prévia o fornece explicitamente com dados fictícios. Produção não habilita mock automaticamente.
A leitura legada não aceita AbortSignal; sua resposta atrasada é ignorada. Adaptadores que o aceitam também recebem o cancelamento.

Solicitações permanecem em tillStorage94, com retorno94/cancel94/decide94/withdraw94. A única mudança no cálculo dessas solicitações é o rateio do saldo líquido restante; schemas e assinaturas operacionais existentes são preservados. Declarações TypeScript passam a aceitar Receipt96, que mantém o estado fiscal recebido sem forçá-lo a false.

## O que depende do backend
Não há reembolso, crédito liberado, troca concluída, cancelamento de venda/nota, movimentação de estoque, perda contabilizada ou estorno bancário nesta entrega. Aceita localmente não equivale a operação autorizada/efetivada.

Contratos necessários, sem estabelecer URLs:
- Consulta autorizada da venda original, com unidade/nome imutáveis, quantidades e totais líquidos, parcelas, loja e referência fiscal; histórico completo das devoluções em todos os aparelhos/usuários.
- Cotação autoritativa da devolução e do cancelamento, descontando devoluções já confirmadas, com validade/revisão e políticas da empresa.
- Criação idempotente da solicitação e decisão do responsável com motivo, sessão e escopo derivados no servidor.
- Recebimento físico, condição/destino do produto (retorno vendável, quarentena/perda, fornecedor), lote/local e auditoria.
- Execução idempotente e atômica dos efeitos cabíveis de estoque/financeiro; chamadas ao provedor para reembolso e ao serviço fiscal para eventos permitidos. Cada dimensão possui estado separado, com consulta de resultado, falhas e resposta incerta.
- Política de crédito/troca, documentos e comprovante final. O software registra a política configurada; não determina direitos ou acordos comerciais por conta própria.
- Reconciliação offline e rejeição de excesso global, preservando registros conflitantes. O saldo local de solicitações não comprova elegibilidade global.

## Como testar
Na rota existente /local/pos, abrir Trocas e devoluções:
1. Consultar venda, verificar itens/parcelas e registrar solicitação com motivo. Mudança de preço atual não muda o valor original consultado.
2. Solicitar uma parte por vez; verificar quantidade e estimativa, retirar uma solicitação e conferir o saldo local recalculado.
3. Abrir nova sessão real de gerente autorizado na mesma estação, selecionar área do operador e revisar com justificativa. A UI mostra apenas aceitação local.
4. Recarregar, filtrar, imprimir ou exportar. Documentos ficam identificados como solicitações não efetivadas.
5. Na prévia isolada, alternar dados alterados, inválidos, venda cancelada e falha. Alteração entre consulta/confirmação exige nova revisão; resposta inválida não vira pedido válido.
6. Cinco idiomas, claro/escuro, larguras 1440/768/390; perfil sem permissão sem formulário/fila. A troca de perfil da demonstração não altera o login real.

Sem dependência nova. Chaves novas cobertas nos cinco idiomas; en/es/fr/zh-CN precisam de revisão nativa. Resultados efetivamente executados em VALIDACAO.txt. Não declara conclusão comercial da Parte1 nem integração fiscal/bancária real.
