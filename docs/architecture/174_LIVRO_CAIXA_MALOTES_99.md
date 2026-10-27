# Frontend99 — livro de caixa e malotes

Atualização de interface sobre96+97+98. Mantém as rotas e a integração de Till94. Não modifica B01-0f6, servidor, APIs, schemas, package.json ou migrações.

## Livro de caixa
- Vendas registradas, solicitações locais e atividade local são origens diferentes. Aceitar uma solicitação não efetiva um movimento nem altera dinheiro das vendas.
- Busca por referência/motivo, filtros por origem, forma de pagamento, estado e período; ordenação e páginas de 25. Tabela com rolagem própria e cabeçalho fixo.
- Identidade por venda+parcela; não descarta parcelas com mesmo ID em vendas diferentes. Centavos inteiros e agregação protegida contra overflow.
- Não calcula saldo da gaveta com uma fonte que só fornece vendas. Fundo inicial/sangria/suprimento oficiais ainda dependem de fonte própria do backend. O relatório98 continua mostrando o resultado oficial disponível.
- Uma fonte incompleta não produz total zero. Em uma falha de atualização, últimos registros permanecem visíveis com aviso e data da última consulta; totais passam a desconhecidos.
- Atualiza ao entrar e a cada 60 segundos enquanto a tela estiver visível. Cancela resultado obsoleto quando sai/troca contexto; não persiste novo cache de vendas. Limite de 20.000 parcelas por consulta, com recusa explícita acima disso; volumes maiores exigem paginação no serviço.
- CSV dos registros filtrados e atividade local, sem seletor de pasta do aplicativo. CSV identifica origem e limites; fórmulas são neutralizadas. O navegador ainda pode pedir destino segundo sua configuração.
- Impressão/PDF pelo diálogo do navegador, com valores dos registros filtrados. Exportação e tela não afirmam liquidação bancária.

## Malote
- Criação lê a contagem confirmada existente em closingStorage92; valor positivo, destino e política explícitos. Não cria saldo ou contagem fictícia.
- Usa as operações existentes seal94/bagStep94 e a persistência com controle de revisão de tillStorage94. Lacre → entrega → recebimento → conferência. Nenhuma etapa é recriada ou apagada.
- Duas pessoas: quem lacrou não recebe nem confere o próprio malote. Conferência final continua exigindo perfil/permissão de gerente. Responsável único só é selecionável por gerente, conforme regra existente.
- Segunda contagem por 13 denominações (cédulas/moedas), incluindo zero explícito. Diferença exige motivo de pelo menos 3 caracteres. Registro confirmado é preservado e não aceita repetir a última etapa.
- Cadeia incompatível com caixa, etapas fora de ordem, conferente inválido, contagem/diferença incoerentes ou horários regressivos interrompem ações. Nada é reparado automaticamente.
- Etiqueta imprime documento próprio de 80 mm com código, destino, valor, política e etapas; não imprime toda a página. Código pode ser copiado; não é prova criptográfica ou autorização remota.
- Aviso de campos não salvos reaproveita a navegação existente. Dois operadores só acessam registros locais de outros usuários no mesmo contexto de empresa/loja/estação por meio do seletor já existente para gerente.

## Limites e integração futura
Toda ação do malote continua sendo um registro deste computador. Não comprova presença física, recebimento por outro aparelho, identidade de pessoa ausente, transporte ou conferência no servidor. O frontend não é barreira de autorização. A regra de permissão precisa ser aplicada pelo backend antes de habilitar recebimento remoto.

O contrato futuro precisa oferecer, sem inventar URLs agora:
1. Consulta paginada de movimentos oficiais do caixa, com IDs, data, tipo, centavos, estado e origem; abertura e saldo oficial separados da lista paginada.
2. Malote com empresa/loja/caixa, lacre, valor declarado, destino e política. Criação idempotente e revisão/versão retornadas pelo servidor.
3. Comandos de entrega, recebimento e conferência: ator derivado da sessão, verificação da política, transação atômica, motivo da diferença, controle de versão, idempotência e auditoria.
4. Consulta de malotes autorizados por responsável/caixa, cadeia íntegra e estado canônico, além de recibo de conferência.
5. Reenvio offline com confirmação idempotente; disputa preservada e sem duplicar movimento financeiro.

Na B01, a fonte legada fornece vendas reais em dinheiro. Os exemplos de cartão/Pix da prévia são fictícios. Integração de pagamentos mistos e movimentos oficiais depende do backend correspondente.

## Testar
No PDV /local/pos, usar as abas já existentes Livro de caixa e Malote.
1. Livro: consultar vendas, filtrar, paginar, pesquisar motivo/referência, exportar CSV e imprimir. Solicitação aprovada localmente não entra no total de dinheiro vendido.
2. Falha/consulta incompleta: total como traço e registro anterior identificado pela última consulta; nenhum saldo oficial inventado.
3. Contar e confirmar dinheiro na tela existente /local/cash/register; voltar ao Malote, escolher destinatário e registrar lacre/entrega.
4. Com outro usuário gerente autorizado na mesma estação, selecionar a área do operador e registrar recebimento/segunda contagem. Para testar sozinho, a prévia permite perfis fictícios; não muda o login real.
5. Diferença de contagem exige justificativa. Recarregar preserva histórico; duas abas tentando gravar sobre mesma revisão geram conflito, sem sobrescrita.
6. Etiqueta imprime somente identificação e cadeia. Testar cinco idiomas, claro/escuro e larguras 1440/768/390.

Prévia explícita com dados fictícios acompanha o ZIP e não exige senha. Não acessa banco/API real. Trocar o perfil nessa prévia não autoriza nenhuma operação no sistema real.

Todas as54 chaves novas têm tradução nos cinco idiomas. Traduções en/es/fr/zh-CN precisam de revisão nativa antes da produção. Nenhuma dependência nova. Resultados efetivamente executados constam de VALIDACAO.txt. Não declara a Parte1 comercial concluída.
