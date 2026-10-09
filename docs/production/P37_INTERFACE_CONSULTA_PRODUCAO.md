# P37 — Interface de consulta de producao

Base no PC: 2a7c600, branch feat/producao-receitas. Sem migracao, schema44.
Rota /local/production ligada ao menu e a Minha area. Permissao manage_production
com modulo production conforme capacidades; a autorizacao final segue na API.

Versoes paginadas, ingredientes com quantidade e unidade preservadas, rendimento,
consulta de capacidade por ID de versao e local, ordens por estado e detalhes
com receita imutavel e responsavel. Capacidade usa saldo apos reservas, mostra
limitantes e razao exata da conversao. Alternativas sao independentes; nao somar.
IDs sao mostrados para localizar referencias sem pressupor permissao ao catalogo.

Cliente novo usa somente GET, Bearer da sessao em memoria, sem cookies ou cache.
Nao escreve banco, reserva, estoque, pedidos nem configuracao de autenticacao.
Quantidades numericas inseguras sao recusadas; strings cumulativas formatadas
por digitos sem arredondamento. Requisicoes mais antigas e respostas de sessao
anterior nao substituem a consulta atual. 401 encerra a sessao local existente.
Erros e carregamento sao explicitos; pagina anterior e proxima disponiveis.

Arquivos compartilhados alterados: router/index.tsx (nova rota lazy),
accessModel.mjs (rota associada a production), LocalWorkspace.tsx (destino do
menu), LocalHome.tsx (cartao), package.json (adicao do teste, sem dependencia).
Revisar estes contratos de navegacao na integracao com a main; sem integracao
silenciosa. Login, credenciais e backend nao alterados.

Validacao local: suite frontend test:local e build TypeScript/Vite. Testes cobrem
precisao maxima, conversao explicita, paginas, receita preservada, permissao,
GET sem efeitos, IDs codificados, resposta invalida e 401. Sem servidor ou DB.
Nao se declara aceite visual ou uso manual no navegador executado.

Ainda pendentes nesta interface: cadastro/alteracao de receitas e ordens,
execucao de reservas/consumo, resultados, etapas, perdas, lotes e qualidade.
Esta entrega e consulta operacional; nao encerra a interface completa.
