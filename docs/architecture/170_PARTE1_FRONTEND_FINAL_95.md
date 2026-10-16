# Parte 1 — pacote acumulado de frontend 95

Esta entrega reúne as interfaces das entregas 86–94 e fecha os dez fluxos de interface enumerados na revisão anterior. Inclui código, instalador com backup, prévia independente sem senha e roteiro de testes. O escopo é frontend; não é uma homologação de pagamentos, emissão fiscal, sincronização móvel ou hardware.

## O que abrir

- `/local/pos`: venda, cliente no primeiro produto, CPF opcional, clube, edição de itens, pagamento e troco após confirmação, suspensão/retomada, sangria/suprimento, aprovações, devoluções, malote e livro de caixa.
- Dentro do PDV, **Pagamentos e documentos**: as novas interfaces integradas. O carrinho continua preservado ao alternar as áreas. No pagamento também há atalhos para cartão/Pix, desconto, clube e nota.
- `/local/cash/register`: fechamento separado, contagem de cédulas e moedas, conferência de comprovantes com câmera quando disponível, revisão das parcelas por venda e encerramento. Inclui acesso ao vínculo com celular.
- Depois do encerramento confirmado: relatório consolidado, impressão/PDF pelo navegador, CSV e arquivo local por empresa/loja/aparelho/usuário. Reabrir a área Relatório permite consultar os fechamentos arquivados.

## Matriz dos dez fluxos

| Fluxo | Interface incluída | Conexão operacional |
| --- | --- | --- |
| Cartão/Pix | Forma e parcela, valor, QR recebido do provedor, copia e cola, validade, espera, aprovação, recusa, cancelamento, resultado incerto e consulta da mesma operação | Novo adaptador tipado; sem PSP conectado por padrão |
| Fiado/crediário | Cliente, consulta de limite utilizado/disponível, parcelas com vencimentos e centavos exatos, solicitação, aprovação/recusa, consulta e cancelamento | Novo adaptador; sem cadastro financeiro fictício no modo real |
| Fiscal | Seleção de venda registrada, CPF, solicitação de emissão, rejeição, contingência, consulta, cancelamento e segunda via PDF fornecida pelo serviço | Novo adaptador; não fabrica autorização nem DANFE |
| Clube | Consulta por CPF válido, cadastro com consentimento, saldo, solicitação de crédito e resgate de pontos | Novo adaptador; não aplica abatimento sem confirmação conjunta da venda |
| Comprovante de venda | Prévia de itens, quantidades, meios de pagamento, total, recebido/troco quando conhecidos, impressão/PDF e consulta para reimpressão | Usa o registro de venda da API já existente; rotulado **não fiscal** |
| Desconto | Bruto, desconto e líquido, motivo, solicitação, resposta e credenciais do supervisor sem persistir a senha | Aprovações locais anteriores preservadas; autorização do serviço separada e não aplicada silenciosamente |
| Reembolso | Venda original, motivo, meio, solicitação, aprovação, parcial/concluído/recusado/incerto, total devolvido/pendente e comprovante | Novo adaptador; preserva a venda original e não presume estorno real |
| Relatório de fechamento | Esperado/contado/diferença, meios, comprovantes, pendências, malote e cadeia de conferência, impressão/PDF, CSV e arquivo | Esperado e diferença reais vêm da resposta do fechamento existente; campos ausentes ficam **—** |
| Celular/câmera | Código temporário, caixa da sessão, vínculo, contato, reconexão, progresso e painel adaptável ao celular para conferir/enviar campos do comprovante | Novo adaptador; não cria rede ou aplicativo móvel por conta própria |
| Equipamentos | Listar/selecionar, conexão, largura de papel, orientação, impressão de teste, gaveta, falha e nova consulta | Novo adaptador; requer conector nativo autorizado no desktop |

Sem conector, a tela informa serviço não conectado e impede executar a operação. A prévia simula os dez fluxos com dados fictícios e aviso permanente. Isso permite revisar o frontend inteiro sem confundir exemplos com transações comerciais.

## Integridade e recuperação

- O cliente grava uma intenção com ID antes de enviar uma operação. Reconsulta usa o mesmo ID; resposta perdida não gera nova cobrança automaticamente.
- Empresa, loja, aparelho e usuário delimitam os metadados locais. Demonstração e uso real usam espaços separados.
- Intenções guardam apenas domínio, referência, valor e estado: não guardam senha, CPF, QR, imagem de comprovante, cartão ou credencial de provedor.
- Atualizações têm comparação de versão para evitar sobrescrever resposta concorrente. Até 100 operações são mantidas; pendências não são removidas para abrir espaço. Ao atingir o limite só operações terminadas podem sair da janela.
- O fechamento confirmado é arquivado com valores imutáveis. Repetição igual é idempotente; valores diferentes para o mesmo caixa são rejeitados. O arquivo local comporta 100 fechamentos; lotação exige arquivamento no backend antes de continuar salvando novos resumos.
- Cobrança, crediário ou desconto em aberto/aprovado bloqueiam a confirmação simplificada em dinheiro daquele carrinho. A venda não é convertida silenciosamente de cartão/Pix para dinheiro.
- Consulta automática de pendências: a cada cinco segundos enquanto a página estiver visível, sem sobreposição de chamadas; tempo máximo da solicitação: 15 segundos. Resultado incerto exige consulta explícita. Operações recentes oferecem Recuperar operação mesmo depois de recarregar a tela; pendências antigas não somem da lista por ultrapassar a janela das dez últimas operações. A recuperação consulta o ID anterior sem transformar o carrinho novo na venda antiga.
- Formulários avisam antes de sair com edição não enviada. Câmeras param ao sair; imagens não são armazenadas por esta entrega.
- Permissões reais continuam obrigatórias no backend. Configuração de equipamentos, vínculo administrativo e autorização de reembolso/desconto exigem perfil com gerenciamento do caixa. Fiscal segue a permissão já existente do projeto.

## Contratos que precisam ser conectados no backend

`Gateway95` em `src/core/local/service95.ts` é um contrato de cliente, não uma lista de URLs inventadas. Por padrão `features=[]`. O código demonstrativo fica somente em `browser95`, fora da aplicação instalada.

Todos os serviços devem derivar empresa, loja, usuário, aparelho, módulos e autorização da sessão, validar o contexto do caixa e rejeitar alterações de conteúdo para o mesmo identificador. Nunca confiar em valores e permissões enviados pela tela.

| Domínio | Comandos e informações necessários |
| --- | --- |
| payments | Criar cobrança/parcela de uma venda ou cotação validada; consultar por ID idempotente; cancelar segundo política do PSP; valor, referência, estado, expiração e QR/código fornecidos pelo provedor |
| credit | Buscar cliente autorizado e limite; aprovar parcelas/vencimentos; consultar e cancelar com geração rastreável do contas a receber |
| fiscal | Configuração por empresa/UF/modelo, emissão/consulta/rejeição/contingência/cancelamento; segunda via PDF gerada pelo serviço; nunca apenas trocar o rótulo da interface |
| loyalty | Consultar/cadastrar com consentimento; saldo; crédito/resgate idempotente vinculado à venda; desfazer pontos em cancelamento |
| discount | Cotação de bruto/desconto/líquido; solicitação; autenticação e permissão do supervisor; aprovação/recusa/cancelamento; aplicação atômica à venda |
| refund | Autorizar a devolução; executar/consultar estorno e parcelas; valor devolvido, pendência, falha e comprovante; movimento de estoque e financeiro rastreáveis |
| devices | Lista autorizada de impressoras/terminais/gavetas, configuração por estação, testes e diagnósticos sem acesso arbitrário ao sistema operacional |
| pairing | Criar código temporário vinculado ao caixa; associar aplicativo autenticado; consultar/reconectar/revogar; receber os campos revisados e ID único do comprovante; deduplicação no servidor |
| report | Demonstrativo oficial por sessão, abertura, vendas por meio, suprimentos, sangrias, devoluções, liquidação/pendências, conferente, malote, histórico e arquivo compartilhado |

O fechamento e a confirmação da venda precisam coordenar esses serviços no backend: consulta do estoque, composição inteira de pagamento, autorização de desconto, fiscal e idempotência. Esta entrega preserva a API existente, que confirma venda em dinheiro; não envia cartão/Pix como se fossem dinheiro. Não há código de servidor neste pacote.

## Como testar

### Prévia isolada, sem senha

Execute `python3 -B ver_previa.py` na pasta extraída. Ela abre uma porta local livre (preferência 3095), sem depender de outra API ou alterar seus dados.

1. Digite **123** para adicionar o produto demonstrativo de R$ 30. No primeiro produto, veja CPF/clube e escolha continuar sem identificar.
2. Vá ao pagamento, informe R$ 100 em dinheiro e confirme. Confira itens, recebido e **troco de R$ 70 após confirmação**. Imprima o comprovante não fiscal.
3. Inicie outra venda e abra Pagamentos e documentos. No cenário Aprovado, inicie o Pix, veja o QR não pagável e consulte o resultado. Não efetue nenhum pagamento: o QR contém somente texto de demonstração.
4. Cenário Resposta perdida: inicie uma cobrança e consulte **a mesma operação**. O botão de nova cobrança deve permanecer bloqueado enquanto o resultado for incerto.
5. Crediário: informe cliente, confira limite e distribua R$ 30 em três parcelas. Revise vencimentos e total.
6. Fiscal: selecione a venda registrada. Teste Rejeitado, solicite contingência e consulte. Em outra venda/contexto, teste emissão e segunda via; o PDF demonstrativo diz **sem validade fiscal**.
7. Clube: na prévia use CPF de teste válido (ex.: 52998224725); consulte 100 pontos, solicite resgate de 20 e confira 80. Cadastro requer nome e consentimento. Não use dados pessoais reais na prévia.
8. Desconto: solicite R$ 5 numa venda de R$ 30; veja líquido R$ 25. Preencha credenciais **fictícias** de supervisor e autorize. A senha desaparece e não é gravada.
9. Reembolso: escolha uma venda, solicite R$ 20 com motivo, escolha Reembolso parcial e autorize. Veja devolvido R$ 10/pendente R$ 10; consulte no cenário Aprovado para concluir e imprima a confirmação demonstrativa.
10. Equipamentos: procure a impressora, selecione-a e teste o cenário Falha; depois retorne a Aprovado e teste novamente. Nenhum hardware é acionado pela prévia.
11. Celular: crie vínculo, consulte para conectar, veja código e contato. Abra Conferência no celular, digite 123456, conecte, revise número/valor/data/meio e envie. Veja o progresso. Câmera exige permissão e dispositivo real; use os campos manuais quando indisponível.
12. Relatório: carregue o demonstrativo, imprima/PDF e baixe CSV. No fluxo de caixa normal, conte cédulas/moedas antes dos comprovantes. Após fechar oficialmente pela API simulada da prévia, confira arquivo local e reabertura.
13. Repita com tema claro/escuro, idiomas pt-BR/en/es/fr/zh-CN, teclado, janela estreita/tablet e zoom de 100%. Os textos novos têm chaves nos cinco idiomas; traduções en/es/fr/zh-CN exigem revisão humana nativa antes de produção.

### Instalação

Na raiz `~/Downloads/zzeus-frontend`, execute `bash "$HOME/Downloads/TitanSystem_Frontend_Parte1_Final_95/aplicar.sh"`. O instalador aceita as bases conhecidas 86–94, faz conferência antes de escrever, preserva customizações reconhecidas de rotas/menu/PDV, cria backup privado e executa testes/build. Conteúdo desconhecido é preservado e bloqueia a substituição; não force hashes. Nenhum push, instalação de dependência, migração ou alteração de banco é feito. Com alterações prévias, o índice é preservado e não há commit automático.

Para iniciar o frontend em outra porta: `cd "$HOME/Downloads/zzeus-frontend/frontend-web"` e `npm run dev -- --host 127.0.0.1 --port 3002`. A API local existente precisa continuar iniciada para o acesso real; a prévia independente dispensa login/API.

## Validação e limites

Testes de regras monetárias/parcelas, contexto, retorno de provedor, estados e traduções; IndexedDB real de teste com reinício, comparação de versão, arquivo imutável e ausência de segredos; interfaces reais React em DOM virtual com HTTP simulado; regressão da venda/troco/contagem/malote/fechamento. Instalador exercitado contra as bases conhecidas, personalizações e repetição idempotente. TypeScript e build Vite verificados.

Não foi demonstrado uso de câmera física, impressora, gaveta, maquininha, Pix real, certificado fiscal, SEFAZ, comunicação remota com celular ou restauração entre computadores. OCR nativo não existe em todos os navegadores: a conferência manual continua disponível. O módulo de planilhas já existente mantém um bundle grande, avisado pelo Vite; esta entrega não adiciona biblioteca pesada, OCR externo ou novo runtime de planilhas.

Lista exata de fontes no arquivo `ARQUIVOS_ALTERADOS.txt`. Resumo dos resultados efetivamente executados em `VALIDACAO.txt` na raiz do pacote. Nenhuma chave nova de tradução ficou ausente.
