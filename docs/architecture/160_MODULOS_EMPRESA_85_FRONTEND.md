# Entrega85 — Logística, Alimentação, Documentos, Compras e Comercial

## Escopo entregue

Frontend React/TypeScript, sem biblioteca nova em produção. Cinco rotas protegidas pela sessão existente e pelas capacidades já recebidas: `/local/fleet`, `/local/food`, `/local/documents`, `/local/purchasing`, `/local/commercial`. Os cinco cartões do menu deixam “Em breve” apenas para os perfis com módulo e permissão correspondentes. O selo informa arquivo local, sem declarar operação comercial integrada.

| Área | Fluxos disponíveis |
| --- | --- |
| Logística e frota | Cadastro de veículo/placa/tipo/capacidade declarada/motorista, disponibilidade e manutenção, odômetro, despesas de combustível/manutenção, viagens com várias paradas, endereço/janela/previsão, conferência da carga, saída, chegada, comprovante ou ocorrência por parada e conclusão. Veículo em viagem não entra em manutenção e não sai em outra viagem simultânea. |
| Alimentação e delivery | Mesas/comandas/balcão/entrega, itens do cardápio e disponibilidade, quantidade e observação de cozinha, fila por etapa, tempo decorrido, desconto com responsável, pagamentos declarados divididos, preparo/pronto/saída/conclusão/cancelamento com motivo, histórico. Pagamento declarado é separado da confirmação bancária. |
| Documentos | Pastas, busca, marcadores, vínculo textual com cadastro/pedido, validade, revisão/aprovação/arquivo/reabertura, upload local com versões preservadas, download e seleção de versão anterior. O conteúdo do arquivo também persiste em IndexedDB no contexto correto. Não é armazenamento oficial, criptografia, compartilhamento externo nem assinatura digital. |
| Compras | Cadastro de fornecedores e condições, disponibilidade, cotações com SKU/unidade/quantidade/preço/frete/prazo/validade, pedidos com vários itens, aprovação humana, envio preparado e resposta do fornecedor declarada com referência, recebimento e justificativa para diferenças. Importação local de XML NF-e com prévia e confirmação, sem escrituração fiscal. |
| Clientes e comercial | Cliente pessoa/empresa, contatos/endereço/segmento/autorização declarada de contato, ativação/inativação, histórico, funil contato–qualificado–proposta–aceito/não avançou, orçamento com itens/desconto/frete/validade/responsável/próximo contato, aceite declarado antes da conversão em pedido local, metas por mês de criação e pedidos aceitos. |

## Ligações entre áreas

- Uma comanda de entrega pode preparar uma viagem na frota, com destinatário e endereço copiados. Exige também permissão de frota, veículo disponível e responsável. A viagem nasce como rascunho: ainda exige conferência e saída.
- Um pedido comercial aceito pode preparar viagem com o endereço do cliente. Repetição não cria duas viagens para o mesmo pedido.
- Compras mantém link para a central de agenda/recebimento anteriormente entregue em operações84. Não há cópia automática de estoque entre os rascunhos85 e84.
- Frota mantém o link para o mapa já existente. O endereço não é geocodificado automaticamente; não há GPS real, cálculo de rota, trânsito ou ETA automático nesta entrega.
- Todas as áreas exportam CSV diretamente e usam a passagem temporária para o módulo de planilhas existente, restrita ao responsável. A prévia isolada não contém o editor completo.
- Documentos possuem vínculo de referência textual. Não há acesso cruzado a arquivos de outro contexto nem vínculo automático com documentos oficiais de outro módulo.

## Estado e persistência

Os registros85 são arquivos de trabalho locais por empresa, loja, usuário e aparelho. Autosave indica conclusão após commit no IndexedDB, apresenta erro e permite tentar novamente. Há proteção contra saída com alterações não salvas. Exemplos só são carregados após confirmação e permanecem identificados. O histórico local registra ação, referência, data e usuário declarado; não é auditoria de servidor à prova de adulteração.

Arquivos aceitos: PDF, PNG, JPEG e TXT, até2MB por versão e16MB de conteúdo referenciado por arquivo de trabalho. A interface não renderiza HTML ativo. Metadados e bytes são armazenados separadamente; uma interrupção entre salvar o arquivo e registrar seus metadados pode deixar bytes órfãos. Limpeza/retensão, criptografia e arquivo oficial pertencem à próxima etapa do backend. Não armazenar documentos oficiais como única cópia. Uploads não são sincronizados ou compartilhados com outro computador.

Dinheiro é calculado em centavos, quantidades em milésimos inteiros, com multiplicação/limites usando BigInt e apresentação com Intl. Capacidade, odômetro e datas são declarados pelo usuário. Não há validação fiscal de CPF/CNPJ, habilitação de motorista, documento legal ou assinatura do XML. Datas de vencimento e relógio do dispositivo não substituem o horário oficial de servidor.

O XML aceito tem até1MB/200itens e unidades un/kg/g/L. Quantidades admitem até3 casas decimais significativas e preços até2; zeros excedentes podem ser removidos sem arredondar o valor. Precisões/unidades não suportadas são recusadas para revisão manual. DOCTYPE/ENTITY são recusados; o parser não busca recursos externos. Reimportação da mesma chave não duplica o pedido local. A chave não recebe validação criptográfica ou autorização da SEFAZ.

## Permissões

| Área | Capacidade existente exigida |
| --- | --- |
| Frota | Licença ativa, módulo logistics e view_orders. |
| Alimentação | Licença ativa, módulo pos e sell. Não inventa um módulo food no contrato existente. |
| Compras/comercial | Licença ativa, módulo orders e view_orders. Alterações são apenas rascunhos; futuras escritas reais precisarão de permissões específicas. |
| Documentos | Responsável/gerente e capacidade autorizada de pedidos, RH ou contabilidade. Não está disponível a caixa comum. |
| Aprovações | Responsável/gerente; backend deverá verificar novamente quando integrado. |

Nenhum perfil, contrato, permissão ou banco de produção é alterado. O frontend não é a barreira de segurança. A prévia tem um seletor de perfis exclusivamente de demonstração, que não é instalado no sistema.

## Adaptadores e contratos de backend necessários

Um adaptador tipado por fonte em `core/business85/adapters`. Por padrão retorna indisponível, sem URL inventada; exemplo somente com flag explícita. Valida o contexto informado contra as capacidades recebidas. A interface local trabalha com seu próprio arquivo de rascunho até a integração de leitura/escrita real.

| Fonte/comando | Contrato necessário |
| --- | --- |
| Veículos e frota | Listar/cadastrar/atualizar veículos, motorista, documentos, disponibilidade, odômetro e despesas; permissões específicas, concorrência e auditoria. |
| Viagens e paradas | Planejar rota e janela, validar capacidade/documentos, reservar veículo, conferir carga, registrar saída/chegada/ocorrência/prova, receber atualização do entregador e fechar. Idempotência e anexos privados. |
| Rastreamento | Pareamento e autorização do aparelho, localização autorizada durante a atividade, consentimento/política de retenção, estado sem sinal e última evidência; mapas, geocodificação e cálculo de rota mediante provedor configurado. |
| Alimentação | Cardápio vinculado ao catálogo, mesas/comandas/itens/complementos, fila da cozinha, impressora, pedido externo/iFood, entregador, pagamentos e fiscal em estados independentes; reenvio idempotente. |
| Documentos | Metadados, upload privado, antimalware, armazenamento/criptografia, versões, permissão por documento/pasta, compartilhamento revogável, revisão/assinatura e retenção/exclusão controladas. |
| Compras | Fornecedores, cotação comparável por unidade/quantidade, pedido, aprovação, resposta real do fornecedor, condição de pagamento, agenda, recebimento e divergência; estoque e financeiro transacionais. |
| XML | Validação fiscal e assinatura quando aplicável, prevenção de duplicidade, mapeamento de produto/unidade/precisão, tributos, manifestação e escrituração separados da prévia de itens. |
| Comercial | Clientes e autorizações de contato, oportunidades, orçamento/revisão/aceite, pedido idempotente, preço/desconto/estoque, metas e comissões oficiais com regras por perfil. |
| Dados comuns | Contexto derivado da sessão, paginação/busca, versão de atualização, cache limitado, outbox/inbox, documentos e eventos de auditoria. Não usar última gravação vence para operações comerciais críticas. |

## Pendências explícitas

Não foram implementados pagamento/TEF/Pix real, iFood real, impressão de cozinha/etiqueta, emissão fiscal, localização ao vivo, otimização de rotas, comunicação real com fornecedor/motorista, baixa de estoque, contas financeiras, comissão paga, assinatura digital ou compartilhamento externo. Não são anúncios de operação homologada. Os fluxos anteriores de PDV, caixa, mapa, RH, planilhas, configurações e saída da sessão são preservados.

## Idiomas, peso e verificação

pt-BR/en/es/fr/zh-CN têm o mesmo conjunto de chaves; chaves literais dos componentes são verificadas por teste. Traduções estrangeiras precisam de revisão por falante nativo. Nomes e endereços de exemplo são dados fictícios, não texto da interface. Não foram adicionadas dependências de produção. Páginas e áreas usam lazy loading; câmera/mapa/editor não são carregados por este pacote sem abrir seus módulos. Mantém o aviso pré-existente de tamanho do editor de planilhas. Não foi medido desempenho em hardware antigo.

Verificações: 33 testes Node selecionados (19 novos), 11 testes do instalador, TypeScript/build Vite e fluxos em DOM virtual. Testes adicionais exercitam XML, precisão/unidades/DTD, IndexedDB e isolamento dos bytes de documentos por contexto, adaptadores sem endpoint e flag de exemplos. Não houve revisão visual em navegador real, celular, impressora, GPS ou API comercial.

Roteiro: carregar exemplos explicitamente; conferir carga e entregar todas as paradas; testar despesa/odômetro; comanda→cozinha→pronto→pagamentos declarados→conclusão; vincular entrega à frota; enviar documento e baixar/restaurar versões; compra→aprovação→envio preparado→aceite declarado→recebimento; XML→prévia→confirmação; cliente→proposta→aceite→pedido→viagem; CSV/planilhas; recarregar após salvo; verificar licença/perfil, cinco idiomas, temas, teclado/ESC, zoom50–150%, tablet/celular no equipamento de destino.
