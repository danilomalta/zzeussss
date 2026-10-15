# Entrega84 — fluxos complementares do frontend

## O que foi implementado

A rota `/local/operations` substitui a central83. O acesso `/local/cash` abre o fechamento novo. A abertura e o fechamento conectados anteriores continuam em `/local/cash/register`, com link explícito. Não foram alterados os endpoints, a emissão de licença, os módulos contratados ou as permissões reais.

| Área | Interações disponíveis nesta entrega |
| --- | --- |
| Fechamento | Várias vendas e parcelas, revisão do pagamento dividido, comprovantes associados por parcela, divergências de valor/modalidade/data/referência/terminal, contagem por cédula e moeda, malote com código e cadeia de entrega/recebimento/segunda contagem. Após conferido, preparar outro fechamento preserva os malotes anteriores. |
| Livro de caixa | Entradas por parcela sem duplicar o total da venda; filtros por data, operador, pagamento e busca; detalhes da origem, novos movimentos e correção compensatória com motivo, preservando o lançamento original. |
| Devoluções | Seleção parcial por item, quantidade restante, destino do item, recuperação e perda separadas; aprovação do responsável; distribuição do reembolso entre as parcelas originais, referência e confirmação individual. Não cancela automaticamente a venda ou documento fiscal. |
| Inventário | Vários procedimentos e produtos, movimentação posterior à contagem, saldo proposto, justificativa, submissão, aprovação ou recontagem; prévia de CSV; seleção e exportação; visualização de auditor sem alterações. |
| Formulação e qualidade | Vários ingredientes e versões de fórmula; aprovação; capacidade pelo insumo limitante; ordens/lotes, produção e consumo reais declarados; ensaios com limites e laudo, quarentena, liberação/rejeição/recolhimento e cadeia insumo–fórmula–lote–destino. Documentos têm prévia temporária e metadados persistidos, sem upload ao servidor. |
| Reposição e entrega | Sugestão com mínimo/máximo, aprovação explícita, terceira recusa sugere contato; condições negociadas; agenda com compromissos Titan/manuais e conflitos de doca/rota/deslocamento; motorista confere saída e recebimento por produto; coleta acompanha até a chegada ao fornecedor. |
| Oportunidades | Faixa de faturamento, atividade, região e período iguais; consentimento e amostra mínima de cinco estabelecimentos; teste opcional condicionado ao fornecedor; aceite do comércio não fecha pedido automaticamente. |
| Resultado por produto | Período/loja/produto, seis meses e mesmo período do ano anterior; receita, custos de produto/mão de obra/perdas/manutenção de estoque, tempo e equipe; contribuição e prévia de desconto. Volume baixo isolado não classifica produto como ruim. |
| Aparelhos | Exemplo com 15 PCs e 20 celulares, busca/filtros/paginação, cotas e filas; solicitação → bloqueio na origem → crédito → conciliação; repetição não duplica crédito; revogação preserva pendências; restauração suspende direitos antigos e nova atribuição consome o saldo central. Três topologias são cenários informativos. |

## Persistência e contexto

Os rascunhos usam IndexedDB, com chave de empresa, loja, identidade e aparelho derivada da sessão existente. Autosave só mostra concluído após o commit do armazenamento. Há aviso de alteração não salva, tratamento de falha e tentativa manual. A carga de exemplos requer confirmação e é claramente identificada. Não há dados de outro cliente ou credenciais no pacote.

Permissões e licença existentes determinam a navegação. Caixa exige POS/venda; inventário exige gestão de estoque; qualidade exige produção; reposição exige pedidos; resultado e aparelhos ficam restritos ao responsável. Aprovações locais são limitadas a responsável/gerente. Isso é comportamento de frontend: o backend deve repetir todas as verificações e derivar o contexto da sessão. Não conceder permissões para testar; usar a prévia isolada.

Os nove adaptadores em `core/operations84/adapters` têm contratos tipados, um arquivo por fonte, e retornam `unavailable` por padrão. Exemplos só são devolvidos com flag explícita na criação do adaptador. Nenhuma URL foi inventada. A UI trabalha sobre o rascunho do contexto, sem apresentar os exemplos como consulta comercial. Os adaptadores são pontos de extensão para a próxima etapa, sem integração de leitura real nesta entrega.

CSV baixa diretamente, com UTF-8/BOM e proteção contra fórmulas injetadas. Abrir em planilha usa a transferência temporária já existente, restrita ao responsável. Não procura diretórios. Arquivos CSV de inventário aceitam até 1MB/500 linhas, com cabeçalho técnico estável:

`sku;unit;location;expected;counted;movements`

Unidades: `unit`, `kg`, `g`, `litre`. Quantidades aceitam vírgula ou ponto, até três casas. Movimentos podem ser negativos. A prévia precisa ser confirmada; linha inválida impede aplicar o lote. A confirmação só acrescenta ao rascunho, sem ajustar estoque comercial.

## Contratos necessários no backend — nomes conceituais, sem URLs presumidas

| Fonte / comando | Contrato necessário e garantias |
| --- | --- |
| Vendas e fechamento | Listar turno/loja/operador, itens e parcelas; saldo inicial, suprimento, sangria, devolução; paginação e versão. Gravar fechamento com conferência, divergências, aprovador e idempotência. |
| Comprovantes e pagamentos | Associar metadados do comprovante à parcela, validar duplicidade por provedor/terminal/referência; estado do provedor separado da evidência observada. OCR pode sugerir campos, nunca confirmar liquidação. |
| Malote | Lacre único, snapshot imutável da contagem, cadeia de custódia, recebimento e segunda contagem com autor/data/motivo e permissão. |
| Livro | Consulta paginada e rastreável; lançamento compensatório, referência ao original e impedimento de compensação duplicada. |
| Devolução | Quantidade disponível por item, solicitação/decisão, disposição, valor/custo/recuperação, reembolso por parcela e status fiscal/pagamento separados. Idempotência e concorrência nas quantidades. |
| Inventário | Snapshot, movimentos durante contagem, linhas, importação com prévia/validação, submissão, recontagem e aprovação. Ajuste final transacional, nunca silencioso. |
| Produção e qualidade | Fórmula versionada, unidades e conversões, insumos/lotes, produção/consumo, ensaios/laudos, aprovação técnica, anexos com upload privado e retenção, rastreabilidade e bloqueio/recolhimento real. |
| Reposição e agenda | Política acordada entre as partes; pedido/aceite/recusa/condições; agenda com compromisso e conflito; motorista, quantidades e provas; recebimento e ajuste financeiro autorizados. |
| Coleta | Autorização do fornecedor, produto/quantidade, próxima entrega ou rota/prazo negociados, retirada, chegada e crédito/dinheiro/produto conforme política. Sem decisão contratual automática. |
| Oportunidades | Agregação consentida com grupo comparável, amostra mínima e proteção contra reidentificação; proposta/teste e aceite bilateral. Não expor vendas individuais de concorrentes. |
| Resultado | Séries por produto/loja/período com origem dos custos, horas/equipe, perdas e sazonalidade; qualidade/completude dos dados e regras de aprovação de preço. |
| Continuidade | Identidades/chaves, cotas exclusivas, bloqueio durável da origem, transferências idempotentes, filas assinadas, revogação e revalidação após restore. Queda de prazo não devolve automaticamente direitos. Backend local SQLite e sincronização efetiva permanecem pendentes. |
| Rascunhos | Armazenamento por contexto, versões/concorrência, auditoria de servidor e política de retenção/limpeza. Os registros locais não constituem prova de servidor. |

## O que não está operacional nesta entrega

- OCR/leitura automática de texto: a câmera tem visualização ao vivo e entrada manual dos metadados, sem capturar ou guardar fotos. Requer permissão do navegador e contexto seguro. Câmera real ainda não testada.
- Captura/liquidação de cartão/Pix, estorno, emissão/cancelamento fiscal, entrega/coleta real, contagem oficial, assinatura de laudos e atualização de estoque.
- Comunicação real entre os 35 aparelhos, eleição de coordenador, criptografia de mensagens, resistência a clonagem e restauração antiga. O painel exercita estados no rascunho, não oferece garantias de protocolo distribuído.
- Consulta real aos dados do PDV nesta central. Até ligar os adaptadores, as vendas exibidas são exemplos explícitos. O link de caixa conectado preserva a operação anterior.
- Persistência dos bytes de anexos: só os metadados permanecem após recarregar; a tela informa essa limitação. Não usar como arquivo oficial.
- Revisão visual em navegador ou hardware/celular real. Foram usados build, testes de domínio e DOM virtual. Não há alegação de certificação, conformidade fiscal ou ensaio químico aprovado.

## Idiomas e peso

As cinco traduções têm as mesmas chaves, sem chaves literais novas faltantes nos componentes. `en`, `es`, `fr`, `zh-CN` precisam de revisão nativa. Nomes de empresas/produtos e referências de exemplo são identificadores, não traduções. Valores monetários são centavos e quantidades são milésimos inteiros; exibição usa Intl. Os componentes são carregados por área, sem dependência nova em produção. O aviso de chunk grande do editor de planilhas anterior continua; esta entrega não demonstra desempenho em PC antigo.

## Validação e roteiro

Testes Node selecionados: 40 aprovados, incluindo 26 novos. Dez testes do instalador verificam integridade, conflitos, idempotência, rotas preservadas, personalizações e links simbólicos. TypeScript e build Vite passaram. Testes de DOM virtual exercitam as nove áreas, câmera simulada com encerramento dos tracks, malote, retorno parcial, inventário, rastreabilidade, agenda, filtros, gráfico, exportação, aparelhos, transferências, autosave, cinco idiomas e restrições de empregado.

No navegador: abrir a prévia isolada, carregar demonstração, conferir R$296 em espécie e três vendas divididas; fechar malote com segunda contagem; testar devolução parcial; submeter inventário e recontar; capacidade58,8kg por insumo limitante; completar ensaio antes de liberar lote; conflito de agenda; três recusas; coleta até fornecedor; comparar só grupo autorizado; filtrar seis meses; percorrer15+15+5 aparelhos; transferir3 unidades sem duplicar no replay; recarregar após salvo. Conferir temas, Tab/ESC, zoom50–150%, tablet e celular no equipamento do usuário.
