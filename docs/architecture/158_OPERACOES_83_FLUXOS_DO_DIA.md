# Entrega 83 — Central de operações (frontend)

Rota: `/local/operations?area=closing`. O menu recebe **Central de operações**, sem substituir as telas operacionais anteriores. A entrada respeita os módulos, licença ativa e permissões já fornecidos pelo servidor. As permissões do cliente não substituem autorização no backend.

## Decisões incorporadas

| Área | Interface entregue | Regra preservada |
|---|---|---|
| Fechamento e malotes | Totais por modalidade nos exemplos, cédulas e moedas, referência de comprovante, lacre com código único, entrega, recebimento e segunda conferência | Venda dividida em parcelas; comprovante não prova liquidação; nenhuma imagem de câmera é armazenada |
| Livro de caixa | Histórico demonstrativo e preparação de suprimento, sangria e correção vinculada | Correção referencia o original; rascunho não é livro financeiro oficial |
| Trocas e devoluções | Compra, canal, datas, motivo, destino do produto, aprovação, solicitação e confirmação simulada de reembolso | Histórico original preservado; reembolso não é contado novamente como perda; pagamento e fiscal independentes |
| Inventário e conferência | Procedimento avulso/semanal/mensal/anual, local, quantidade registrada e contada, movimentos posteriores e aprovação | Empresa decide a frequência; contagem não ajusta estoque automaticamente; saldo reservado offline não é liberado |
| Formulação e qualidade | Versão de fórmula, lote, validade, capacidade e perda prevista, referência de laudo, quarentena e liberação simulada | Documentação e responsável técnico conforme atividade; não calcula segurança de misturas ou valida formulação |
| Reposição e recebimento | Mínimo, máximo, demanda, recusas, aceite humano, confirmação do fornecedor, despacho e recebimento; janela, faltas, pagamento e coleta | Três recusas sugerem contato, não geram pedido; condições pertencem às empresas; nenhum boleto, Pix ou nota é emitido |
| Oportunidades | Faixa de faturamento, atividade, região, período, consentimento, espaço disponível, aceite de teste e avaliação do comércio | Somente grupo comparável autorizado; nenhuma compra de espaço, pedido ou teste é fechado automaticamente |
| Resultado por produto | Receita, custo vendido, mão de obra, perdas, contribuição e gráfico | Volume baixo não significa prejuízo; considerar sazonalidade e negócios comparáveis |
| Aparelhos e continuidade | Rede local/servidor/nuvem, quota exclusiva e simulação de transferência com bloqueio na origem; recuperação | Sem bloqueio não há crédito; expiração não libera; restauração antiga exige revalidação |

## Escopo verdadeiro

Esta entrega implementa **UI, estado de preparação, cálculos de prévia, navegação, tradução e contratos de integração**. Todas as confirmações são explicitamente simuladas. Adaptadores, um por área, retornam `unavailable` por padrão. O botão **Mostrar exemplos** habilita dados sintéticos, sem usar registros reais da empresa. **Preencher com exemplo** solicita confirmação antes de substituir os campos da área.

Nenhuma chamada POST comercial foi criada. Não existe câmera/OCR, assinatura criptográfica, QR escaneável do malote, TEF/Pix real, documento fiscal, transferência P2P, emissão de quota, cálculo tributário ou auditoria durável implementados nesta entrega. O código único do malote é texto copiável com UUID de demonstração; não é autorização de custódia. As telas de operação existentes permanecem acessíveis.

Rascunhos são gravados em IndexedDB com chave empresa + loja + usuário + aparelho, apenas no computador atual. Não armazenam tokens ou senhas. Salvamento ocorre após 600 ms sem edição; o indicador é atualizado depois da conclusão da transação. Há proteção ao sair com alterações pendentes, erro explícito e tentativa manual. No logout a tela e o conteúdo em memória são desmontados; relatórios temporários são apagados pelo mecanismo existente `sessionReport76`. Rascunhos de trabalho persistem para o mesmo usuário e contexto, sem serem usados como fonte oficial de estoque, dinheiro ou autorização offline.

O histórico do rascunho é limitado a 500 ações. Ele **não** substitui um livro de caixa imutável, trilha de auditoria do servidor ou prova de aprovação. No desktop final esses documentos devem ser persistidos no núcleo local autorizado, com isolamento por usuário, criptografia e política de retenção apropriados.

Campos de etapas já submetidas ficam bloqueados no rascunho. **Novo procedimento** limpa os campos da área após confirmação, preservando o histórico de simulações. Para ações com aprovação usa-se o papel existente `owner`/`manager`, além da permissão do módulo para entrar na área. O backend deverá fornecer autorizações específicas por ação; não se adicionam papéis ou permissões à sessão por esta interface.

Valores monetários usam centavos inteiros. Quantidades aceitam três casas decimais e são convertidas em milésimos inteiros; sinais negativos só são admitidos para movimentos líquidos. Não há conversão automática massa/volume ou entre unidades incompatíveis. Capacidade de produção é uma prévia por um insumo limitante, não um motor completo multicomponente ou validação química. As regras do produto e suas aprovações técnicas continuam necessárias.

CSV baixa o rascunho e os exemplos visíveis, rotulados; não abre seletor de pasta. Fórmulas de planilha são neutralizadas. **Abrir em planilha** usa o repasse temporário existente, autorizado para o responsável, sem criar endpoint ou manter relatório em armazenamento permanente. O novo arquivo da planilha é uma cópia do rascunho, não um vínculo automático com a fonte.

## Integrações a implementar no backend

Os caminhos HTTP não são definidos aqui. O contrato tipado `OperationsAPI83` exige contexto autenticado, versão esperada, operação idempotente e resposta versionada. Definir recursos e políticas antes de fixar as rotas:

1. **Fechamentos**: turno, caixa, operador, vendas e parcelas; abertura + dinheiro recebido + suprimentos − sangrias − devoluções − troco; comprovantes sem imagem retida; comparação por identificadores de adquirente, terminal, modalidade, valor e data. Uma soma geral não basta para associar uma notinha à venda.
2. **Malotes**: criação idempotente; código único por empresa/loja/fechamento; valor e cédulas congelados no lacre; responsáveis de cada custódia; conferência independente e divergências. Uma tentativa repetida não cria outro malote.
3. **Livro de caixa**: eventos imutáveis, referências entre venda e parcelas, correções compensatórias, auditoria durável e consulta paginada por contexto. Não somar evento de venda e pagamentos como duas receitas.
4. **Devoluções**: itens e quantidades restantes elegíveis, evidências, política e direitos do consumidor, aprovação específica, destino do produto, reembolso idempotente por parcela e situação fiscal separada. Consulta ao provedor antes de repetir uma solicitação incerta.
5. **Inventário**: versão/snapshot da contagem, movimentos concorrentes, reconferência e aprovação; impedir ajuste que recrie saldo reservado ou desconsidere dispositivos isolados. Acesso de auditor concedido e revogável.
6. **Produção e qualidade**: formulações versionadas de múltiplos insumos; unidade, concentração e parâmetros aprovados; consumos, rendimento e perda reais; lotes, laudos, bloqueios, aprovação técnica e árvore completa de rastreabilidade/recall. Regras regulatórias por atividade não estão codificadas.
7. **Mercado–fornecedor**: sugestão, aceite/recusa, consentimento do indicador de demanda, três recusas/contacto sugerido; confirmação comercial; agenda de descarga considerando paradas manuais; motorista confere saída e entrega, destinatário confirma; recolhimento de devolução e confirmação da chegada; créditos e pagamentos conforme acordo, nunca decisão unilateral do software.
8. **Comparações**: agregação consentida, tamanho mínimo de grupo e proteção contra reidentificação; faixa de faturamento + atividade + região + período equivalentes. Não usar vendas individuais ou dados de concorrentes identificáveis. Oferta/teste e pedido só após decisões das empresas.
9. **Resultado**: margem de contribuição, mão de obra e custos atribuídos, perdas, capital imobilizado, prazo de validade, sazonalidade e consumo na produção. Gráficos não decidem sozinhos se um produto é bom ou ruim.
10. **Continuidade**: quotas duráveis por empresa/loja/SKU/unidade/aparelho; débito confirmado pela origem antes de crédito; nenhuma retirada unilateral da origem desconectada; reenvio idempotente; restauração antiga detectada; pendências de aparelhos revogados preservadas para análise; backup e restauração testados. Bluetooth/Wi-Fi/5G não produzem conexão compartilhada sem um caminho real de rede. Interface não altera essas garantias.

## Idiomas e desempenho

pt-BR, en, es, fr e zh-CN: mesmas chaves, datas/números/moeda com Intl, traduções estrangeiras marcadas para revisão nativa. Sem chaves novas conhecidamente faltantes. Fonte mínima de 13 px. Paleta deriva dos tokens existentes, sem azul fixo; ícones SVG monocromáticos. Grade responsiva, diálogos nativos, foco visível e redução de movimento respeitada. Nenhuma dependência nova em produção; página carregada sob demanda; somente o pequeno rótulo do menu é carregado na tela inicial.

## Verificação

- 18 testes de regras e contratos de frontend: dinheiro, quantidades, parcelas, duplicidade, contagem, recuperação, permissões, aprovação, capacidade, comparação e idiomas.
- TypeScript sem erros e build Vite completo (com avisos anteriores de tamanho do editor de planilhas).
- Testes de componentes em DOM virtual: nove áreas, preenchimento de exemplos, malote completo, aprovação e bloqueio de campos, quarentena, três recusas, recebimento, reembolso, contribuição, comparação, gravação IndexedDB e cinco idiomas. Não equivalem a teste visual em um navegador, dispositivo ou API real.
- O navegador remoto não conseguiu acessar o servidor de desenvolvimento. A revisão visual clara/escura, mobile/tablet e zoom deve ser feita com o roteiro do pacote. Não foi declarada demonstração real de operações de backend.

## Arquivos

- `src/modules/tenant/pages/LocalOperations83.tsx` e `operations83.css`.
- `src/core/operations83/`: regras `.mjs` + declarações, contratos, IndexedDB, exemplos, menu leve e dicionários.
- `src/core/operations83/adapters/`: base, índice e um adaptador por área.
- `src/i18n/operations83/`: cinco idiomas e rótulos mínimos do menu.
- `src/shared/brand/OperationsIcon83.tsx`.
- Integração pontual no router e no menu principal; demais rotas preservadas.
- `tests/operations83.test.mjs` e este documento.
