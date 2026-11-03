# Frontend108 — Central de comprovantes

Requer a entrega107, confirmada no computador do usuário com141 testes e build19,70s. Este pacote entrega duas frentes de interface, sem alterar backend, API, migração, permissões, saldo oficial ou pagamentos. A central substitui a lista simples de comprovantes na etapa2 do fechamento existente. Leitura por câmera/OCR106, contagem107 e conciliação97 permanecem disponíveis.

## Consulta e ações

A central apresenta considerados, vinculados à parcela, a revisar, desconsiderados e soma dos considerados. A soma inclui comprovantes pendentes; não representa recebimento bancário. Busca literal por NSU, operadora e terminal, sem distinção de acentos; filtros por forma, situação e período local. Datas inválidas ou invertidas geram erro, sem transformar falha em lista vazia. Paginação de20 registros. Limites de validação existentes:1000 comprovantes por conferência,5000 vendas por consulta.

Detalhes mostram valor, forma, data/hora, NSU, operadora, terminal, estado e motivo da pendência. Alterações no registro após abri-lo bloqueiam impressão e desconsideração até nova conferência. A lista não mostra identificadores internos como nomes de pessoas. O NSU é exibido porque faz parte da conferência autorizada do caixa atual.

Desconsiderar exige confirmação com forma, valor e referência; preserva registro, vínculo anterior e histórico. Não cancela venda, não solicita estorno e não envia comando à maquininha. Usa o salvamento IndexedDB/CAS existente. Só mostra sucesso se o salvamento retornar verdadeiro; falha preserva o registro. A aprovação anterior do responsável é retirada, pois a evidência mudou. Trava síncrona impede envio duplicado pelo mesmo clique. O pai existente mantém guardas de contexto, sessão, alterações não salvas e concorrência.

CSV contém somente a lista filtrada, incluindo situação. Usa o escapador existente contra fórmulas, delimitadores e aspas; download direto por Blob, sem seletor de pasta do aplicativo. O navegador pode perguntar o destino conforme sua preferência de downloads. Impressão individual reutiliza print95, texto escapado e aviso de que conferência não confirma liquidação. Não inclui token, escopo ou chave interna. Não há compartilhamento externo nem novo armazenamento.

## Arquitetura e validação

receipt108.mjs valida a origem antes de filtrar, separa estados e soma centavos com BigInt antes de verificar o limite seguro. receiptExport108.ts concentra CSV/impressão. ReceiptDesk108 não consulta API nem cria cache próprio; recebe somente o contexto já autorizado de CashClosing92. Não adiciona dependências. Adapta-se aos tokens dos dois temas e idiomas existentes;14px auxiliar,16px nos campos, alvos48px, foco visível, controles nomeados e estados textuais.58 chaves novas no pacote:29 desta área e29 da revisão final, completas nos cinco idiomas. Revisão linguística nativa continua recomendada para produção.

Endpoints novos necessários nesta entrega: nenhum. Conferência local e encerramento oficial continuam distintos. Centralização entre aparelhos, retenção corporativa e auditoria inviolável exigem contratos e autorização no backend; não são simuladas aqui.

Testar: registrar/associar comprovantes na etapa de pagamentos; buscar NSU e filtrar forma, situação e período; abrir detalhes; cancelar uma desconsideração e verificar preservação; confirmar e conferir histórico/total; exportar filtro e imprimir detalhes. Uma parcela dividida não deve virar três vendas. Uma referência igual não deve comprovar recebimento duas vezes. Conferir os dois temas, os cinco idiomas,390/768/1440px e falha de salvamento na prévia fictícia.

Limites: sem teste com adquirente real, câmera física, banco comercial ou duas instâncias reais do aplicativo. A prévia é explicitamente fictícia e não persiste nem fecha caixa. Não declara a Parte1 inteira concluída.
