# Pedidos e aceite autenticado entre empresas — 25/10/2026

Base: backend `55fdec11843dfd782679767edb58939176ad4c92`, branch `feat/backend-b02a-compat`. Esta unidade preserva os fluxos manuais e acrescenta APIs locais e provas assinadas. Não altera frontend/Electron nem os dois serviços de testes existentes.

## Operação entregue

Cada instalação possui banco, empresa/loja/aparelho, sessão humana e chave próprios. O dono confere a identidade empresarial e o fingerprint público da contraparte por canal independente e registra um vínculo específico. Vínculo comercial não aprova pareamento de dispositivos, não concede acesso ao banco remoto e não permite que o token de uma empresa seja usado na outra.

1. `GET /local/v1/company-trade/identity`: chave pública, fingerprint e identidade técnica local. Não comprova identidade legal/CNPJ por si só.
2. `POST /company-trade/peers`: dono vincula fornecedor ao cadastro local existente, ou cliente à contraparte. Inclui fingerprint e referência da conferência. IDs e chave ficam imutáveis; revogação é durável. Todas as rotas abaixo usam o prefixo `/local/v1`.
3. `POST /purchase-orders/:id/company-trade/export`: usuário autorizado exporta somente pedido aprovado, não cancelado, sem recebimento/decisão anterior. Seleciona um fornecedor vinculado ao pedido. É snapshot de um item, quantidade/unidade/identificação originais, destino exato e prazo de até sete dias. Não exporta catálogo completo, preço/custo, banco ou credenciais.
4. `POST /company-trade/orders/import`: operador do fornecedor importa o envelope assinado do cliente já vinculado. Não cria venda, reserva, produção ou estoque automaticamente. IDs de produto da origem são referências do pedido; não são mapeados silenciosamente ao catálogo do fornecedor.
5. `POST /company-trade/orders/:message_id/decision`: funcionário autorizado do fornecedor confirma quantidade/prazo ou recusa com motivo. A decisão é assinada pela estação após autenticação e autorização da sessão humana. O funcionário não fornece chave privada ou identidade de origem no corpo.
6. `POST /purchase-orders/:id/company-trade/decision`: operador do comércio aplica a resposta. Assinatura, vínculo ativo, destino, validade, hash do envelope original, produto/unidade/quantidade, pedido e ausência de decisão anterior são conferidos na mesma transação. O resultado passa a `evidence_mode=authenticated_company`, `external_verified=true`; isso significa assinatura de estação vinculada e decisão autorizada pelo servidor do fornecedor, não fiscalização da capacidade física ou identidade legal externa.
7. As entregas existentes ficam limitadas à quantidade confirmada. Estoque só muda na conferência de recebimento. Obrigação, aprovação independente, retirada e vínculo financeiro mantêm os contratos atuais. Pedido/aceite não movimentam dinheiro, não faturam nem reservam automaticamente.

`GET /company-trade/messages` tem filtros direction/kind, paginação explícita, total/has_more e somente registros da loja autorizada. `GET /company-trade/messages/:id` recupera o envelope original. `GET /company-trade/peers` mostra também vínculos revogados. `GET /company-trade/operations/:operation_id` recupera recibo original restrito ao ator/aparelho, inclusive após falha de resposta. O reenvio da mesma operação devolve recibo; corpo divergente ou tentativa de duplicar com outra operação retorna conflito. Não apagar ou reenviar operações incertas automaticamente.

## Assinatura e transporte

Ed25519 cobre o domínio `titan-company-order-v1`, byte NUL e os bytes originais do payload. O payload é JSON UTF-8 compacto na ordem e escaping de `TradeMessage` (Go encoding/json), com TODOS os campos presentes. Payload e assinatura são strings base64 na API. O parser rejeita forma não canônica, campos extras/duplicados/nulos, números alternativos, contexto incorreto, payload acima de8192bytes, assinatura adulterada e prazo inválido. A API externa tem limite de corpo32768bytes. O prazo não libera nem reserva estoque.

A chave privada vem de `stationfile.OpenVerified` na inicialização do executável; o servidor confere sua chave pública contra o aparelho aprovado. Construtores sem assinante mantêm leitura/importação quando autorizadas, mas exportação/decisão de saída retornam503. Não há fallback para chave enviada por HTTP.

O cliente autorizado transporta explicitamente os envelopes entre as duas APIs. Os testes fazem esse relay por chamadas HTTP reais. NÃO é um relay público, worker de envio automático, descoberta de fornecedores ou demonstração de tráfego comercial pela Internet. Nenhuma URL recebida é buscada pelo backend. Assinatura protege integridade/autenticidade, não cifra o conteúdo; uso remoto exige HTTPS e implantação autorizada. Perfis de rede existentes não foram relaxados. Não publicar a API operacional na Internet ou selecionar banco/estação/licença pelo usuário.

## Restauração e revogação

Snapshot antigo não conhece revogações posteriores. Ao restaurar arquivo com vínculos entre empresas, `titan-backup` grava uma nova geração aleatória de bloqueio SOMENTE na cópia recuperada, antes de publicar o destino. Se o bloqueio não persistir, recusa publicar. Sessões e chaves de recuperação continuam revogadas conforme as proteções existentes. O banco de origem não é alterado.

Pedidos, assinaturas e recibos antigos continuam legíveis; novas exportações, importações e decisões ficam bloqueadas com503/`company_trade_review_required`. Consulte `GET /company-trade/recovery`. O dono deve revisar fingerprints atuais e revogações por canal autenticado, revogar vínculos inadequados e registrar a revisão em `POST /company-trade/recovery/review`, com a geração corrente e referência. Essa decisão é explícita e auditada: não há revisão automática. Um gerente não pode desbloquear; revisão de geração anterior não desbloqueia outra restauração. Operações históricas não são apagadas. O backend não descobre sozinho revogações remotas ocorridas enquanto estava desconectado.

## Persistência e compatibilidade

Migração incremental64 cria cinco tabelas separadas (peers, messages, applied, operations, recovery) e gatilhos de integridade/imutabilidade. Migrações1–63 permanecem byte a byte preservadas. O backup valida esquemas25–64 e os backups antigos são migrados somente na cópia descartável restaurada. A inspeção `pilot_cash_v1` continua cobrindo suas14tabelas nomeadas: não inclui as novas tabelas de troca. A prova destas é feita pela releitura das mensagens/recibos e decisões pelas APIs após restauração.

**Não aplicar ZIP64 antigo sobre esta árvore.** O pacote de devoluções anteriormente pendente não foi executado. Esta migração64 pertence a esta base e tem outro conteúdo/checksum: qualquer outra migração64 exige comparação e reconciliação de fontes antes de instalar. O validador recusa divergência, sem forçar/apagar histórico. Binário anterior não deve abrir banco que já migrou para64; não há downgrade automático. Nesta entrega nenhum banco dos serviços8182/8282 é migrado.

## Verificação reproduzível

Com Go compatível e dependências já disponíveis, na pasta backend (não aponta para banco comercial):

```bash
go vet ./...
go test ./... -count=1
go build -o /tmp/titan-backup ./cmd/titan-backup
go test -race ./internal/localdb/supplycycle -run '^TestTrade' -count=1
go test -race ./internal/localapi \
  -run '^TestTrade|^TestPilotCash(SaleToSupplierFinanceRestartAndRestore|PartialReceiptDiscrepancyRestartAndRestore|AuthenticatedSupplierRestartAndRestore)$' \
  -count=1 -v -args -pilot-backup-cli=/tmp/titan-backup
```

O terceiro piloto é o MESMO circuito PDV→caixa→estoque→reposição→pedido→recebimento parcial→financeiro, com fornecedor em outra empresa/banco/chave/sessão/processos. O fornecedor reinicia após importar o pedido e restaura seu backup depois de decidir; o comércio verifica a mesma assinatura original e depois reinicia/restaura o circuito completo. Sem mock de API/negócio, sem envio comercial a terceiros e sem alegar automação de cliques. A licença e a instalação sintéticas são apenas fixture.

Testes adicionais cobrem adulteração de cada byte, assinatura de outro protocolo/chave, contexto, quantidade, prazo, revogação, dono versus gerente, sessão de outra empresa, logout, concorrência, rollback completo se o recibo não persistir, consulta paginada e backup antigo sem ressuscitar confiança. A ferramenta especializada de revisão não está disponível; revisão local e regressões não são auditoria externa de segurança.

CI dedicado executa suíte inteira e os três pilotos com detector de corridas e CLI real, publica logs e binários Linux. Resultado e SHA devem ser conferidos no run, não deduzidos da existência do workflow. Evidências e resultados finais ficam no registro único de continuidade e no pacote desta unidade.

## Limites preservados

Uma exportação e uma decisão final por pedido; renegociação, troca de chave para a mesma identidade de aparelho e replay automático de negócio não foram adicionados. Uma instalação restaurada precisa de revisão humana dos vínculos. Não há reserva/capacidade produtiva automática no fornecedor, pagamento eletrônico/fiscal, sincronização offline entre empresas, relay/Internet comercial homologados ou instalador Windows validado. O frontend destas APIs permanece uma tarefa própria: esta entrega é backend. Não equivale a concluir todo o ERP.
