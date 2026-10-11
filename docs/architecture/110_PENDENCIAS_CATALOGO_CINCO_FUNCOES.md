# Entrega 35 — Cinco funções de pendências do catálogo online

Base do PC: main, 622e5d5 (entrega 34). Backend oficial somente. A branch de produção permanece separada.

## Funções entregues

| Função | API GET (prefixo /api/v1) | Resultado |
|---|---|---|
| Resumo de manutenção | /catalog/quality/summary | Total de produtos filtrados, produtos com pelo menos uma pendência e contagens por categoria |
| Produtos sem GTIN ativo | /catalog/quality/no-active-gtin | Lista real; códigos inativos não resolvem a pendência |
| Produtos sem descrição | /catalog/quality/no-description | Descrição ausente ou composta só por espaços ASCII, tabulações e quebras de linha |
| Produtos com estoque online zerado | /catalog/quality/zero-stock | Estoque exatamente zero no PostgreSQL |
| Relatório de tarefas em CSV | /catalog/quality/export.csv | Página de pendências, com filtros e indicação explícita de continuação |

Descrição é informação editorial opcional: a lista auxilia manutenção e não declara produto inválido.
GTIN pode não ser aplicável a alguns produtos. Estoque zero pode ser uma situação normal.
Nenhuma regra fiscal, custo, categoria, fornecedor, estoque mínimo ou sincronização é presumida.
O saldo PostgreSQL não comprova saldo SQLite, reserva de produção ou disponibilidade no caixa.

## Permissões e consistência

- Empresa e usuário vêm da sessão autenticada. Owner, admin, manager e stock podem consultar; caixa, funcionário e contador não podem.
- Além do guarda HTTP, cada consulta verifica no banco sessão não revogada, validade, papel atual e empresa ativa.
- Reutiliza schema GTIN 14 existente. Não cria migração nem altera SQLite, backup, produção ou frontend.
- Contagem e página usam a mesma transação repeatable-read e ordem crescente por ID.
- Consulta não cria produto, código, venda, auditoria de alteração ou evento outbox.
- Dados incoerentes, contagem divergente, erro SQL ou falha de commit retornam erro; nunca resultados fabricados.
- pending_products conta produtos únicos. As três contagens de categoria se sobrepõem e não devem ser somadas como quantidade de produtos.
- Cache-Control no-store em respostas autenticadas; relatório com nosniff e nome de arquivo constante.
- Limite de 10 segundos do contexto existente. Busca por trecho pode percorrer catálogo grande; não foi feito teste de carga comercial.

## Filtros e paginação

q: trecho literal do nome ou SKU, até 120 caracteres. %, _ e barra invertida são escapados.
Busca sem distinção de maiúsculas via ILIKE; não promete tolerância a acentos ou erros de digitação.
active: active (padrão), inactive ou all. Resumo aceita somente q e active.
Listas: limit 1–100 (padrão 50), offset 0–1.000.000.000 (padrão 0).
Parâmetros duplicados, desconhecidos, valores inválidos e corpos em GET são rejeitados.
As três listas têm categoria fixa; issue não pode sobrescrevê-la.
CSV aceita também issue: any (padrão), no_active_gtin, no_description ou zero_stock.

Cada página tem total, has_more, next_offset quando houver continuação, generated_at e source_hash.
Hash SHA-256 representa filtros, contagem e itens daquela página, não um token de autenticação ou snapshot durável.
Páginas são snapshots independentes: alterações entre chamadas podem deslocar offset. Para relatório estável de várias páginas,
o operador deve evitar edições simultâneas e conferir os totais. Não existe promessa de exportação integral consistente entre páginas.

## CSV

Colunas: product_id;version;sku;nome;ativo;price_cents;estoque_online;pendencias.
Valores monetários são centavos inteiros. Identificadores/versões continuam inteiros exatos.
CSV limitado a uma página (até 100 produtos), até 256 KiB; listas não são exportadas silenciosamente como relatório completo.
Headers X-Titan-Report-Total, Count, Has-More, Next-Offset e Source-Hash informam cobertura.
Scope é online-catalog. Text-Escape é apostrophe.
Nomes/SKUs com prefixos de fórmula, inclusive após espaços, recebem apóstrofo somente no relatório.
Campos continuam escapados pelo encoding/csv. O catálogo armazenado não é alterado.
Este relatório é para conferência, não para reimportação nos modelos CSV anteriores.

## Demonstração e aceite

1. Cadastrar quatro produtos fictícios em banco isolado: um sem descrição/GTIN/estoque; um só sem GTIN;
   um só sem descrição e com GTIN ativo; um completo, com saldo positivo e GTIN ativo.
2. Resumo deve retornar products=4, pending_products=3, no_active_gtin=2, no_description=2, zero_stock=1.
3. Consultar as três listas com limit=1; percorrer next_offset sem repetir o item na mesma situação estável.
4. Exportar issue=any&limit=1: CSV contém só uma linha; headers anunciam total=3 e continuidade.
5. Corrigir descrição pela API existente: a pendência desaparece na próxima consulta.
6. Inativar produto: sai do filtro padrão, continua disponível em inactive/all.
7. Outra empresa não pode consultar esses produtos; sessão revogada e papéis incompatíveis são negados.
8. Repetir consultas: quantidade de recibos/outbox de alteração permanece inalterada.

Essas tarefas integram o teste PostgreSQL opt-in existente TestPostgresCatalogCreationAndDiscountAtomicity.
O instalador roda em banco novo isolado antes de criar commit. Não usa .env ou banco comercial.
Não autoriza exportação fiscal, venda no celular ou integração da produção.

## Verificação da entrega

O LEIA_ME do pacote registra verificações executadas no ambiente de criação.
Testes unitários com sqlmock validam contrato SQL e falhas, mas não substituem PostgreSQL real.
Teste real exige PostgreSQL no PC; não confundir execução opt-in ignorada com aprovação.
O script exige sucesso do runner isolado, preserva alterações em falha e não reaplica automaticamente.
