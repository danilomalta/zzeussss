# P71 — Cinco exportações CSV da rastreabilidade

Base de aplicação: `71ba497`, branch `feat/producao-receitas`.

A consulta consolidada da P70 recebe cinco ações de download:

1. Ingredientes: planejado, reservado e consumido por produto, com unidade explícita e colunas inteiras em milésimos mais representação decimal exata.
2. Reservas: contagens de ativas, consumidas e liberadas e identidade/autoria da reserva ativa ou consumida atual. Sem reserva atual aparece explicitamente; não inventa linhas de eventos de liberação.
3. Etapas: posição, estado, revisão e responsável de cada etapa, preservando o responsável planejado da ordem separadamente. Ausência do plano é uma linha com `plano_informado=false` e campos de etapa vazios.
4. Resultado/perdas: planejado, produzido, diferença de rendimento, perdas declaradas e diferença sem classificação. Resultado não informado tem flag falsa e campos medidos vazios; produção medida como zero mantém zero explícito.
5. Lotes/qualidade: somente os lotes da página consultada, com offset, total histórico, status, quantidade histórica, fabricação/validade e último parecer humano com critério, motivo e autoria. Totais globais do resultado repetem-se nas linhas e são identificados como `nao_somar`. Lote anulado conserva sua quantidade histórica e informa `inclui_total_classificado=false`.

## Escopo e formato

Exporta apenas o objeto consultado e validado da P70; nenhum GET adicional, POST, reserva ou consumo de estoque. Não concatena páginas automaticamente, não estima saldo atual nem libera venda com base no parecer humano. Todos os CSVs identificam ordem, estado/revisão, versão imutável, local e responsável planejado. O usuário pode consultar novamente antes de exportar caso precise de dados mais recentes.

Reutiliza a geração CSV existente: UTF-8 com BOM, separador ponto e vírgula, aspas escapadas e CRLF. Valores com prefixos de fórmula são protegidos; inteiros fora da faixa segura são recusados. Quantidades decimais têm três casas, vírgula decimal e nenhum separador de milhar. Não converte massa em volume. Campos de sessão, senha e fila de operações não são exportados.

Lotes sem linhas produzem apenas o cabeçalho no gerador; o botão da interface fica desabilitado. Ausência de resultado desabilita a exportação de lotes. Os quatro outros relatórios se referem à ordem inteira consultada; somente o relatório de lotes é paginado. O número `pagina_0` no nome dos quatro primeiros arquivos segue o mecanismo de nomeação existente e não indica uma consulta paginada desses relatórios.

## Arquivos compartilhados e contratos

- `ProductionTrace.tsx`: acrescenta os cinco downloads ao painel de consulta, sem mudar o carregamento da rastreabilidade.
- `ExportCSV.tsx`: aceita cinco novos tipos e uma descrição opcional de escopo; as telas existentes mantêm a descrição padrão. Texto do botão passa a “Baixar CSV”, compatível com consultas de uma ordem ou de uma página.
- `operationsExport.mjs`: acrescenta os cinco nomes à lista restrita de arquivos permitidos no download local. Mantém Blob, remoção do link e revogação da URL; não muda os CSVs anteriores.
- `package.json`: inclui os novos testes, sem mudar dependências ou lockfile.

Nenhuma API, autorização, migração ou compatibilidade de backup nova. SQLite permanece no schema 44, com `0044_production_receipt_voids.sql` como última migração. Autorizações de acesso são verificadas na consulta original pelo backend.

## Verificação e aplicação

Os testes novos verificam cinco relatórios, linhas ausentes, produção zero, quantidades exatas, autoria humana, etapas, paginação, lotes anulados, proteção de fórmulas, download local, campos não exportados e rejeição de objetos inconsistentes. Executar todos os testes locais do frontend e build. Validar também testes backend de rastreabilidade, suíte Go completa uma vez, vet e corrida; o teste backend inclui backup/restauração da cadeia existente.

Aplicador exige a base e árvore limpa, valida SHA256 e patch, executa os testes, confere o conjunto exato dos oito arquivos e faz commit somente após sucesso. Reconhece esta entrega já registrada. Sem merge, push, servidores ou bancos comerciais/demonstração. Não foi realizado aceite visual em navegador.
