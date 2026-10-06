# PDV visual integrado, inspirado no protótipo Tytan

Base de aplicação: 768adb1. O ZIP `Tytan_PDV_Demo_v1` era um frontend independente; sua extração em Downloads não modificou o projeto. Esta entrega adapta a organização visual para o React oficial, na rota existente `/local/pos`.

## Alterações

Entrada de produtos, tabela da venda e resumo permanecem em áreas próprias. O catálogo auxiliar pode ser recolhido. A tabela mostra descrição, quantidade editável, preço na unidade cadastrada e subtotal; última linha adicionada recebe destaque discreto. Remoção permite desfazer a última remoção, agrupando um registro posterior do mesmo produto/local sem duplicar a linha.

Busca aceita nome, SKU e código de barras da página carregada (50 produtos). Enter adiciona, setas selecionam resultados, Escape limpa a busca e Tab navega. Código numérico exige correspondência exata; códigos duplicados são recusados. Repetições agrupam produto e gôndola. Quantidade respeita a unidade e milésimos; dinheiro continua em centavos com arredondamento exato. Totais por unidade não misturam kg/liter/etc.

O botão **Ir para pagamento** abre um diálogo de dinheiro com total estimado, valor recebido e troco. O diálogo mantém foco; ao sair retorna à entrada de produtos. Enquanto está aberto, não registra leituras na venda. Fechamento durante operação em andamento é bloqueado; Escape pode fechar quando não há envio em andamento. A confirmação chama o mesmo `createPOSOperations`, com persistência dos identificadores, Web Locks e recuperação de resultado incerto já existentes. Preços e estoque são revalidados no backend. Dinheiro recebido e troco orientam a entrega física; o payload continua contendo apenas o valor da venda, sem preço enviado pelo cliente.

Temas e logo são os componentes compartilhados atuais. Navegação lateral, caixa/fechamento, consulta de turno, relatório, histórico e controles de acesso permanecem integrados. Não foi copiado o servidor Python nem o catálogo fictício do ZIP.

## Recursos não implementados aqui

Pix/cartão, pagamento dividido, desconto, CPF/documento fiscal, impressão, suspensão/retomada de rascunhos e periféricos não ganham confirmação fictícia na tela real. A interface informa sua indisponibilidade. O rascunho ainda em edição não é persistido ao recarregar; apenas operações já enviadas têm a recuperação existente. Ajuda informa esse limite. Não há laboratório de falhas simuladas no PDV operacional.

A indicação de API/turno informa o resultado da consulta; não promete monitoramento contínuo da internet nem sincronização entre aparelhos. A identificação de operador mantém o ID real, sem nome inventado.

## Validação

Testes de cliente/modelo: scanner exato/ambíguo, agrupamento por produto/local, edição, arredondamento ponderado, desfazer, limite de linhas e overflow; suítes existentes continuam cobrindo perda de resposta, pagamento e fechamento pela API. Executar `npm --prefix frontend-web run test:local` e `npm --prefix frontend-web run build`; `git diff --check`.

Backend e schema não foram modificados; testes Go não são repetidos exclusivamente por esta alteração visual. Nesta preparação passaram testes do frontend e build. Aceite visual, foco de diálogo e layout em Firefox nas telas do usuário ainda exigem demonstração manual; testes de modelo e build não comprovam esses comportamentos.

## Demonstração

Reinicie apenas o frontend que usa a pasta `frontend-web` ou o auxiliar de demonstração que você iniciou; não utilize `tytan-pdv-demo/iniciar.py` para acessar este PDV. Para instalação isolada com todos os módulos, use `python3 -B tools/demo_pdv.py --profile completo --api-port 8186 --web-port 3005`, escolhendo portas livres. Use o ID e senha dessa instalação; abra **PDV**, abra o caixa, escolha gôndola, registre por SKU, edite/remova/desfaça, confira troco e confirme dinheiro. Confirme o registro no histórico e teste fechamento. Verifique claro/escuro e janela estreita sem alterar o banco real.
