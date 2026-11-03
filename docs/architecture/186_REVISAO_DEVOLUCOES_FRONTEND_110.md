# Frontend110 — Revisão de devoluções e acompanhamento

## Base e escopo

Requer109, instalada no computador do usuário com181 testes aprovados e build20,89s. Esta entrega acrescenta revisão explícita antes de registrar uma solicitação de troca/devolução/cancelamento e um resumo da fila filtrada. Altera apenas Returns100 e adiciona modelos/componentes/traduções/testes/documentação. Não muda Go, SQL, APIs, autenticação, contratos, preço autoritativo B01, pagamentos, estoque ou notas. Não executa reembolso nem representa direito/política de troca; decisão e execução continuam com os responsáveis e serviços existentes.

## Fluxo de solicitação

Localizar venda → escolher produto, quantidade, solução e motivo → Revisar antes de registrar → conferir dados e reconhecer os efeitos → confirmar o registro → confirmação nativa existente → reconsultar comprovante → gravar solicitação local pelo mecanismo anterior. O fluxo não abre uma chamada de reembolso. Troca, crédito e reembolso continuam sendo apenas soluções solicitadas.

Revisão mostra tipo, produto quando aplicável, quantidade, solução, estimativa em centavos e motivo. Cancelamento integral apresenta o valor total e não se confunde com devolução de item. A estimativa usa exatamente estimateReturn100/parseReturnQuantity100: proporcionalidade, arredondamento, produtos por unidade/peso e limites locais não foram reescritos. Cancelamento não combina com solicitações locais ativas de devolução. Venda cancelada, origem desbalanceada, quantidade insuficiente, valor zero ou campos inválidos bloqueiam a revisão.

Reconhecimento do operador fica vinculado a snapshot de contexto, usuário, comprovante validado, solicitações relacionadas e campos atuais. Alterar quantidade, solução, motivo, comprovante, contexto ou solicitações relacionadas exige nova conferência. Voltar mantém os campos e devolve foco ao botão de preparação; ao abrir, o foco vai ao título da revisão. Confirmar sem checkbox ou usando snapshot antigo não chama o registro. Cliques repetidos usam trava síncrona, além da trava já existente no registrador.

No registro, o comprovante é reconsultado pelo cliente existente. Se o serviço retornar dados diferentes, a tela atualiza e exige revisão, sem gravar a solicitação. O limite de quantidade é recalculado dentro da mutação existente, mantendo sua verificação de concorrência. Gravação local continua sob Till94/IndexedDB e revisão CAS. Nenhum schema foi adicionado. Falha booleana agora informa explicitamente que a gravação não ocorreu; campos continuam disponíveis. Erro na decisão do responsável também informa falha e preserva justificativa. Nenhum erro bruto ou segredo é exibido.

O callback da revisão não equivale a sucesso: o componente apenas chama o registrador. Sucesso depende do retorno de mutate no pai e da mensagem anterior de solicitação salva localmente. Cancelar a confirmação nativa não cria registro. Solicitação criada não altera a venda original, o saldo, a nota, o estoque ou os pagamentos.

## Acompanhamento

Resumo acompanha exatamente os filtros de texto/situação da fila existente: aguardando revisão, aprovada localmente, recusada e retirada. O valor em destaque soma somente solicitações aguardando análise. Não é perda realizada, dinheiro devolvido, custo contábil ou saldo de caixa. Há também contagem por reembolso/troca/crédito/cancelamento no conjunto filtrado, incluindo as situações escolhidas. A aprovação sai do total pendente; não vira pagamento executado.

Centavos são inteiros; soma intermediária com BigInt e rejeição de overflow. Chaves duplicadas ou valores não positivos não são tratados como uma fila vazia. Solicitações e comprovante não são alterados pelos modelos de resumo/revisão. A análise continua limitada ao computador/contexto existente; não afirma abranger devoluções salvas em outros dispositivos.

## Permissões e integração

canCreate, locked, actor.permissions e manager94 existentes continuam controlando ações. Área de outro operador pode consultar e não criar nova solicitação. Revisar/aceitar/recusar permanece restrito ao perfil existente. Esta entrega não altera nem amplia permissões. O backend deve verificar autorização e histórico completo ao executar qualquer operação real.

Novos endpoints necessários nesta entrega: nenhum. Reutiliza ReturnSourceContext100 e o fallback existente de comprovantes, ClosingSourceContext92 e mutate do armazenamento local. Sem dependência nova, polling novo ou cache novo. Contratos futuros de efetivação de reembolso/estoque/fiscal não foram inventados nem apresentados como conectados.

## Idiomas e apresentação

14 chaves novas completas em pt-BR/en/es/fr/zh-CN, somadas aos dicionários anteriores. Nenhuma chave nova ausente; tradução estrangeira ainda requer revisão nativa antes da produção. Números por Intl, moeda BRL. Tokens neutros existentes, sem novo azul ou dourado; textos14px, campos herdados16px, botões48px, foco visível e estados textuais. Novas áreas se adaptam a celular/tablet/desktop. Não adiciona animações ou trabalho em segundo plano.

## Testes e aceite

20 testes novos,540 no conjunto completo do frontend: proporcionalidade e centavos, limites locais, estados recusado/retirado, cancelamento separado, origem cancelada/desbalanceada, unidades e peso, snapshots, mudanças de contexto/comprovante, chaves duplicadas, resumo por estado/solução, overflow, imutabilidade e cobertura de textos nos cinco idiomas. Typecheck app, build e git diff --check. Firefox: preparar/revisar sem escrever, checkbox obrigatório, quantidade alterada invalidando confirmação, voltar preservando campos/foco, cancelar diálogo, erro preservando campos, clique duplo, decisão por responsável, valor pendente separado de aprovado, filtros, comprovante alterado pelo leitor e consulta somente leitura. Claro/escuro ×5 idiomas ×3 larguras.

Instalador verifica hashes e base109, cria backup, preserva alterações locais conflitantes e testa antes de concluir, sem commit/merge/push/banco comercial. VALIDACAO.txt registra evidências. A prévia usa fontes fictícias explicitamente substituídas por contexto e mutação em memória, sem API real; não está ligada a uma rota de produção.

Testar /local/pos → Trocas e devoluções: escolher venda/item, quantidade, solução e motivo; revisar e verificar estimativa; alterar quantidade e conferir bloqueio; voltar sem perder dados; confirmar e acompanhar o pedido na fila. Verificar operador comum sem botões de decisão e responsável com justificativa obrigatória. Na prévia, testar falha de gravação, alteração do comprovante e modo somente consulta. Não testados: reembolso bancário, estoque físico, documento fiscal, serviço de devolução real ou concorrência entre aparelhos. Esta entrega não declara a Parte1 inteira concluída.
