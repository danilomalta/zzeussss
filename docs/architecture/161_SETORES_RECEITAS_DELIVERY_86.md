# Entrega86 — separar operação, entregas, alimentação e receitas

## Correção de organização

As capturas mostravam dois problemas: Produção abria a central técnica de “Formulação e qualidade”, com cadastro pouco evidente; Alimentação e Delivery era uma única tela que pressupunha restaurante. Agora a organização é explícita e configurável.

| Acesso | Conteúdo |
| --- | --- |
| `/local/sectors` — Setores e operação | Atividade e setores; ativação independente de cardápio, fila de preparo/cozinha, mesas e receitas/produção. |
| `/local/delivery` — Entregas | Pedido, destinatário, origem, separação, entregador, saída, prova de entrega/ocorrência e histórico. Não contém cozinha ou mesas. |
| `/local/recipes` — Receitas e fichas técnicas | Ingredientes/saldo de planejamento, receitas versionadas, capacidade, reservas e consumo/rendimento real declarado. |
| `/local/food` — Alimentação e cardápio | Cadastro/edição de cardápio, categorias, descrição, SKU, preço, disponibilidade, receita vinculada quando autorizado, grupos de escolhas/adicionais e atendimento. Cozinha e mesas só aparecem se ativadas. |
| `/local/food/previous` | Consulta aos atendimentos anteriores da85, preservados. O novo acesso não apaga o banco anterior. |

No menu, Produção passa para Receitas e fichas técnicas. Entregas ganha card independente; Alimentação aparece após configuração explícita. As opções não concedem módulos ou permissões. As demais áreas85 permanecem em seus acessos anteriores. A área técnica84 continua disponível para os usos anteriores, sem substituição de dados.

## Configuração sem pressupor restaurante

O primeiro acesso solicita organização do arquivo local. Atividades disponíveis: mercado/varejo, farmácia, lanchonete/hot dog, restaurante, indústria/produção e outra. A atividade não ativa funções automaticamente. Setores são escolhidos entre depósito, caixa, padaria, açougue, separação, balcão, cozinha, produção, expedição e outro. É possível combinar mercado + padaria + açougue sem duplicar a empresa.

Cardápio, cozinha, mesas e produção têm chaves independentes. Cozinha exige cardápio; mesas não são obrigatórias para hot dog. Uma farmácia pode configurar apenas balcão/separação/expedição e usar Entregas sem conteúdo de restaurante. A configuração é local por contexto (empresa/loja/usuário/aparelho); propagação por empresa dependerá do backend. Perfil sem autorização não configura a empresa.

## Receita utilizável sem exemplos

Os botões Cadastrar receita e Cadastrar ingrediente ficam no início da página. O estado vazio orienta o cadastro, não exige carregar demonstração. É possível cadastrar ingredientes também pelo editor de receita e voltar ao formulário preservado.

Receita registra nome, setor, versão, rendimento e unidade, perda prevista, tempo, etapas e ingredientes com quantidade/unidade. Cadastro de ingrediente informa unidade, custo por unidade, localização, validade e saldo de planejamento. Unidades de peso e volume convertem com cálculo inteiro (kg↔g, L↔mL); unidades incompatíveis são recusadas.

Exemplo validado:100 pães usam5kg de farinha e100g de fermento. Com20kg de farinha e300g de fermento, capacidade300 pães. Reservar100 reduz a capacidade restante para200. Duas receitas que usam o mesmo ingrediente não somam suas capacidades; a reserva de uma reduz o saldo disponível da outra.

Fluxo local: reservar → iniciar → conferir consumo/rendimento → concluir. Reservar não baixa o saldo físico declarado; retira do disponível. Concluir desconta o consumo declarado uma única vez e encerra a reserva. Outras reservas continuam protegidas. Cancelar reserva é permitido antes do início; após iniciar, conferir o consumo e concluir o procedimento. Validade vencida deixa o ingrediente indisponível ao cálculo. Perda prevista é aplicada uma vez; diferença entre rendimento planejado e real fica visível.

Esses saldos são de planejamento local. Não são leitura do estoque oficial, reserva distribuída ou baixa no PDV. Produto concluído aparece no planejamento; disponibilidade no caixa e no estoque real ainda dependem da integração. A tela informa isso em uma nota, sem fingir comunicação ativa.

## Entregas independentes

Cadastro manual com referência do pedido, destinatário/contato/endereço, conteúdo, setor, janela negociada, taxa declarada e entregador. Estados: aguardando separação → em separação → pronto para saída → em entrega → entregue; ocorrência e cancelamento com motivo. Saída exige entregador ativo; conclusão exige referência de prova. Ocorrências têm histórico. Entregador em entrega não é desativado silenciosamente.

Tempo decorrido é derivado dos registros locais, não telemetria. GPS, posição ao vivo, comunicação remota e ETA automático não foram implementados nesta entrega. O mapa anterior continua acessível. Nenhum pagamento, nota fiscal, mensagem ou baixa oficial acontece por mudar os estados desta tela.

## Cardápio com cadastro e edição

Cadastro/edição de nome, categoria, descrição, SKU, preço e disponibilidade. Receita vinculada só é oferecida a quem possui acesso à produção. Grupos de escolhas têm nome, mínimo/máximo, opções e preços adicionais. Escolhas obrigatórias são validadas; uma opção não soma duas vezes. Pode-se criar vários grupos, remover opções e retirar itens de novos pedidos sem apagar histórico.

Atendimento permite escolher produto, adicionais e quantidade; o total usa os preços do momento e conserva o item no pedido. Fila da cozinha só aparece com o recurso ativo. Mesas só são coletadas/exibidas quando ativadas. Preparar entrega cria um registro na central genérica, sem duplicar o pedido ao repetir. Encerrar atendimento é uma confirmação local, não liquidação bancária, venda fiscal ou baixa de estoque.

## Preservação de dados

Botão Trazer cadastros anteriores oferece importação explícita por contexto: receitas/formulações da84; cardápio e entregas da85. Registros anteriores permanecem intactos; referências de origem evitam duplicação no reenvio. Exemplos antigos continuam identificados como demonstração. Importar não ativa cozinha, mesas ou produção automaticamente.

Os dados novos ficam em IndexedDB separado, com contexto derivado da sessão existente, autosave após commit e aviso de alterações não salvas. CSV baixa diretamente; planilhas usam a passagem temporária existente e permissão de responsável. Não foi criado nenhum endpoint fictício.

## Backend que ainda falta

- Configuração oficial de atividade/setores e recursos por empresa/loja, com autorização e distribuição aos aparelhos.
- Ingredientes e receitas ligados ao catálogo/estoque: unidade, lote/validade, custo, versões e origem verificável.
- Ordens de produção, reservas e consumo atômicos, idempotência, perdas, rendimento e entrada de produto pronto por setor.
- Eventos para atualizar disponibilidade do caixa sem descontar ingredientes novamente na venda.
- Entrega ligada ao pedido real, funcionário/aparelho autorizado, prova de entrega, rastreamento consentido, agenda e conciliação financeira.
- Cardápio ligado a preços/catálogo, estoque, cozinha/impressão, POS/pagamentos/fiscal e parceiros configurados.
- Permissões por setor e função no servidor. O frontend não é barreira de segurança nem reserva distribuída.

## Verificação

50 testes Node selecionados (17 novos),11 testes do instalador, TypeScript e build. Testes adicionais verificam importação idempotente sem modificar fontes, rótulo de exemplos e isolamento por contexto no IndexedDB.

DOM virtual: configuração de farmácia sem cozinha/cardápio; lanchonete com recursos escolhidos e sem mesas; cadastro real de formulário, sem carregar exemplos; cardápio com adicional e pedido de2×(R$15+R$3)=R$36; dois ingredientes e receita com capacidade300, reserva/conclusão de lote reduzindo capacidade200; entregador/prova de entrega; autosave,5 idiomas e restrições de empregado.

Foram inspecionadas as capturas fornecidas. Não foi declarada revisão visual do novo build em navegador real, nem teste de GPS, PDV comercial ou outro aparelho. Paleta/tema e fundo pessoal existente são preservados. Sem dependência nova em produção; módulos carregados sob demanda. O aviso anterior de tamanho da planilha continua.

pt-BR/en/es/fr/zh-CN têm as mesmas chaves e textos novos presentes nos componentes. Traduções estrangeiras aguardam revisão nativa. Dados de negócio não são traduções da interface.
