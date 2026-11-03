# Produção integrada e inicialização desktop67

## Implementação

A inspeção completa confirmou que receitas, capacidade, ordens, materiais, etapas, resultados, perdas, lotes, qualidade e custos já tinham clientes e telas. Esta entrega não duplica esses módulos. Acrescenta uma ponte de aceite reproduzível usando os clientes reais das telas e corrige uma limitação observada do desktop: encerramento sem informação quando outra instância já está aberta e janela oculta sem diagnóstico quando a interface não fica pronta.

O desktop passa a informar instância existente; mostra a janela no ready-to-show ou no did-finish-load; detecta falha de carregamento do frame principal, encerramento do renderer e prazo de20segundos sem janela pronta. Mostra erro sem fazer reload/reenvio de operações. Mensagens não imprimem URL, conteúdo de renderer, token ou arquivo de conexão. Não muda perfil, serviço, autenticação ou dados. Os testes usam eventos simulados para esses cenários; isso não demonstra abertura gráfica no Linux do proprietário.

Novo modo explícito npm start -- --software-rendering (ou TITAN_RENDERING_MODE=software) chama app.disableHardwareAcceleration antes de ready. É uma tentativa de compatibilidade com o aviso MESA de permissão observado, não alteração do driver ou garantia de corrigir toda falha gráfica. Sandbox permanece habilitado. O atalho gerado com modo software leva o mesmo argumento para a aplicação. Argumentos duplicados/arbitrários permanecem recusados pelo iniciador. Nenhuma dependência nova.

## Aceite de produção executado

A ponte production67.bridge.mjs usa createProductionRead/createProductionCommands, os mesmos módulos importados pelas telas. Testa TCP real com Fiber e SQLite schema65 descartável: consulta da receita/capacidade; publica versão2 preservando versão1; plano simultâneo com insuficiência de ingrediente compartilhado; custo corrente e cenários parcial/completo; cria ordem vinculada à versão1; aprova; configura duas etapas; reserva e consome materiais; executa etapas em sequência; registra27000 de produção boa para30000 planejados; registra3000 de perda; declara lote27000 e inspeção aprovada. Consulta histórico, rastreio e recibos.

Após o POST do resultado ser efetivamente confirmado pelo backend, a ponte descarta a resposta para simular perda da rede. Recria o cliente com o armazenamento preservado e recupera pelo recibo original sem novo POST. O banco confirma somente um resultado e os saldos físicos finais. O fixture é sintético, não o banco de demonstração. Inclui execução com detector de corridas.

Isso verifica integração cliente/API e conciliação, não cliques nas telas, reinício do servidor, restauração de arquivo, operação comercial, rastreabilidade de ingredientes por lotes que não existiam, retrabalho ainda sem contrato ou homologação sanitária. O backend testado é a5bbcdf. Não instala nem reinicia backend/serviços.

## Aplicação

Requer entrega66 aplicada. O usuário pode ainda estar no commit019a94f com a66 não commitada; o aplicador reconhece o marcador e os pós-estados necessários sem exigir commit66 nem misturar as alterações independentes. Só escreve os caminhos declarados e apêndice de continuidade/marcador67. Preserva os oito arquivos locais de carrinhos. Antes das escritas: git apply --check, candidata privada completa, suíte frontend, TypeScript, testes desktop e build de produção; revalidação de todas as fontes e HEAD após os testes. Backup e rollback condicional preservam edições concorrentes.

atualizar.sh reúne aplicação, compilação explícita do desktop e abertura em modo software com um novo atalho. Feche a janela anterior para não focar uma compilação antiga. Não mata processos, não apaga cache/perfil, não faz sudo, não altera AppArmor/permissões do Linux, não usa --no-sandbox. Se o problema gráfico persistir, a nova informação de falha é necessária para corrigir a causa efetiva.

Sem commit/push automático. O commit opcional desta entrega confere também os20pós-estados da66 e inclui seus arquivos caso ainda estejam pendentes, sem misturar carrinhos ou edições independentes. Se houver divergência, recusa e preserva os arquivos. Não use git add geral. A67 não representa o ERP inteiro concluído: fiscal, pagamentos eletrônicos, smartphone independente, Windows e aceite visual/comercial continuam no inventário.
