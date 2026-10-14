# Entrega 70 — planilhas e integração no cliente

## Cinco áreas entregues

1. Interface compacta: grade ocupa a área restante, menu Arquivo, controles extras recolhidos, congelamento e localizar/substituir. Motor Univer existente, com os mesmos cinco presets já instalados na entrega 69: núcleo, filtros, ordenação, formatação condicional e validação. Não há dependência de Microsoft Excel.
2. Persistência local: IndexedDB separado por empresa/loja/aparelho/usuário; autosave a cada dois segundos quando há mudanças; proteção de revisão contra duas abas; abrir, pesquisar, renomear, duplicar, excluir e recuperar a última planilha. Até 20 versões com snapshots a cada cinco minutos de alterações, limitados a 12 MB por documento com versões. Falha de gravação não é apresentada como sucesso.
3. Modelos e ferramentas: oito modelos com fórmulas reais, cópia de valores da seleção para outra planilha, resumo por categoria e gráfico existentes, exportações Titan/XLSX/CSV existentes e impressão/PDF pelo navegador com cabeçalho, rodapé, orientação e área. Desfazer/refazer do motor continua durante a sessão e troca de tema/idioma; abrir outra planilha ou restaurar versão reinicia esse histórico.
4. Leitura dos módulos: prévia com colunas selecionáveis; catálogo (inclusive custo quando o servidor autoriza), vendas básicas, funcionários e pedidos de compra pelos clientes existentes. Blocos vinculados com atualização manual, ao abrir e por intervalo, filtros persistidos, rastreamento de inserção/remoção de linhas e colunas, conferência de colisão antes de expandir e pausa do vínculo. `TITAN("catalog","price","SKU")` consulta dados previamente carregados nesta sessão; não executa HTTP durante o cálculo e retorna `#N/A` quando não há dado. A descrição da função aparece no motor e na ajuda. Não calcula totais mensais nem consulta estoque real por uma fórmula inexistente.
5. Assistentes: prévia de importação de preços, inventário, produtos e funcionários, mapeamento, validação básica e CSV de erros; formulário de compartilhamento, exportação e compartilhamento de arquivo quando o navegador permite. Links de e-mail e WhatsApp apenas abrem a composição; o usuário precisa anexar o arquivo. Botão contextual Abrir em planilha nos módulos permitidos. Filtros que já estão na URL são transportados; filtros apenas em estado interno de uma tabela precisam ser escolhidos na prévia.

## Dados e permissões

- Nenhum POST novo é enviado a outros módulos nesta entrega. Aplicar importação em lote, compartilhar ACL, criar link compartilhável e enviar ao portal do contador permanecem bloqueados porque não há cliente/contrato validado para essas operações.
- A interface de armazenamento empresarial é `CompanySheetStorage`, configurada por `configureCompanySheets`. Não existe chamada a uma rota fictícia. Sem conector configurado, usa IndexedDB. A autenticação e a autorização definitivas continuam no servidor.
- Os valores de blocos vinculados e os resultados armazenados de fórmulas são removidos da cópia persistida. Configuração, fórmulas e células manuais permanecem. Ao abrir, uma fonte autorizada precisa ser consultada novamente. Na perda de conexão, últimos valores ficam somente na sessão aberta. Resposta 401/403 limpa valores da fonte; 401 invalida a sessão.
- Pausar o vínculo mantém valores nesta sessão, sem gravar seu cache no navegador. Cópias locais de uma seleção pertencente a uma planilha vinculada também retiram os valores derivados. Arquivos baixados deliberadamente podem conter dados sensíveis: revise o destinatário.
- O histórico Atividade é local e editável pelo titular do navegador. Não substitui auditoria durável do servidor, nem é prova de autoria. A interface de compartilhamento não dá acesso a outra pessoa.
- O escopo local é também por origem do navegador: mudar host ou porta abre outro IndexedDB. Use uma porta fixa para não parecer que os arquivos sumiram.
- Vendas leem até 500 registros e avisam quando a listagem pode estar incompleta; pedidos leem a página oferecida pelo cliente e mostram limitação quando ela está cheia. Não apresentam esses recortes como demonstrativo contábil integral. Filtro de data aplica-se a vendas e pedidos; busca textual a catálogo, funcionários, fornecedor de pedidos ou identificador/status da venda.
- Salário, ponto, escala real, saldo por local, lotes/validade, vendedor, DRE e contas a pagar/receber não são deduzidos de dados incompletos. Estoque e financeiro têm apenas exemplos atrás de `VITE_SHEETS_MOCKS=true`, desligada por padrão e marcada DEMONSTRAÇÃO na prévia.

## Contratos de backend necessários (propostas, não rotas implementadas)

| Área | Contrato necessário |
|---|---|
| Planilhas | listar/ler/criar/atualizar/excluir documentos; identidade obtida da sessão; revisão obrigatória/If-Match; limites de tamanho; ACL por usuário/perfil; nenhum tenant confiado ao corpo do cliente |
| Versões | listar snapshots, restaurar com revisão esperada e operação idempotente; retenção e registro de autor no servidor |
| Compartilhamento | destinatários autorizados, concessão/revogação leitura/edição, validação de cada fonte também na leitura de arquivo compartilhado, link revogável e com expiração |
| Vendas | agregados por produto, operador e meio de pagamento, período e loja; fechamento de caixa; identificação de abrangência completa e versão do relatório |
| Estoque | saldos por produto/local, lotes, validade, mínimo; filtros e paginação com snapshot consistente |
| Compras/financeiro | pedidos paginados, vencimentos, contas a pagar/receber, recebimentos, despesas e DRE com regras e completude explícitas |
| RH | fontes distintas para funcionários, escalas e ponto, permissões por campo; salários nunca expostos apenas porque há permissão para listar nomes |
| Importação | prévia validada no servidor, token/digest da prévia, operação idempotente e revisão esperada; aplicação atômica, rollback e recuperação do recibo; relatório de erros, sem senha em planilha |
| Contador | pacote autorizado, upload com limites e recibo, acesso ao portal e auditoria de envio |
| Auditoria | eventos imutáveis de criação/edição/vínculo/compartilhamento/importação, autor derivado da sessão |

As leituras atuais usam `/local/v1/catalog/search`, `/local/v1/sales`, `/local/v1/staff` e `/local/v1/purchase-orders` através dos clientes existentes. Nenhum endpoint online foi conectado presumindo que uma sessão local fosse equivalente à online.

## Em breve e limites explícitos

- Tabela dinâmica completa está desabilitada; o resumo por categoria da entrega 69 continua disponível.
- Aplicação de importações e compartilhamento entre pessoas/contador dependem dos contratos acima. Não há confirmação simulada de escrita real.
- O envio considera uma aba ou uma seleção; importação atômica de várias abas demanda contrato adicional. Cópia para outra planilha copia valores, não transforma referências de fórmulas.
- Não há fidelidade total de Excel: VBA, Power Query, integração Microsoft, recursos avançados de gráficos e tabela dinâmica não são adicionados por este pacote. XLSX continua seguindo o conversor já instalado na entrega 69.
- Blocos vinculados são intervalos mantidos pelo adaptador. Ctrl+Z trata células do motor; para interromper uma fonte use Pausar vínculo. Não há promessa de restauração do vínculo pela pilha nativa de undo.
- pt-BR, en, es, fr e zh-CN têm o mesmo conjunto de chaves novas, sem chaves novas faltantes. en/es/fr/zh-CN estão marcados para revisão por falante nativo. Campos e nomes provenientes dos dados não são traduzidos artificialmente.

## Verificação realizada e no computador do usuário

Foram verificadas as assinaturas TypeScript da integração com os presets 1.0.4, 112 testes Node da base disponível (99 anteriores e 13 novos), e um ensaio isolado em Chromium com o motor Univer real e os componentes novos. Esse ensaio usa respostas HTTP controladas: valida prévia e inserção, armazenamento IndexedDB real, remoção de caches vinculados, rejeição de revisão antiga, fechamento do modal, altura sem scroll da página, cinco idiomas, troca de tema mantendo o documento/undo, função TITAN e recuperação ao recarregar. Não foi um teste em banco comercial, nem acesso a dados de produção.

O código completo dos auxiliares 68/69 do computador do usuário não foi recebido neste ambiente. As declarações usadas para conferência de tipos e o ensaio isolado não são incluídos nem substituem esses auxiliares. O instalador mantém esses arquivos e a configuração npm, executa TypeScript/build e TODOS os testes `.test.mjs` na base real antes de registrar o commit. Se uma verificação falhar, preserva alterações e backup, sem reaplicar automaticamente.
