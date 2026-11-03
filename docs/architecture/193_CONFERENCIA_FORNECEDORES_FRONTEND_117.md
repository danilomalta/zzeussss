# Frontend117 — conferência por fornecedor

## Fluxo entregue

Em `/local/payables/agenda`, a ação **Visão por fornecedor** transforma as parcelas já consultadas em uma conferência por fornecedor: quantidade de documentos/parcelas, principal aprovado, pendente, recusado e cancelado, aplicado em dinheiro declarado, saldo aprovado, saldo vencido e próximo vencimento com saldo. Agrupamento usa a identidade do fornecedor, nunca somente seu nome. Detalhes mostram as parcelas daquele fornecedor com acesso ao documento e ao assistente116 existente.

Faixas de vencimento: posterior à referência, na referência, 1–7, 8–30, 31–60, 61–90 e mais de90dias antes. As faixas dividem somente saldo aprovado, não principal nem pagamentos novamente. Datas são calculadas por dia UTC, incluindo ano bissexto, sem depender de horário de verão. Valores são inteiros em centavos, com somas por BigInt e recusa de overflow.

A lista114, sua seleção, exportação oficial, filtros e abertura por documento permanecem. O relatório117 usa as parcelas selecionadas ainda presentes no filtro; sem seleção, usa as filtradas. O escopo é explícito em tela e nos arquivos. Indicadores114 acima continuam referindo-se ao período completo, conforme sua descrição; indicadores117 correspondem ao escopo filtrado/selecionado. Paginação15fornecedores e20parcelas evita renderizar10mil linhas simultaneamente. Ordenação por nome, saldo, saldo vencido e próximo vencimento; indisponível fica no fim na ordenação por data.

## Dados e autorização

Nenhum endpoint novo, POST, mock em produção, cache persistente ou alteração de backend. Reutiliza `GET /local/v1/finance/payables/report.csv?from=...&to=...&as_of=...`, cliente e validador114, sem nova requisição ao alternar a visão. O mesmo limite de366dias inclusivos,10mil parcelas,4MiB e contrato CSVv2 permanece. Resumo inválido/erro de transporte não vira zero. A nova agregação também verifica identidades, datas, quantidades, estados, duplicidades e totais seguros.

O contexto, token e `view_accounting` são verificados pelo contêiner existente. A API continua responsável pela autorização real e pelo isolamento. Transferência às planilhas mantém a política já existente de perfil proprietário e fonte financeira autorizada. Não aumenta acesso do operador. Dados carregados permanecem em memória; exportação requer ação do usuário. Não altera fila durável116 nem remove comandos incertos no logout.

Não representa toda a empresa nem todo o financeiro: só obrigações conhecidas do aparelho, empresa e loja no período. Referência classifica vencimentos; saldos/pagamentos são os atuais na consulta, não histórico naquela data. Não calcula DRE, lucro, posição bancária, imposto ou confirma pagamento eletrônico. Não soma documentos e parcelas novamente. Principal não aprovado é informativo e nunca saldo a pagar ativo.

## Arquivos e impressão

CSV: resumo, escopo, período, referência, leitura, moeda, unidade em centavos, fornecedores e parcelas. Reaproveita `csv97` para aspas, separadores e neutralização de fórmulas.

XLSX: três abas (Resumo, Fornecedores, Parcelas), números inteiros em centavos, rótulos como strings, filtro e congelamento nas tabelas, configuração A4paisagem. Reutiliza ExcelJS já instalado, com import dinâmico somente na exportação. Nenhuma dependência adicionada. Cabeçalho indica unidade; metadados completos ficam em Resumo. Valores que começam com `=` são texto, nunca fórmulas.

Impressão/PDF: iframe dedicado com nós de texto, títulos, metadados, tabelas, cabeçalhos repetidos e CSSA4paisagem. A ação abre a impressão do navegador; PDF é escolhido pelo usuário nessa janela. Não é serviço de geração de PDF nem integração com impressora. O iframe é removido após impressão, mudança de dados, saída ou timeout. Não há HTML vindo do fornecedor. Arquivos podem conter dados financeiros: o usuário decide onde guardá-los.

Planilha Titan: cópia do resumo por fornecedor, até500fornecedores, mantendo centavos e período/referência. Reutiliza transferência76 em memória, consumo único, contexto e expiração existentes. Não mantém vínculo automático nem aplica alterações ao financeiro. Acima do limite exige filtro, nunca truncamento.

## Limites conhecidos e backend

Não existem botões simulando criação/aprovação de documentos, recebimento de mercadoria, baixa bancária ou envio ao contador. Essas operações dependem de escopos próprios. Nenhum novo contrato é necessário para117. Continuam necessários para o produto futuro: consulta paginada e agregada por empresa/loja autorizada, visão histórica de saldos caso desejada, exportações assíncronas acima do limite atual e portal externo do contador com compartilhamento revogável/auditoria. Estes são requisitos propostos, não endpoints implementados.

Mantém as áreas ainda não implementadas como estão; nenhum selo é promovido a operacional. A implementação117 não conclui o ERP nem o desktop. Frontend, backend e empacotamento são frentes separadas.

## Verificação

Executar `node --test tests/suppliers117.test.mjs tests/agenda114.test.mjs tests/finance116.test.mjs tests/payables113.test.mjs`, typecheck e build. Testar erro, vazio, data alterada, pesquisa, seleção, limpeza, paginação, fornecedor com nome idêntico a outro, textos com aspas/fórmula/HTML, limite500, soma de valores e faixas.

Na aplicação real: usar conta autorizada em ambiente de teste com contrato CSVv2 existente; abrir Financeiro → Agenda de vencimentos → Visão por fornecedor. Consultar um período, conferir soma das parcelas, exportar, abrir XLSX e enviar resumo às planilhas. Abrir documento pelo detalhe deve chegar ao assistente116 sem qualquer escrita automática. Verificar proprietário, leitor autorizado e perfil sem acesso, cinco idiomas, temas claro/escuro, tablet/celular, foco e teclado. Nada exige criar venda ou pagamento comercial para demonstrar a tela.

Prévia117 é exclusivamente fictícia, usa o componente real e o cliente114 com transporte em memória. Inclui96parcelas,24fornecedores, erros/vazio/dados inválidos, cinco idiomas e dois temas. Nunca é importada pelo produto. A entrega inclui evidências reais dos testes; impressão nativa/PDF, serviço do usuário, Windows e desempenho em hardware antigo não são certificados aqui. Traduções têm paridade de chaves, com revisão humana estrangeira pendente.
