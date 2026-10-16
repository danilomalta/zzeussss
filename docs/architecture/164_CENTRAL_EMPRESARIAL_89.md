# Central empresarial — entrega89

## Escopo entregue e limite operacional

Esta entrega cobre o frontend solicitado e incorpora as correções87/88. O instalador aceita as bases conhecidas86,87 e88 sem substituir personalizações não reconhecidas. A existência de uma tela ou um rascunho salvo não concede o selo Operacional.

| Pedido | Interface e comportamento entregues | Integração oficial ainda necessária |
|---|---|---|
| Menu | PDV primeiro e com largura total; selos repetidos discretos; estado disponível para leitor de tela/legenda; nome de exibição ou empresa vinculada | API com nomes oficiais e fontes de evidência operacional |
| Fundo | Foto sem desfoque ou mínimo; preferência antiga preservada; reduzir efeitos desliga filtro; vidro permanece | Preferências entre aparelhos |
| Fiscal | Prévia de XML NF-e/NFC-e, confirmação explícita, registro de metadados, filtros e exportação; protocolo é informação do arquivo, sem validar autenticidade | Assinatura/autorização, A1/CSC seguros, emissão, rejeição, eventos, cancelamento, inutilização e contingência por jurisdição |
| PDV | Frente de caixa existente preservada; nova conferência somente de leitura de catálogo, caixa, vendas e até3 recibos | Demonstrar venda ponta a ponta, reinício/restauração, pagamentos adicionais, fiscal, hardware e sincronização |
| Financeiro | Títulos a pagar/receber, vencimentos, baixas parciais, centros de custo, fluxo por data e resultado gerencial em caixa; extrato OFX com prévia e conciliação individual | Contas, liquidações e conciliação oficiais, bancos/boletos/Pix, competência contábil e auditoria central |
| Integrações | Cadastro local de preparação com nome, provedor e ambiente; catálogo de categorias e diagnóstico local existente | Autorização segura do provedor, conexão, credenciais, webhooks e estado real; rascunho não é conexão |
| Estoque e Compras | Consulta real do catálogo; contagem com saldo informado, diferença e conferência local; acesso direto ao estoque/movimentos e Compras85, que mantém fornecedor, cotação, pedido, aprovação, recebimento e XML | Consulta oficial de saldos por local, envio de contagem para aprovação e ajuste, entrada XML autorizada e movimentos transacionais |
| Pessoas e acessos | Leitura do cadastro real; nome/perfil/status; acesso direto à criação/revogação existente | Permissões granulares/auditoria central que não possuem consulta integrada nesta entrega |
| Produção/PCP | Visão da capacidade e ordens do planejamento de Receitas86; quadro reservado/em produção/concluído; vínculo direto para cadastrar/reservar/consumir no fluxo existente | Ordens compartilhadas por setor, programação de recursos, consumo e lotes oficiais |
| CRM e promoções | Reuso de clientes/funil do Comercial85; campanhas com piso pelo custo informado; regras de cashback, mínimo e validade; simulador exato | Aplicar campanha no PDV, saldo de fidelidade por cliente, resgate, antifraude e comunicação autorizada |
| E-commerce/omnichannel | Pedidos de canal cadastrados manualmente; revisão, aprovação, separação, envio, entrega/cancelamento; referência externa única por canal | Captura/retorno ao canal, catálogo publicado, pagamento, reserva de estoque e frete |
| Multiempresa/filiais | Preparação do cadastro de filiais; contexto autenticado continua intocado; botão de troca oficial fica indisponível | Listar contextos autorizados, criar unidade e trocar sessão/contexto pelo servidor |
| Tarefas/notificações | Responsável, prazo, prioridade, área, etapas e alertas reais derivados dos prazos locais | Eventos compartilhados de notas/caixa/estoque, notificações móveis e atribuição colaborativa |
| Backup/privacidade | Backup cifrado dos dados89; prévia antes da restauração;10 versões locais; exclusão dos rascunhos e versões desta central | Backup integral do banco comercial e dos outros módulos, restauração validada, políticas de retenção e solicitações LGPD no servidor |

## Fiscal em São Paulo

O SAT não é oferecido como nova emissão: a vedação de CF-e-SAT a partir de01/01/2026 e sua substituição por NF-e/NFC-e foram verificadas no Comunicado SRE06/2025 e na RC32089/2025, da SEFAZ-SP. As regras de outras UFs e operações continuam dependendo de mapeamento fiscal. A importação de XML da interface não comprova assinatura, protocolo autêntico, enquadramento tributário nem autorização.

- https://legislacao.fazenda.sp.gov.br/Paginas/Comunicado-SRE-6-de-2025.aspx
- https://legislacao.fazenda.sp.gov.br/Paginas/RC32089_2025.aspx

## Contratos a implementar no backend — não endpoints inventados

1. Contexto: nomes oficiais, unidades autorizadas, criação de unidade, troca autorizada de sessão; escopo de empresa/loja e revogação de acesso em toda operação.
2. Fiscal: configuração não secreta e armazenamento protegido de credenciais; documento/evento com estados próprios, prévia validada, comando idempotente, consulta de autorização e auditoria; certificado nunca no IndexedDB.
3. Financeiro: título, parcela, centro de custo, liquidação parcial, estorno e conciliação por linha; eventos da venda/compra; relatórios por caixa e competência identificados; importação OFX idempotente por conta/identificador.
4. Conectores: catálogo de provedores, iniciar autorização, callback/webhook verificado, status e última conferência; segredos somente no serviço seguro. Um nome salvo localmente não deve virar status conectado.
5. Inventário: saldo e versão por produto/local/lote, contagem, divergência, autorização e aplicação em transação; importação de XML com prévia e documento autenticado.
6. PCP: ficha/versionamento, saldo/necessidade, ordem, setor, recurso, lote, reserva, consumo, rendimento e perda; atualização entre aparelhos com idempotência e autorização.
7. CRM/canais: clientes autorizados, consentimento, campanha, regra e conta de fidelidade, resgate idempotente; pedido de canal e reserva de estoque no mesmo fluxo oficial de vendas.
8. Pessoas/auditoria: lista paginada de eventos autorizados e permissões por perfil; dados pessoais/salariais não entram em cache permanente como efeito de consulta.
9. Tarefas/eventos: atribuição por usuário/setor, prazo, mudança de etapa, eventos confirmados dos módulos e notificações com recibo.
10. Backup/LGPD: agendamento, status/resultado de restauração, retenção, exportação e exclusão autorizada; diferenciar dados fiscais sujeitos a guarda de preferências apagáveis.

Fontes realmente usadas: clientes locais existentes para catálogo, caixa/vendas/recibo e funcionários; IndexedDB de Receitas86 e Comercial85 para seus próprios rascunhos. Fontes ausentes retornam indisponível. Nenhuma emissão, cobrança, alteração de estoque oficial ou aprovação comercial acontece nesta entrega.

## Persistência, segurança e desempenho

- Novo banco local `titan-enterprise89`, chave pelo contexto completo empresa/loja/usuário/aparelho. Dados pessoais consultados em Funcionários permanecem no estado da tela e não são replicados nesse banco.
- Alterações locais validam estrutura/tipos/limites e revisão esperada. Uma janela não sobrescreve silenciosamente outra. Auditoria89 é atividade do rascunho e não substitui registro imutável no servidor.
- Limite de2,5MB por arquivo e1000 linhas por coleção; tabela mostra até200 linhas e mantém rolagem interna. Catálogo traz uma página e avisa quando parcial.
- Dinheiro é calculado em centavos inteiros; quantidades de inventário usam milésimos. Desconto não pode levar o preço arredondado abaixo do custo informado. Baixa não pode superar o saldo. Conciliação é individual por valor/direção e não reutiliza a mesma baixa.
- Backup89 usa AES-256-GCM, PBKDF2-SHA256 com150000 iterações, sal/IV aleatórios. Senha fica na memória e é limpa após o uso; contexto e conteúdo são validados antes da restauração. O arquivo contém apenas esta central, não backups oficiais do ERP. As dez versões são consultadas e apagadas apenas no próprio contexto.
- Nova área carregada por rota lazy; sem dependência nova em produção. O editor de Planilhas anterior continua sendo a principal pendência de bundle; esta entrega não mede uso de memória em PC antigo.
- Fonte pt-BR e mesmos campos nos cinco idiomas. Traduções estrangeiras precisam de revisão nativa. Prévia isolada usa flag de demonstração; não altera acesso na aplicação real.

## Verificação

Testes de regras monetárias, baixas parciais, conciliação única, parser OFX, piso de campanha, cashback, etapas de pedidos, permissões, validação de arquivo e idiomas. Runtime de IndexedDB com isolamento, concorrência, versões, criptografia, contexto/senha incorretos e exclusão. DOM virtual cobrindo os fluxos, restrição de caixa e troca dos cinco idiomas. Build e diff-check. Demonstração visual em navegador/hardware real e backend comercial não foi executada.
