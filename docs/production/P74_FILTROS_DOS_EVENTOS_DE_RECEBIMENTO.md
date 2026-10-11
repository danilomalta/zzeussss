# P74 — Cinco filtros dos eventos de recebimento

Base: `3a9ec5f`; branch `feat/producao-receitas`.

No histórico de recebimentos em `/local/receiving`, acrescenta cinco filtros combinados por AND, aplicados somente aos eventos da página já carregada:

1. Tipo: recebimento original, recusa integral ou anulação. Um recebimento original posteriormente anulado continua no tipo recebimento e conserva sua correção vinculada.
2. Referência exata da entrega: comparação sensível a maiúsculas. Anulações não têm referência de entrega neste contrato e ficam fora deste filtro, mesmo se o original aparecer na página. Não presume referências a partir de registros ausentes.
3. ID exato do local: recebimentos e anulações têm local; recusas integrais não têm local e ficam fora quando este critério está aplicado.
4. ID exato do operador do evento: no recebimento é o operador original; na anulação é o operador da correção. Não usa o autor do original para atribuir a autoria de uma correção.
5. Trecho do motivo: busca literal sem diferenciar maiúsculas/minúsculas, com normalização Unicode NFC e preservação dos acentos. Não interpreta o texto como expressão regular.

## Escopo e consistência

Não cria endpoint ou filtro global do servidor. Reutiliza os dados já carregados por `GET /local/v1/purchase-orders/:id/receiving-history?offset=N`, protegidos pela autorização existente. Nenhuma chamada HTTP extra, escrita ou acesso a outra página.

Antes de selecionar eventos, revalida a página inteira com o validador de resumos da P72. Um filtro que excluiria um evento inconsistente não o torna válido: referências, balanços, correções, duplicidade e paginação ainda são verificados. Os objetos originais não são alterados.

A interface distingue eventos exibidos, eventos carregados na página e total histórico global. Nenhum resultado significa somente ausência nesta página para os critérios aplicados. Os totais globais e resumos da P72 continuam calculados sobre o objeto original completo da página, não sobre os eventos filtrados.

Os campos editados são aplicados somente ao clicar em “Filtrar nesta página”. A tela mostra os filtros efetivamente aplicados; “Limpar filtros” restaura os eventos da página. Troca de página, pedido, sessão ou nova consulta remonta o componente e limpa os critérios.

O CSV existente continua exportando a página completa anterior aos filtros de exibição; sua descrição explicita esse escopo. Não altera o formato, dados ou validações dos CSVs. Nenhuma quantidade, unidade, motivo, autoria ou timestamp dos eventos é recalculado ou corrigido pelo filtro.

## Arquivos compartilhados

`LocalReceivingHistory.tsx`: substitui a renderização simples da lista pelo componente de filtros, reutilizando o componente existente de cada evento. Acrescenta descrição explícita ao CSV. Mantém totais, resumos, ações de recebimento, consulta e paginação.

`frontend-web/package.json`: inclui os novos testes no comando `test:local`; nenhuma dependência ou lockfile novo.

SQLite permanece no schema 44, última migração `0044_production_receipt_voids.sql`. Sem API, migração ou compatibilidade de backup nova.

## Verificações e entrega

Testes dos cinco critérios, combinação AND, eventos originais anulados, referência/local ausentes, autoria da correção, maiúsculas e acentos, caracteres de expressão regular tratados literalmente, normalização de campos, limites em bytes, escopo de página, ausência de resultados, preservação dos objetos e rejeição da página inconsistente antes da filtragem.

Aplicador verifica base limpa, checksum e patch; executa testes específicos `TestHTTPPurchaseReceivingHistory`, suíte Go completa uma vez, vet, corrida, todos os testes locais do frontend e build. Confere os sete arquivos e faz commit somente após sucesso. Reconhece a entrega já registrada. Sem merge, push, servidores ou bancos comerciais/demonstração. Não houve aceite visual em navegador.
