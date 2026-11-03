# Frontend107 — Contagem de cédulas e moedas

## Base e escopo
Requer106. A instalação106 no PC foi confirmada com131 testes e build21s. Esta entrega troca somente a interface de contagem da etapa inicial de CashClosing92. Reutiliza as denominações brasileiras, cálculo em centavos, persistência IndexedDB por contexto, revisão CAS e sequência existentes: dinheiro → pagamentos/comprovantes → revisão final. Não muda Go, SQL, APIs, licença, sessão, adquirente, fiscal, saldo oficial ou contrato. Não fecha caixa na prévia. Nenhuma nova dependência de produto e nenhum endpoint inventado. O backend continua responsável pelo encerramento oficial e suas permissões.

## Interface
Cartões de cédulas e moedas mostram valor unitário, quantidade e subtotal. Botões +/− com48px, campo numérico, seleção do valor ao focar, Enter avança para a próxima denominação, setas ajustam uma unidade. A última tecla Enter leva ao botão Revisar contagem. Mantém as13 denominações do modelo existente, inclusive um centavo; não altera regras monetárias.

Resumo fixo dentro da área mostra cédulas, moedas, quantidade de unidades e total contado. Não recebe nem revela o saldo esperado. Quantidade em branco representa zero para o cálculo, mas não equivale a caixa fisicamente conferido. Contagem zero exige confirmação explícita de que o caixa realmente não contém dinheiro. Não depende só de cor; estados são textuais e botões têm nomes acessíveis.

## Revisão e gravação
Preencher → revisar detalhamento por denominação → confirmar conferência física → salvar pela função existente → pagamentos. O detalhamento da revisão contém somente denominações com quantidade positiva; total zero tem aviso próprio. Alterações externas no vetor de quantidades invalidam a revisão mesmo se o total continuar igual. A confirmação fica vinculada ao snapshot exato das quantidades e é verificada antes de chamar o salvamento existente.

Falha de gravação mantém os valores e a revisão na tela; não avança. Trava síncrona impede duplo clique no mesmo envio. Não substitui idempotência no backend nem promete deduplicar novas tentativas após uma falha de comunicação. O retorno booleano de save existente decide se avança. O indicador de confirmação deriva do rascunho realmente salvo, não de timeout ou animação.

Limpar pede confirmação, preserva os valores se cancelada e exige nova conferência quando efetivado. Não há exclusão do banco ou limpeza de outras áreas. Editar/limpar também desmarca a aprovação anterior do responsável, pois uma contagem alterada precisa de nova revisão. Motivo e histórico existentes são preservados.

Troca de contexto recria o componente pela chave scope/cash e limpa somente estado transitório de revisão. Scope é derivado pelo aplicativo existente. O componente não recebe token, nome ou CPF, não lê dados de outros caixas, não chama API e não cria cache novo. As guardas de alterações não salvas e persistência CAS do pai continuam ativas. O contexto não é uma autorização: execução comercial exige validação do backend.

## Exatidão
Reutiliza cashCount/cashDenominations. Só aceita quantidades inteiras ASCII, até9 dígitos conforme limite anterior do modelo; zeros à esquerda digitados são normalizados. Botões não permitem quantidade negativa nem exceder o máximo. Valida tamanho e tipos do vetor antes de resumir; dados inválidos não viram caixa vazio. Valores em centavos inteiros, soma de cédulas/moedas consistente com o total do modelo anterior. Nenhuma comparação com recebimentos bancários é feita nesta etapa.

## Integração preservada
CashClosing92 passa os mesmos counts, countConfirmed, locked, change e save. O novo onChange desmarca countConfirmed e approved. O novo onConfirm salva countConfirmed:true com revisão CAS e só muda para etapa1 se save retornar true. Comprovantes106, conciliação97, origem de vendas, revisão final, botão de fechar e callback onClose foram preservados. Não muda o armazenamento nem migra rascunhos anteriores.

## Idiomas e aparência
29 chaves novas em pt-BR/en/es/fr/zh-CN, completas. Traduções estrangeiras precisam de revisão nativa antes da produção. Números/moeda com Intl; BRL em todos os idiomas. Tokens neutros existentes, sem azul/dourado novo. Texto14px, campos16px e botões48px inclusive dentro do CSS do fechamento. Cartões responsivos, resumo flexível e tabela com rolagem interna quando necessário. Não adiciona animação ou execução periódica. A prévia possui controles identificados como demonstração, separados da interface traduzida do produto.

## Testes e limites
480 testes do frontend, incluindo10 novos; typecheck do tsconfig.app.json, build Vite e git diff --check. Firefox cobrindo: confirmação de zero, +/−, zeros à esquerda, total, Enter/setas, alteração externa invalidando a revisão, erro com valores preservados, clique duplo, troca de caixa de teste, bloqueio de edição, limpar/cancelar, cinco idiomas ×três larguras ×dois temas, sem rolagem horizontal global, erro de runtime ou chamada de API na prévia.

Não executados: aceite no PC do proprietário, contagem física, fechamento contra backend real, conflito IndexedDB entre duas instâncias reais, banco comercial, transportes entre aparelhos ou periféricos. O teste externo da prévia simula mudança de estado; CAS continua na implementação anterior. Não afirma conclusão da Parte1 inteira. Relatórios oficiais e pagamentos continuam sujeitos às integrações existentes.

## Como testar
1. Instalar depois da106. /local/pos → fechamento → etapa dinheiro. Digitar2 cédulas deR$200 e3 moedas deR$1 deve resultar emR$403,00. O esperado não aparece.
2. Testar +/−, setas, Enter e valor inválido. Revisar, voltar e corrigir. A revisão deve exigir confirmação de novo.
3. Limpar: cancelar preserva valores; confirmar apaga quantidades do rascunho e retira sua aprovação anterior. Revisar zero deve pedir confirmação específica.
4. Confirmar conta apenas salva a contagem e segue para pagamentos após sucesso. Não encerra caixa. As próximas etapas continuam mostrando débitos/créditos/Pix e divergências pela integração existente.
5. Usar a prévia para simular erro de gravação, clique repetido, alteração externa e troca de caixa, sem dados reais. No aplicativo instalado testar claro/escuro, idiomas e celular/tablet.
