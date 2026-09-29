# TitanSystem — fluxo mercado–fornecedor e estados (Fase 1)

**Estado:** contrato de comportamento para implementação posterior. Nenhum estado abaixo representa uma integração já operacional.

## Reposição até recebimento

1. Venda confirmada localmente gera movimento de estoque. Uma contagem da **gôndola** ou previsão baseada em venda/saldo pode gerar sugestão; o sistema distingue gôndola, depósito e mercadoria reservada.
2. Sugestão mostra produto, quantidade, fornecedor vinculado, regra e dados usados. Gerente/comprador pode aprovar, editar com motivo ou rejeitar. **Nunca** cria pedido vinculante nem cobrança sem aprovação humana.
3. Pedido aprovado usa identificador estável, é registrado localmente e enviado uma vez logicamente, apesar de possíveis reenvios físicos. Fornecedor aceita, rejeita ou contrapropõe quantidade/prazo. Mudança exige nova confirmação do mercado conforme política.
4. Fornecedor prepara/produz, informa lote, quantidade disponível e previsão. Pedido e expedição mantêm seus históricos em cada empresa.
5. Solicitação de doca informa local, janela, tipo de veículo e capacidade necessários. O serviço da agenda **confirma atomicamente** uma vaga ou oferece alternativa; uma sugestão offline não é reserva confirmada.
6. Na entrega, mercado confere quantidades, lotes e divergências. Somente o efetivamente aceito gera entrada de estoque e obrigação financeira correspondente. Divergência e devolução são eventos auditados.
7. Financeiro vincula pedido, entrega aceita, documento e condições. Relatório do contador deriva de operações reais e concessões explícitas; não declara emissão fiscal sem autorização efetiva.

## Estados e transições permitidas

| Entidade | Estados propostos | Regra decisiva |
|---|---|---|
| Venda | rascunho → concluída_local → pendente_sync → recebida_remoto; cancelada por compensação | Só `concluída_local` afeta caixa/estoque; falha na gravação reverte tudo |
| Sugestão | proposta → aprovada / rejeitada / expirada | Proposta não é pedido e não sai para fornecedor |
| Pedido | aprovado_local → envio_pendente → recebido_fornecedor → aceito / contraproposto / rejeitado → em_preparação → expedido → recebido_parcial / recebido_total → encerrado | Aprovação, aceitação e mudança são eventos distintos com autor e horário |
| Reserva de doca | solicitada → confirmada / alternativa / rejeitada / expirada → chegada → ocupada → concluída; cancelamento auditado | Só serviço que controla capacidade marca `confirmada`; idempotência e concorrência são obrigatórias |
| Recebimento | aguardando → conferência → parcial / completo / recusado → fechado | Saldo aumenta apenas pela quantidade conferida e aceita |
| Produção | planejada → em_etapa → inspeção → pronta / retrabalho / descarte → expedida | Etapa, perdas, qualidade e lote têm evidência e autor |
| Ponto | marcado_local → pendente_sync → registrado / divergente → corrigido_por_aprovação | Horário do aparelho e recebimento são registrados separadamente; correção não apaga marcação original |

Os nomes são de negócio; IDs, enums e esquema concretos ficam para a implementação. Não converter erro de rede automaticamente em `rejeitado` ou `confirmado`.

## Identificadores e concorrência

- Comando, venda, pedido e reserva têm IDs gerados no cliente e chaves de idempotência estáveis.
- Uma doca possui capacidade e janelas controladas pelo mesmo árbitro transacional. Duas empresas concorrentes não recebem a mesma vaga se a capacidade permitir apenas uma.
- Estoque por local é resultado de movimentos imutáveis. Atualização entre aparelhos offline requer reconciliação, não sobrescrever o saldo de outro aparelho.
- Envio ao fornecedor revela somente dados do pedido aprovado e dados de entrega necessários, nunca a lista completa de clientes ou vendas.
- Fuso horário, horário local, início/fim da janela, duração, tolerância de atraso, remarcação e cancelamento exigem parâmetros do operador da doca.

## Exemplo de aceite futuro, sem simulação enganosa

Um teste integrado cria venda, reduz saldo em movimento local, gera sugestão, exige clique de aprovação, entrega uma única mensagem ao fornecedor apesar de reenvio, confirma uma janela sem sobreposição, registra entrega parcial e só então aumenta o estoque aceito. O relatório do contador mostra origem de cada valor. Até esse teste existir, a interface deve rotular cada parte como `planejada` ou `indisponível`.
