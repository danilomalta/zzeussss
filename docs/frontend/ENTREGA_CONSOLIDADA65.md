# Frontend consolidado com devoluções schema65

Base frontend `d3d727d535b3446f71d782027b1ce1f435d6f07b`, backend consultado `a5bbcdf3420ca4fa6c471fcdd52794f86f84c0dc`. Esta entrega amplia o frontend existente; o tamanho do pacote não é uma medida de funcionalidades novas nem de conclusão comercial.

## Implementação nova

- `/local/customer-returns`, acessível pelo menu para quem tem `cancel_sale`: consultar venda, conferir saldo exato por item, selecionar quantidade em milésimos e destino, calcular cotação autoritativa, registrar motivo e revisar antes da confirmação. Produtos são identificados pelas referências originais fornecidas pelo contrato; não são inventados nomes/unidades ausentes.
- `restock` exige declaração explícita de aptidão; `no_restock` é a opção inicial. O contrato recebe a quantidade na unidade da venda, não uma conversão silenciosa.
- Operação e conteúdo são persistidos antes doPOST, sem token/senha na fila. Escopo: empresa/loja/usuário/aparelho. Bloqueio entre abas impede dois envios locais simultâneos; o backend continua responsável por transação e idempotência entre clientes.
- Após resposta perdida, consultar pelo mesmo `operation_id`; nenhuma recuperação é automática. Reenviar exige ação explícita e preserva o corpo exato. Operação que pode ter sido enviada não pode ser descartada como rascunho. Rascunho não enviado só é retirado após consulta que confirma ausência.
- Resposta após logout, troca de contexto ou expiração não limpa a fila antiga nem iniciaPOST no novo contexto. Mudanças em localStorage de outra aba atualizam a indicação de pendência.
- Recibo, histórico, impressão e CSV. React e impressão escapam texto; CSV usa a proteção existente para fórmulas. Histórico completo é conciliado com todas as linhas, quantidades, centavos e recibos; respostas parciais não viram totais.
- `official101` aceita bruto = líquido + cancelado + devolvido. Pagamentos originais continuam pelo valor registrado bruto menos cancelamentos; o reembolso aparece separado. As duas métricas novas precisam concordar e ser coerentes com o dinheiro registrado. Serviço antigo sem métricas mantém a ausência explícita, sem fabricar zero.
- Relatório oficial usa o formatador exato de centavos do journal na tela, impressão e CSV. Novos textos em pt-BR/en/es/fr/zh-CN; revisão nativa ainda necessária.

## Limites

Novas devoluções só no caixa original ainda aberto, com saldo suficiente, licença e permissão atuais. Não acrescenta estorno eletrônico/fiscal, restituição em outro turno/aparelho, crédito ao cliente, inspeção sanitária automática ou reconstrução de lotes não registrados. Consulta/recuperação não é confirmação de recebimento bancário.

O frontend antigo continua utilizável com serviços antigos, mas a área de devoluções recebe erro explícito caso o serviço não exponha as quatro operações de schema65. O pacote não inicia/atualiza backend, não escolhe banco/estação/chaves nem muda os serviços8182/8282. Aplicar fontes não promove automaticamente o estágio desktop instalado; promoção e configuração exigem decisão operacional separada.

## Aceite desta entrega

Resultados executados constam em VALIDACAO.txt e evidencias/ no pacote. A suíte completa é `npm run test:all`; a lista histórica `test:local` permanece preservada e menor. `test:returns65` executa o conjunto novo de contrato e fluxo. O teste de ponte usa backendGo real/SQLite temporário e clienteNode real, sem mock de negócio. Seu harnessGo fica somente nas evidências, fora do patch de backend.

A tentativa de instalar Chromium foi bloqueada por download truncado. Por isso não se declara navegação/aceite visual ou impressão física executados nesta entrega. Há roteiro manual abaixo. Uma primeira tentativa de build com sourcemaps atingiu o heap de2GB; o build de produção sem sourcemaps, com memória explícita, passou. O aviso de chunks grandes foi preservado.

## Roteiro manual em serviço descartável autorizado

1. Entrar com gerente/dono autorizado e verificar contexto. No menu, abrir Devoluções de clientes. Com perfil sem `cancel_sale`, o atalho não aparece e a página não cria cliente autorizado.
2. Consultar uma venda real de teste em dinheiro, ainda com caixa original aberto. Conferir item, quantidade disponível e unidade do comprovante original.
3. Informar quantidade com até três casas, motivo e destino. Cotar; conferir total; marcar revisão; confirmar. O recibo deve mostrar o identificador original e o histórico deve reduzir o saldo.
4. Interromper a resposta depois do envio em ambiente de teste. Recarregar/login no mesmo contexto: consultar pendência. Nunca criar outroID para resolver a incerteza. Reenviar somente o mesmo pedido se autorizado.
5. Em duas abas, tentar confirmação simultânea. Testar armazenamento indisponível e troca de sessão antes da resposta. Nenhum caso pode gerar envio novo sem proteção ou apagar o pedido antigo.
6. Encerrar o caixa de teste e comparar relatório, CSV e impressão: venda bruta3198, devolução1599, venda líquida1599. Com abertura1000 e sem outras movimentações, esperado2599. Isso é um cenário sintético, não valor de produção.
7. Tentar nova devolução depois do fechamento: deve ser recusada. Recuperação do recibo original continua disponível conforme permissão vigente.
8. Conferir os cinco idiomas, claro/escuro,390/768/1280, teclado, leitor de tela e impressora alvo. Identificar problemas por tela/ação, preservando dados e sem limpar perfis indiscriminadamente.

## Inventário

`PENDENCIAS_FRONTEND65.md`, `.json` e `.html` listam96itens em32frentes, com rotas declaradas e referências. `existente_exige_aceite` significa implementação a validar/aprofundar, não ausência total. O inventário não substitui requisitos comerciais desconhecidos e não atribui percentual global.
