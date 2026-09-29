# TitanSystem — falhas, decisões e aceite (Fase 1)

**Estado:** proposta verificável. A Fase 1 documenta decisões; não implementa serviços, migrações ou interfaces.

## Comportamento nas quatro falhas

| Falha | O que continua | O que aguarda | Teste exigido na fase de implementação |
|---|---|---|---|
| Internet da loja cai; LAN ativa | PDV no PC e celular com bancos locais; comunicação LAN se coordenador acessível | Fornecedor, contador, licença remota e confirmação de doca | Concluir venda, reiniciar app, conferir transação/outbox e sincronizar uma vez ao reconectar |
| Roteador/LAN cai | Cada dispositivo já pareado opera isolado com seu banco, dentro das políticas locais | Consolidação entre aparelhos, compartilhamento e reserva | Vender em dois aparelhos isolados, reconectar e reconciliar movimentos sem perda nem sobrescrita |
| PC/servidor local cai | Smartphone com banco próprio e autenticação local válida continua vendendo | Funções dependentes de periféricos/serviço do PC | Desligar PC, vender no celular, reiniciar celular e conferir venda persistida antes do envio |
| Energia da loja cai | Aparelhos ainda alimentados podem registrar; nenhum periférico desligado funciona | Operação nos aparelhos sem energia, rede sem nobreak e serviços externos | Interromper energia/encerrar processo durante gravação em ambiente de teste e verificar transação íntegra |

Política offline não autoriza aceitar cartão/PIX sem confirmação real, emitir nota fiscal como autorizada, aprovar limite financeiro sem alçada ou confirmar uma doca sem árbitro acessível. O usuário vê pendências e divergências de forma clara.

## Decisões do produto a confirmar antes das próximas fases

| Decisão | Opções | Recomendação para o primeiro piloto |
|---|---|---|
| Hospedagem dos dados comerciais | Somente local; backup cifrado escolhido pelo cliente; nuvem privada do cliente; armazenamento cifrado no Titan | Local com backup cifrado controlado pelo cliente e relay de mensagens mínimas; oferecer outras modalidades explicitamente |
| Recuperação das chaves | Cliente guarda material; custódia assistida; sem recuperação | Cliente guarda cópia de recuperação testada. Explicar que perda total da chave implica perda do conteúdo |
| Controle de estoque em múltiplos aparelhos sem rede | Bloquear item; cotas locais; permitir com risco e reconciliar | Cota para itens críticos, política explícita para os demais e alerta de divergência; nunca prometer saldo global exato offline |
| Coordenador LAN | Necessário; opcional; nenhum | Opcional; celular mantém banco próprio e autonomia se o PC cair |
| Serviço de docas | Doca operada pelo mercado; fornecedor; operador logístico | Organização que opera a doca define capacidade; árbitro único confirma janelas e publica somente metadados necessários |
| Cobrança/licença sem conexão | Bloquear vendas; período de tolerância; licença permanente | Período de tolerância definido em contrato; venda em andamento não pode ser interrompida |
| Integração fiscal e pagamentos | Construir agora; integrar provedores por fase; declarar indisponível | Delimitar em fase própria, com provedores e requisitos reais; comprovante não fiscal até integração homologada |
| Primeira implantação | Mercado com um fornecedor; rede inteira | Um mercado, um fornecedor e uma doca, depois ampliar com métricas observadas |

Essas recomendações não são autorização para lançar produto, captar dados ou prometer conformidade. O dono do produto escolhe explicitamente os itens antes de congelar o contrato da Fase 1.

## Evidência de aceite da Fase 1

- Cada categoria da matriz de dados possui responsável, armazenamento primário, destinatário autorizado e metadados mínimos visíveis ao Titan.
- Há papéis distintos para mercado, fornecedor, funcionário, contador e operador Titan, com ações permitidas e limites.
- Venda, pedido, produção, reserva e recebimento têm estados, eventos de aprovação e regras para duplicação/concorrência.
- As quatro falhas têm comportamento e teste verificável; o celular não depende do PC para registrar venda.
- O texto distingue arquitetura planejada de código existente, aponta a limitação do `localhost` e não promete nota fiscal ou pagamento confirmado sem integração.
- Decisões do quadro acima são marcadas como `aprovada`, `ajustar` ou `pendente` pelo proprietário do produto antes de iniciar a Fase 2.

## Limites e próximas entregas

Fase 2 constrói a persistência local em banco descartável de teste antes de afetar qualquer instalação. Fase 3 trata identidade e autorização. Fases posteriores fazem venda, celular offline, sync, reposição, doca, produção e contador. A meta de 5–6 meses exige priorização e piloto reduzido; estes documentos não tornam tais funcionalidades prontas.

## Decisão do proprietário — 29/09/2026

Aprovadas todas as recomendações do quadro acima para o primeiro piloto. Mudanças futuras exigem novo registro de decisão.
