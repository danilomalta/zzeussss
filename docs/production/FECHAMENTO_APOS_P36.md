# Estado apos P34–P36

Base P33 ab96a99. Aceite no PC depende dos tres commits e suas verificacoes.
Recusa integral local nao gera estoque. Correcao integral por erro de lancamento
mantem originais e so compensa com saldo livre suficiente. Historico consolidado
preserva quantidades acima de Number.MAX_SAFE_INTEGER como strings decimais.
Nenhuma tela nova neste lote. Nenhum envio/aceite remoto presumido.

Recebimento local agora cobre parcial, diferencas, recusa integral, duplicidade,
unidades explicitas e anulacao por erro com protecao de reservas. Nao equivale
a devolucao fisica ao fornecedor, acerto financeiro/fiscal, aceite de excesso
ou conversoes comerciais: essas regras precisam de criterio comercial explicito
se forem exigidas pelo escopo final; nao as declarar entregues por inferencia.

Continuam pendentes: integracao real entre empresas/confirmacao de compras;
aceite global de TODOS os escritores e restauracao; revisao de contratos e
colisoes entre branches; tres interfaces completas (producao, catalogo/estoque,
compras/recebimento). Regra de contagem de frentes nao e percentual nem numero
fixo de patches. Infra/auth/recuperacao permanecem com o outro chat.

SQLite0043 e0044 livres na base desta branch, utilizadas neste lote. Backup
lista explicitamente25..44, preserva validacoes e nao aceita schemas futuros.
Nenhuma migracao antiga alterada. Sem main/merge/push ou bancos reais.
