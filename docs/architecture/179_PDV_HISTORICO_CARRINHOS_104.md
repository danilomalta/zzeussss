# Frontend104 — Histórico do PDV e carrinhos suspensos

## Base e escopo
Requer frontend103 já aplicado sobre102. Somente frontend. Dois componentes existentes modificados: SalesHistory.tsx e Till94.tsx. Não altera LocalPointOfSale.tsx da B01, localClient, Go, migrações, banco real, autenticação, contrato/licença, pagamento, estoque ou preço oficial. Sem novas dependências e sem commit/merge/push.

## Histórico de vendas na operação
Substitui os textos fixos em português por cinco idiomas e utiliza Intl. Reutiliza a consulta existente de vendas com paginação do servidor de20 linhas. Filtros locais por referência da venda/turno, período e situação; CSV da página filtrada e consulta direta por referência. A tela avisa expressamente que filtros e exportação cobrem somente a página carregada, sem inventar um total de todas as vendas. Uma página cheia pode ter uma próxima vazia; o contrato antigo não informa contagem global.

Referências completas de venda e caixa ficam em detalhes, sem exibir UUID de operador como nome. IDs duplicados, datas inválidas e totais fora do intervalo seguro são rejeitados. Erro não vira consulta vazia bem-sucedida. Respostas de outra sessão, página ou contexto antigo são ignoradas e não atualizam o comprovante. O cliente legado continua com seu limite de10s e seu contrato de venda em dinheiro; o componente não afrouxa a validação para fingir que pagamentos avançados estão ligados.

O callback onRead original foi preservado, assim como o comportamento de abrir o registro na tela existente. O histórico continua oculto enquanto o PDV estiver no modo de operação em tela cheia, pois a condição no componente pai não foi alterada. Nenhum histórico em segundo plano novo, cache persistente novo ou chamada de pagamento.

## Carrinhos suspensos
A aba existente Vendas suspensas passa a ter busca por produto/referência, filtro de situação, paginação de5 e prévia dos itens. Mostra preço ao suspender versus preço carregado agora, total anterior versus atual, e indica as linhas alteradas. Cálculo em centavos inteiros com o mesmo arredondamento por linha do PDV.

Se faltar um produto no catálogo carregado ou a unidade tiver mudado, o total atual fica desconhecido e a retomada bloqueada, sem usar preço antigo silenciosamente. A lista de produtos da tela pode ser parcial: isso significa produto não carregado, não exclusão ou falta física de estoque. Atualizar/carregar o catálogo e tentar novamente.

Se o preço mudou, é necessário marcar que conferiu as alterações. A conferência é ligada ao fingerprint do carrinho/preços/unidades/quantidades/estado; mudança posterior invalida a confirmação. Venda atual, bloqueio original da operação e carrinho concluído também impedem retomada. Após confirmação humana, usa resume94 e mutate/CAS já existentes. Não cria um segundo mecanismo de persistência, não reserva estoque e não registra venda. O backend continua responsável por conferir preço, saldo e permissão na finalização.

## Integração, dados e tradução
HistoryContext104 adapta apenas localClient.history e localClient.receipt existentes; não inventa URL nova. A fonte é substituída explicitamente por dados fictícios apenas na prévia104. A aplicação continua sujeita à autorização do servidor.

pt-BR/en/es/fr/zh-CN:26 chaves novas completas. Fontes de14–16px, botões44px, controles rotulados, foco visível e tabelas com rolagem interna no celular. Cores do tema existente; sem azul novo. Traduções estrangeiras precisam de revisão nativa antes de produção.

## Como testar no sistema
1. PDV em modo normal: ver Histórico de vendas. Filtrar a página, avançar e voltar, exportar CSV e consultar registro por linha ou referência. A comparação por período não é consulta global. Verificar erro de fonte e troca de sessão durante consulta.
2. Suspender um carrinho com o mecanismo existente. Abrir Vendas suspensas, procurar pelo produto e abrir prévia. Alterar preço no catálogo de teste e atualizar a lista: retomar deve exigir conferência.
3. Sem produto/unidade no catálogo carregado, retomar deve ficar bloqueado. Concluir venda atual antes de retomar outra. Após retomar, conferir o carrinho e seguir o checkout autoritativo existente.
4. Trocar cinco idiomas, claro/escuro e larguras1440/768/390. Usar Tab/Enter/Espaço. No modo em tela cheia, não deve aparecer histórico no rodapé da operação.

## Prévia
ver_previa.py abre PREVIA.html sem login. São exemplos fictícios sem API, cobrança, estoque real ou sessão de produção. Os controles de teste simulam erro, registro inválido, resposta atrasada, alteração de preço, catálogo incompleto e venda atual. Retomar usa o próprio resume94 sobre dados demonstrativos em memória; não é confirmação de venda real.

## Evidência e limites
425 testes do frontend aprovados; build TypeScript/Vite aprovado. Firefox testou paginação e filtros, CSV, consulta direta, descarte de registro inválido/atrasado, reprecificação, invalidação da conferência, bloqueios e retomada pelo mecanismo94, cinco idiomas, três larguras e dois temas. Somente adaptadores de demonstração. Evidência final em VALIDACAO.txt.

Não executados servidor Go real, caixa comercial, pagamento/PSP/SEFAZ, impressora física ou smartphone. Não afirmar que a Parte1 inteira está operacional. Histórico global com filtros no servidor exigiria novo contrato paginado; esta entrega não inventa esse endpoint. Confirmação real de pagamentos mistos continua no chat de backend.
