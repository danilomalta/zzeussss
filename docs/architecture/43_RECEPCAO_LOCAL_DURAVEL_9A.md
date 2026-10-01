# Fase 9A — Recepção autenticada e durável

Base: 8f11330. Esta etapa cria a inbox local, autorização de pares e
recibos assinados compatíveis com a outbox. Não entrega transporte HTTP,
worker automático nem comunicação operacional entre empresas.

## Escopo e confiança

Somente dispositivos diferentes da mesma empresa E loja, previamente
cadastrados e aprovados por pareamento. O dono concede explicitamente
cada combinação origem, destino e tipo de evento. A chave pública vem
do registro aprovado, nunca da mensagem recebida.

Tipos desta versão: sale.committed, stock.operation, cash.open e cash.close,
com schema_version 1. Mensagens de outras empresas, lojas, tipos ou versões
não são aceitas. Isso não substitui os futuros vínculos comerciais entre
mercado e fornecedor.

Grant/Revoke exigem sessão humana e prova do aparelho resolvidas pelo
chamador, ManageStaff e papel owner. Concessão, revogação e auditoria são
transacionais; repetir a mesma decisão não duplica auditoria.
Receive também exige que a concessão tenha um emissor owner ainda ativo.
Revogar aparelho, dono emissor ou concessão impede novas recepções e
repetições. Rotação de chave requer nova concessão explícita.

Nenhuma rota foi exposta nesta fase. A chave privada de Receive pertence
ao dispositivo receptor e deve vir da inicialização verificada. Chaves de
licenciamento não são usadas como chaves de aparelhos.

## Mensagem e recibo

A origem assina metadados versionados com Ed25519 e separador de protocolo,
incluindo destino, identidade do evento, empresa, loja, aparelho, operação,
agregado, tipo, versão do esquema e SHA-256 dos bytes exatos do conteúdo.
Alterar conteúdo ou destino invalida a assinatura. Tentativas de envio são
contabilidade local de retry e não alteram a identidade da mensagem.

O receptor verifica pareamento, concessão, assinatura e destino. Persiste
mensagem e recibo assinado na mesma transação e só devolve sucesso após
commit. Mesma mensagem retorna o MESMO receipt_id e assinatura, inclusive
após reabrir o banco. Reutilizar evento ou operação com outros dados falha.

NewReceiptVerifier copia a chave pública confiável e vincula a confirmação
ao receptor esperado. É compatível com outgoing.Confirm: a origem só marca
acked após verificar o recibo. Recibo adulterado não confirma a fila.

## Recebido não significa aplicado

incoming_events tem status received. Esta etapa não cria uma venda,
pagamento ou movimento de estoque no destinatário e não valida todo o
conteúdo de cada evento de negócio. Aplicação, ordem causal, conflitos,
reconciliação e confirmação comercial terão controles separados.

O recibo prova persistência no destino, não conclusão do pedido ou negócio.
Não representa nota fiscal, confirmação PIX, reserva de doca ou estoque
global disponível.

Receber histórico autorizado não exige nova licença para cada mensagem.
As mutações comerciais locais continuam com os contratos da fase 8.

## Privacidade e transporte futuro

O conteúdo JSON desta versão é assinado, MAS NÃO CRIPTOGRAFADO, e fica no
SQLite do destinatário. Arquivo com permissão privada não é criptografia.
NÃO usar este formato para enviar dados operacionais por um relay Titan.

Antes de abrir comunicação por rede: transporte seguro, proteção do
conteúdo adequado ao destinatário, autenticação operacional, limites,
provisionamento confiável e tratamento das chaves devem ser implementados.
Para empresas distintas haverá consentimento comercial próprio e somente
dados autorizados de pedidos/entregas. Vendas, custos e dados de funcionários
não serão replicados automaticamente ao fornecedor.

Não há descoberta de aparelhos, cópia automática dos registros de identidade,
transferência de chaves, TLS, HTTP, criptografia ponta a ponta, recuperação de
backup ou sincronização de revogações nesta entrega.

## Migração e verificação

0017 cria apenas tabelas novas de pares, auditoria e inbox. Não altera
dados comerciais existentes nem executa migrações PostgreSQL. O teste de
reabertura passa a exigir 17 migrações.

Testes usam dois arquivos SQLite independentes descartáveis. Os registros de
pareamento são preparados no cenário de teste; isso não é provisionamento
de produção. Testes cobrem recibo real confirmado pela outbox, repetição,
escopo, assinatura, conflitos, revogação, rotação, falha de persistência,
reabertura e concorrência. Também verificam que receber evento de venda
não aplica uma venda no destinatário.

Go indisponível no ambiente de preparação: testes, vet e compilação são
obrigatórios no PC antes de considerar esta etapa entregue.
