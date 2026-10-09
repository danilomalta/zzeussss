# Entrega 32 — Múltiplos códigos GTIN por produto online

Base PC main c94dcbf, entrega 31 validada em PostgreSQL real. Seis APIs, migração 14 incremental, transações, recibos, auditoria/outbox e testes. Não altera branch de produção, SQLite, frontend ou banco comercial.

## Identificação e escopo real

Código é string ASCII numérica com 8, 12, 13 ou 14 dígitos e dígito verificador modulo10. Todos zeros recusados; não remove espaços, sinal, acento ou caracteres enviados pelo leitor. Não converte para número, preserva zeros na forma cadastrada. Chave canônica completa à esquerda com zeros até 14 dígitos; formas equivalentes competem pela mesma chave única na empresa. UPC/EAN equivalente não cria um segundo produto.

Referências primárias: [GS1 cálculo manual](https://www.gs1.org/services/how-calculate-check-digit-manually), [GS1 GTIN em mensagens](https://www.gs1.org/edi-xml/technical-user-guide/Item_Numbers). Valida estrutura/dígito e equivalência, não comprova atribuição, fabricante ou licença GS1; não consulta sites externos.

Cada código identifica a mesma unidade vendável do produto. Não representa caixa/fardo com multiplicador, peso/preço embutido de balança, código interno livre, GS1-128/2D/Application Identifiers, SKU ou código fiscal. Esses modelos exigem contratos separados. SKU atual permanece independente. Cadastro de código não torna o PDV local capaz de lê-lo: estas APIs são exclusivamente PostgreSQL online; integração local/desktop/mobile ainda pendente.

Até 50 códigos por produto, incluindo inativos. Código não é editado, transferido, excluído ou liberado para outro produto após inativar. Correção exige inativar o antigo e cadastrar outro GTIN distinto. Outro cliente pode usar o mesmo GTIN na sua própria empresa; nunca compartilha produtos ou preços.

## APIs e permissões

| Método e rota | Função |
|---|---|
| GET /api/v1/produtos/{id}/barcodes | Lista real; state=all/active/inactive, limit padrão 50 máximo 50, offset máximo 1000000000, contagem e has_more |
| POST /api/v1/produtos/{id}/barcodes | Adiciona código ativo; operation_id, expected_version do produto, code string e reason |
| POST /api/v1/produtos/{id}/barcodes/{barcode_id}/active | Inativa/reativa; operation_id, expected_version, expected_code_version, ativo booleano e reason |
| GET /api/v1/catalog/barcodes/lookup?code=... | Resolve GTIN à forma original + produto/preço/version reais; exige produto e código ativos |
| GET /api/v1/catalog/barcodes/operations/{operation_id} | Recupera recibo pelo próprio ator e empresa, inclusive após re-login |
| GET /api/v1/produtos/{id}/barcodes/history | Auditoria administrativa paginada, todos responsáveis da empresa; snapshot original |

Owner/admin/manager/stock adicionam e inativam/reativam. Cashier só lista/lê. Histórico administrativo restrito a owner/admin/manager. Stock pode recuperar seu próprio recibo, mas não o histórico dos demais. Employee/accountant não acessam estas APIs. Corpo GET, query desconhecida/repetida/vazia e entrada POST com campos extras/duplicados/null/coerções são recusados. Limite JSON 8192 bytes, motivo 500 caracteres sem controles.

## Transação e versões

A sessão, papel, empresa ativa e usuário são revalidados no banco. Escrita bloqueia identidade da operação; cadastro também bloqueia chave canônica antes do produto. Produto é bloqueado, expected_version conferida; estado confere também expected_code_version. Mudança de barcode incrementa catalog_version do produto e code_version quando aplicável, mas não muda preço, SKU, nome, ativação do produto ou estoque. Uma prévia de lote de preços antiga precisa ser refeita após alteração do código.

Cadastro, update de versão do produto, snapshot do antes/depois, motivo, ator e outbox são uma transação. Valores retornados pelo banco são comparados com o esperado; trigger que muda/suprime escrita ou outbox impede sucesso. Falha/commit incerto não retorna recibo fabricado. Duas operações/produtos concorrentes pelo mesmo GTIN não podem ambos ganhar.

UUID do código novo é o operation_id do cadastro; posteriores alterações têm seus próprios UUIDs. Replay idêntico devolve recibo original antes de consultar versões atuais. Mudança de produto, código, ator, motivo, ação ou versão usando mesma operação gera conflito. Não aumenta preço/versão nem publica outra outbox em replay. No-op de ativo/versão obsoleta e duplicata retornam 409; código que não pertence ao produto não é alterado.

Após timeout preserve UUID e corpo; consulte /catalog/barcodes/operations/{operation_id}. Um 503 não comprova ausência. Recibo pertence ao ator autenticado; outro ator recebe 404, ainda que seja administrador. Para consulta administrativa use histórico por produto.

## Leituras e auditoria

Lookup e listas usam repeatable read para não misturar barcode e produto durante edição. Inativar produto impede lookup de todos seus códigos, sem alterar estado dos aliases. Inativar só um código preserva os outros. Lista vazia devolve items=[]; contagem/linhas são conferidas na mesma página. Histórico ordena timestamp/UUID, mantém before=null no cadastro, snapshots e versões originais nas mudanças; não reescreve preço histórico com o atual. Paginação não é snapshot durável entre requisições.

Outbox é intenção de publicação; não comprova que o caixa recebeu o código. Não há transporte novo, cobrança ou documento fiscal nesta entrega.

## Migração, verificação e instalação

PostgreSQL 000014_online_catalog_barcodes.sql: códigos únicos por empresa/chave, vínculo composto de produto, operações/ator, snapshots e outbox. Histórico próprio versão 14/checksum/bloqueio/transação protege migração repetida/futura/adulterada. Manutenção explícita titan-online migrate-catalog aplica sequência 8→14; check-catalog verifica. API e aplicador não migram banco comercial automaticamente.

Testes cobrem EAN/UPC/GTIN equivalentes, check digit e zeros, entrada estrita, papéis, no-op/versões, duplicidade inclusive inativo, capacidade, resposta perdida, trigger de produto/código, outbox suprimida, commit incerto, listas/lookup/auditoria, isolamento e migrações. Suite PostgreSQL isolada inclui ciclo HTTP com múltiplos códigos, duplicata entre produtos, duplicata permitida entre empresas, inativação/reativação, preço posterior/replay original, rollback tardio e disputa concorrente por GTIN.

Preparação verifica Go, vet, race, builds e contratos de 104 rotas. PostgreSQL real não está disponível aqui: o aplicador exige suite catalog em banco novo isolado no PC antes de commit. Não faz push, migração comercial ou servidor; preserve alterações/saída se falhar e não reaplique automaticamente.
