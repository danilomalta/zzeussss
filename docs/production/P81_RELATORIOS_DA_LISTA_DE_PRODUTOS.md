# P81 — Cinco relatórios CSV da lista de produtos

Base `7193f46`, branch `feat/producao-receitas`. SQLite permanece 44.

1. Demandas originais da conferência, com quantidades/unidades explícitas.
2. Saldos físico, reservado global e livre na unidade de cada produto.
3. Cobertura de cada demanda e cobertura matemática da lista inteira.
4. Demanda convertida e faltas exatas na unidade do estoque.
5. Sobras hipotéticas por produto, com ausência explícita quando insuficiente.

Exporta somente o resultado carregado da P80. Não envia GET, reserva, consome, cria pedido/ordem ou grava storage. Os geradores revalidam os IDs/local, demandas numéricas e saldos originais e recalculam todos os derivados via `stockListCoverage`; flags, contagens e frações adulteradas não são usadas. Sem novo campo no resultado ou contrato da API.

## Conteúdo e exatidão

Todas as linhas identificam local, produto e escopo `lista_isolada_leituras_independentes`. Demanda preserva quantidade em milésimos e decimal com vírgula/três casas. Saldos permanecem por produto; não há total somando produtos, mesmo com unidades iguais.

Cobertura repete as contagens globais da lista em colunas `*_nao_somar`, mantendo flags por produto e da lista inteira. Campos `snapshot_lista_atomico`, `disponibilidade_futura_garantida` e `estoque_reservado_pela_conferencia` são false. Não há instante único de leitura no contrato e o CSV não inventa data do servidor.

Demanda convertida, falta e sobra usam numerador decimal textual/denominador decimal textual, em milésimos da unidade do estoque: quantidade física = numerador / denominador / 1000. Reutiliza conversões P80: unidade igual, kg/g e liter/ml, sem massa-volume ou contagem-massa. Intermediários BigInt maiores que o inteiro seguro preservam todos os dígitos. Planilhas devem importar inteiros extensos como texto para evitar arredondamento automático.

Sobra de produto insuficientemente coberto fica vazia, com estado `demanda_insuficientemente_coberta`. Sobra zero real pode aparecer quando há cobertura exata. `movimentacao_confirmada=false` em todas as linhas. Faltas mostram zero real para produto suficiente. Nenhum resultado autoriza execução ou garante saldo futuro.

CSV UTF-8 BOM, campos entre aspas, ponto e vírgula, CRLF, escape de aspas e proteção contra fórmulas preservados. NUL em texto selecionado recusado. Não exporta tokens, credenciais ou campos extras dos saldos. Até dez produtos únicos; fonte inválida bloqueia o relatório inteiro.

## Contratos compartilhados e interface

`operationsExport.mjs`: cinco nomes explícitos adicionais de download, sem alterar nomes anteriores/formato. `ExportCSV.tsx`: amplia união de tipos. `StockListCoverage.tsx`: cinco botões sobre o resultado carregado. `package.json`: registra testes. Não altera `stockList`, backend, banco, migração ou backup.

Nomes `TitanSystem_lista_<tipo>_pagina_0.csv` mantêm o contrato existente de download; esta lista não é paginada. Nova consulta/edição/sessão limpa resultado e botões conforme P80. URL Blob/link temporários continuam com limpeza existente. Não é protocolo de troca entre empresas ou recibo de movimentação auditada.

Testes dos cinco conjuntos, fontes/derivados, unidades/frações/extremos, insuficiência versus zero, metadados excluídos, escaping e downloads. Frontend completo/build; backend HTTP de disponibilidade, suíte completa uma vez, vet e corrida dos testes específicos. Patch/script sobre base limpa, base errada e etapa já registrada.

Sem main, merge/push, bancos comerciais/demonstração ou aceite visual em navegador.
