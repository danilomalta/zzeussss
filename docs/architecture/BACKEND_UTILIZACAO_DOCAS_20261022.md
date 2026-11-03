# Utilização histórica de docas — backend — 22/10/2026

Complemento APÓS Indicadores de Docas. Duas consultas autenticadas, JSON e CSV,
sem migração e sem alteração do frontend ou Electron.

## Cálculo

Seleciona uma doca da empresa/loja da sessão. Período semiaberto `[from,to)`,
até 31 dias, em segundos inteiros RFC3339; não aceita fim no futuro. Normaliza
fusos em UTC. O denominador é o tempo corrido do período, não um calendário
de expediente. Não há cadastro de jornada das docas nesta entrega.

O tempo real usa início/fim efetivos de descarga, mesmo quando a reserva
planejada ficou fora do período. Recorta as extremidades no intervalo consultado.
Descarga ainda em curso usa o fim do período e mantém `still_unloading=true`;
isso não registra uma conclusão. Intervalos efetivos sobrepostos são inconsistência
409, nunca soma acima de 100%. Eventos adjacentes são aceitos.

O planejado é a união das últimas janelas registradas, incluindo concluídas,
excluindo canceladas e faltas. Pode sobrepor quando um horário histórico já
foi reutilizado. Não conta o mesmo segundo duas vezes. Não reconstitui o que
se sabia antes de um reagendamento ou cancelamento; o histórico continua na
API de eventos existente. Doca hoje desativada não perde seu relatório passado.

Tempos e frações usam racionais exatos em strings `numerator/denominator`.
Os campos `*_fraction` são proporções entre zero e um, NÃO segundos nem
porcentagens já multiplicadas por 100. Tempos preservam nanossegundos.
Sem descarga registrada pode resultar em zero observado, mas não comprova
doca livre, produtividade baixa, ociosidade ou capacidade disponível.

Inclui série por dia civil UTC, pronta para gráficos. Primeiro e último dia
podem ser parciais: o denominador é só a parte consultada daquele dia.
Todos os dias e totais permanecem no resumo independentemente da paginação.
Não converte automaticamente para o fuso local da empresa; a interface deve
identificar que os dias desta série são UTC.

## Leitura e limites

Uma transação de leitura valida `manage_stock`, `view_orders`, usuário ativo,
empresa/loja e aparelho aprovado. Não exige renovar contrato para consultar
histórico autorizado. Nenhuma autorização é delegada ao frontend.

Por segurança, analisa no máximo 5.000 reservas em TODO o histórico da doca
selecionada, inclusive fora do período e canceladas. Acima disso responde 422
sem totais parciais. Reduzir apenas o período não remove esse limite; uma
consulta indexada por intervalos efetivos ou uma agregação persistida será
necessária para docas com histórico maior. Não há arquivamento automático.

Detalhes ordenados por início efetivo recortado e ID, `limit` 1–100 e `offset`
0–1.000.000. Resumo independente da página. CSV contém sempre cabeçalho e
linha `summary`, todas as linhas `day_utc` e os `segment` da página. IDs textuais recebem
proteção contra fórmulas. CSV não é confirmação fiscal, de entrega ou pagamento.

## Endpoints

- GET `/local/v1/logistics/insights/utilization`
- GET `/local/v1/logistics/insights/utilization.csv`

Parâmetros obrigatórios: `dock_id`, `from`, `to`. Opcionais: `limit`, `offset`.
Rejeita duplicados, desconhecidos, corpo GET, período inválido/futuro. Respostas
sem cache. Contrato completo em `docs/api/dock-utilization.openapi.json`.

## Escopo preservado e pendências

Não reserva horário, muda estado, envia alerta, confirma fornecedor, recebe
estoque ou movimenta dinheiro. Não determina culpa do motorista/fornecedor.
Requer tela futura para gráficos e uso com o serviço real. Testes locais usam
dados descartáveis. A combinação com o backend B13 do usuário será validada
pelo aplicador antes de gravar; não foi executada integralmente neste ambiente.

Arquivos novos: biblioteca e teste `dockagenda/utilization*.go`, handlers e testes
`localapi/dock_utilization*.go`, contrato OpenAPI e este registro. Modifica apenas
o ponto de montagem em `dock_insights.go` e, se existirem, dois mapas de rotas.
