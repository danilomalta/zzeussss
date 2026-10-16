# Operação — frontend90

## O que foi implementado

Rota lazy `/local/operation?area=...`, com sete áreas autorizadas. Menu e atalho no PDV levam à central, preservando as rotas integradas anteriores. Não há alteração no backend, instalação de dependências, banco comercial ou push.

| Área | Fluxos disponíveis | Limite real |
|---|---|---|
| PDV | Busca no catálogo real, teclas de produtos consultados, inclusão manual, quantidades, pagamentos divididos, suspensão/retomada, revisão, desconto e cancelamento com motivo; modo de tela cheia com rolagem interna e sem histórico de vendas | Preparação persistente. A venda oficial em dinheiro continua em `/local/pos`. A nova preparação não cobra, não reserva e não baixa estoque |
| Caixa | Consulta paginada de vendas e respectivos recibos oficiais; seleção explícita para conferência; resumo por forma de pagamento; abertura/suprimento/sangria locais; contagem por cédulas/moedas; registro de comprovante, sugestões por valor/forma/horário/referência, vínculo único; malote com código e cadeia de entrega; CSV/planilha | Resumo só das vendas consultadas e selecionadas, nunca presumido como turno completo. Encerramento oficial permanece em `/local/cash/register`. Comprovante não é liquidação confirmada |
| Fiscal | Preparação de destinatário, CPF/CNPJ, modelo55/65, ambiente, natureza, endereço, itens e códigos NCM/CFOP/CST-CSOSN; pagamentos; revisão; solicitações fiscais; prévia de impressão; ligação com XML89 e configurações | Códigos e dígitos verificadores são apenas validação de entrada. Sem cálculo tributário oficial, assinatura, autorização, DANFE autorizado ou transmissão. Eventos são pedidos locais, sem protocolo |
| Estoque | Contagem multilinha por local/lote/validade, saldo-base informado, entradas posteriores, diferença, motivo, envio, recontagem e aprovação local por responsável; transferência preparada; exportações | Não consulta saldo oficial por lote/local e não aplica ajuste. Ocultar saldo na tabela não é mecanismo de segurança do servidor. Planejamento não equivale a inventário oficial |
| Compras | Itens, prazo, pagamento, política acordada de troca, aprovação, registro de confirmação do fornecedor, checklist e motorista antes da saída, agenda de doca com conflito, recebimento parcial e saldo pendente, coleta/troca até registro de retorno ao fornecedor | Confirmação do fornecedor é informada manualmente, não enviada entre empresas. Sem geração automática de nota, crédito, cobrança, entrada de estoque ou mensagem ao motorista |
| Perdas | Quantidade, custo, preço, validade, vendas do período e comparação de igual duração; cobertura; proposta de desconto/troca/perda; aprovação; autorização explícita abaixo do custo; aviso na recompra do mesmo SKU; gráfico acessível e exportação | Dados informados, sem inferir que baixo volume é prejuízo. Não aplica preço, perda ou ação ao fornecedor; custo não presume despesas não informadas |
| Produtos/preços | Consulta real do catálogo, cópia explícita para preparação, novos produtos, edição, embalagem/conversão, mínimo/alvo, variação, seleção múltipla e prévia de desconto com piso no custo, confirmação local, etiquetas textuais e impressão | Alterações não atualizam catálogo oficial. Etiquetas são prévia com código em texto, sem gerar barras. Balança, impressora de etiquetas e publicação de preços ainda exigem integração |

Trocas/devoluções de venda e livro de caixa existentes84 permanecem acessíveis pela conferência, sem duplicar nem apagar os registros anteriores. Não foi alterada a implementação desses fluxos. Não foi prometida equivalência de todas as operações ao backend comercial.

## Câmera e comprovantes

- Solicitação explícita de câmera; sem áudio, gravação ou persistência de imagens.
- Leitura opcional pelo detector nativo de texto, somente se o navegador o disponibilizar. Não foi adicionado OCR pesado ou IA.
- Onde não houver detector (inclusive navegadores sem essa API), o botão de leitura fica indisponível e existe preenchimento manual/texto para revisão.
- Nenhuma leitura preenche e confirma pagamentos automaticamente; é necessário revisar e registrar referência, adquirente, terminal, forma, valor e data/hora.
- Falta homologar leitura automática confiável no Firefox, aplicativo móvel e desktop. Portanto essa parte não deve ser anunciada como OCR universal concluído.

## Persistência e autorização

- IndexedDB `titan-operation90`, chave por empresa, loja, identidade e aparelho autenticados. Não armazena token, senha, certificado ou imagem de comprovante.
- Rascunhos salvos explicitamente; aviso ao sair com formulários/contagem não salvos. Valores pesquisados no catálogo ficam na memória; só a cópia solicitada é persistida.
- Revisão esperada dentro da transação de leitura+gravação: duas janelas não sobrescrevem uma à outra silenciosamente. Erro mantém a versão anterior.
- Dinheiro em centavos, quantidades em milésimos. Limites:4MB por espaço,1000 registros por coleção,200 linhas por venda/pedido,30 pagamentos; tabelas exibem até200 linhas.
- Permissões existentes e licença ativa. Caixa requer pos/sell; estoque/produtos inventory/manage_stock; compras orders/view_orders; fiscal owner/fiscal; perdas responsável ou gerente autorizado ao estoque. Não há usuário universal nem concessão de privilégios.
- Revisões e histórico90 são locais, não auditoria imutável do servidor. Dados não se tornam compartilhados entre usuários só por estarem na mesma loja.
- O backup89 cobre somente89; não inclui automaticamente os novos registros90. Backup integral e restauração conjunta permanecem pendentes.

## Contratos de backend necessários — sem inventar endpoints

1. Venda: comando idempotente e estado consultável para pagamentos divididos, suspensão, desconto com aprovação, cancelamento e devolução. Saldo/reserva, pagamento e fiscal separados.
2. Caixa: consulta completa por turno/operador/período, grupos por forma, movimentos, conferência, diferenças, fechamento, malote e custódia autorizada; distinguir observado de confirmado pelo provedor.
3. Fiscal: cadastro tributário, cálculo/prévia, certificado/segredos no serviço seguro, comandos e eventos idempotentes, autorização/rejeição, contingência e impressão do documento legal correspondente.
4. Inventário: saldo e revisão por produto/local/lote, contagem cega entregue com permissão apropriada, movimentos posteriores assinados/versionados, revisão e aplicação atômica; transferências.
5. Compras: fornecedor/condições, cotação, pedido e aceite das partes, reserva de doca com controle de concorrência, conferência de carga e recebimento, divergências, coleta, retorno, crédito ou reembolso aprovado.
6. Perdas: dados comparáveis por período, lote/validade, custo e ações aprovadas; perdas e descontos auditados e ligação com a revisão da recompra.
7. Catálogo: versão esperada, lote de alterações com validação e resultado por linha, conversões, variantes, políticas mínimo/alvo, preços e etiquetas/hardware.

Interfaces `Command90`, `CommandResult90` e `CommandAdapter90` estão tipadas. Adaptador de comandos retorna indisponível até existir contrato documentado. As consultas existentes continuam usando o cliente local real. A fonte de saldo por lote/local retorna indisponível, sem fabricar zero ou buscar outro ambiente.

## Verificação e aceite

Testes de regras, persistência em IndexedDB simulado, concorrência, contextos, DOM virtual das sete áreas e cinco idiomas; TypeScript/Vite. Roteiro de teste manual acompanha o pacote. Não houve navegador visual real, câmera, impressora, maquininha, aparelho móvel ou execução comercial do backend. Esses testes continuam necessários antes de chamar o conjunto de Operacional.

Novos textos têm chaves correspondentes nos cinco idiomas; traduções estrangeiras requerem revisão nativa. A aplicação mantém a identidade e temas atuais; não inclui nova paleta azul/dourada. Nova rota lazy, sem nova dependência de produção. O editor de Planilhas anterior permanece pesado e não foi modificado nesta fase.
