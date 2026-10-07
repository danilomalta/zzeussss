# Entrega 14 — Contratos detalhados de caixa e vendas locais

Base: `cfee4fe`. Documentação e testes de oito operações HTTP existentes. Nenhum handler, migração, preço, permissão ou estado comercial é alterado.

`docs/api/cash-sales.openapi.json` documenta turno atual, abertura, fechamento, sangria/suprimento, venda, histórico, registro operacional e cancelamento integral. Campos vêm dos structs reais; descrições refletem os parsers, transações, permissões e contratos POS atuais. O mapa OpenAPI aponta para esse documento.

O teste de contrato lê os schemas e confere o ciclo HTTP em banco temporário: consulta de turno sem saldo, abertura repetida, suprimento/sangria com retry, venda nova/repetida, recibo, histórico cheio/vazio, cancelamento integral/repetido, recibo cancelado, fechamento/repetição, turno vazio e consulta de resultado. Também verifica conservação do dinheiro e ausência de efeitos duplicados. O validador de testes foi ampliado para aceitar nullable e conferir limites de tamanho de arrays; contratos de catálogo/estoque continuam testados.

Regras explícitas:

- Centavos e milésimos são inteiros; preço da venda deriva do catálogo e fica persistido. O cliente não envia preços, ator, empresa nem troco. Pagamentos devem somar exatamente o total.
- Apenas dinheiro é concluído. Pix/cartão não estão integrados; o backend rejeita pagamento não verificado. Documento retornado é operacional, com `fiscal_authorized=false`.
- `Current` é cego: `session` tem apenas ID/data ou é null. Saldo esperado/diferença aparecem no resultado de fechamento. A diferença é declarado menos esperado.
- Abrir/fechar exige `sell`; mover dinheiro exige `manage_cash`; cancelar exige `cancel_sale`. Módulo POS e autorização são verificados na transação para escrita. Leituras autorizadas de registros permanecem após expiração do contrato.
- Cancelamento é integral e local, no aparelho e turno original aberto. Pagamento passa a `reversed`, venda a `cancelled`; estoque/dinheiro são devolvidos com auditoria/outbox. Isso não é estorno bancário ou cancelamento fiscal.
- Retry depende de ID, contexto e payload originais. Reabertura de turno fechado e reenvio de venda cancelada não são equivalentes a retry válido. Não iniciar nova escrita automaticamente após timeout/409/503.
- JSON exato rejeita desconhecidos/duplicados/nulos/ausentes e conteúdo extra. Limites de body: 4096 bytes abertura/fechamento; 8192 movimentos/cancelamento; 128 KiB venda. Venda: 1–500 itens e 1–8 pagamentos.
- No PDV, item `unit` exige milésimos múltiplos de 1000; item pesável usa arredondamento exato por linha. Origem do item deve ser `shelf`.

Não implementado por esta entrega: leitura de comprovantes no smartphone, integração de maquininha, fechamento inteligente, fiscal, devolução parcial, descontos, Pix/cartão, impressão física ou SOS. Esses recursos continuam no roteiro funcional.

Validação exigida: checker de documentos/referências, teste de contrato, API local e bibliotecas caixa/venda, vet e race direcionado. Todos os bancos desses testes são temporários; nenhum banco real ou porta é utilizado. Integração da branch de produção continua separada.
