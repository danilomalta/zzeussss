# Entrega37 — Acompanhamento de descontos em cinco funções

Base no PC: main c684691. Somente backend online. Reutiliza schema16 da entrega36;
sem migração adicional, sem alteração em SQLite, backup, produção ou frontend.

| Função | GET sob /api/v1/discounts/report/ |
|---|---|
| Resumo por estado e decisões sem recibo | summary |
| Sugestões pendentes | pending |
| Sugestões aprovadas | approved |
| Sugestões recusadas | rejected |
| Exportar página CSV | export.csv |

Consultas reais de discount_suggestions e products, com junção de recibos por empresa
E sugestão. Owner/admin/manager. Stock/cashier/employee/accountant não acessam.
Tenant e operador vêm da sessão; revalidação transacional de sessão/papel/empresa,
com repeatable-read e isolamento de contagem/itens na mesma requisição.
Nada é gravado por essas cinco funções, inclusive auditoria, estoque e outbox.

q: texto literal por nome ou SKU, até120 caracteres; %, _ e barra são escapados
para não virar wildcard. Não promete tolerância a acentos/erros de digitação.
active: all (default), active ou inactive. Produtos e sugestões excluídos logicamente
não entram; produto inativo entra por padrão para que decisões antigas não sumam.
Rotas de estado fixo não aceitam status na query. Exportação aceita all/PENDING/
APPROVED/REJECTED. Paginação: limit1–100 (default50), offset0–1bilhão (default0).
Resumo não aceita paginação/status. Query extra/duplicada, valores inválidos e corpo
em GET retornam400. Empresa sem resultados retorna zero e items=[], sem inventar dados.
Ordenação por ID crescente; contagem, has_more, next_offset, generated_at e source_hash.

Price/version são ATUAIS: current_price_cents/current_product_version. O relatório
não apresenta esses campos como preço histórico de venda/decisão. Os snapshots de
revisão permanecem no recibo da entrega36; não são sobrescritos por esta consulta.
Percentual é discount_bps inteiro (1500=15,00%). Aprovação não comprova aplicação na venda.
Motivo da sugestão e motivo humano são separados. has_receipt=false, operation_id=""
e review_reason="" identificam sugestões sem recibo, inclusive decisões legadas;
final_without_receipt as conta sem fabricar autor, motivo ou histórico.

CSV UTF-8, delimitador ;, cabeçalho e apenas a página selecionada. Metadados HTTP
indicam total/count/has_more/next_offset/hash/scope. Textos com prefixo de fórmula
recebem apóstrofo na exportação sem modificar armazenamento. Limite256KiB por página;
se exceder, retorna503 sem truncar. Download é relatório, não arquivo para importar.
no-store e nosniff. Cada página é um snapshot independente: alterações concorrentes
entre requisições podem mudar o conjunto; não promete exportação global congelada.

Sem schema16, consultas indisponíveis503; sem DDL automático. Os comandos explícitos
de manutenção da entrega36 continuam necessários antes do uso comercial.

Testes: filtros literais, vazio, paginação, legado sem recibo, três estados, valores
monetários/percentuais exatos, dados inconsistentes, falha do banco/commit, permissões,
sessão revogada, CSV protegido e ausência de gravações. O teste PostgreSQL real opt-in
inclui contagens isoladas, decisões existentes, inativação e consulta HTTP do CSV.
LEIA_ME registra o que foi executado aqui e o aceite obrigatório no PC.
