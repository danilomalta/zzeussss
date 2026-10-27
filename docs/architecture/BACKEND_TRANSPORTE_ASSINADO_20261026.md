# Transporte operacional explícito de pedidos e decisões assinados

26/10/2026. Base a5bbcdf3420ca4fa6c471fcdd52794f86f84c0dc, branch feat/backend-b02a-compat. Unidade iniciada em25/10. Nenhuma migração, mudança no backend em execução ou tela nova. As527rotas continuam as mesmas. Não é conclusão de todo o ERP.

## Problema e comportamento

As APIs de troca já validavam as assinaturas e decisões entre instalações independentes, mas o piloto transportava os documentos pelo próprio driver. Novo executável `titan-trade` faz esse transporte por HTTP real, com job explícito e tentativa durável. Não abre banco, não lê chave de estação, não cria/exporta pedido, não aprova compra, não escolhe fornecedor e não decide atendimento.

O operador utiliza uma mensagem já exportada por uma instalação autorizada. A ferramenta consulta o documento original, valida assinatura Ed25519 e domínio comercial existente, direção enviada, mensagem, destino e chaves fixadas. Confere empresas/lojas/aparelhos/usuários nos dois serviços, que usam sessões distintas. A identidade legal das empresas continua exigindo conferência dos donos por canal independente; fingerprint é identidade técnica, não prova de CNPJ.

Pedido é enviado somente a company-trade/orders/import; decisão é enviada somente ao purchase-orders/id/company-trade/decision correspondente. O backend destinatário continua sendo a autoridade de permissão, licença, vínculo, assinatura, prazo, operação e transação. O cliente não substitui essas regras. Nenhuma URL vem do documento assinado; destinos são configuração explícita. Transporte não cria movimento financeiro/estoque por si só.

## Comandos executáveis no terminal

```bash
titan-trade --help
```

Os três modos usam --job ARQUIVO --state CAMINHO_ABSOLUTO --target-token ARQUIVO. send e retry também exigem --source-token ARQUIVO. Tokens nunca são argumentos em texto nem impressos.

- `--mode send`: primeira tentativa, no máximo1POST. Se o estado já existe, recusa reenvio e manda preservar/recuperar.
- `--mode recover`: apenas leitura do recibo original no destino. Zero POST e zero consulta à origem; funciona com a origem desligada e sem seu token. Assinatura conferida pela chave pública previamente fixada e persistida antes do envio.404 não confirma entrega nem apaga tentativa;401 não cria nova sessão ou efeito.
- `--mode retry`: reenvio explícito com o mesmo job, operação, mensagem e conteúdo, somente depois de consultar o recibo e obter404. Não muda IDs para contornar conflito. Se o recibo existe, recupera sem POST. Se a tentativa já foi confirmada mas o destino perdeu o registro, recusa novo envio; investigar restauração/contexto.

Não há retry automático, worker, daemon, agendamento ou envio após logout/troca de contexto. Operação já recebida pelo servidor pode persistir mesmo que a resposta ou a sessão se perca depois; fica incerta até consulta. Conferências anteriores ao POST não garantem revogação instantânea entre serviços independentes; a autorização vigente da mutação é conferida pelo backend destinatário.

## Contrato do job e arquivos privados

JSON version1, sem campos extras ou duplicados, inclusive em objetos aninhados. Campos obrigatórios: version, source, target, message_id, target_peer_id e operation_id. Cada endpoint source/target contém url, party{tenant_id,store_id,device_id}, actor_id e fingerprint_sha256. Fingerprint:64hexadecimais minúsculos. URLs terminam exatamente em /local/v1, sem credenciais, query, fragmento ou caminho codificado. Empresas devem ser distintas. IDs têm1–128caracteres ASCII seguros (letras/números, ponto, hífen, sublinhado e dois-pontos), sem caminho relativo.

HTTPS fora de127.0.0.1 e::1; certificado validado pelas raízes do sistema. Não aceita HTTP para hostname, nem mesmo localhost. Não segue redirecionamento nem proxy herdado do ambiente. Timeout15s por chamada/2min por comando; corpo de resposta limitado a128KiB. Não fornece insecure/skip-verify, URL automática, importação de CA ou exposição de servidor à Internet. Instalação com CA particular precisa da administração correta do trust store, não de desligar TLS.

Job/tokens: arquivos regulares0600, sem symlink; job até16KiB e token até4KiB. Pasta imediata do estado:0700, sem symlink, caminho absoluto. Token deve ser uma única cadeia ASCII sem espaços internos. Leitura confere inode antes/depois de abrir. Tokens são usados só em memória e nos cabeçalhos das APIs esperadas; runtime não garante apagar cada cópia imutável de string antes do GC. Não alegar proteção contra processo malicioso com o mesmo usuário/controle do sistema.

Estado contém versão, hash do job, hash do envelope, chave pública da origem e status. Não contém token, documento, motivo, produto ou chave privada. Lock flock Linux protege concorrência entre invocações e libera após queda do processo, sem apagar a prova de incerteza. Estado escrito em arquivo temporário0600, fsync, rename e fsync da pasta ANTES do POST; falha de persistência impede envio. Após resposta, só confirma se ID da operação, mensagem e envelope coincidirem. Se houver falha depois do efeito remoto, recuperação usa o mesmo recibo. Não editar job/state para tentar resolver falha; comparar/investigar, mantendo os originais.

A regra de confiança após restauração continua obrigatória. Antes de enviar, origem e destino precisam estar liberados pela revisão explícita dos donos. Recuperar recibo já gravado é leitura e não libera confiança. No piloto, o dono do fornecedor revê sua geração restaurada e os fingerprints antes de transmitir a decisão guardada. O remetente não usa o token do comércio para se passar pelo fornecedor.

## Validação e limites

Regressões usam duas APIs Go/Fiber com SQLite independente e contratos sintéticos válidos. Proxy de falha só interrompe transporte antes/depois de transação REAL; não substitui resultado comercial. Casos: duas direções, CLI compilada, perda de resposta, recuperação com origem desligada,404 seguido de retry explícito, sessão inválida sem reenvio/limpeza, contexto/chave incorretos, restauração bloqueada, redirecionamento sem vazamento e certificado TLS não confiável. Arquivos/JSON/locks e help sem arquivos também têm regressões.

Piloto completo de dinheiro/devolução com CLI: comércio164requisições/seis processos +fornecedor22requisições/três processos =186chamadas/nove execuções. Estoque5000milésimos, caixa1574centavos e obrigação em aberto0; mesmos IDs após reinício/restauração. Resultado financeiro e comercial não foi alterado para acomodar o transporte. A chamada de revisão pós-restauração é uma ação explícita do dono na fixture, não aprovação implícita pelo cliente.

CI compila o sétimo executável e o exige nos pilotos com detector de corridas. Os relatórios finais e instalação constam no registro único de continuidade; não confundir rodada focada com suíte integral aprovada. Primeiro erro da fixture era updated_at obrigatório ausente; corrigido no dado descartável. Outro comando passou flag de localapi para pacote diferente e foi rejeitado; repetido corretamente usando variável do executável, sem afrouxar os testes.

Linux é a plataforma validada. Outra plataforma falha explicitamente sem lock durável equivalente, em vez de executar sem proteção. Não é relay público, descoberta de empresas, mensageria com entrega automática, interface de fornecedor, implantação HTTPS comercial, fiscal, Pix/TEF ou validação de todas as funções do ERP. O operador ainda precisa de endpoints e sessões autorizados. Nada foi escolhido/ativado nos serviços8182/8282 existentes.


Atualização posterior nesta frente: [fila operacional](BACKEND_FILA_ASSINADA_20261026.md). As observações de ausência de worker acima descrevem o modo avulso original; queue watch entrega somente mensagens enfileiradas por autorização explícita, sem reenviar operações incertas automaticamente.
