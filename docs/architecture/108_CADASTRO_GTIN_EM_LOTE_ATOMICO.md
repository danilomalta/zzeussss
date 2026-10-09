# Entrega 33 — Cadastro GTIN em lote online

Base PC: main 0cd4629, entrega 32 validada no PostgreSQL isolado. Pacote independente da branch de produção. Não escreve frontend, SQLite nem banco comercial.

## Fluxo entregue

Um gerente/cadastrador envia uma lista de produtos já existentes, versões atuais e GTINs conferidos. A prévia retorna o código original, sua chave canônica e o snapshot antes/depois de cada produto. O apply revalida tudo sob bloqueio e só confirma após gravar todos os códigos, versões, auditorias, recibos e outboxes. Uma falha em qualquer item ou na última outbox desfaz o lote inteiro.

| Método/rota | Entrada e resultado |
|---|---|
| POST /api/v1/catalog/barcodes/batches/preview | operation_id, reason e items; retorna preview_hash e antes/depois sem escrita |
| POST /api/v1/catalog/barcodes/batches/apply | Mesma entrada mais preview_hash; retorna recibo original do lote e recibos individuais |
| GET /api/v1/catalog/barcodes/batches/{operation_id} | Recupera recibo da própria empresa/ator; consulta não cria nem repete a operação |

Owner/admin/manager/stock autorizados. Cashier/employee/accountant recusados antes das escritas. A autorização no banco é revalidada inclusive na consulta do recibo. Administrador diferente não recupera o recibo privado do responsável: a auditoria administrativa existente por produto continua disponível aos perfis autorizados.

## Limites e validação

JSON application/json até 32768 bytes; objeto estrito em ambos os níveis. Campos desconhecidos, repetidos, nulos, valores numéricos no code, quantidades decimais nos IDs/versões e conteúdo após JSON são recusados. Motivo obrigatório até 500 caracteres sem controles; espaços externos normalizados.

Um lote tem de 1 a 50 produtos distintos e cadastra um novo GTIN por produto. Dois códigos canonicamente equivalentes não cabem no mesmo lote, mesmo em produtos diferentes. A ordem da entrada não muda a identidade: itens são normalizados por product_id. Para acrescentar vários códigos ao mesmo produto use operações individuais ou novos lotes com sua versão atualizada; não aceita expected_version repetida para simular alterações sequenciais.

GTINs conservam as regras da entrega 32: string ASCII 8/12/13/14, dígito verificador, original preservado, chave canônica 14 dígitos e reserva única por empresa mesmo inativado. Até 50 aliases por produto incluindo inativos. Produtos inativos podem receber cadastro administrativo, mas lookup segue exigindo código e produto ativos. O lote não ativa produto, não modifica preço/SKU/estoque e não transfere código.

## Prévia e concorrência

Prévia usa uma transação com snapshot repetível e nenhuma escrita em produto, recibo, auditoria ou outbox. Falha em produto ausente/estrangeiro, versão obsoleta, capacidade ou GTIN ocupado interrompe a prévia; não retorna um hash aplicável parcial.

Hash vincula empresa, ator, UUID do lote, motivo, código e estado/versão antes/depois de cada produto. É um controle de consistência, não uma autorização nem uma assinatura secreta. Aplicação refaz a prévia com bloqueio; outra edição/adição entre prévia e aplicação exige nova prévia. Nunca substitui silenciosamente expected_version.

Apply bloqueia identidade do lote, identidades dos filhos, todas as chaves canônicas em ordem crescente e produtos em ordem crescente. A escrita individual usa operação → chave → produto; o lote não mantém produto bloqueado enquanto espera outra chave. UUIDs dos filhos derivam deterministicamente do lote e product_id em namespace separado dos lotes de preço.

Filho previamente utilizado por cadastro individual é conflito, não é anexado como se tivesse sido criado pelo novo lote. Mesma operação/ator/entrada normalizada/preview_hash devolve recibo original antes de consultar versões atuais. Alterar motivo, produto, GTIN, versão, hash ou ator com o mesmo UUID gera 409. Retry não incrementa versões nem duplica recibos/outboxes.

## Persistência e recuperação

Migração PostgreSQL 15 incremental acrescenta online_catalog_barcode_batches, online_catalog_barcode_batch_outbox e seu histórico de checksum. Sem migração SQLite, números reservados da produção ou mudanças no backup local. titan-online migrate-catalog é manutenção explícita; inicialização da API não executa DDL. Versão futura/checksum adulterado são recusados; migração repetida não reaplica DDL.

Cada filho reaproveita integralmente a implementação individual da entrega 32 dentro da transação do lote. As auditorias e outboxes por código permanecem disponíveis; recibo e outbox do lote ligam todos os filhos. Eventos são registros duráveis locais ao PostgreSQL, não comprovação de sincronização com caixas/aparelhos.

Em perda de resposta preserve UUID, motivo, itens e hash originais. Consulte GET do lote. 200 comprova o resultado gravado; 503 mantém a incerteza e não deve disparar repetição automática. 404 isolado não é motivo para inventar nova identidade: tentativa explícita usa exatamente o mesmo UUID e corpo. O replay retorna os snapshots originais mesmo que depois um preço seja alterado ou código inativado.

## Exemplo de avaliação

POST preview:

```json
{"operation_id":"11111111-1111-4111-8111-111111111111","reason":"Etiquetas conferidas","items":[{"product_id":10,"expected_version":2,"code":"4006381333931"},{"product_id":20,"expected_version":4,"code":"036000291452"}]}
```

Use IDs/versões reais da sua empresa; os números acima são ilustrativos. Apply envia a mesma entrada com preview_hash retornado. Confirme os dois filhos no histórico por produto, consulte o lote e repita o mesmo apply: versões e contagens devem ficar iguais. Não usar produtos/dados reais para testes de falha.

## Verificação e limites do aceite

Testes de entrada estrita, capacidade, duplicidade canônica, versões, sessão revogada, snapshots inválidos, replay/ator, hash obsoleto, falhas tardias, outbox suprimida e commit incerto. Contratos JSON/rotas conferidos. A suíte opt-in PostgreSQL acrescenta migração repetida/futura/adulterada, fluxo HTTP, isolamento, prévia sem efeitos, recuperação, versão obsoleta, rollback após todos os filhos e replay simultâneo do mesmo lote; preserva os testes anteriores.

Preparação executa Go, vet, detector de corrida e builds. PostgreSQL real não está disponível no ambiente de preparação; aplicar.sh exige a suíte catalog em banco novo isolado no PC antes de commit. Testes aprovados não garantem ausência absoluta de falhas.

Não inclui importação de GTIN via CSV/XML, lote de ativação/inativação, desfazer lote, embalagens/caixa/fardo com conversão, etiquetas pesáveis, fiscal, consulta GS1 externa, interface ou PDV local. Correção continua pelo lifecycle individual já auditado; nunca exclui códigos para reutilizá-los. Integração dessas funções tem aceite próprio.
