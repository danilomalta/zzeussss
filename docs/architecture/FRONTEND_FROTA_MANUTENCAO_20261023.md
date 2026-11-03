# Frota, equipamentos e manutenção - frontend

Data: 23/10/2026. Base publicada: `2ed977bd27336a39a0758d5b2fc57d87a7dc4845`, branch de trabalho do frontend `feat/backend-checkout-authority`.

## Escopo e estado real

Extensão da área existente `/local/fleet`, dentro do arquivo de trabalho `business85`. Não é uma oficina comercial independente nem uma implementação do aplicativo móvel. Não há novo endpoint, banco de backend, alteração de Electron ou chamada de rede introduzidos nesta unidade.

O usuário definiu: gerente ou operador abre um pedido na aba **Pedidos**, pelo smartphone. O pedido deve aparecer no monitor da Mecânica e alertar o smartphone do mecânico. O frontend desta unidade prepara o formulário responsivo, a fila e o atendimento. Entrega entre aparelhos, push, som em segundo plano e aplicativo móvel ainda dependem de backend e cliente móvel próprios. O teste de áudio é manual, exclusivamente no aparelho onde o botão é pressionado.

## Acessos

- `/local/orders` ganha um link para **Pedidos de manutenção de frota e máquinas**, quando as permissões existentes permitem.
- `/local/fleet?area=fleet&fleetTab=requests`: abrir pedido e acompanhar histórico.
- `fleetTab=mechanics`: fila da mecânica e teste manual de áudio.
- `fleetTab=equipment`: máquinas, empilhadeiras e outros equipamentos, além da situação consolidada dos veículos existentes.
- `fleetTab=costs`: despesas e parâmetros gerenciais por bem.
- Os veículos, viagens e gastos anteriores continuam na mesma área. A nova interface carrega sob demanda.

Permissões existentes: módulo `logistics` ativo e `view_orders`. Não foi criado papel de mecânico nem permissão de servidor fictícia. O usuário com acesso operacional pode abrir pedido. Gerente/dono autoriza reparo, cancela após autorização, reabre e libera. O avaliador assume o atendimento e registra seu andamento. Dados e lançamentos gerenciais de custos exigem `view_accounting`; alterações de custos e parâmetros também exigem gerente/dono. Sessão e os quatro identificadores são conferidos antes de mutações locais. Autorização real interaparelho continuará sendo responsabilidade do backend.

## Pedido e atendimento

Campos: bem, problema, local informado, prioridade normal/urgente/emergência, impossibilidade de uso, operação afetada e referência opcional de viagem/pedido. A referência é textual: não comprova vínculo oficial nem modifica a entrega.

Fluxo: aberto -> avaliado -> reparo autorizado -> em reparo -> aguardando conferência -> liberado. Durante reparo pode aguardar peça/terceiro e depois retomar. Cancelamento e reabertura têm controle próprio. Todas as ações exigem diagnóstico/justificativa e mantêm sequência local de revisão, ator, horário e histórico.

Somente a revisão esperada pode ser modificada. Isso protege ações obsoletas dentro do arquivo carregado; não é consenso entre abas/aparelhos. A persistência legada business85 ainda não tem coordenação transacional entre abas. Não vender essa proteção como idempotência de backend ou resolução de conflitos distribuídos.

Pedidos bloqueantes e estados de reparo/peças/conferência impedem o despacho no planejamento local da frota. Um pedido não bloqueante sinaliza avaliação pendente. Liberação de uma ocorrência não apaga outra ocorrência bloqueante nem a manutenção previamente marcada no veículo. Nenhuma viagem já em andamento é cancelada automaticamente.

## Equipamentos e custos

Máquinas e empilhadeiras podem ser cadastradas sem placa, usando patrimônio/referência, local e horímetro em horas inteiras. Próxima revisão informada gera indicação visual ao atingir o horímetro; zero significa não informada. Não há leitura de sensor, agendamento automático ou telemetria.

Gastos: combustível, peças, mão de obra, pneus, seguro, documentação, pedágio e outros. Quantias são registradas em centavos inteiros, com recusa de estouro e casas decimais excedentes. Gastos legados de cada veículo entram uma vez na soma; o operador é avisado para não relançá-los na nova lista. Não existe deduplicação global de comprovantes ou reconhecimento automático do mesmo gasto em listas diferentes.

Parâmetros por bem: aquisição, residual estimado, vida útil em meses, meses considerados, receita atribuída manualmente, quilômetros/horas considerados, horas paradas e impacto estimado da parada. A atualização substitui os parâmetros gerenciais daquele bem; não cria lançamento contábil.

Depreciação acumulada = piso((aquisição - residual) * meses considerados / vida útil), em centavos. Não é depreciação fiscal nem cotação de revenda. O resultado parcial considera somente receita manual menos gastos conhecidos e depreciação gerencial. Impacto estimado da parada aparece separado, sem segunda dedução. Parâmetros ausentes não viram lucro zero; divisão por zero não gera custo/km ou custo/h. Indicadores por km/h têm apresentação aproximada. Totais abrangem o arquivo local inteiro, sem filtro de período.

CSV próprio de custos em centavos, identificado como rascunho, usando o exportador existente com proteção contra fórmulas em textos. Nenhuma baixa financeira, compra de peças, entrada de estoque ou comprovação fiscal é feita.

## Persistência e compatibilidade

O campo opcional `Workspace85.maintenance` mantém compatibilidade com arquivos antigos. A base IndexedDB e sua chave de escopo existentes são reutilizadas, sem aumentar sua versão nem apagar arquivos. Formulários usam `forms.fleetMaintenance`. Escopo inclui empresa, loja, usuário e aparelho; por isso outro usuário/aparelho NÃO vê essa fila por sincronização nesta etapa.

Dados novos inválidos bloqueiam ações da manutenção e mostram erro, sem substituir o registro. Limites: 500 equipamentos, pedidos, custos e perfis; 200 eventos por pedido; limite total de tamanho do workspace legado preservado. Auditoria local é editável pelo dono dos dados e não constitui prova inviolável.

Continuam valendo as limitações do arquivo legado: sem coautoria confiável entre abas, sem acesso compartilhado entre identidades e sem integração oficial de frota. O botão preexistente de carregar demonstração substitui o workspace após confirmação; não deve ser usado sobre registros que se pretende manter.

## Verificações

- 43 testes novos: estados, autorizações, revisão obsoleta, despesas, cálculos, corrupção, serialização e cinco idiomas.
- Suíte disponível de frontend: 1.566 testes aprovados, zero falhas, zero ignorados, Node 24.19.0.
- TypeScript completo aprovado na base materializada do commit indicado, com alterações desta unidade.
- Renderização SSR das quatro abas nos cinco idiomas: 20 composições com dados descartáveis, sem escrita ou backend. SSR não equivale a teste visual ou de cliques.
- Build Vite completo aprovado; ver tempos e limitações no registro de continuidade. Dependências preexistentes reutilizadas; nenhum pacote adicionado.
- Não executados: ensaio interativo em navegador, teste físico de smartphone/áudio, reinício real de navegador com IndexedDB, tráfego entre aparelhos ou integração com serviço real.
- CI remoto e instalação no computador do usuário não são comprovados por esta entrega.

## Próxima integração com backend/móvel

Definir contrato real para cadastro/leitura de bens, abertura de chamado, destinatários autorizados, atribuição, orçamento e aprovação, peças/serviços, revisão concorrente, recibos idempotentes, histórico e eventos duráveis. Propostas de nomes de operações não devem ser anunciadas como rotas implementadas.

Separar: pedido gravado, evento disponível, enviado, recebido no aparelho e lido/assumido pelo mecânico. Prever reenvio e ausência de conectividade sem duplicar chamado. Som depende das permissões e condições do sistema móvel, sem garantia com app fechado.

Ainda faltam neste frontend: prestadores/orçamentos completos, anexos, guincho/substituição, edição ampla de equipamento e medidores, preventivas por calendário e quilometragem, filtro por período dos custos, documentos/seguros, vínculo oficial com viagens/produção, custos realizados auditados e integração financeira. Não declarar o módulo operacional ou todo o frontend concluído.


## Integração local revisada

Aplicado à base3013a4f em23/10/2026, preservando docas/recebimentos e o registro único. Os números e limites de validação na seção acima descrevem a preparação recebida. Evidência ATUAL:43testes da unidade,1662da suíte completa, TypeScript/build aprovados;27casos SSR incluindo erro de limite e separação km/h; circuito local de6eventos e reabertura/persistência comprovados na prévia descartável. Detalhes, tempos e pendências estão em CONTINUIDADE_TITANSYSTEM.md revisão22.

Corrigidos resumo de custos fora do limite exato (aviso sem queda da aba ou zero fictício) e unidades da próxima revisão na tabela mista (veículo km, equipamento h). Idiomas e temas percorridos na interface de equipamentos; perfil financeiro restrito e separação de arquivos entre usuários conferidos. Isso não comprova integração oficial.

Contratos do complemento Backend Conferência Técnica foram recebidos, mas o CHECK recusou aplicação por ausência do predecessor Frota/Manutenção e a árvore atual possui sequência posterior de migrações. Não definir rotas novas por suposição nem copiar testdata como predecessor. Este frontend continua local, sem chamadas a esses contratos; ausência de envio, checklist oficial, push e recebimento em smartphone deve continuar explícita.
