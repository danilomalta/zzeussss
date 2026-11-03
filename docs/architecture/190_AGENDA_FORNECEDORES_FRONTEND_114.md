# Frontend114 — agenda de fornecedores e planilhas

## Base e resultado

Frontend113 instalado no PC: 45 testes aprovados, fail0 e build20,71s, conforme o log enviado. Continua a área financeira sem alterar cadastro, PDV, backend, migrações ou regras comerciais. Nova rota lazy `/local/payables/agenda`, acessível por Agenda de vencimentos em `/local/payables`. O caminho de volta abre Documentos e pagamentos. Não há dependências novas ou polling.

Consulta período inclusivo até366 dias e uma data de referência. O período inicialmente é o mês atual. Indicadores mostram saldo aprovado conhecido, separado por vencimento anterior, na referência e futuro. Os valores vêm da agenda do servidor para o aparelho/empresa/loja da sessão, e não representam todo o financeiro ou pagamentos externos desconhecidos. A data de referência classifica vencimentos; os pagamentos/saldos continuam atuais à hora da consulta. Não reconstrói saldo histórico.

Tabela com parcelas, fornecedor, documento, estado, vencimento, principal, dinheiro aplicado e saldo. Busca literal sem distinção de acentos, filtros por estado/vencimento/saldo aberto, seleção por página ou individual e paginação25. Os indicadores são do período integral consultado; os filtros abaixo afetam linhas e exportação filtrada. Alterar datas deixa claro que o resultado anterior é snapshot e bloqueia exportação até consultar o novo período. Falha ao carregar nova consulta remove dados anteriores; ausência de dados não vira zero. Zero só aparece após validar relatório vazio completo.

## Contrato existente e integridade

Única chamada nova do cliente: `GET /local/v1/finance/payables/report.csv?from=AAAA-MM-DD&to=AAAA-MM-DD&as_of=AAAA-MM-DD`, já existente no contrato B02B12 enviado. Não foi inventado endpoint de agenda JSON. Fonte CSVformato2,20 colunas,13 indicadores conhecidos, até4MiB e10000 parcelas. CSVformato1 é recusado. A chamada usa Bearer em memória, cache no-store, credentials omit e timeout15s; sem empresa/loja/usuário do navegador em query/corpo. Contexto confiável e permissões são verificados antes de renderizar a fonte, e continuam obrigatórios no servidor.

Parser trata aspas, aspas duplicadas, delimitadores e BOM, valida todas as linhas antes de mostrar o conjunto: versão, escopo, BRL, datas/janela, IDs, estados, quantidade de colunas, inteiros, limites, parcelas repetidas, indicadores ausentes/repetidos, classificação de vencimento e relações entre principal/aplicado/saldo. Recalcula os13 indicadores com BigInt para comparar com a fonte, sem somar novamente resumo e detalhes. Fonte truncada/incompatível/inconsistente é rejeitada integralmente, nunca apresentada parcialmente como completa.

O relatório é completo no escopo declarado pelo serviço local e período, não em outros aparelhos ou empresas. A conferência matemática do cliente não substitui auditoria, autorização ou garantias de completude do backend.

## CSV e planilha

Dois downloads diretos por Blob: agenda do período (inclui resumo e detalhe em seções distintas) e linhas filtradas/selecionadas (apenas detalhes). Quando há seleção, o segundo exporta somente as linhas selecionadas entre as filtradas. A exportação usa csv97, preserva centavos inteiros, padroniza aspas/BOM e protege células contra fórmulas de planilha. Por isso o download da agenda não é cópia byte a byte do CSV original nem um recibo imutável. O snapshot original em memória não é alterado.

Abrir na planilha usa stageReport76/takeReport76 já existentes: contexto idêntico à sessão, transferência única em memória,60s de validade e limpeza ao trocar token/encerrar sessão. Respeita a regra existente: perfil owner e sourceAllowed('finance',capabilities). Não concede acessos novos. Até500 parcelas por envio; acima disso exige selecionar um subconjunto, sem truncar silenciosamente. Copia somente os dados consultados e as datas, não gera bloco vinculado ou atualização automática.

As colunas monetárias da planilha são explicitamente marcadas como centavos inteiros para preservar exatidão, inclusive acima dos limites seguros de representação decimal em reais. Strings são células de texto e inteiros são células numéricas pelo reportBook76 existente. Não mistura resumo e detalhes na matriz exportada. Nenhum relatório/token é salvo por esta área em localStorage/IndexedDB; eventual gravação da planilha segue a ação e o armazenamento já existentes do módulo de planilhas.

Links de cada parcela apontam para `/local/payables?document=ID`. O113 abre o documento se estiver entre os carregados e consulta seus pagamentos sem fazer escrita. Referência não carregada recebe aviso, sem inventar que o registro inexiste. Uma listagem de documentos com erro não provoca aviso falso de documento não encontrado. A validação da query não concede acesso ao documento; o escopo continua sendo o da consulta autenticada.

## Segurança e limites

Leitura exige `view_accounting`, sem exigir licença Finance ativa para consulta histórica, conforme o contrato. Nenhuma escrita de negócio nesta etapa. Na troca de contexto/token, o componente desmonta, aborta chamadas e descarta memória. Respostas antigas não entram em outra sessão. Não registra dados pessoais ou tokens no console. Fonte indisponível,403,401,409,422 e formato incompatível têm mensagens próprias;401 invalida a sessão.

Pendentes: criação/aprovação/rejeição/cancelamento de documentos na interface, seleção assistida das fontes de caixa, pagamento direto/cofre, meios eletrônicos e fiscal, recebimento/pedido, sincronização entre aparelhos e fila durável de comandos. As restrições do113 continuam. Esta agenda não resolve a recuperação de comandos do113 após queda de energia nem faz conciliação bancária.

## Validação

33 testes novos,640 testes totais de frontend aprovados.78 testes selecionados no instalador, typecheck app, build e diff-check. Testes cobrem CSV, datas/leap year/366dias, integridade numérica, limites, ausência/truncamento, filtros, transferência de contexto única/expirada, tipagem de células e cliente HTTP sem escrita.

Firefox:60 parcelas fictícias, paginação, busca, seleção, envio real ao mecanismo efêmero de transferência/consumo em memória, centavos exatos, downloads CSV, datas alteradas e exportação bloqueada, erro/corrupção sem totais, vazio válido, filtro de saldo, bloqueio de planilhas sem permissão, cinco idiomas x três larguras x dois temas, alvos de seleção44px, sem rolagem horizontal da página (tabela tem rolagem interna). Ensaio separado verifica abertura automática do documento carregado e referência ausente sem escrita. Regressão visual/funcional113 de pagamento, resposta perdida/recuperação, devolução e consulta também aprovada.

45 novas chaves completas em pt-BR/en/es/fr/zh-CN; nenhuma ausente. Revisão linguística humana estrangeira pendente. Prévia é isolada e fictícia, usa o cliente/parser real com transporte injetado; não acessa API, banco, planilha comercial ou dinheiro real. O teste de transferência da prévia não monta a biblioteca completa de planilhas. O aplicativo real navega para o módulo existente. Não foi executada a combinação com o servidor B02B12 completo no PC do usuário. Não declara financeiro ou Parte1 completos.

## Testar no aplicativo

1. Com B02B12 ativo, abrir `/local/payables` e Agenda de vencimentos. Consultar mês e referência; conferir indicadores e parcelas. Testar366dias, intervalo inválido e mudança de referência sem interpretar saldo como histórico.
2. Buscar fornecedor, filtrar estado/vencimento/saldo, mudar páginas e selecionar. Baixar agenda do período e CSV filtrado; conferir que resumo não foi somado outra vez ao detalhe.
3. Como owner com acesso financeiro permitido nas planilhas, selecionar até500 parcelas e abrir na planilha. Conferir títulos de centavos, datas e números. Sem permissão, o botão fica bloqueado e a consulta continua funcionando.
4. Abrir documento de uma parcela, conferir pagamentos no113 sem gravar nada. Se a referência não estiver na lista carregada, o sistema informa e mantém a consulta disponível.
5. Testar erro de servidor, CSV antigo/corrompido, vazio, expiração/troca de sessão, claro/escuro, cinco idiomas, tablet/celular e teclado. Em nenhum desses testes há pagamento automático.
