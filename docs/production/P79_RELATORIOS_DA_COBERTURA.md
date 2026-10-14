# P79 — Cinco relatórios CSV da cobertura da ordem

Base `3d7106c`, branch `feat/producao-receitas`. SQLite permanece 44.

1. Plano preservado: identidade/revisão, receita/versão, local, responsável, batidas e saída planejada.
2. Capacidade consultada: batidas possíveis, excesso planejado, cobertura matemática e limitantes.
3. Demanda total dos ingredientes preservados pelo plano.
4. Faltas exatas na unidade da receita com saldo livre na unidade do estoque.
5. Sobras hipotéticas por ingrediente na unidade do estoque.

Cinco botões no painel da P78, gerados exclusivamente da conferência já carregada. Não repetem GETs, alteram estoque ou tornam a conferência uma autorização. Ordem planejada continua exigindo aprovação humana independente. As limitações de concorrência da P78 permanecem explícitas.

## Contratos compartilhados

O resultado local `OrderCoverage` acrescenta `source`, contendo cópias independentes do rastro e da capacidade que produziram o cálculo. Não muda a API, o banco, o número de GETs nem a validação da última leitura da ordem. Os snapshots permanecem em memória da tela, são limpos pelo mecanismo existente quando a consulta/sessão muda e não são gravados em storage. Geradores revalidam/recalculam a partir desses snapshots; não confiam nas quantidades, arrays, flags ou identidades derivados que possam ter sido alterados pelo chamador.

`operationsExport.mjs` acrescenta cinco nomes explícitos à lista de downloads, preservando anteriores e formato. `ExportCSV.tsx` amplia a união de tipos. `ProductionOrderCoverage.tsx` acrescenta os cinco botões. `package.json` registra testes. `orderCoverage.mjs/.d.mts` recebem somente o campo local adicional. Sem modificações em leitores/escritores backend.

## Conteúdo e exatidão

Cada linha identifica ordem/estado/revisão, receita/versão/revisão, local, responsável e instante da capacidade. Escopo é uma conferência isolada, sem reserva. Reportar a ordem como aprovada não implica aprovação pela conferência. Capacidade inclui `disponibilidade_futura_garantida=false` e `estoque_reservado_pela_conferencia=false`.

Limitantes são IDs em um array JSON dentro de uma célula CSV, sem inferir nomes ou usar uma união ambígua por delimitador. Demanda total continua vinculada às batidas preservadas, sem normalização por rendimento. Faltas incluem todos os ingredientes, com zero real quando suficiente. Sobra insuficiente fica vazia com estado explícito `ingrediente_insuficiente`, não é zero disponível. Sobra suficiente pode ser zero real e continua uma estimativa, com `movimentacao_confirmada=false`.

Frações são preservadas em numerador decimal textual e denominador decimal textual. Valores representam milésimos da unidade indicada: quantidade física = numerador / denominador / 1000. Sem floats ou arredondamento; conversões continuam somente as compatíveis validadas na P78/P73. Quantidades originais em milésimos e decimais legíveis com três casas/vírgula são preservadas. Intermediários podem superar o inteiro seguro e permanecem strings exatas. Planilhas devem importar inteiros extensos como texto para evitar arredondamento automático.

CSV UTF-8 com BOM, ponto e vírgula, campos entre aspas, CRLF, escape de aspas e proteção existente contra fórmulas. NUL em campo selecionado é recusado. Não exporta tokens, credenciais, storage, estágio, auditoria interna ou campos extras do rastro/capacidade. Identidade do responsável é autorizada e selecionada explicitamente.

Os nomes mantêm `TitanSystem_cobertura_<tipo>_pagina_0.csv`, por compatibilidade com o componente existente. Não há paginação nesta conferência; conteúdo identifica a ordem/instante. Download repetido é administrado pelo navegador. Não é um protocolo de troca entre empresas nem uma gravação auditada de produção.

## Verificação

Testes de cinco conjuntos, identidade/instante, faltas/sobras, frações, quantidade insuficiente versus zero, snapshots inválidos, dados derivados adulterados, escape/fórmulas, campos excluídos e download local. Frontend completo/build. Backend HTTP de capacidade/rastreabilidade, suíte completa uma vez, vet e corrida dos testes específicos. Patch/script verificados contra base limpa, base errada e etapa registrada.

Sem migração/ajuste de backup, main, merge/push, bancos comerciais/demonstração ou aceite visual em navegador.
