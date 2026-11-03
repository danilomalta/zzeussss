# Evidências locais do piloto em dinheiro — 25/10/2026

São logs reais preservados das execuções locais. Não são logs de GitHub Actions. Os identificadores são exclusivamente de fixtures descartáveis; não há banco, senha, token, chave ou dados comerciais neste diretório. Os logs não foram reescritos para simular um resultado. SHA256SUMS permite conferir os arquivos publicados; não constitui assinatura ou atestado externo da execução.

## Arquivos e resultado

- [native-final.log](native-final.log): TestPilotCashSaleToSupplierFinanceRestartAndRestore, PASS,2,125s.101requisições pelo transporte de produção Electron, quatro execuções isoladas do aplicativo e quatro processos API Go/SQLite reais. Não há automação de cliques Electron: a automação dirige as chamadas de API pelo transporte real.
- [native-race.log](native-race.log): mesmo circuito com detector de corridas, PASS,14,914s de pacote (teste13,89s).
- [screen-flow.log](screen-flow.log): primeira execução visual, FAIL após1800,53s, porque o navegador bloqueou na confirmação final. Essa falha não foi apagada nem contada como aprovação.
- [screen-resume.log](screen-resume.log): retomada dos mesmos negócios após snapshot consistente descartável; renovação apenas da licença sintética. PASS,1647,621s, incluindo espera humana. Venda, pedido, recebimento, obrigação, retirada, vínculo e encerramento foram produzidos pelas telas, sem POST de negócio do driver. Os cliques foram conduzidos por ferramentas de navegador; esta fixture exige marcadores humanos/assistidos DONE e FINISH e NÃO é um E2E de cliques autônomo em CI.
- [ci-scope.json](ci-scope.json): consulta à API GitHub Actions encontrou zero execuções nas duas branches. O workflow existente só dispara por push em main/develop ou PR destinado a elas. Não há link de CI verde para apresentar.

## Histórico CI: repositório e branches são escopos diferentes

A nova consulta confirmou32execuções no repositório. O [run32 da main](https://github.com/danilomalta/zzeussss/actions/runs/39305932267), commit024dd3f50d2280c9c1ae6e2ba3f829053bbe84c7, terminou com sucesso. Isso não é o teste novo do piloto nem torna os seus commits posteriores cobertos por aquele run.

“Zero execuções” refere-se EXCLUSIVAMENTE a feat/backend-b02a-compat e feat/backend-checkout-authority, não à main nem ao histórico inteiro do Actions. As quatro URLs REST exatas consultadas estão em [ci-scope.json](ci-scope.json). O código recente já está publicado nas branches de trabalho; o README/main não descreve automaticamente essas branches. Nenhum merge na main é feito nesta publicação documental.

## Escopo que foi exercitado

Uma empresa/loja/aparelho, catálogo e licença sintéticos; pagamento somente em dinheiro. Venda de2unidades a1599centavos: total3198, recebido4000, troco802. Fundo de caixa1000. Saldo físico5000→3000milésimos após venda. Política mínimo4000/alvo5000 sugere2000; aprovação humana distinta cria o pedido.

O fornecedor é EXTERNO ao Titan. O operador registra uma confirmação obtida por outro canal, usando supplier-decisions. O teste NÃO envia um pedido a outra empresa nem obtém aceite autenticado do usuário do fornecedor. A expressão “confirmação do fornecedor” deve ser lida como REGISTRO MANUAL de confirmação externa neste recorte.

A viagem identificada recebe conferência de carga e chegada; o recebimento é INTEGRAL: entregue2000, aceito2000, sem falta/avaria/divergência neste circuito. Retorna o físico a5000. Obrigação2050 vinculada ao recibo; criador não pode aprovar, outro gerente aprova. Retirada2050 é vinculada à parcela sem retirar dinheiro novamente. Saldo documental0. Fechamento1000+3198−2050=2148; diferença0.

Portanto, esses logs NÃO comprovam o cenário mais amplo de aceite autenticado entre empresas seguido de recebimento parcial com divergência. Esse cenário e seu CI precisam de aceite próprio; não atribuir a eles o PASS deste circuito integral/manual.

## Registro da persistência e restauração

No teste automatizado, a API é encerrada abruptamente depois da venda; outro processo abre o MESMO SQLite e recupera o recibo original sem novo POST de venda. O circuito então continua até o financeiro. Dezesseis recursos/recibos/rastreios são comparados por igualdade antes e após outro reinício.

Com o serviço ativo, backup.Create cria snapshot consistente criptografado; backup.Verify valida; uma chave incorreta é recusada. backup.Restore restaura em caminho NOVO, e uma segunda tentativa no mesmo destino é recusada. Uma quarta API abre o banco restaurado. A sessão anterior recebe401; novo login recupera os mesmos16recursos, IDs e valores. Reenvios explícitos dos mesmos IDs não duplicam efeitos.

Depois da restauração, o teste também exige por SQL:1venda;1parcela em dinheiro;2movimentos de caixa, com entradas positivas3198;1recebimento;1obrigação;1vínculo de pagamento; soma dos movimentos de estoque5000. O relatório de caixa exige1venda líquida3198, retirada2050, esperado2148/diferença0; a obrigação exige aplicado2050/saldo0. Esses são asserts do teste, não um dump de tabelas publicado. Os logs mostram o resultado das assertivas e as chamadas; não contêm imagens dos bancos nem as chaves temporárias.

Na fixture das telas, os MESMOS negócios são conferidos por SQL e sete recursos HTTP, seguidos de reinício, criação/verificação/restauração do backup e comparação desses recursos. O log registra401da sessão antiga e200dos recursos após novo login. Só depois da releitura visual de venda3198, caixa fechado, estoque5000 e obrigação2050/saldo0 foi enviado FINISH. Os bancos/chaves da fixture são temporários e não foram publicados.

## Fontes e reprodução

Backend: branch feat/backend-b02a-compat; fontes incorporados em f0a126b2cb6fd12433c9bed48568a7373cc59e56, guia atualizado em13d9aea7266d8e866ada0e826f17817ceb1fbd43. Frontend: implementação62df646d8130717631f0dc52694cca6f3d1c0627, registro final746bf38c181cf1c9aae63b9aa30c6d9baf1f2ffb. Logs são execuções locais durante o desenvolvimento anterior a esses commits; não anunciar que um runner remoto executou exatamente esses SHAs.

Código de aceitação: backend/internal/localapi/pilot_cash_acceptance_test.go; transporte: pilot_native_client_test.go e desktop/tests/pilot-api-client.cjs; fixture visual: pilot_screen_flow_test.go. Instruções completas em [PILOTO_DINHEIRO.md](../../PILOTO_DINHEIRO.md). A build React usada pelo ensaio tem source_hash4e7a50ff66a205baadc9a6693b37e061a0e89596583d8df8c9d950995f358dc7.

Para repetir o circuito HTTP sem instalar nada, no terminal, com Go compatível e dependências existentes:

```bash
cd "$HOME/Downloads/zzeus-frontend-backend-b02a/backend"
env -u TITAN_SESSION_TEST_DATABASE_URL -u TITAN_CATALOG_TEST_DATABASE_URL \
  -u TITAN_CASH_JOURNAL_BRIDGE -u TITAN_DELIVERY_BRIDGE -u TITAN_MAINTENANCE_BRIDGE \
  GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off \
  go test ./internal/localapi \
  -run '^TestPilotCashSaleToSupplierFinanceRestartAndRestore$' -count=1 -v
```

O Go padrão1.24 deste Ubuntu não atende go.mod; na execução histórica foi usado o Go1.25.8 já existente no cache. Para a variante Electron, fornecer juntos os três parâmetros opcionais documentados no guia. Sem esses parâmetros a repetição usa TCP/HTTP direto, ainda com handlers e SQLite reais, porém não testa o transporte Electron.

Não há base de requisitos medida para converter esse recorte em porcentagem de todo o TitanSystem. Conclusão deste circuito local não equivale à conclusão do ERP, implantação comercial, fiscal/TEF/Pix ou instaladores Windows/Linux.
