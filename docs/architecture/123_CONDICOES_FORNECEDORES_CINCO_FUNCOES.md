# Entrega48 — condições internas de fornecedores por produto

Base PC main97f9c44. Cinco funcionalidades, migração PostgreSQL000021, sem alterações em SQLite/produção.

1.Criar vínculo único entre fornecedor interno e produto da mesma empresa, com SKU do fornecedor declarado, preço unitário em centavos, mínimo inteiro e prazo em dias. Cadastro idempotente, ID igual UUID da operação.
2.Editar condições/ativação com expected_version, motivo e nova revisão; par fornecedor/produto imutável. Não substitui pela última gravação; conflito409. Um par inativo continua existindo e deve ser editado, não recadastrado.
3.Consultar vínculo atual por ID na empresa, inclusive inativo e após exclusão lógica do produto.
4.Listar com filtros supplier_id/product_id/active e paginação1..100; total/página no mesmo snapshot e ordenação produto/fornecedor/UUID. Páginas distintas independentes.
5.Recuperar recibo histórico pelo UUID da operação e próprio ator. Replay exige mesmo ator/alvo/conteúdo e retorna o snapshot original mesmo depois de revisões ou pai inativo.

## Condições declaradas e limites

Preço unitário BRL0..999999999999 centavos, mínimo1..2147483647 unidades inteiras, prazo0..365 dias. Zero é preço explicitamente declarado gratuito, não preço desconhecido. O preço usa a unidade inteira já usada pelo estoque online; não representa pacote nem converte unidade física. SKU do fornecedor não é conferido externamente. Dados não são cotação confirmada, identidade externa ou aceite comercial. Não cria compra, pedido, reserva, movimento, custo atual do produto, produção ou agenda. Outbox durável permanece sem despacho.

Active é configuração do vínculo, não prova permanente de elegibilidade do fornecedor/produto. Gravação ativa e ativação revalidam ambos sob locks e recusam inatividade/exclusão. Inativação de vínculo existente continua permitida após pai inativo/produto excluído. A inativação posterior de pai não altera automaticamente o vínculo; uso em compra futura precisa revalidar tudo novamente. Histórico e identidade preservados. Cadastro exige existência local de ambos, inclusive quando cadastrado inativo.

## Consistência e segurança

Owner/admin/manager, tenant derivado da sessão, revalidação persistente dentro da transação. Recibo individual restrito ao ator, empresa alheia sem acesso. Criação/edição, before/after auditado e outbox confirmados por readback e mesma transação. Locks de operação evitam dupla execução; índice único protege o par; revisão com rowlock evita atualização perdida. Dados corrompidos/DDL ausente retornam503 sanitizado.

JSON integral estrito até8KiB, sem repetidos/extra/null, dinheiro/quantidades inteiros; GET sem corpo. Filtros sem duplicados/desconhecidos/vazios e product_id canônico. Nenhuma query aceita empresa fornecida pelo cliente.

## Migração e verificação

CLI migrate-supplier-offers/check-supplier-offers exige suppliers20. Ledger21 SHA256, versão/checksum estranhos recusados, DDL transacional e lock. Startup da API não executa DDL. Teste no instalador usa banco novo isolado, nunca banco comercial.

Testes locais: parser/limites/identidade imutável, permissões/query, migração fresh/replay/futuro/checksum, transação/auditoria/outbox e rollback/readback, página corrompida, replay e sessão, Go suite/vet/race/builds, contratos181 rotas e14 testes Python. PostgreSQL real obrigatório no PC antes do commit: par único, centavos, revisão/concor­rência, pais e isolamento entre empresas, lista paginada, recibo por ator/histórico, inativação/replay após pai inativo, exclusão lógica, outbox ignorada rollback integral e catálogo/estoque/custo preservados. Não há PostgreSQL disponível no ambiente de preparação; não declarar teste real local.
