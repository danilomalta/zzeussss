> Integração desta árvore: base local58 preservada, Frota em0059 e Conferência Técnica em0060. A adaptação está documentada em BACKEND_FROTA_BASE58_20261023.md. Os números/comandos do pacote de origem abaixo são históricos; não reaplicar o consolidado antigo nem fabricar marcadores para fazê-lo aceitar esta árvore.

# Frota e manutenção interna — 23/10/2026

Entrega de fontes do backend Go/SQLite. Não altera frontend, Electron, bancos operacionais, autenticação central nem contratos de pagamento. Nenhum serviço ou banco comercial é iniciado pelo pacote. Não é um módulo de oficina vendendo serviços ao público.

## Escopo implementado

- Cadastro e revisão de caminhões, carros, motos, vans, máquinas, empilhadeiras e outros ativos; referência única por empresa/loja (comparação NOCASE do SQLite, ASCII).
- Estado administrativo disponível/bloqueado/retirado; reativar deixa bloqueado até liberação explícita. Histórico preservado, sem exclusão física.
- Hodômetro/horímetro em milésimos inteiros; equipamento sem medidor exige zero. Redução de leitura exige ação gerencial `correct_meter` com motivo. A base km/hora/nenhum é imutável.
- Viagem planejada, saída após conferência declarada da carga, retorno com leitura e cancelamento do planejamento. Um único planejamento/viagem em andamento por ativo; nova falha impede a saída mesmo se o planejamento foi feito antes dela.
- A leitura inicial da viagem usa o maior valor entre leitura manual e retornos anteriores comprovados. Retornos não alteram silenciosamente o cadastro do ativo. Uma correção de leitura menor não apaga a continuidade já comprovada das viagens. Substituição física de medidor com reinício exige futura modelagem própria; não forçar o contador para contornar a continuidade.
- Pedido de manutenção criado na aba de pedidos por gerente ou operador autorizado, inclusive cliente smartphone: problema, ativo, local, urgência, impacto declarado e referência. Usuário/dispositivo vêm da sessão; corpo não pode escolher outro ator.
- Fila filtrável, triagem, responsável, orçamento, aprovação, início, espera por peça, inspeção, liberação, cancelamento, retenção de segurança e reabertura. Motivo e revisão em cada ação.
- Prestadores ativos/inativos; orçamento imutável de peças/mão de obra/outros. Orçamento de valor zero é permitido com nota explícita e aprovação; não significa pagamento ou dispensa tácita.
- Planos preventivos por data ou leitura. Consulta mostra vencidos; não gera manutenção, compra ou autorização automaticamente. Atualizar/reativar um plano é ação gerencial.
- Despesas atribuídas ao ativo por combustível, peças, mão de obra, pneus, seguro, documentos, pedágio ou outros. Receitas atribuídas de frete/serviço/outros e impactos estimados de parada/atraso/outros separados. Lançamentos gerenciais imutáveis, anuláveis com histórico.
- Perfil de aquisição, residual, vida útil em meses e início de uso; relatório por período com depreciação linear e valor contábil estimado. Não é avaliação de mercado.
- Relatório JSON/CSV de despesas, receitas atribuídas, impacto estimado separado, depreciação do período e contribuição parcial. Totais exatos em strings decimais, inclusive acima do limite seguro de JavaScript. Razão exata para custo por km/hora; denominador zero indica indisponível, nunca dividir por zero.
- Avisos duráveis por revisão do chamado, cursor e confirmação por usuário/aparelho. Base para painel do mecânico e consulta pelo smartphone; não simula push, som ou recebimento do sistema operacional.
- Consulta de recibo, histórico, idempotência, revisão otimista, transação única, outbox e preservação de evidências.

## Permissões e contratação

Todas as rotas exigem sessão legítima, empresa/loja do serviço, usuário ativo, dispositivo aprovado e `view_orders`. As permissões existentes podem ser concedidas/revogadas pelos mecanismos atuais; não há criação de papel fictício de mecânico nem login especial. Uma conta sem `view_orders` não pode usar o módulo apenas porque possui smartphone.

Ativos, prestadores, planos, criação de viagens, custos e perfis exigem owner/manager. Quotes/expenses/income/impacts/profiles e relatório de custos exigem também `view_accounting`; aprovação de chamado exige essa permissão adicional. Não expor custos à fila operacional. Um mecânico é usuário existente autorizado, atribuído ao chamado; um motorista é usuário autorizado atribuído à viagem.

Operador autorizado pode abrir chamado. Triagem pode assumir chamado aberto. Depois, início/espera/inspeção são do responsável ou gerente. Aprovação, reatribuição, liberação e remoção de retenção são gerenciais. Solicitante pode cancelar seu chamado ainda aberto/em triagem; cancelamento posterior exige gerente e mantém bloqueio (`cancelled_blocked`) até `clear_hold`.

Toda escrita exige contrato assinado ativo do módulo `Logistics` e dependências existentes. Não há autorização inferida do menu. Leituras e recuperação de recibos exigem permissões atuais, mas não renovação do contrato comercial. Revogar sessão/aparelho bloqueia acesso atual; não elimina registros. Não enfraquecer o middleware ou a verificação das assinaturas para uma demonstração.

## Pedido → mecânica

1. Frontend gera `id` e `operation_id` antes do primeiro envio e preserva corpo/contexto.
2. POST `/local/v1/fleet/requests`, com dados validados, persiste chamado, operação, evento, aviso e outbox juntos.
3. Painel/cliente do mecânico consulta `/fleet/notices?after=...&limit=...`; depois consulta o chamado autorizado. Chamados sem atribuição são avisos do estabelecimento; com atribuição, destinatário/solicitante/gerente podem ler o aviso.
4. Cliente pode confirmar o aviso com POST `/fleet/notices/{id}/ack`. Isso comprova a declaração do cliente, não que tocou um som ou que uma pessoa viu a tela.
5. Triagem identifica problema, atribui responsável, gerente registra orçamento e aprova. Técnico executa e declara inspeção. Gerente libera. O pedido não compra peças nem debita estoque/caixa automaticamente.
6. Mudança de sessão/empresa/loja/aparelho invalida autorização do frontend para nova escrita. Resultado incerto exige GET do recibo usando o mesmo contexto original; nunca inventar novo ID para repetir a ação.

## Matriz de ações e `data`

O envelope exige exatamente `operation_id`, `id`, `expected_revision`, `action`, `reason`, `data`. Campos extras, duplicados, nulos indevidos e JSON adicional são rejeitados. `reason` é obrigatório, até1000 bytes UTF8. IDs até128 bytes, sem barra, controles ou espaços nas extremidades. `data` é objeto plano até16384 bytes; corpo total até24576. Inteiros não aceitam números fracionários ou strings.

| Recurso | Criação | Ações posteriores e dados |
|---|---|---|
| assets | Asset, revisão0 | update/correct_meter: Asset completo; block/release/retire/activate: `{}` |
| requests | OpenRequest, revisão0 | triage/assign/approve/start/wait/inspect/release/cancel/clear_hold/reopen/note: sempre `{assigned_to, quote_id}`; ambos vazios exceto o correspondente a assign/approve |
| trips | TripOpen, revisão0 | depart: `{loaded:true,end_meter_milli:null}`; return: `{loaded:false,end_meter_milli:valor}`; cancel: `{loaded:false,end_meter_milli:null}` |
| providers | Provider, revisão0 | update: Provider completo; enable/disable: `{}` |
| plans | Plan, revisão0 | update: Plan completo; disable: `{}` |
| quotes | Quote, revisão0 | Imutável; outra cotação recebe novo ID antes de aprovar. Sem edição/exclusão |
| expenses/income/impacts | Amount, revisão0 | void: `{}`; não editar nem apagar valor original |
| profiles | Profile, revisão0 | update: Profile completo; um perfil por ativo |

Em ações, ID do caminho e do corpo devem coincidir e `expected_revision>=1`. Criação exige `create` e revisão0. Todos os campos dos DTOs devem constar, inclusive strings vazias permitidas. Plan.due_meter_milli e TripAction.end_meter_milli podem ser null; outros campos não podem ser null. Notas de orçamento/perfil e referência de lançamento financeiro devem ser não vazias. `from/to`, due_at e incurred_at exigem RFC3339 com segundos inteiros, UTC normalizado, anos2000..2200. Data de início de uso ISO, anos1900..2200.

## Disponibilidade e concorrência

Disponibilidade é uma consulta operacional, não autorização futura. Saída revalida tudo na transação. Bloqueiam: estado manual não disponível, chamado marcado bloqueador aberto/em triagem/aprovado, chamado em serviço/espera/inspeção/retido mesmo se não marcado inicialmente, viagem planejada/saída.

Cancelar após aprovação não libera automaticamente. Liberar um chamado não libera outros chamados nem remove bloqueio manual. Timeout de aviso/chamado não libera ativo. Consultas e escrita validam recibos/continuidade dos registros operacionais usados; divergência retorna409. Fonte acima do limite retorna422; jamais soma apenas parte e declara disponibilidade.

Revisão otimista evita duas alterações diferentes na mesma revisão. `operation_id` único por empresa/loja/aparelho em todos os recursos deste módulo. Reenvio do mesmo corpo JSON canônico retorna fotografia original com `repeated=true`, mesmo que o registro já tenha evoluído. Mudança de corpo/ator com mesmo ID retorna409. GET `/fleet/operations/{id}` é vinculado ao ator/aparelho originais. Recibo não é confirmação financeira externa.

Operação, recurso, evento, aviso (para chamado) e outbox gravados na mesma transação. Falha ou escrita ignorada por trigger aborta sem efeitos parciais. Outbox `fleet.changed` carrega apenas contexto, tipo, ID, estado e revisão; não inclui custos/notas. Não existe consumidor externo/push novo neste pacote.

## Relatório econômico

Período explícito `[from,to)`, máximo366dias. Despesas/receitas/impactos entram por `incurred_at`; viagens retornadas entram inteiras por `returned_at`, sem rateio entre meses. Usa estado atual dos registros e perfil atual; não é relatório histórico fechado como estava na época. Lançamento anulado não entra nas somas, mas é contado separadamente.

Depreciação acumulada usa meses completos UTC, aniversário ajustado ao último dia quando o mês é menor; limita a aquisição menos residual. Depreciação do período é acumulado(to)-acumulado(from). Contribuição parcial = receita atribuída - despesas - depreciação do período. Impacto estimado permanece à parte para evitar dupla contagem. Não equivale a lucro líquido, fluxo de caixa, receita fiscal, pagamento bancário confirmado ou obrigação a pagar.

Somas usam big.Int e strings exatas. Valores unitários de entrada0..9007199254740991; Amount positivo. Orçamento soma até esse limite. Nenhum float monetário. Denominador da razão é milésimo de km/hora; para reais/km, interpretar razão sem perda, multiplicando numerador por1000 e dividindo por100 e denominador. CSV protege células com início `=+-@` após espaços, inclusive contribuição negativa serializada como texto protegido.

## Paginação e limites

Listas/históricos: limit1..100, padrão50; offset0..1000000; ordenação ID ou revisão. Total é do filtro inteiro, não soma da página. Filtros opcionais vazios, repetidos ou desconhecidos retornam400. GET não aceita corpo.

Fontes de cálculo: máximo5000 registros econômicos (expense/income/impact/profile/trip) por ativo, máximo5000 planos para preventiva, máximo5000 viagens para leitura derivada e máximo5000 registros request/trip para disponibilidade. Limite deliberado e explícito, sem truncagem; grandes históricos precisam evolução para agregação verificada. Listas paginadas continuam disponíveis. Verificação de integridade da lista cobre página retornada, não toda loja.

## Banco, atualização e recuperação

Nova migração somente aditiva `fleet_maintenance.sql`, com tabelas STRICT, chaves compostas, referências, índices e unicidades. É0048 na base de desenvolvimento/produção recuperada ou0057 na base B02B13; nenhuma migração anterior é reescrita ou renumerada. A SQL é executada apenas na próxima inicialização normal autorizada do backend.

O aplicador valida hashes do predecessor Tratativas, toda sequência de migrações, conflitos e estrutura dos pontos de ligação. Cria candidata de fontes descartável, roda testes e vet antes de escrever. `--check` não aplica. Aplicação cria backup privado e marcador/hash; repetição idêntica não altera. Em conflito, preserva o trabalho e interrompe. Não roda frontend ou Electron e não abre banco comercial. Recuperação é pelos fontes do backup com serviço parado e revisão dos caminhos; não substituir banco por backup de fontes.

## Ainda dependem de integração/entrega futura

- Ligar o frontend de rascunhos a estes endpoints e validar sessão/recovery entre aparelhos reais. Não importar `created_by`, permissões ou status aprovado de rascunho como prova.
- Push nativo, som, instalação app/iPhone/Android, recibo de entrega do provedor, fila/retry externo. O backend oferece avisos consultáveis duráveis, não essas integrações.
- GPS/telemetria, mapa/roteirização ao vivo, combustível via cartão/bomba, hodômetro automático e sensores.
- Almoxarifado de peças: reserva/consumo/estorno oficial e compras autorizadas, usando contratos reais; este pacote não inventa movimentos.
- Títulos financeiros/fiscal/adquirente, pagamento a oficina, folha/hora trabalhada real e contratos externos.
- Upload/antivírus de documentos/fotos, checklist técnico estruturado, assinatura técnica e manutenção com medidor substituído.
- Preventiva recorrente/reprogramação automática mediante política aprovada; capacidade/oficina, SLA negociado e garantia de fornecedor.
- Integração Linux/Windows e smartphones reais; a base local de testes é Linux Go1.25. O instalador valida lógica da numeração B13 em testes sintéticos; não prova o binário B13 completo no PC.

Esses itens não são apresentados como concluídos. O relatório do pacote registra exatamente os testes executados e suas limitações.
