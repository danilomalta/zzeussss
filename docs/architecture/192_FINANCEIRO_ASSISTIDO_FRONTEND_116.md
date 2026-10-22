# Frontend116 — conferência financeira com B02B13

## Entrega e limite

Integra `/local/payables` aos saldos e origens elegíveis do B02B13 e aos comandos/recibos B02B12 já usados no113. Mantém agenda114, exportação e transferência para planilhas existentes; não altera pedidos115, PDV, backend, banco, permissão ou dependência. O frontend115 foi confirmado no PC com41testes e build20,69s.

O fluxo entregue é: consultar documento existente → consultar parcelas/saldos → escolher uma sangria elegível → revisar valor, parcela, recebedor e referência → confirmar vínculo → consultar recibo → atualizar saldos/histórico. Devolução integral: escolher pagamento → consultar suprimentos elegíveis → selecionar origem → revisar motivo/valor → confirmar → consultar recibo. São vínculos de dinheiro já movimentado, sem nova movimentação física ou transferência.

Não inclui criar/aprovar documentos, pagamento direto/cofre, devolução parcial, Pix/TEF/banco, fiscal, confirmação do fornecedor ou recebimento de mercadoria. Esses contratos/fluxos continuam responsabilidade de etapas próprias. Não declarar financeiro, ERP ou Parte1 completos.

## Componentes e compatibilidade

`LocalPayables113.tsx` passa a montar Finance116 com contexto validado de sessão/capacidades. O componente113 recebe pontos opcionais para detalhe assistido, bloqueio durante confirmação e atualização após comando. Quando esses pontos não são utilizados, o modo113 anterior permanece compatível para consumidores antigos. Na rota de produção o formulário manual113 não é exibido: erro/ausência de B02B13 não faz fallback para digitação de IDs ou dados fictícios.

A lista documental continua sendo a projeção carregada do B02B12; não inventa completude global nem nova paginação no servidor. Busca/CSV cobrem os documentos carregados. Fontes B02B13 usam20porpágina, `offset`, `total_count` e `has_more`, com ordenação do servidor. Linhas de pagamento mantêm paginação20 do cliente113.

As parcelas são validadas e somadas com BigInt. Conferem identificação, ordem, principal, vencimento, bruto/devolvido/aplicado/saldo e resumo. Estado não aprovado tem saldo ativo0, sem apagar principal ou histórico. Falha/incompatibilidade produz erro, não zero ou resultado parcial. Uma mudança documental incompatível exige atualizar/reabrir o documento.

Cada fonte confere finalidade, alvo, parcela, ator autenticado, moedaBRL, tipo de movimento, valor máximo/exato, data, ordenação/duplicidade e metadados de paginação. `complete=true` não significa cobertura de outras lojas ou bancos. `reservation_created=false` e `write_authorization_checked=false` são obrigatórios; consulta não reserva nem autoriza escrita. A transação do backend é a autoridade em caso de concorrência.

O seletor apresenta valores e datas, sem exigir UUID digitado. Na revisão, referências de operação/caixa podem ser abertas em detalhe discreto. Sem nomes de caixa no contrato, não inventa rótulos de turnos. Atualizar ou paginar limpa seleção/revisão. Seleção expira após120s ou recuo do relógio local; é necessário consultar novamente. Não há polling contínuo.

## Recuperação durável

Uma única fila financeira por empresa/loja/aparelho/usuário, com Web Locks. O comando mínimo é salvo e lido de volta antes da primeira requisição. Contém IDs originais, corpo necessário, quantidade em centavos para conferir recibo, parcela e marca de incerteza. Não contém senha/token nem cache documental. Recebedor/referência/motivo necessários ao comando permanecem somente enquanto ele não está resolvido.

Abrir/reabrir lê a fila sem POST automático. Consultar usa o recibo autoritativo existente. Repetir exige ação explícita, consulta primeiro e utiliza os mesmos IDs/corpo. Consulta com erro não desencadeia POST. Resposta perdida ou POST confirmado seguido de falha no GET mantém incerteza. Recibo com valor, parcela ou corpo diferente não libera a fila.

Antes do POST, `uncertain=true` é persistido. Somente uma recusa definitiva da primeira tentativa pode retornar a `uncertain=false`; recusa posterior não rebaixa uma incerteza anterior. Descarte exige consultaGET404 e `uncertain=false`. GET404 isolado não prova falha de uma tentativa incerta. Resultado confirmado remove a fila; não há prazo para apagar automaticamente resultado desconhecido.

Sem Web Locks ou persistência confiável não há nova escrita. Listas, histórico e saldos continuam consultáveis. A fila mínima sobrevive ao logout para recuperação pelo mesmo contexto; apagar no logout poderia perder a prova de uma escrita incerta. Dados carregados ficam em memória e saem ao trocar sessão/contexto. Não registrar payloads em console.

Resolva pendências de uma tela113 antiga antes de atualizar: a versão anterior mantinha a tentativa em memória, e esta entrega não consegue recuperar retrospectivamente comandos que nunca foram persistidos.

## Autorizações

Leitura exige `view_accounting`; escrita exige também `manage_cash`, contrato ativo com `core` e `finance`. Consulta de recibo pendente é permitida ao leitor autorizado mesmo com contrato expirado. Contexto deriva exclusivamente da sessão e é verificado contra capacidades; o backend continua a barreira de segurança. Não amplia acesso de operador nem concede permissão por flag. A fila de outro contexto não é lida por esta interface.

## Contratos utilizados

Referência B02B13 copiada sem alterar o contrato do backend em `docs/api/frontend116-readiness.reference.json`.

| Método | Caminho em `/local/v1/finance` | Finalidade |
|---|---|---|
| GET | `/payables` | Documentos carregados, cliente113 |
| GET | `/payables/{id}/installment-balances` | Snapshot completo por parcela, B02B13 |
| GET | `/payables/{id}/cash-sources?installment_id={id}&limit=20&offset=N` | Sangrias próprias elegíveis, B02B13 |
| GET | `/payable-cash/{id}/return-sources?limit=20&offset=N` | Suprimentos próprios elegíveis, B02B13 |
| GET | `/payables/{id}/cash-payments?limit=20&offset=N` | Histórico/resumoB02B12 |
| POST | `/payables/{id}/cash-payments` | VínculoB02B12 com `operation_id,payment_id,installment_id,withdrawal_operation_id,recipient,reference` |
| POST | `/payable-cash/{id}/return` | Devolução integral com `operation_id,supply_operation_id,reason` |
| GET | `/payable-cash/operations/{operation_id}` | Recibo imutável e recuperaçãoB02B12 |

Nenhum endpoint novo é necessário para o escopo entregue. Este ZIP não aplica nem inicia B02B13: o serviço apontado pelo frontend deve oferecer esses contratos. Contrato antigo retorna indisponibilidade e bloqueia o fluxo assistido. Instalar B02B13 na outra worktree não troca automaticamente um serviço antigo ainda em execução.

## Validação

Testes Node utilizam clientes reais com transporte descartável: integridade de parcelas/resumo, precisão máxima, fontes próprias, limite/ordem/paginação, regras de retorno integral, expiração de seleção, isolamento, persistência, locks, reenvio idempotente e falhas antes/depois do commit. Regressões113/114 preservadas. Typecheck, build e diff-check obrigatórios.

Prévia explícita usa fixtures em memória, Finance116 e os mesmos clientes; não chama API ou banco comercial. Exercita consulta, paginação20+2, revisão/checkbox, vínculo, perda de resposta, reabertura/consultaGET, devolução, conflito/descarte seguro, API antiga, dados inválidos, perfil de consulta e falta de locks. “Reopen screen” conserva memória fictícia; recarregar a página apaga fixtures/fila fictícia. A rota real usa armazenamento durável existente do navegador.

Testar no PC: serviçoB02B13 no destino real, documento aprovado, parcela com saldo e movimentação própria elegível. Selecionar, revisar e confirmar uma vez. Conferir recibo e saldos; devolver integralmente com suprimento elegível; confirmar histórico. Exercitar concorrência e perda de resposta em ambiente descartável. Nunca criar movimentação financeira comercial apenas para testar a interface.

Interface traduzida empt-BR,en,es,fr,zh-CN, reutilizando chaves113 e novas116. Datas/valores comIntl; temas claro/escuro e focoâmbar, sem azul, alvos48px e dialog nativo com teclado/Esc. Campos e confirmação em andamento avisam antes de sair. Revisão humana das traduções estrangeiras, leitor de tela completo, hardware antigo e demonstração contra o servidor do usuário permanecem pendentes. Nenhuma dependência nova. O build geral ainda sinaliza chunks grandes já existentes da planilha; esta entrega não declara esse problema resolvido.
