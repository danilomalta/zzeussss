# Frontend108 — Revisão final do fechamento

## Fluxo

Contagem de dinheiro107 → comprovantes/integração/conciliacão existentes → revisão final108 → solicitação ao encerramento existente → resultado oficial do caixa. Apenas a última tela de conferência foi redesenhada; o callback onClose(amount) mantém o contrato anterior. O valor enviado é dinheiro contado em centavos, nunca o total bruto de vendas.

Quatro destaques mostram dinheiro contado, vendas carregadas, total quando a consulta está completa e quantidade de comprovantes a revisar. A tabela separa dinheiro, débito, crédito, Pix e crediário, com registrado nas vendas, comprovante vinculado, confirmado pela integração e sem comprovação. Comprovante e integração para a mesma parcela não são somados duas vezes. Uma consulta incompleta mostra traço para totais desconhecidos, sem apresentar zero. Abertura/sangrias/suprimentos permanecem no cálculo oficial; não se inventa saldo de gaveta a partir das vendas.

Checklist: contagem confirmada, consulta completa/incompleta, pendências, gravação e liberação conforme gate97. O gate existente permanece a autoridade local: falta de contagem bloqueia; origem inválida bloqueia; consulta parcial, parcela sem prova ou evidência problemática exige responsável, aprovação e justificativa. Essa revisão local não substitui autorização do servidor. O componente não transforma o usuário em gerente nem altera perfis.

Revisão do responsável permite aprovação e justificativa pelo fluxo existente, exigindo salvar antes de solicitar encerramento. Erro de gravação preserva a edição. Campos ainda não registrados de comprovantes também bloqueiam o envio por unsaved do pai. Todos devem ser salvos ou descartados no fluxo adequado antes de prosseguir.

## Confirmação e concorrência

Confirmação final fica associada ao snapshot exato de contexto, revisão do rascunho, contagem, evidências, modo, aprovação, motivo, vendas e completude. Mudar qualquer desses dados exige conferir de novo. Checagem é repetida no clique, antes da confirmação nativa. Trava síncrona impede duplo envio do mesmo clique. O callback existente é void e inicia operação assíncrona no pai; portanto a interface mostra somente que o resultado deve ser acompanhado no caixa. Não presume sucesso, não produz comprovante oficial e não reenvia automaticamente.

Se o pai reportar falha e permitir tentar novamente, o operador precisa confirmar a revisão outra vez. Idempotência/recuperação da operação continua sob o mecanismo existente. A interface não gera novo identificador de fechamento e não interfere no carrinho, cotação autoritativa B01, pagamentos ou relatório oficial101.

CSV e impressão são rotulados como prévia, com totais/pendências e ressalva de que não comprovam encerramento nem liquidação bancária. São somente leitura. Não registram o caixa como fechado. Não há novas chamadas de API, persistência adicional, polling ou dependência.

## Testes

20 testes novos entre as duas frentes e500 testes de frontend no conjunto preparado. Exemplo fictício deR$315: dinheiro100 + débito96 + crédito119. Comprovantes das parcelas não mudam providerConfirmed; dinheiro contado100 é enviado como10000 centavos, sem transformar315 em dinheiro. Outros casos: fonte incompleta, responsável/justificativa, modo integrado exigindo confirmação da fonte, origem inválida, mudança após revisão, contagem não confirmada, soma exata, referências, filtro literal, datas e overflow.

Prévia Firefox: busca e período, paginação, CSV seguro, impressão, revisão de registro alterado, cancelamento da confirmação, gravação falha, registro preservado, bloqueio sem confirmação, duplo clique, valor contado, autorização local com justificativa, consulta parcial e mudança invalidando confirmação. Cinco idiomas ×três larguras ×dois temas nos dois fluxos. Build/typecheck e checagem do instalador documentados em VALIDACAO.txt. Aceite contra backend real continua necessário; nenhum teste é evidência de liquidação bancária ou conformidade fiscal.

Testar no aplicativo: no fechamento, confirmar cédulas/moedas, registrar e vincular comprovantes, seguir para revisão final. Conferir tabela e checklist, voltar e alterar um registro, retornar e verificar necessidade de nova confirmação. Como operador sem responsabilidade, deixar pendência e verificar bloqueio. Como responsável autorizado, registrar revisão/justificativa, salvar e só então confirmar. Resultado oficial depende do backend já existente. Nenhuma funcionalidade de backend nova foi inventada ou alterada.
