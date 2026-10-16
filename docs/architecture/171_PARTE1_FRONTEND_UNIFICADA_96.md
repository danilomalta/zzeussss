# Parte 1 — frontend unificado96

Pacote cumulativo das interfaces86–96. Substitui a revisão95 e reúne venda,
pagamento, comprovante, fechamento, conferência, malote e livro de caixa.
Escopo: frontend. Não contém implementação de servidor ou homologação comercial.

## As seis lacunas da revisão95

| Lacuna | Resultado nesta entrega | Evidência |
| --- | --- | --- |
| Pagamento dividido em telas desconectadas | Um checkout: produtos, CPF/clube, desconto autorizado, pontos, nota solicitada, dinheiro/cartões/Pix/crediário por parcela e confirmação única | Firefox: venda30 com10 dinheiro,10 débito e10 Pix, entregue20, troco10 |
| CPF/clube repetidos | Dados do primeiro produto reaproveitados, editáveis antes da cotação; senha do supervisor só em memória e apagada após envio | Firefox: CPF de teste, clube, desconto5 e pontos1 gerando líquido24; sem nova digitação de CPF |
| Carrinho não suspenso perdido | Autosave IndexedDB por empresa/loja/aparelho/usuário/modo/caixa; retoma UUID e quantidades, consulta preços e locais atuais; mudança de preço exige revisão | Firefox: recarga recupera o produto; testes de conflito e isolamento de armazenamento |
| OCR sem suporte no Firefox | Tesseract WASM local, carregado sob demanda; câmera ou imagem, progresso/cancelamento, revisão dos campos, sem guardar foto | Firefox real: comprovante fictício em português, valor30 e NSU123456 reconhecidos |
| Impressão genérica | Prévia e impressão de58mm,80mm ouA4; largura, quebra de texto, itens, meios e troco | Firefox: popups58/80mm medidos; texto externo escapado |
| Validação só em DOM virtual | Testes em Firefox real, claro/escuro, cinco idiomas,1440/768/390px e zoom50–100% | Scripts de teste; sem transbordamento horizontal da página |

## Venda e retomada

O pagamento é composto em centavos exatos. O dinheiro entregue cobre apenas a
parcela em espécie. O troco aparece após confirmação e permanece junto dos
produtos até iniciar a próxima venda. F8 abre o desconto no mesmo checkout.
O item ativo, o modo de tela cheia, o fechamento separado e os atalhos anteriores
foram preservados.

Antes de cobrar, grava-se o ID e o estado pendente. Perda de resposta vira
resultado incerto; a ação disponível é consultar o mesmo ID. A confirmação final
também é gravada antes do envio, e pode ser consultada após recarregar. Alterar
o preço do catálogo não cria outra operação para aquele carrinho em recuperação.
Não se cria uma nova cobrança para contornar um erro.

Terminal não integrado: a parcela aceita NSU/referência, adquirente, terminal,
data/hora e revisão explícita. O estado diz registrado para conferência;
não significa liquidação bancária. A conciliação distingue confirmação pelo
provedor de comprovante conferido manualmente. Não há dados de cartão nos campos.

Checkout avançado sem adaptador: fica indisponível, sem simular sucesso. A venda
simples em dinheiro continua na API existente, com ID estável para evitar
duplicação na retomada. Desconto, Pix e nota não são enviados como dinheiro.

## Fechamento

Continua em `/local/cash/register`: primeiro contar cédulas/moedas, depois
conferir comprovantes e parcelas, revisar diferenças e encerrar. As vendas do
checkout são lidas pelo adaptador de fechamento quando ele estiver conectado.
Livro de caixa, malote com identificação, responsáveis e relatório95 são
preservados. Não houve substituição de registros reais por totais fictícios.

OCR reconhece texto, não prova que a transação foi liquidada nem que o dinheiro
existe na gaveta. A leitura não confirma sozinha a correspondência da venda.
Não se presume produto na notinha. O operador revisa antes de gravar os campos.
São cerca de13MB de recursos locais, carregados só ao pedir leitura. Um worker
por vez, imagem limitada, timeout e encerramento do worker. Campos manuais seguem
disponíveis, inclusive em máquina sem câmera. O pacote não chama OCR externo.

## Contrato de integração — sem URLs inventadas

`Gateway95.checkout96` é opcional e vem ausente no modo real.
O contrato TypeScript está em `src/core/local/checkout96.d.mts`.

| Método | Responsabilidade do backend/conector |
| --- | --- |
| quote | Recalcular produtos/preços/estoque, validar CPF/clube, pontos, autorização do supervisor, fiscal solicitado; retornar cotação com validade e líquido |
| charge / chargeStatus / cancelCharge | Cobrar/consultar/cancelar parcela pelo mesmo ID, conforme PSP e crediário; QR/valor/estado autoritativos |
| recordManual (opcional) | Registrar comprovante revisado e deduplicá-lo no contexto correto, sem presumir liquidação |
| finalize / status | Validar cotação e parcelas, finalizar uma única venda atomicamente com estoque/financeiro/outbox/pontos; retornar o comprovante e recuperar por ID |
| receipt / sales (opcionais) | Reimpressão e histórico completo por sessão para fechamento e conciliação |

O servidor deve derivar empresa/loja/usuário/aparelho/permissões da sessão,
rejeitar outro conteúdo para o mesmo ID, garantir idempotência e conciliar
pagamento/fiscal em estados separados. Fiscal depende de serviço autorizado e
regra aplicável; solicitar nota na tela não emite documento fiscal por si só.
Os demais contratos95 (reembolso, equipamento, pareamento, clube, relatório)
estão preservados em `170_PARTE1_FRONTEND_FINAL_95.md`.

Persistência local não guarda CPF, senha, nome do cliente, QR ou imagem no plano
de pagamento. Guarda identificadores, valores, itens e estados necessários para
recuperar a operação. CPF/clube continuam em memória; depois de recarregar, a
cotação confirmada é identificada pelo ID, sem armazenar documentos pessoais.
Cache local é uma medida de recuperação, não backup entre computadores.

## Prévia independente

`python3 -B ver_previa.py`: abre uma porta local livre, sem senha e sem API.
Dados fictícios, QR não pagável, nenhuma emissão fiscal ou cobrança real.

1. Digitar123, adicionar produto30. Informar CPF de teste ou continuar sem CPF.
2. Pagar com dinheiro10, débito10, Pix10. Informar20 entregues. Conferir valores,
   iniciar e consultar as duas parcelas, finalizar. Ver produtos e troco10.
3. Prévia de impressão: escolher58mm e80mm. Não confundir comprovante não fiscal
   com documento fiscal. PDF usa a impressão do navegador.
4. Nova venda, adicionar produto e recarregar: carrinho reaparece.
5. Abrir desconto: preencher credenciais fictícias, desconto5, resgate10pontos,
   dinheiro24, entregue30. Na demonstração cada ponto equivale a10centavos:
   esta conversão é somente fixture, não regra comercial do Titan.
6. Terminal manual: escolher débito/cartão manual, inserir referência/adquirente/
   terminal/data e confirmar revisão. Fica registrado para conferência.
7. Fechamento: contar dinheiro antes de conferir comprovantes. Ler uma imagem
   ou câmera, revisar valor/data/meio/NSU e confirmar. Não usar dados reais.
8. Alternar os cinco idiomas e os dois temas. Testar teclado, tablet/celular.

## Instalação e limites

Instalador aceita bases conhecidas86–95 e reaplicação idempotente. Confere antes
de escrever, cria backup privado, preserva personalizações reconhecidas e
interrompe em arquivo desconhecido sem forçar substituição. Não pede coleta de
arquivo. Não altera package.json, não instala dependência, não migra banco,
não faz push. Executa as suites monetárias e build. Índice com trabalho prévio
fica preservado.

Recursos de câmera física, impressão física, gaveta, maquininha, Pix/SEFAZ reais
e comunicação remota com celular continuam dependendo dos conectores/backend
e teste nos equipamentos. Esta entrega fecha as lacunas de interface listadas,
não declara o ERP inteiro operacional. O bundle grande das planilhas preexiste;
OCR é separado e preguiçoso. Traduções en/es/fr/zh-CN requerem revisão nativa.
Nenhuma chave96 ficou ausente entre os cinco idiomas.

Fontes exatas: ARQUIVOS_ALTERADOS.txt. Resultados executados: VALIDACAO.txt.
