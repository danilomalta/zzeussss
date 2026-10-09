# P39 — Interface de pedidos e historico de recebimento

Consulta /local/receiving, area orders, ligada a Minha area e /local/orders.
Busca paginada por fornecedor/produto, estados comercial e recebimento distintos,
itens preservados. Historico exige tambem manage_stock. O servidor autoriza todas
as consultas. Total global nao e soma da pagina. Milésimos com unidade explicita.

Eventos originais, recusas e anulacoes exibidos separadamente, com operador,
referencia, local, motivo e quantidades. Originais anulados marcados, sem somar
novamente mercadoria. Totais cumulativos acima do inteiro seguro do JS continuam
strings decimais; valida recorded-voided=effective, planned=effective+remaining
antes de apresentar. Anulacao e correcao de lancamento, nao devolucao fisica.

Clientes somente GET, sem reserva, baixa, entrada, POST automatico ou IDs novos.
Erros, carregamento, respostas fora do contexto e fim da sessao sao explicitos.
Sem nova migracao, schema44 e backup permanecem identicos.

Compartilhados: router/index.tsx nova rota lazy; accessModel.mjs associa receiving
a orders; LocalOrders.tsx link e texto correto sobre entradas posteriores;
LocalHome.tsx cartao; package.json adicao de teste. Revisar navegacao na integracao
com main. Login, recuperacao e servicos online nao alterados.

Testes locais frontend test:local (86) e build TypeScript/Vite aprovados, incluindo
strings grandes, anulacao vs aceite efetivo, recusa integral, paginas, filtro,
IDs codificados, GET sem efeitos e erro409. Backend suite/vet verificados no lote.
Nenhum servidor ou banco real aberto. Aceite visual no navegador nao executado.

Pendente: formularios de autorizacao, recebimento, recusa e anulacao com fila de
idempotencia; envio/aceite remoto e integracao das branches. Nao se declara a
interface de compras completa com esta tela de consulta.
