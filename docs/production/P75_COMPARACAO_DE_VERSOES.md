# P75 — Cinco funções de comparação de receitas

Base autorizada: `bef60de`, branch `feat/producao-receitas`. SQLite permanece 44.

1. Consultar duas versões imutáveis pelo ID, com identificação da receita e revisão.
2. Comparar nome, produto resultante, unidade e rendimento por batida.
3. Identificar ingredientes adicionados, com quantidade e unidade da comparação.
4. Identificar ingredientes removidos, com quantidade e unidade da origem.
5. Comparar ingredientes alterados e contar os preservados.

A direção é origem → comparação. Revisões anteriores são permitidas e claramente identificadas. Versões não precisam ser consecutivas. IDs e revisões distintos da mesma receita são obrigatórios; nenhuma comparação entre empresas/receitas ou resposta parcial é publicada. O servidor continua responsável pela autorização e isolamento em cada GET.

## Contrato compartilhado

A página `LocalProduction.tsx` acrescenta o painel `RecipeComparison`. O manifesto de testes em `package.json` inclui o novo teste. Não se altera o contrato de escrita nem os validadores compartilhados existentes.

Reutiliza `GET /local/v1/production/recipe-versions/:id` duas vezes, com Bearer, sem cookies, cache ou corpo. Consultas independentes são paralelas; resultados só são apresentados após ambos passarem pela validação. Uma falha 401 prevalece sobre outra falha para invalidar a sessão no mecanismo já existente. Alteração de campos/sessão ou resposta ultrapassada limpa/descarta o resultado via `useReadTask`.

As versões são imutáveis, mas os dois GETs não são uma transação única. Não se consulta nem compara o estado de ativação, catálogo atual, disponibilidade ou ordens. Falhas de autorização e referências inexistentes mantêm as mensagens do leitor local.

## Quantidades exatas

Entradas positivas inteiras seguras até 9007199254740991 na escala de milésimos da unidade explícita. Revisão até 2147483647; até 100 ingredientes únicos; referências limitadas a 128 bytes UTF-8. A validação adicional é exclusiva deste leitor. Ingrediente não pode referenciar o próprio produto resultante.

Diferença = quantidade da comparação convertida à unidade da origem − quantidade da origem. Conversões somente kg/g (1000) e liter/ml (1000), ou mesma unidade, incluindo meter e unit. Sem conversão entre volume/massa, contagem/massa ou comprimento/massa. Produto resultante substituído não tem diferença aritmética de rendimento.

Intermediários e magnitudes usam BigInt; a diferença é uma fração exata em milésimos, com numerador e denominador como strings. Valores convertidos podem ultrapassar o inteiro seguro sem perda ou arredondamento. A apresentação preserva divisões fracionárias. Não se normalizam ingredientes pelo rendimento, nem se soma disponibilidade para versões alternativas. Mudança de unidade aparece mesmo com quantidade equivalente. Quantidades fracionárias de unit já aceitas na receita original são preservadas.

## Verificação e limites

Testes unitários cobrem identidade/revisão, ingredientes, conversões, diferenças negativas, equivalência, incompatibilidade, extremos, resposta trocada, falha parcial e autorização; frontend completo e build TypeScript/Vite. Backend: testes HTTP de receitas, suíte completa uma vez, vet e corrida dos testes HTTP de receitas. O script verifica base, checksum, manifesto e árvore limpa antes de registrar somente esta entrega.

Sem API nova, migração, ajuste de backup, reserva, consumo ou produção. Sem merge/push, alteração de main ou bancos comerciais/demonstração. Não foi realizado aceite visual em navegador.
