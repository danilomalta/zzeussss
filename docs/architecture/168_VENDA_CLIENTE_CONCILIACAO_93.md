# Venda e conferência93

## Comportamento entregue
- Primeiro produto abre identificação opcional com CPF e pergunta sobre clube.
  Continuar sem identificar sempre disponível. CPF inválido impede salvar o campo,
  não impede vender sem CPF. A pergunta não reaparece a cada item.
- CPF mascarado no resumo, edição disponível. Preferência de clube é intenção,
  não confirmação de cadastro, saldo ou pontos. Novo cadastro abre solicitação91.
- CPF da venda em edição fica em memória, não em localStorage/IndexedDB ou logs.
  É limpo ao cancelar, esvaziar ou concluir a venda. Preenche o campo fiscal quando
  o operador solicita CPF na etapa de pagamento, sem fingir emissão fiscal.
- Fechamento real ampliado, denominação e rótulo em linhas separadas, botões neutros.
- Após confirmar contagem, resumo automático por dinheiro/débito/crédito/Pix,
  vendas, comprovantes associados e valores sem comprovação. Fonte real92 consulta
  histórico/recibos da sessão, atualiza ao abrir e a cada60s com a página visível.
- Papel com valor diferente pode ser associado MANUALMENTE à venda do mesmo meio
  e com horário próximo, após confirmação. Venda315 / papel219 deixa96pendente.
  Não altera venda, não confirma pagamento nem dispensa revisão responsável.
- Anular papel reabre a diferença. Duplicidade de NSU e de parcela segue protegida
  pelo mecanismo92. Valores não atribuídos ficam separados das vendas.

## Limites e contratos ausentes
O cliente de API existente valida somente venda e recibo em dinheiro. Esta entrega
não altera essa regra e não cria endpoints imaginários. Fontes simuladas existem
apenas na prévia isolada. Cartão/Pix/divisão no envio oficial, clube integrado,
CPF na nota e emissão fiscal permanecem dependentes do backend apropriado.

Dinheiro de vendas NÃO é saldo esperado de gaveta. Fundo de abertura, sangrias,
suprimentos, devoluções e ajustes precisam de extrato oficial da sessão. O atual
contrato retorna apenas sessão/abertura temporal e o cálculo oficial após fechar.
Não foi usado dinheiro de vendas como se fosse esse saldo. O fechamento continua
enviando a contagem ao cálculo oficial existente e recuperando operação incerta.

Necessários no backend: extrato/resumo de caixa com todos os movimentos e totais
por forma; parcelas idempotentes de pagamentos por venda/responsável/aparelho;
conciliação e comprovação separadas de liquidação; cliente/CPF e consulta/admissão
do clube com autorização; CPF/CNPJ no fluxo fiscal; persistência da conferência
e das aprovações por empresa/loja/turno. Definir os endpoints reais no backend.

Leitura automática de comprovante92 depende de TextDetector; Firefox pode exigir
transcrição. Não há OCR portátil nem pareamento câmera celular→PC nesta entrega.
Não foram adicionados SDK pesado, processamento contínuo de vídeo ou dados de cartão.

## Verificação
Regressões de venda/fechamento; cálculo315/219/96, parcelas, anulação e contexto;
DOM virtual do PDV real com primeiro produto, CPF inválido/válido, próximo cliente,
dinheiro/troco e fechamento com a venda da prévia. TypeScript e build.
Verificação visual em navegador real e câmera física continuam necessárias.

Roteiro: primeiro produto → CPF opcional/clube → continuar → segundo item sem
repetir pergunta → pagamento → compra confirmada/troco → próxima venda sem CPF
anterior → Fechar caixa → contar cédulas/moedas → resumo automático das vendas →
conferir comprovantes → revisar → fechar. Testar5idiomas, dois temas e responsividade.
Chaves novas completas nos5idiomas; en/es/fr/zh-CN precisam de revisão nativa.
