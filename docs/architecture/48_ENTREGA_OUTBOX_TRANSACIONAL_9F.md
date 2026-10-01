# Fase 9F — Uma entrega da outbox com confirmação transacional

## Entrega

`incoming.DeliverPendingOnce` integra a fila SQLite, assinatura, criptografia,
cliente HTTP e confirmação local. Cada chamada entrega no máximo um evento
pendente já gravado por uma operação de negócio. Não cria vendas novas nem
aplica o evento às tabelas de negócio do destino. Não inicia worker, listener
permanente, comandos CLI ou transmissão automática ao iniciar o PDV.

Empresa, loja, origem, destino, chave privada e endpoint devem vir de configuração
aprovada do processo. O protocolo continua limitado a aparelhos da MESMA empresa
e loja. Não é autorização de compartilhamento comercial entre empresas.

## Fluxo

1. Em uma transação curta, validar o pareamento da origem, correspondência da
   chave privada Ed25519, pareamento do destino, vínculo público X25519 assinado,
   revisão e vínculo ativo do dono aprovador.
2. Encerrar essa transação antes de qualquer comunicação HTTP.
3. Ler um evento pendente da origem, na ordem atual da outbox. Se não houver
   evento, retornar `Empty=true`, sem requisição HTTP.
4. Assinar, cifrar com a chave aprovada e executar uma tentativa HTTP da 9E.
5. Em falha, manter o evento pendente e tentar registrar `FailedAttempt` com
   contexto separado limitado a dois segundos, mesmo após cancelamento da rede.
   Se o registro também falhar, reportar ambos os erros; não declarar sucesso.
6. Após recibo verificado, iniciar outra transação curta: revalidar os dois
   pareamentos, chave privada de origem, assinatura do vínculo X25519, dono ativo,
   revisão, aprovador e as mesmas chaves usadas antes do envio.
7. Recarregar o evento original e verificar novamente a assinatura do recibo
   contra o conteúdo persistido. Inserir `outbox_receipts` e atualizar o evento
   para `acked` na mesma transação, verificando uma linha afetada.

Um erro no commit deixa a confirmação incompleta reportada como falha; nunca
retornar sucesso antes de concluir a transação. Uma repetição concorrente aceita
o mesmo recibo já confirmado e rejeita recibo divergente.

## Revogação e falhas

O estado capturado antes da rede não basta para confirmar. Revogação observada
na transação final ou alteração de assinatura/revisão/chave/aprovador impede o
ack. O destino pode já ter recebido a mensagem; isso não permite ignorar a
revogação na origem. O dono precisa resolver a configuração antes de retomar.
Essa verificação protege a confirmação, mas não desfaz um envio já realizado.

Não há transação de banco mantida durante HTTP, portanto a espera de rede não
prende a única conexão SQLite. A função continua sendo síncrona: a integração
posterior deve executá-la fora da thread da interface, com pausa entre tentativas.

Recepção concluída seguida de falha local mantém o evento pendente. Ao repetir,
inclusive após reabrir o banco, usar os mesmos IDs e conteúdo. O receptor devolve
seu recibo estável. Não remover eventos nem recriar operações para destravar fila.

Esta etapa não configura roteamento persistente. A outbox atual tem uma única
confirmação por evento: usar UM destinatário autoritativo por fila. Distribuição
para vários destinos exige fila/estado por destinatário em outra etapa; não usar
esta rotina para prometer entrega a múltiplos aparelhos.

Entrega de um evento já concluído não exige uma nova autorização humana ou
assinatura comercial para a operação original. Não usar esta função para criar
operações com módulo expirado. As verificações de licença e usuário permanecem
nas funções de negócio e na API. Pareamento/consentimento do receptor continuam
necessários. Registros pendentes não devem desaparecer por expiração de licença.

## Estado e verificação

Retorno: `Empty`, `EventID`, `ReceiptID` e `Repeated`. Apenas sucesso após commit
confirma a origem; `Repeated` indica repetição detectada pelo destino.
Erros não devem ser exibidos junto de chaves, conteúdo ou tokens em logs.

Não há novas migrações. As confirmações usam o esquema de recibos existente da
fase 5C. Testes novos usam bancos descartáveis e servidores temporários:
entrega HTTPS até fila vazia; falha remota e tentativa persistida; rollback local
da confirmação e recuperação após reabertura; revogação durante HTTP; chave
ausente/estrangeira sem requisição; concorrência sem duplicar mensagem ou recibo.

Resultados Go devem ser observados no PC. Não declarar sincronização automática,
replicação de estoque, failover do smartphone ou serviço em produção concluídos
com base apenas nesses testes.
