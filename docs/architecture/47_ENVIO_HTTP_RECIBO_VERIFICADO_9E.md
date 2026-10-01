# Fase 9E — Tentativa de envio HTTP e recibo verificado

## Entrega

`incoming.PostSealedHTTP` envia um envelope cifrado e só retorna sucesso depois
de validar a prova do recibo com o verificador configurado para aquele evento e
destinatário. Um status HTTP 200/201 não é confirmação suficiente.

Esta função não consulta ou altera o banco, não lê automaticamente a fila,
não confirma eventos, não registra tentativas e não inicia worker/listener.
Integração ao comando e ao serviço de outbox será uma etapa posterior. A origem
deve buscar eventos com `outgoing.Pending`, adquirir chaves aprovadas usando 9C,
assinar/cifrar e, após o recibo verificado, chamar `outgoing.Confirm`. Essa última
função também verifica a prova e a identidade do evento antes da transação de
confirmação. O teste demonstra o encadeamento em bancos descartáveis.

## Transporte

- Uma tentativa por chamada, com timeout total de 10 segundos e contexto
  cancelável. Não executar na thread responsável pela interface do PDV.
- HTTPS com certificado validado para comunicação em rede. HTTP é aceito apenas
  com IP literal de loopback, por exemplo 127.0.0.1, para uso local/testes.
- Endereço exato `/sync/v1/events`, sem credenciais na URL, consulta ou fragmento.
- Redirecionamentos são recusados, sem reenviar conteúdo a um destino diferente.
- Cookie jar desativado nesta chamada; o transporte não usa a sessão humana do PDV.
- Cliente HTTP configurado deve ser confiável. Transporte padrão com
  `InsecureSkipVerify` é rejeitado; transportes customizados são responsabilidade
  de configuração do processo e não devem vir de campos de requisição.
- Corpo cifrado limitado ao contrato da 9D; resposta JSON limitada a 8 KiB.
  Conteúdo comprimido, campos duplicados/extra/nulos e JSON adicional são recusados.
- 201 exige `repeated=false`; 200 exige `repeated=true`. Em ambos os casos,
  assinatura, IDs e SHA-256 devem corresponder ao evento enviado.

Nenhuma mensagem ou erro do servidor é copiado para o erro público da função.
Falhas de rede retornam erro genérico e o evento permanece pendente porque esta
função não modifica a fila. URLs de destino e configurações TLS precisam ser
administradas fora de requisições livres; isso não oferece descoberta de aparelhos.

## Recuperação e confiança

Uma resposta perdida após persistência é resolvida repetindo o MESMO evento e
operação, com nova cifra. O destino retorna o recibo estável sem nova inserção.
Falha de envio não autoriza apagar evento ou criar outro ID de operação.
Worker futuro deverá implementar espera progressiva e persistir as tentativas.

Antes de cada envio, o processo deve validar novamente os pareamentos, aprovação
da chave pública e escopo. Não manter indefinidamente uma chave/verificador
capturado antes de revogação. O cliente HTTP não consulta o estado de revogação;
o receptor revalida sua autorização no banco. Rotação durante comunicação exige
tratamento na integração posterior, sem confiar em chaves anunciadas pelo relay.

Recepção ainda significa armazenamento de mensagem, não aplicação de venda ao
estoque central, confirmação fiscal, pagamento ou recebimento físico de mercadoria.
O protocolo continua restrito à mesma empresa e loja. Comunicação comercial entre
empresas e clientes de smartphone independentes continuam pendentes.

## Verificação

Testes com bancos descartáveis e servidores temporários em loopback incluem:
troca HTTPS usando chave X25519 aprovada; recibo autenticado e confirmação explícita
da fila; resposta forjada/malformada/excessiva; status incoerente; preservação da
pendência; bloqueio de redirect e endpoint inseguro; TLS sem validação recusado;
cancelamento e recuperação após resposta perdida.

O teste usa uma CA/certificado de `httptest`, configurado apenas no cliente de
teste. Não desativa a verificação TLS. Não instalar certificados de teste em produção.
Resultados Go devem ser observados no PC antes de declarar esta fase validada.
