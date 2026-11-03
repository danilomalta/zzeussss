# Frontend103 — Central de histórico, vendas e revisões do caixa

## Escopo e base
Pacote exclusivamente frontend, requer frontend102 já aplicado. Mantém96–102. Dois arquivos existentes alterados: Report98.tsx e Phone95.tsx; novos componentes/modelos/adaptador/testes/traduções e prévia. Sem Go, SQL, migração, alteração de permissões ou banco comercial. Sem dependências novas e sem commit/merge/push.

## 1. Histórico e comparação
Histórico dos fechamentos arquivados pelo mecanismo95, isolado por empresa/loja/aparelho/usuário e modo da sessão. Busca literal por referência, período inicial/final incluindo o último dia, filtro sem diferença/falta/sobra, ordenação por data com desempate estável, paginação de10 registros, botão Abrir relatório e CSV apenas dos resultados filtrados. Comparação de dois arquivos distintos da mesma origem mostra a variação exata do segundo em relação ao primeiro para esperado, contado e diferença. Não soma dinheiro de caixas diferentes e não apresenta lucro ou produtividade.

O arquivo local não é índice oficial de todos os caixas da empresa. Preserva o limite existente de100 fechamentos por origem. O botão Abrir respeita telas com relatório fixo. Continua possível selecionar o caixa atual no seletor existente. O resumo oficial101/102 permanece independente e exige consulta ao servidor B02B2.

## 2. Vendas e comprovantes
Lista do caixa selecionado reutiliza a fonte ClosingSource92 já carregada pela tela, sem fazer uma segunda varredura. Busca por referência, período, estado e meio; paginação de10; CSV das linhas filtradas; indicação explícita quando a fonte é parcial/indisponível, sem apresentar esta lista como total completo do caixa. Card legado e other permanecem distintos de débito/crédito.

Consulta por linha ou referência de venda, validada contra o caixa selecionado. Mostra itens, quantidades, preços originais e subtotais; partes de pagamento e estado gravado; total e indicador fiscal do registro. CSV, prévia e impressão nos formatos58mm/80mm/A4. Nenhuma cobrança, reembolso ou emissão fiscal ocorre. Impressão é comprovante de registro, não documento fiscal.

Reutiliza gateway.checkout96.receipt quando integrado, ou localClient.receipt legado. A alternativa legada continua restrita ao contrato em dinheiro que já validava; este pacote não afrouxa o cliente para fingir suporte real a vendas mistas. A prévia e os testes usam adaptador explícito com exemplos. Não inventa nova rota. Quando o registro não tem nome/unidade imutável do produto, usa item por ordem e referência com aviso; não consulta o preço atual nem inventa nome histórico. Não mostra o UUID do operador.

Valida IDs de venda/caixa, quantidades positivas, métodos conhecidos, estados gravados, IDs duplicados e somas em centavos com BigInt. Mudança de caixa, empresa/usuário/token ou saída da tela cancela a atualização visual pendente. Dados do comprovante ficam só na memória. Não há cache permanente novo nem dados de funcionário em logs.

## 3. Solicitações
Fila somente leitura do estado Till94 já carregado para a origem atual. Busca por motivo/referência, filtro de situação, paginação e CSV. Exibe aguardando revisão, aprovada localmente, recusada ou retirada. Motivo e justificativa de revisão são preservados. Não calcula saldo efetivo dessas solicitações, não registra movimento e não permite aprovação por uma nova entrada que contorne as permissões existentes. Para decidir, usar a área original correspondente.

## 4. Leitura de comprovantes pelo celular
Phone95 agora limpa valor, método, data e referência ausentes a cada nova leitura OCR. Uma notinha não herda os dados da anterior. Método vazio exige seleção e não passa na validação do envio. Os controles existentes de câmera, parada e descarte dos frames foram preservados. Nenhuma foto é salva por esta entrega. Reconhecimento real continua dependente do OCR/hardware e da conexão já disponível no mecanismo95.

## 5. Aparência, acessibilidade e idiomas
Paleta/temas existentes sem azul novo; alvos de44px, fontes mínimas13/14/16px, foco visível, rótulos em formulários, captions/cabeçalhos nas tabelas e detalhes acessíveis por teclado. Tabelas rolam internamente no celular. Formatação por Intl e consultas sem temporizador de atualização em segundo plano. pt-BR/en/es/fr/zh-CN:60 chaves novas completas; nenhuma chave nova vazia ou ausente. As quatro traduções estrangeiras ainda requerem revisão por falante nativo antes de produção.

## 6. Prévia separada
PREVIA.html e ver_previa.py abrem uma demonstração sem login ou senha real. Dados fictícios e comandos de pagamento indisponíveis. As fixtures ficam em preview103, sem importação por rotas de produção. Arquivos IndexedDB da demonstração usam origem103 e modo demo, separados dos arquivos live. A prévia não faz chamadas à API. Reabrir preserva registros demonstrativos sem falha de revisão.

## Contratos existentes e pendências do backend
Reutilizados: fonte de vendas existente, consulta de comprovante /local/v1/sales/{id}, relatório oficial /local/v1/cash/sessions/{id}/report da B02B2 e adaptador checkout96.receipt quando implementado. Nada novo é exigido para a parte local desta entrega.

Ainda faltam para alcance empresarial: índice oficial paginado de fechamentos por loja com autorização de gerente; consulta oficial global de solicitações e estados aplicados; nomes/unidades imutáveis em itens de recibo; integração completa dos pagamentos mistos com confirmação e movimentação financeira. São contratos a definir no chat de backend; não foram inventados endpoints nem respostas bem-sucedidas para eles.

## Validação e limites
Suíte completa frontend:415 testes aprovados. Testes de domínio novos cobrem filtros, datas, isolamento, centavos, duplicidade, comparações, comprovantes, estados gravados, OCR e traduções. Firefox com respostas interceptadas: filtros e comparação, CSVs, comprovante315 e partes, impressão58mm, registro inválido, fila local, mudança de caixa durante consulta, cinco idiomas, três larguras e dois temas. Testes independentes da prévia cobrem carregamento sem API e reabertura. Resultados finais em VALIDACAO.txt.

Não executados: servidor Go real/B02B2 integrado, PSP/adquirente, SEFAZ, câmera física, impressora térmica física, comunicação entre celulares ou aceite visual no PC do proprietário. Não declarar a Parte1 inteira concluída por estes testes.
