# Cadastro — frontend 112

## Resultado

A primeira etapa de `/register` passa a ter três seções: Sobre a empresa, Atividade e setores e Recursos. Porte e armazenamento usam cartões com opções e explicações visíveis; setores usam chips com checkbox nativo. O resumo mostra escolhas, módulos, preço mensal e economia. No desktop permanece visível durante a rolagem, com rolagem interna de detalhes se necessário; abaixo de 1000px passa para depois do formulário.

As quatro etapas existentes, recomendações por atividade, dependência cozinha/cardápio e seleção de módulos foram preservadas. O contador aceita de 1 a 10000 usuários, permite digitação e botões de mais/menos e atualiza o resumo. Usuários, porte e armazenamento não alteram preços: o cálculo existente soma somente módulos. Nenhuma nova regra comercial foi criada.

## Limites e integração

Esta entrega altera frontend de cadastro. Não cria conta, contrato, licença, cobrança ou autorização. A conclusão continua salvando somente a proposta de perfil existente em sessionStorage (`proposal87`). Não envia nome, e-mail ou empresa ao servidor. Não muda backend, API, banco, PDV, B01 ou permissões. Nenhuma dependência nova.

Não foram inventados endpoints. Para transformar a revisão em cadastro real, o backend deve fornecer o contrato autorizado de criação/validação da empresa e conta, identidade, proposta e contratação. A interface deverá consumir esse contrato quando existir; a estimativa exibida não substitui validação de preço no servidor.

## Acessibilidade e temas

Rádios e checkboxes nativos com labels associados, navegação por teclado, foco visível, controles de 48px ou mais e textos auxiliares de pelo menos 13px. Paleta neutra existente, sem azul ou dourado. Respeita redução de movimento. As 21 novas chaves têm versões pt-BR, en, es, fr e zh-CN; nenhuma chave nova ausente. Revisão linguística humana não realizada.

O ajuste de overflow necessário ao resumo fixo é limitado à classe `registration112-page`, adicionada ao montar o cadastro e removida ao desmontar. Não altera o layout das telas internas.

## Como conferir

1. Abrir `/register` em claro e escuro. Conferir três cards, rótulo Atividade separado do select e resumo lateral.
2. Selecionar porte e armazenamento, digitar usuários e usar +/−. Testar vazio, 0, fração e limite 10000; inválidos não avançam.
3. Trocar atividade para Indústria: módulo recomendado e estimativa seguem o cálculo existente. Alterar setores e recursos e conferir o resumo. Cozinha marca cardápio; desmarcar cardápio desmarca cozinha.
4. Avançar, mostrar módulos opcionais, adicionar/remover módulos. Sem módulos não avança. Voltar preserva escolhas.
5. Preencher os dados, revisar e confirmar. A mensagem final informa que nenhuma conta foi criada ou cobrada.
6. Testar os cinco idiomas, Tab, setas nos rádios, Espaço nos checkboxes, zoom e larguras desktop/tablet/celular. Em celular o resumo fica depois dos cards, sem rolagem horizontal.

## Validação

15 testes novos; suíte total de 575 testes de frontend aprovada. Typecheck do aplicativo e build aprovados. Firefox verificou as quatro etapas, validações, regras de cálculo, 5 idiomas × 3 larguras × 2 temas, controles de pelo menos 48px, navegação por teclado e contraste de texto secundário nos tokens padrão. Prévia é demonstração de interface; não testa cadastro real, contratação ou pagamento.
