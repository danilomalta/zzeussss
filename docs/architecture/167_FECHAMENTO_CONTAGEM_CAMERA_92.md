# Fechamento de caixa92

## Entrega
O componente CashClosing92 substitui o formulário simples na página real
`/local/cash/register`, acessível por Fechar caixa no PDV. Não é apenas outra
tela na central de preparação.

1. Contagem cega de cédulas e moedas, quantidade por denominação, subtotais e total.
2. Confirmação da contagem, seguida de câmera e revisão de comprovantes.
3. Revisão de dinheiro, débito, crédito e Pix; autorização com justificativa do
   responsável quando há pendência; envio do total contado à operação existente.

Cada comprovante guarda apenas valor, data/hora, modalidade, NSU/autorização,
operadora, terminal e vínculo à parcela. Não há gravação de foto/vídeo. A mesma
parcela não recebe duas provas ativas. Anulações preservam o registro.
Uma compra dividida é conciliada por parcela, sem comparar cada cartão ao total.
Horário tem tolerância de15min; coincidências múltiplas exigem escolha explícita.
Valor ou horário divergente permanece visível, sem inventar uma confirmação.

A câmera exige ação do usuário, permissão e contexto seguro/localhost.
O adaptador nativo TextDetector é usado apenas quando disponível. Não há motor
OCR portátil incluído: Firefox e outros navegadores podem exigir transcrição.
Não anunciar leitura universal ou teste em hardware real. Dados extraídos precisam
de revisão explícita antes de registrar. Câmera para ao sair da etapa/desmontar.

## Persistência e integração
IndexedDB com empresa/loja/usuário/aparelho e sessão do caixa; revisão otimista
impede sobrescrever outra janela. Contagem editada invalida a confirmação. Alterações
não salvas geram aviso antes da saída. Conferência é local, não auditoria oficial.

O adaptador consulta histórico e recibos REAIS do cliente existente, somente da
sessão atual. Busca limitada a1000 registros e concorrência de4; consulta incompleta
mostra valores desconhecidos, não zero. A API atual aceita recibos em dinheiro.
Não foram inventados endpoints de cartão/Pix, de liquidação ou de câmera remota.
Na demonstração isolada, injeção explícita fornece parcelas fictícias; não integra
o aplicativo instalado. Conferir papel não confirma pagamento na adquirente.

O saldo esperado em espécie continua calculado pelo fechamento oficial existente
(abertura, vendas e movimentações). Não foi substituído pela soma de vendas. Na
confirmação, envia-se somente o total contado, preservando IDs e recuperação de
operação incerta do PDV. A autorização local não altera permissões do servidor.

## Ainda necessário
- Motor OCR compatível com Firefox/desktop e validação com comprovantes reais.
- Pareamento autenticado para câmera de smartphone conferir o caixa do PC.
- Endpoint de parcelas de todas as formas de pagamento da sessão, com horário,
  referências da adquirente, autorização e liquidação separados.
- Persistência oficial e auditável da conferência, anexos de metadados, aprovação,
  malote, remetente e conferente; vínculo ao fechamento oficial. O rascunho de
  malotes anterior permanece em `/local/operations?area=closing`, não foi declarado
  transporte de valores operacional.
- Testes em equipamento real, permissões, câmeras, luminosidade e impressos.

## Testar
Abrir o caixa, vender e acessar Fechar caixa. Conferir denominações, recarregar,
confirmar contagem; abrir câmera, negar/autorizar, revisar campos, registrar e anular.
Na prévia, venda315 = dinheiro50 + débito219 + crédito46, às12h locais de16/10/2026.
Usar comprovantes com esses valores e horários. Alterar219para315 deve apontar
divergência. Repetir NSU não pode duplicar. Conferir ambas as parcelas libera a
revisão; pendências exigem responsável com justificativa. O dinheiro contado150
corresponde a abertura100 + venda50 somente nessa demonstração.
Testar cinco idiomas, temas, teclado, celular e duas janelas concorrentes.
Idiomas estrangeiros requerem revisão nativa. Chaves novas presentes nos5 idiomas.
