# Frontend70 — recuperação financeira preservada

Esta entrega acrescenta quatro testes de regressão e corrige o cliente financeiro116 existente. Não acrescenta confirmação bancária nem pagamento automático.

O comando é copiado e validado antes de aguardar o Web Lock: alterações feitas pelo chamador durante a espera não mudam seus identificadores, origem ou referência. Ao recuperar um recibo ou descartar um comando não enviado, o cliente só limpa uma pendência equivalente à consultada. Uma pendência substituída permanece preservada. Falha na remoção do armazenamento gera erro e permite consultar novamente o mesmo recibo por GET, sem segundo POST. Reordenação equivalente das propriedades JSON não impede a recuperação.

## Aplicação e convivência

Exige a entrega68 já aplicada, sem reaplicá-la. Inclui a69 se ausente; quando seu marcador e seus sete hashes já conferem, preserva a instalação. Mudanças próprias ainda não registradas das entregas68/69/70 são adicionadas por uma lista explícita. Continuidade e trabalho independente de carrinhos ficam fora do commit. Não faz push, merge, mudança de autenticação/sessão, backend, migração ou banco. O desktop é compilado antes da abertura; o aplicativo pode focar uma instância antiga ainda aberta.

Os fontes completos e o backend de referência do pacote69 servem para consulta e reprodução isolada. O aplicador não os copia indiscriminadamente sobre o projeto nem inicia serviços. As evidências em evidencias_historicas69 são da entrega69; não são execução nova do ciclo70.

## Limites do aceite

Os quatro testes70 usam transporte e armazenamento descartáveis. Nesta reconstrução não foi repetida a demonstração completa compras→recebimento→financeiro no navegador/API/SQLite. A pasta aceite preserva o reproduzível de produção69, identificado como tal. Os testes de cliente, TypeScript e build não substituem login e aceite visual no PC nem homologação comercial. O inventário96 da69 continua aberto; fiscal, PSP/bancos, operação móvel independente e homologação de hardware não são declarados concluídos.
