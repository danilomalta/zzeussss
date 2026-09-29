# Fase 5C — Leitura e confirmação auditável da outbox

`outgoing.Pending` lê eventos locais pendentes por empresa, loja e aparelho, em ordem estável. `FailedAttempt` contabiliza tentativa sem descartar a mensagem. `Confirm` só marca `acked` com um recibo cujos IDs e hash do payload correspondem ao evento e cuja prova foi verificada por um `Verifier` fornecido pela camada de transporte. O recibo fica em `outbox_receipts`; repetir o mesmo recibo é seguro.

Nenhum transporte ou verificador de produção foi implementado nesta fase. O chamador deve autenticar o aparelho antes de formar `DeviceContext`. O verificador futuro terá de validar assinatura ou prova autenticada do destinatário, para o tenant e aparelho corretos, **depois** de o destino persistir a operação de forma idempotente. Uma resposta HTTP 200 genérica não basta.

O worker antigo em `internal/core/sync/cloud_sync.go` contém uma verificação de internet e uma mensagem de sincronização simulada. Ele não consome esta outbox. A Fase 5D o remove. As vendas permanecem disponíveis apenas no SQLite local: este pacote não envia dados nem oferece integração com cloud, LAN ou aplicativo móvel.
