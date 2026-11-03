# Frontend105 — Movimentos e autorizações do caixa

## Escopo e base
Somente frontend; requer104. A instalação104 no PC foi confirmada com108 testes e build22,38s. Esta entrega modifica apenas Till94.tsx entre os arquivos anteriores; novos componentes, modelo, traduções, testes e prévia completam o pacote. Não modifica Go, SQL, APIs, contrato/licença, saldo, estoque, adquirente ou caixa comercial. Não chama API de movimento nem registra uma autorização bancária. Não substitui B01/B02B1/B02B2, não faz merge/commit/push e não atualiza dependências.

## Criação em duas etapas
As abas Movimentos e Autorizações reutilizam request94 e o mutate/CAS do useTill94. Em Movimentos, escolhe sangria ou suprimento; em Autorizações, solicita desconto vinculado ao carrinho atual. Valor em centavos exatos, positivo e com motivo de3 a400 caracteres. Desconto não ultrapassa o total original.

Primeira etapa preenche; segunda mostra tipo, valor e motivo, além de total original/proposto no desconto. A pessoa confirma o registro ou volta para editar. Mudança de referência do carrinho ou valor de base invalida a prévia, exigindo nova conferência. Uma trava síncrona evita que clique repetido dispare duas gravações no mesmo fluxo da tela. Isso não substitui idempotência comercial no backend nem promete evitar duplicação após queda entre commit e resposta. Em caso de falha, os campos ficam preservados e a mensagem orienta conferir os registros antes de reenviar.

A aprovação é sempre local neste pacote. Criar/aprovar não movimenta dinheiro, não abre gaveta, não executa sangria e não confirma Pix/cartão. O usuário vê esse limite na tela e nos documentos exportados. Nenhum dado de saldo esperado foi inventado.

## Revisão individual
Lista pesquisável por motivo/referência, filtro por tipo/situação, paginação de10, CSV e impressão da solicitação. A revisão começa por uma solicitação específica, não por uma justificativa global compartilhada com todos os botões. Mostra tipo, valor, motivo e estado, e a decisão exige justificativa própria. Trocar de solicitação com justificativa não salva pede confirmação de descarte.

O snapshot inclui ID, solicitante, data, tipo, valor, motivo, referência, estado, dados de desconto/parcelas/devolução e revisão. Se ele mudar, botões de decisão ficam bloqueados até recarregar e conferir a seleção. A mesma conferência ocorre dentro do callback de mutate antes de decide94/withdraw94, além da proteção de revisão CAS existente. Uma falha de gravação não é apresentada como aprovação.

As regras de perfil foram preservadas: manager94 exige owner/manager com manage_cash; caixa comum não aprova; criador ou gerente pode retirar pedido nos estados já permitidos; outro caixa não retira o pedido de terceiros. Não foi acrescentado veto a proprietário revisar solicitação própria, pois essa política não existia. O backend continua a barreira de autorização para qualquer futura execução real.

Gerente continua podendo selecionar registros de outro operador na mesma estação/loja pelos controles existentes. Nesse espaço, criação fica oculta; revisão permitida continua disponível. A mensagem orienta selecionar o próprio espaço para criar. Sem sell/manage_cash, o componente não mostra formulários nem a lista. Não houve ampliação de permissão, leitura de outra empresa ou nova rota de dados.

## Histórico e proteção dos dados
Exibe criação/revisão/retirada usando journal já persistido. Preserva a ordem de inserção do diário mesmo se o relógio local mudou, sem usar horário do aparelho para decidir prioridade financeira. Datas/IDs inválidos e duplicados são rejeitados nessa visualização. Operadores aparecem como Você/Outro responsável quando só existe ID, sem inventar nomes nem exibir UUID pessoal.

CSV usa a proteção contra fórmulas de csv97. Impressão usa print95 com escape de texto e aviso de solicitação local. Não exporta o identificador pessoal do operador nem o fingerprint completo do carrinho. Nenhuma foto, senha, token, dado de cartão, cache persistente novo ou log sensível foi acrescentado.

O aviso de campos não salvos, bloqueio de saída e beforeunload existentes do Till94 foram preservados. Formulários não desmontam só porque o store fica ocupado, evitando perda de campos no retorno de uma falha. Troca de espaço/tipo/operador mantém os limites do componente pai e reinicia o formulário pela chave de contexto.

## Interface e idiomas
Cinco idiomas: pt-BR/en/es/fr/zh-CN.29 chaves novas completas, sem texto de produto novo vazio. Quatro traduções estrangeiras precisam de revisão por falantes nativos antes de produção. Usa Intl, paleta e temas existentes sem azul novo; botões44px, campos16px, textos14px, foco visível e rótulos explícitos associados às áreas de texto. Layout em duas colunas no desktop e uma no celular. Sem bibliotecas novas ou tarefas em segundo plano.

## Prévia e testes
PREVIA.html/ver_previa.py funcionam sem login, com controles explícitos de demonstração. Dados ficam em memória; não usam API nem movimentação real. É possível simular caixa/gerente/sem permissão, falha de gravação, alteração por outra aba e mudança do carrinho. Usa os próprios modelos request94/decide94/withdraw94, não aprovação fictícia de banco.

Evidências finais em VALIDACAO.txt:437 testes completos do frontend; build TypeScript/Vite; Firefox cobrindo valor inválido, duas etapas, falha com campos preservados, duplo clique, caixa sem aprovação, gerente com motivo, alteração invalidando revisão, recarga explícita, auditoria, CSV, retirada, perfil sem acesso, prévia de desconto invalidada e cinco idiomas ×três larguras ×dois temas.

Não executados Go real, banco comercial, adquirente/TEF/Pix, SEFAZ, câmera física, gaveta/impressora física, transporte entre aparelhos ou aceite no PC do proprietário. A Parte1 inteira não está concluída por esta entrega. Execução oficial de movimentos/autorizações e sincronização comercial dependem de contrato e implementação do chat de backend; nenhum endpoint foi inventado aqui.

## Conferência no aplicativo
1. Instalar depois da104. PDV → Movimentos: preencher sangria/suprimento, conferir valor/motivo e registrar. Deve aparecer aguardando revisão, sem saldo oficial alterado.
2. PDV → Autorizações: solicitar desconto do carrinho atual. Alterar carrinho antes de registrar deve invalidar prévia. Desconto maior que o total é recusado.
3. Gerente abre solicitação, informa motivo e aprova/recusa. Conferir aviso de aprovação local. Registrar outra decisão por outra sessão deve impedir uso do snapshot antigo.
4. Buscar e filtrar; conferir criação/revisão no histórico; baixar CSV e imprimir. Retirar somente pedido próprio ou como gerente autorizado.
5. Testar alteração não salva ao navegar, erro de persistência em cópia de teste, cinco idiomas, claro/escuro e celular. Perfil sem permissão não deve ver os controles. A autorização definitiva de qualquer execução exige backend.
