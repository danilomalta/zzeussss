# Frontend97 — conferência do fechamento

Atualização sobre o frontend96, com preservação dos arquivos alterados pela B01-0f6. Não implementa pagamentos, financeiro, autorização fiscal ou outro serviço do backend.

## Resultado

- Fluxo existente preservado: contar dinheiro → conferir pagamentos → revisar e enviar o encerramento.
- Conferência combinada de pagamentos confirmados pela integração e comprovantes vinculados. Uma parcela não é somada novamente por ter as duas evidências.
- Opção exclusivamente integrada continua exigindo confirmação eletrônica; uma fotografia não atende a essa opção.
- Pendências por venda, detalhamento das parcelas, filtro por meio/status, busca por número/data/referência e paginação de20 registros.
- Valores desconhecidos ficam como traço. Fontes malformadas ou duplicadas não permitem encerramento por um checkbox local.
- Referência repetida normalizada por operadora/terminal/referência não aumenta a cobertura. Parcela é identificada por venda+ID, evitando colisão entre vendas.
- Associação provisória serve à investigação e não confirma uma parcela. Continua exigindo revisão; não altera a venda original.
- Nova leitura OCR limpa campos não reconhecidos. Apenas operadora e terminal são preservados para agilizar a próxima notinha. Campos podem ser limpos explicitamente.
- CSV baixa pelo navegador, sem API de seleção de pasta. Exportação é da conferência local, não demonstrativo bancário nem documento fiscal.
- Cinco idiomas, temas existentes, alvos touch, foco visível, estados com texto e ícone, sem biblioteca nova. Listas paginadas e cálculo memorizado.

## Integração existente

O componente continua dentro de `CashClosing92`, na rota existente `/local/cash/register`. Mantém `ClosingSourceContext92`, persistência `closingStorage92`, controle de versão e callback de fechamento. Não cria URLs nem troca o endpoint de encerramento.

`providerConfirmed=true` só deve vir do conector/backend autorizado. O frontend não preenche esse valor automaticamente no modo real. A fonte legada de vendas em dinheiro segue funcionando. Cartão/Pix e parcelas comerciais dependem da próxima implementação do backend.

Saldo esperado em espécie inclui abertura/movimentos e vem do serviço oficial. O frontend não tenta reconstruí-lo só a partir das vendas. A contagem local é enviada pelo fluxo existente; esperado/diferença oficiais continuam dependendo da resposta.

A revisão do responsável nesta tela é registro local; não substitui permissão ou aprovação exigida no servidor. O pacote não acrescenta autorização de supervisor ao backend.

## Compatibilidade com B01

Nenhum arquivo do manifesto da B01-0f6 é escrito por esta atualização. Não altera `LocalPointOfSale.tsx`, `localClient`, `quotedCart`, `Checkout96`, `service95`, package.json, backend ou migrações. Não faz merge nem push e não cria outra worktree.

O aplicador aceita os arquivos anteriores exatos do pacote96, ou a própria versão97 para repetição idempotente. Conteúdo diferente é preservado e interrompe a atualização antes de escrever. Cria backup privado e executa testes e build. Não faz commit automático.

## Como conferir

Na prévia isolada:
1. Conte230 em espécie e confirme a contagem.
2. Veja uma venda fictícia315 com dinheiro100, débito96 confirmado e crédito119 ainda pendente.
3. Leia ou transcreva o comprovante de crédito119, data17/10/2026 às13:01, referência e operadora/terminal; revise e registre.
4. A pendência vai a zero, sem exigir outro comprovante do débito já confirmado. A prévia não realiza pagamentos.
5. Selecione duas vendas semelhantes: um comprovante não é atribuído por palpite; selecione a parcela correta. A outra venda continua pendente.
6. Troque para consulta incompleta: totais ficam desconhecidos e encerramento automático permanece bloqueado.
7. Baixe CSV, recarregue para recuperar a conferência e teste os temas/idiomas.

## Validação e limites

Resultados efetivamente executados constam de VALIDACAO.txt. Testes de Firefox usam fonte simulada explicitamente isolada e OCR WASM local com imagem fictícia. Não comprovam adquirente, câmera física, gaveta, impressão física, SEFAZ, sincronização ou o financeiro da B02.

O PDF de diagnóstico e o plano de backend continuam válidos como roteiro. Esta atualização termina pontos da interface de conferência; não declara toda a Parte1 comercial concluída. Nenhuma chave nova ficou sem tradução. en/es/fr/zh-CN precisam de revisão nativa antes de produção.
