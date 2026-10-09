# Inventario apos P28-P30

Base: P27 f5cd65c. Testes locais do lote nao substituem aceite no PC.
Apos aplicar e registrar, inativacao global de produtos passa a ter
revisao/auditoria/historico e regras dos escritores existentes, com busca
por estado. Nenhuma tela nova foi implementada por este lote.

Restam QUATRO blocos de backend, com esforcos diferentes e sem percentual:
1. Confirmacao de compras e integracao externa real entre empresas; o
   cancelamento local ja existe e nao significa pedido enviado.
2. Recebimento conferido de compras: parcial/diferencas/unidades/duplicidade
   e movimentos atomicos preservando reservas.
3. Aceite integrado global: producao/venda/contagem/ajuste/recebimento
   concorrentes, rollback, isolamento e backup do ciclo inteiro.
4. Fechamento dos contratos/API e revisao conjunta das branches.

Fluxos completos de tres interfaces continuam aguardando integracao e
aceite: producao; catalogo/estoque; compras/recebimento. Telas existentes
de demo/cadastro/busca nao equivalem ao fluxo completo de producao/compras.
Infra/auth/senha ficam com outro chat. Financeiro/fiscal/mobile e roteiro
global nao entram nesta contagem. Quatro blocos nao significa quatro patches.

Migracao 0040 livre na base da branch; demais etapas nao criam migracao.
Na integracao futura conferir colisoes e importar eventos somente com
contratos revisados. Sem merge, push ou alteracao da main neste lote.
