# Frontend111 — Conferência e retomada de carrinhos suspensos

## Base e escopo

Requer110, confirmada no computador do usuário com201 testes aprovados e build20,65s. Redesenha a área existente de carrinhos suspensos no PDV. Suspended104 mantém sua entrada e contrato de propriedades/callback, encaminhando ao novo Suspended111. Till94, armazenamento, rotas, cotação B01 e backend permanecem inalterados. Nenhum endpoint, dependência, schema ou timer novo. O carrinho não vira venda, pagamento, reserva de estoque ou documento fiscal ao ser consultado, impresso ou retomado.

## Organização

Indicadores acompanham os filtros: carrinhos listados, em andamento e com preços alterados/itens indisponíveis. Busca por nome, SKU e referência, literal e sem distinção de acentos. Estados em andamento/todos/suspenso/retomado/concluído e filtro só com alterações ou ausências. Paginação de6, até30 registros conforme o limite anterior. Código interno do carrinho e usuário não são usados como nomes na interface. Número exibido é posição de consulta, não um identificador permanente.

Cartões apresentam data, estado, número de linhas, amostra dos produtos, total com catálogo carregado e contagem das alterações/ausências. Abrir conferência move foco para o título. Área permite abrir detalhes mesmo bloqueada para mutação; retomar continua bloqueado. Filtros/exportação são somente leitura. Não inventa prazo de expiração, limpeza automática ou exclusão de carrinhos.

## Conferência

Tabela por produto mostra quantidade/unidade, preço ao suspender, preço do catálogo carregado, subtotal antigo e subtotal atual. Totais antigo/atual/diferença ficam separados. Valores vêm do catálogo em memória, não de consulta de estoque ou preço em tempo real. Produto ausente ou unidade alterada deixa preço/subtotal/total atual desconhecido, sem reutilizar preço antigo e sem substituir falta por zero. Reposição e preço final continuam sob a validação existente da venda.

Usa preview104 e os mesmos arredondamentos de centavos do PDV. Catálogo indexado uma vez por inspeção, usando apenas o subconjunto de produtos referenciados em cada cálculo; busca/filtro não reprocessa o catálogo quando só o texto muda. Inspeção memoizada por referências de carrinhos/produtos. Até200 linhas por carrinho e20000 produtos, conforme os limites anteriores. Soma e multiplicação com BigInt nas funções existentes e rejeição de overflow. IDs duplicados, localização inválida e origem malformada não viram uma lista vazia.

Confirmação explícita de itens/quantidades/preços precede a retomada, inclusive sem alteração de preço. Snapshot inclui carrinho completo, estado, origem/localização das linhas e preços/unidades relevantes. Mudança após confirmar invalida o reconhecimento. A verificação é refeita no clique, inclusive se um objeto tiver sido alterado em memória. Esta confirmação é de interface e não cria aprovação comercial ou autorização no servidor.

Produto ausente, carrinho concluído, venda atual ocupada ou workspace bloqueado impedem retomada. Carrinho retomado pode continuar aparecendo conforme o comportamento anterior; no aplicativo real a venda carregada bloqueia retomar outro. O mecanismo de suspensão original e limite de30 continuam iguais. Não libera estoque, não baixa quantidade e não define a situação fiscal.

Clique final mantém o diálogo de confirmação existente. Trava síncrona evita envio duplicado pelo mesmo clique. Chama o mesmo onResume(id), que no pai usa resume94, mutate/CAS e o preenchimento da operação. Sucesso do callback fecha a conferência e informa que o carrinho foi enviado à operação; não declara venda concluída. Falha mantém o registro e exige nova conferência. Cancelar o diálogo não chama o callback. Não há repetição automática nem nova chave de venda gerada nesta área.

## Exportação e impressão

CSV baixa somente o filtro atual: data, estado, número de linhas, total antigo/atual, quantidade de itens indisponíveis e preços alterados. Não contém usuários, identificadores internos ou dados de pagamento. Escapador csv97 existente protege valores de fórmulas, aspas e delimitadores. Download direto por Blob; o navegador pode perguntar o destino conforme suas preferências, mas o aplicativo não procura pastas.

Impressão contém estado, totais e linhas com quantidade/subtotais, com aviso de que não é venda, pagamento ou reserva. Reutiliza print95 com texto escapado; não emite nota ou recibo de pagamento. É permitido imprimir uma conferência incompleta com traços e ressalvas, sem liberar retomada. Somente o próprio fluxo de venda pode concluir a operação.

## Segurança e integração

Sem endpoints novos. Reutiliza rows/products/locked/cartBusy/onResume passados por Till94. Contexto e permissões vêm do fluxo anterior. O componente é recriado na troca de scope/cash pela chave já existente no pai. Não cria cache permanente nem persiste filtros ou documentos. Não altera token, dados pessoais, produto, política de preço ou integração de pagamento. Validação/isolamento/autorização do backend continuam necessários.

25 chaves novas completas nos cinco idiomas pt-BR/en/es/fr/zh-CN. Revisão nativa das traduções estrangeiras continua pendente. Números/moeda/data com Intl, BRL em todos os idiomas. Tokens neutros existentes, sem novo azul/dourado, texto14px, campos16px e alvos48px; foco visível, estados textuais e tabela com rolagem interna para preservar colunas no celular. Não adiciona animações.

## Validação e teste

20 testes novos,560 de frontend no conjunto: arredondamentos, peso, ausência/unidade alterada, duplicidade, dados inválidos, busca literal/SKU, filtros, reconhecimento obrigatório, bloqueios, conclusão, preço/quantidade/localização/ator/referência alterados, overflow, imutabilidade, limites e idiomas. Typecheck app, build e git diff --check; Firefox nos dois temas ×5 idiomas ×3 larguras. Testes de navegador: ausência de escrita antes da confirmação, preço alterado após revisão, catálogo ausente, operação ocupada, readonly com consulta, cancelar diálogo, falha preservando carrinho, duplo clique, registro concluído, paginação, CSV e impressão identificada como conferência.

Instalador confere hashes, preserva arquivos conflitantes, cria backup privado e roda221 testes selecionados, typecheck/build, sem commit/merge/push/banco comercial. Prévia explícita em memória, com fontes fictícias e callback de teste. Não testado: aceite no computador do usuário, fluxo real contra backend B01, concorrência entre dois aparelhos, impressão física ou reserva de estoque. Não declara a Parte1 inteira concluída.

Teste /local/pos → Carrinhos suspensos: suspender uma venda pelo fluxo existente, procurar produto/SKU, abrir, comparar valores e confirmar antes de retomar. Repetir após atualizar catálogo. Produto removido/unidade diferente deve bloquear. Carrinho concluído não pode ser retomado; venda atual com itens também bloqueia. Na prévia, testar preço alterado, falha, consulta somente leitura e clique duplo sem venda real. Conferir claro/escuro, idiomas e celular/tablet.
