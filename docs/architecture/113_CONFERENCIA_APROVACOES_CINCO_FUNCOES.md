# Entrega38 — Conferência de aprovações, cinco funções

Base do PC: main6203ef8. Backend online; schema16 já existente. Sem migração nova.

| Função | GET sob /api/v1/discounts/checks/ |
|---|---|
| Resumo de aprovações e divergências | summary |
| Aprovados cujo preço atual mudou | price-changed |
| Aprovados com produto agora inativo | inactive |
| Conferência de uma sugestão aprovada | suggestions/:id |
| Exportação CSV de uma página | export.csv |

Compara o produto ATUAL com o snapshot gravado na aprovação humana da entrega36.
current_price_cents/current_product_version e approved_price_cents/approved_product_version
são distintos. Snapshot nunca é sobrescrito. Aprovação sem recibo não ganha snapshot:
has_receipt=false, operation_id="", approved_* omitidos e issue=no_receipt.
As flags disponíveis: no_receipt, inactive, price_changed e version_changed.
Uma alteração só de descrição incrementa a versão e gera version_changed, mesmo sem
mudar preço. O resumo conta attention por sugestão; contagens de flags se sobrepõem.
Lista price-changed mostra somente diferença de preço; inactive somente inativos.

A API não declara desconto elegível para venda. issues=[] significa somente que estas
comparações não detectaram divergência; não valida validade temporal, fiscal, margem,
quantidade, pagamento, loja, regra de aplicação ou mudanças posteriores à resposta.
Mesmo após conferência, checkout futuro precisa revalidar a operação transacionalmente.
Não cancela nem reaprova sugestão e não movimenta preço, estoque, caixa ou outbox.
Aprovações legadas sem recibo exigem conferência humana; dado histórico não é inventado.
Produtos ou sugestões excluídos logicamente não entram; pendentes/recusados não entram.
Sugestão fora de empresa, não aprovada ou ausente retorna404 na consulta individual.

Owner/admin/manager. Tenant/operador derivados da sessão; contexto/papel/empresa e
sessão ativa revalidados dentro de repeatable-read. Contagens e página compartilham
snapshot da requisição. Outra empresa vê zero/lista vazia ou404 individual.
Revogação é observada em requisições posteriores; não é promessa de cancelar uma
resposta já iniciada. Dados/snapshot inconsistentes retornam503, sem valores fabricados.

q: literal por nome/SKU, até120 caracteres, %/_/barra escapados. Sem busca fuzzy/fiscal.
Resumo aceita somente q. Individual não aceita query. Listas aceitam q, limit1–100
(default50), offset0–1bilhão(default0), sem override de filtro fixo.
CSV aceita issue: any(default), all, price_changed ou inactive. any inclui mudança
só de versão e legado sem recibo. unknown/duplicate query e corpo GET são rejeitados.
Ordenação por ID; total/has_more/next_offset/generated_at/source_hash; vazio é items=[].

CSV UTF-8 com ;, proteção de fórmulas em texto, coluna aprovada vazia quando sem recibo,
até256KiB, somente a página. Cabeçalhos de total/count/has_more/next_offset/hash/scope;
no-store/nosniff. Snapshot independente entre páginas, sem exportação global congelada.
Sem schema16,503; sem DDL automático ou alteração de SQLite/backup/produção/frontend.

Testes com mocks cobrem comparação exata, legado, snapshot inconsistente, regras de
filtros, contador/página incompatíveis, vazio, permissões, sessão revogada e CSV.
Teste PostgreSQL opt-in cobre aprovação seguida de preço/inativação/dados, histórico
preservado, contagens, isolamento, consulta individual, HTTP CSV e ausência de gravações.
LEIA_ME distingue validação executada aqui do aceite PostgreSQL exigido no PC.

## Correção da execução isolada
O cenário38 inicia um novo app Fiber sobre o mesmo banco e as mesmas sessões.
Isso separa o orçamento HTTP do longo cenário anterior; não desativa nem aumenta
o limite de100requisições/minuto por IP da produção. Um teste separado envia
100requisições e exige429 na101ª. Nenhuma asserção de negócio é removida.
