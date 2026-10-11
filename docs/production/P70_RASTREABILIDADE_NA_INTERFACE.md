# P70 — Cinco consultas consolidadas da ordem

Base de aplicação: `fceb744`, branch `feat/producao-receitas`.

Na página de produção local, informar o ID da ordem e consultar:

1. Ingredientes: quantidades planejadas, reservadas e consumidas, por produto e unidade explícita.
2. Reservas: contagens de ativas, consumidas e liberadas; identificação e autoria da reserva ativa ou consumida atual.
3. Etapas: sequência, estado, revisão e responsável de cada etapa; ausência do plano indicada explicitamente.
4. Resultado: planejado, produzido, diferença de rendimento, perdas declaradas e diferença ainda sem classificação; motivo e data da conclusão.
5. Lotes: totais produzidos, classificados e sem lote, página de lotes registrados/anulados e último parecer humano com critério, motivo e autoria.

## Contrato e consistência

Reutiliza `GET /local/v1/production/orders/:id/trace?lot_offset=N`, com paginação de 50 lotes e autenticação pelo token da sessão local. Não há endpoint, permissão ou escrita novos. A autorização e o isolamento de empresa, loja e aparelho continuam no backend existente. A consulta consolidada já é executada em uma única transação de leitura no servidor.

Ao trocar a página dos lotes, toda a consulta é renovada; os cinco painéis exibem o mesmo retrato retornado pelo servidor. O ID editado ou a sessão alterada limpa o resultado; respostas antigas são descartadas pelo `useReadTask` existente. A leitura não utiliza cookies, cache, armazenamento de operações ou POST.

O cliente reutiliza a validação da execução e acrescenta validação das perdas, balanço global de lotes, paginação, duplicidade, vínculos ao resultado/produto/unidade e identidade do parecer com os metadados originais do lote. Ordem concluída exige etapas concluídas quando houver plano. Totais são inteiros seguros; os balanços usam BigInt e a exibição usa a escala de milésimos da unidade explícita. Não há conversão entre massa e volume.

Lotes anulados preservam sua quantidade histórica, mas não compõem o total classificado. A soma dos lotes registrados da página pode ser menor que o total global, nunca maior. As quantidades representam a ordem e o resultado, não o saldo de estoque atual. Parecer humano não é liberação automática de venda. Reservas liberadas são contadas; esta tela não anuncia um histórico individual de cada liberação.

## Arquivos compartilhados

`frontend-web/src/modules/production/pages/LocalProduction.tsx`: inclui uma seção somente de leitura. Os formulários e contratos de mutação existentes permanecem independentes.

`frontend-web/package.json`: acrescenta o arquivo de testes ao comando `test:local`; sem mudança de dependências ou lockfile.

SQLite permanece no schema 44, última migração `0044_production_receipt_voids.sql`. Nenhuma migração ou alteração de backup. O teste backend de rastreabilidade inclui restauração da cadeia completa pelo backup existente.

## Verificação

Executar testes específicos `TestHTTPProductionTrace`, suíte Go completa uma vez, `go vet ./...`, detector de corrida dos testes de rastreabilidade, todos os testes locais do frontend e build TypeScript/Vite. Os testes novos cobrem fases sem resultado, quantidades exatas, vínculos, etapas incompletas, perdas, totais globais/paginados, anulação de lote, parecer humano, rejeição de IDs/offsets inválidos, uma única chamada GET e recusas de sessão/acesso.

Não foi realizado aceite visual em navegador. O aplicador confere base, árvore limpa, checksum, conjunto exato de arquivos e registra somente a P70 após os testes. Reconhece a P70 já registrada pelo assunto do commit e reversibilidade do patch. Sem merge, push, execução de servidores ou acesso a bancos comerciais/demonstração.
