> Integração desta árvore: base local58 preservada, Frota em0059 e Conferência Técnica em0060. A adaptação está documentada em BACKEND_FROTA_BASE58_20261023.md. Os números/comandos do pacote de origem abaixo são históricos; não reaplicar o consolidado antigo nem fabricar marcadores para fazê-lo aceitar esta árvore.

# Conferência técnica e tempo declarado da manutenção —23/10/2026

Complemento incremental ao Backend Frota e Manutenção de23/10/2026. Trata de manutenção INTERNA de frota e máquinas. Não cria oficina comercial, aplicativo móvel, pagamento, estoque de peças ou folha. Preserva as rotas e os chamados anteriores.

## Entrega

23 operações HTTP adicionais, 19 caminhos, contrato em `docs/api/fleet-workshop.openapi.json`. Modelos de checklist, exigência por chamado, inspeções imutáveis, horas declaradas pelo usuário autenticado, revisão gerencial, histórico, recibos, totais por chamado e CSV. Estado e autorização sempre no servidor.

Nova SQL `fleet_workshop.sql`, número0049 na base recuperada ou0058 na base B02B13 com antecessores. Tabelas STRICT, índices por contexto/chamado/usuário, unicidade por operação e por revisão. Sem alteração de SQL antiga, sem triggers ou mudanças no executor de migrações.

## Fluxo técnico

1. Gerente cria modelo com nome, nota e1..50 itens. Cada item tem ID único, texto e marcação de obrigatório. Modelo não é editado: crie outro para nova versão. Desativar impede novas vinculações, preservando os modelos já copiados para chamados existentes.
2. Gerente exige esse checklist em um chamado aberto, em triagem ou aprovado. `id` da exigência é o próprio request_id. A exigência copia integralmente o modelo, registra revisão e revisão do chamado conferida. Pode substituir antes do serviço, mediante revisão exata e novo motivo. Não pode remover ou trocar durante serviço/espera/inspeção.
3. Técnico atribuído ou gerente registra a inspeção com todos os itens exatamente uma vez. Respostas pass/fail/not_applicable. Falha ou não aplicável exigem justificativa; item obrigatório não aplicável resulta reprovado. Qualquer falha, mesmo em item opcional, reprova. Itens opcionais não aplicáveis com motivo são permitidos.
4. Backend calcula passou/reprovou; cliente não envia aprovação, ator ou estado. Inspeções são imutáveis. Correção é uma nova inspeção, preservando a anterior. Sempre vale a última por sequência durável do servidor, não o ID escolhido pelo cliente ou o relógio do celular.
5. `inspect` e `release` do chamado original verificam a exigência e a inspeção na MESMA transação da mudança. Sem inspeção, com reprovação ou evidência incompatível, retornam409 e preservam bloqueio.
6. Novo início/retomada (`start`), reatribuição (`assign`) ou reabertura (`reopen`) invalida inspeção anterior. A inspeção é vinculada à revisão dessa etapa e à versão da exigência. Notas e a própria passagem para inspeção não apagam evidência válida. Nova falha informada em inspeção posterior bloqueia a liberação, mesmo depois de já entrar em inspection.
7. Só gerente libera. `passed` é resultado técnico declarado; não libera ativo nem substitui aprovação gerencial. Outras falhas e bloqueios da frota continuam valendo.

Chamados antigos SEM exigência continuam com o fluxo anterior. A configuração é decisão humana. Se já existiu exigência e seu registro sumiu ou divergiu, a ausência NÃO é interpretada como dispensa. Inspeção apagada ou alterada não faz o servidor recorrer silenciosamente a uma aprovação anterior. Inconsistência retorna409 para investigação.

Cancelamento/clear_hold continuam administrativos conforme pacote anterior: não se tornaram aprovação técnica ou reembolso. Não usar cancelamento para fingir inspeção concluída. Histórico identifica a decisão gerencial.

## Trabalho e horas

Usuário registra intervalo UTC de segundos inteiros, descrição e revisão do chamado. Servidor deriva trabalhador da sessão, duração exata e estado declared. Intervalo deve ser positivo, até16horas, com término não futuro. Aceita declaração retroativa; não prova presença, início real do serviço ou ponto. Gerente não registra em nome de outro usuário por este endpoint.

Somente responsável atribuído ou gerente pode declarar em chamado in_service/waiting_parts/inspection. Intervalos declarados ou confirmados do mesmo trabalhador não podem se sobrepor, inclusive em outro chamado da loja. Intervalos contíguos são permitidos. Usuários distintos podem trabalhar simultaneamente.

Gerente confirma ou rejeita uma declaração. Não é obrigatoriamente outro usuário: decisão e autor ficam identificados. Autor pode anular sua declaração ainda não revisada enquanto o chamado está em trabalho; gerente pode anular registro declarado/confirmado/rejeitado. Nada é apagado. Rejeitados/anulados não impedem nova declaração corrigida; os totais continuam separados para não contabilizar duas vezes.

Não calcula salário, adicional, jornada legal, presença comprovada, folha ou pagamento. Não converte automaticamente segundos em despesa: custos de mão de obra continuam no lançamento gerencial de despesas do pacote anterior. Nenhum preço é revelado por estas consultas técnicas.

## Permissões

Todas as rotas: sessão atual, empresa/loja e aparelho aprovado, membro ativo, view_orders. Escritas: contrato assinado ativo de Logistics e dependências existentes. Modelo/exigência e revisão de horas exigem owner/manager. Inspeção e declaração de trabalho exigem responsável atribuído ou gerente. O corpo não escolhe tenant/store/actor/device/worker/state/passed/seconds.

Não cria novo papel mechanic nem novo login. Smartphone e monitor usam os mesmos contratos quando integrados a uma sessão e serviço autorizados. Código de interface, som, push e offline móvel não estão neste complemento.

## Contratos e validação

Mesmo envelope do Fleet: `operation_id`, `id`, `expected_revision`, `action`, `reason`, `data`, todos obrigatórios. Máximo24576bytes/corpo,16384bytes/data. Objetos aninhados com chaves extras/duplicadas/ausentes e nulos indevidos são recusados. Não converter inteiros em floats. IDs até128bytes sem controles/barra/espaços nas extremidades. Motivo até1000bytes e não vazio.

| Recurso | Criação data | Ação e data |
|---|---|---|
| /fleet/workshop/templates | CheckTemplate: name,items[{id,label,required}],note | disable com{}; não editar modelo |
| /fleet/workshop/requirements | BindingInput: request_id,expected_request_revision,template_id | replace com BindingInput completo antes do serviço |
| /fleet/workshop/inspections | InspectionInput: request_id,expected_request_revision,expected_binding_revision,answers[{item_id,outcome,note}],note | Nenhuma; registro imutável |
| /fleet/workshop/labor | LaborInput: request_id,expected_request_revision,started_at,ended_at,description | confirm/reject/void com{} |

Criação: action=create, expected_revision=0. Ações: id igual ao caminho, revisão atual>=1. Nota geral da inspeção obrigatória; nome/texto dos itens obrigatórios; nota do modelo pode ser vazia. Booleano required deve constar até quando false. Todos os campos especificados devem estar presentes.

Listagem GET no recurso: request_id/state opcionais não vazios; limit1..100 padrão50; offset0..1000000; ordem por ID, total do filtro inteiro. GET/{id}, GET/{id}/history, POST/{id}/actions. GET não aceita corpo, query duplicada/desconhecida. Relatório JSON/CSV: GET /fleet/requests/{id}/workshop[.csv], sem query. CSV é resumo por chamado, não exportação de todos os apontamentos; células protegidas contra fórmulas.

## Transação, revisão e recuperação

Operação, recurso e outbox `fleet.workshop.changed` são gravados juntos. Escrita ignorada, conflito ou falha abortam tudo. Revisão otimista impede duas revisões gerenciais incompatíveis. Conferência de estado/revisão do chamado e de permissões ocorre na mesma transação.

Chaves de operação não podem colidir entre a API Fleet anterior e workshop no mesmo tenant/store/device. A guarda é transacional e preserva a regra anterior; não depende de trigger. IDs são gerados antes de enviar, não trocados depois de um resultado incerto.

Recuperação deste complemento: GET /fleet/workshop/operations/{operation_id}, vinculada ao usuário/aparelho originais, mantendo permissões atuais. Não confundir com /fleet/operations/{id}, que recupera as operações antigas de ativos/chamados. Mesmo corpo JSON canônico e operation_id devolve fotografia original com repeated=true. Corpo diferente409. Arrays preservam a ordem do envio para idempotência; resposta da inspeção apresenta os itens na ordem do modelo. Guarde corpo original para retry.

Histórico: fotografias originais em ordem de revisão, ação e motivo. Verificação de integridade confere revisões contíguas, chaves e recibo atual; histórico valida cada recibo da página. Não é assinatura digital qualificada nem garantia contra um administrador que adultera coerentemente todo o banco; protege falhas/divergências de persistência detectáveis. Não descartar evidência conflitante.

Outbox contém só tipo/ID/chamado/revisão/estado, sem descrição do trabalho, respostas dos itens ou valores. Ainda não existe consumidor push novo. Avisos de chamados do pacote anterior continuam funcionando; alterações técnicas não são apresentadas como push recebido no smartphone.

## Resumo e limites

Mostra revisão atual do chamado, exigência, última inspeção, se satisfaz a exigência atual, segundos declared/confirmed/rejected/voided separados em strings decimais e origem máxima consultada. Inspection_satisfied não autoriza usuário, não declara que o gerente liberou e pode mudar após nova atividade/inspeção.

Limite1000 registros técnicos por chamado para resumo; até1000 inspeções/chamado para a guarda; até1000 eventos do chamado para determinar a etapa; até1000 registros de trabalho do mesmo usuário/loja para verificar sobreposição. Excesso retorna422 com max_source_records1000, sem truncagem ou liberação automática. Listas/históricos continuam paginados. Grandes históricos exigem agregação/verificação incremental futura. Não confundir com limite5000 de outros relatórios Fleet.

## Instalação e aceite

Exige pacote Frota e Manutenção de23/10/2026, com todos os antecessores. Aplicador valida o marcador e hashes exatos, cadeia de SQL e caminhos. Nova SQL49/58, testes de migração/snapshot atualizados sem reescrever história. Verifica candidata descartável antes de aplicar, cria backup privado, recusa conflito, reconhece repetição. Não inicia serviço, não abre banco comercial, não altera frontend/Electron nem faz commit/push.

Testes de aceitação: ciclo técnico com checklist reprovado e correção; último resultado vence por sequência, não por ID; reatribuição/retomada invalida; modelo desativado preserva cópia já exigida; usuário não atribuído negado; revisão gerencial; intervalos sobrepostos rejeitados; reenvio/concorrência; rollback; evidência ausente/divergente bloqueia; fonte acima do limite falha; backup/restauração e migração repetida preservam dados.

## Pendências separadas

Integração das telas, cinco idiomas/temas e validação real com aparelhos; push/som e comprovante de entrega do provedor; fotos/anexos/antivírus; assinatura técnica qualificada; peças reservadas/consumidas no almoxarifado; contas/pagamentos/fiscal/folha; sincronização entre bancos móveis; preventiva recorrente; sensores/GPS; pilotos Windows/Linux e configurações reais B13. Não são marcados como entregues aqui.
