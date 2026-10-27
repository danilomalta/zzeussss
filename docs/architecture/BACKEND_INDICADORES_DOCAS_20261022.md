# Indicadores e planejamento de docas — 22/10/2026

Complemento de backend após Agenda de Docas. Cinco consultas de API, sem nova
migração, escrita operacional, licença automática ou alteração de frontend.

## Janelas livres planejadas

`GET /local/v1/logistics/insights/availability`: doca, início/fim até7dias e
duração desejada5..480minutos. Analisa reservas/chegadas/descargas que cruzem a
janela; devolve intervalos livres e o primeiro encaixe para a duração pedida.
Um intervalo livre pode ser maior que8horas; `first_fit_ends_at` é apenas um
encaixe dentro dele, que ainda precisa da reserva oficial com limite de8horas.

Janelas usam [início,fim), segundos inteiros e UTC. Horários adjacentes não
colidem. Doca desativada não devolve janelas. Sobreposição armazenada indevida
é erro de integridade, não uma janela aparentemente válida.

`confirmed_available=false` sempre: é uma observação planejada, não autorização.
Uma reserva concorrente ainda pode ocupar o horário; usar o endpoint oficial
de reserva com ID persistente e tratar conflito/recibo. A consulta não reserva.
Descarga atual e contagem de chegados esperando são mostradas mesmo quando o
horário planejado dessa descarga ficou fora do período consultado. Não prevê
quando uma descarga atrasada terminará nem promete disponibilidade física futura.

## Métricas por fornecedor

`GET /suppliers/performance` e `.csv`: grupo por supplier_id, na mesma empresa
e loja; filtros de doca/fornecedor e início/fim até366dias. O critério é início
PLANEJADO no intervalo [from,to), não instante da chegada ou do pagamento.
Isso evita duplicar uma reserva em dois períodos adjacentes, mas não representa
uma coorte de descargas concluídas no período.

Contagens dos seis estados, amostras de chegada, chegadas atrasadas pelo menos
um segundo, esperas até o início e descargas concluídas. Espera de veículo ainda
na fila não entra na média como se fosse observação final. Descarga em curso
não entra na média de duração concluída. Canceladas não entram nas amostras de
chegada, mas permanecem na contagem de cancelamentos.

Médias usam nanossegundos observados e racionais exatos em segundos, n/d como
strings; sem amostras ficam null. Não arredondam cada evento para segundos antes
de somar. Totais gerais são da coorte completa, mesmo quando a página de
fornecedores está vazia. Ordenação por ID, sem ranking automático de fornecedor,
atribuição de culpa, comparação entre empresas ou promessa de entrega.

## Pendências e observações

`GET /pending` e `.csv`: lê reservas/chegadas/descargas abertas, sem filtro de
data para não esconder pendência antiga. Filtros opcionais de doca/fornecedor.
Prioridade determinística:1pedido cancelado exigindo resolução local;2descarga
em curso;3chegada aguardando;4fim planejado passado exigindo revisão. Desempate
por horário planejado e ID. Reserva futura comum não é alerta.

Fim planejado passado NÃO vira `no_show`, cancelamento ou liberação automática.
Pedido cancelado não cancela agenda. Estes resultados são observações e não
notificações persistidas/enviadas; não criam tarefas, eventos ou decisões.

## Acesso, limites e exportação

Todas as consultas usam uma transação autorizada: usuário/aparelho atuais com
manage_stock e view_orders, empresa/loja derivadas da sessão. Contrato não é
renovado para leitura histórica. Cache-Control:no-store; nenhum dado em console.

No máximo5000reservas por consulta; acima disso retorna422 com código estável,
sem totais parciais. Reduzir janela ou filtrar fornecedor/doca. Paginação de
saída1..100, offset0..1000000; limites valem antes da paginação de fornecedores
e pendências. CSV contém apenas a página retornada e protege campos textuais
contra fórmulas. Médias ausentes saem em branco; racionais saem como n/d.

Datas em2000..2200, precisão de segundo nos filtros. Campos duplicados,
desconhecidos, nulos fornecidos, filtros vazios e valores fora dos limites são
recusados. A fonte não contém promessa de rota, localização ou hora de chegada
fornecida por motorista. Não inferir culpa de métricas operacionais locais.

## Aplicação e pendências

Exige Agenda de Docas original intacta e base46 ou B13+Agenda/55. Preserva todas
as SQLs e rotinas de backup. Apenas acrescenta unidades/contrato/arquitetura e
montagem; mapas globais de rota são ampliados quando existentes. Aplicador faz
ensaio em candidata, testes, vet, checagem de preimagens e backup dos fontes.
Não abre banco comercial, não inicia serviço e não faz commit/merge/push.

Ainda faltam ligar às telas, validar serviço/perfis reais descartáveis no PC,
alertas persistentes, integração motorista/fornecedor, previsões de rota,
sincronização distribuída e piloto/desempenho em equipamento antigo.
