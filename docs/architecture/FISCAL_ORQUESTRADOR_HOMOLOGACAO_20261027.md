# Orquestrador técnico NF-e/NFC-e em homologação — 27/10/2026

Base: `2b75dfce4158efebf93bb2daf5f4ff669fc35f1f`, sobre o journal fiscal68.
Trabalho na worktree isolada `fiscal-homolog-journal-20261027`. Nenhum serviço
8182/8282, banco comercial, frontend ou Electron foi alterado. Não houve instalação.

## Caminho acrescentado

`fiscalcore.Client.IssueHomologationNFe` exige um `InvoiceRecorder` do host
confiável. O adaptador SQLite `fiscal.HomologationRecorder.ValidateInvoice`
confere sessão original, contrato e revisão do perfil e compara o formulário
fiscal à venda congelada: emitente, CRT, UF, município, série/chave, produtos,
quantidades, preços, total e parcelas. Os valores são racionais/inteiros, sem
float. Linhas seguem a ordem dos IDs do snapshot; SKUs repetidos não são fundidos.

O recorte aceita venda em dinheiro confirmado, emissão normal em ambiente2,
sem descontos, frete, seguro ou outros acréscimos. Não infere valores ausentes.
Valores de impostos que alterariam o total são recusados neste recorte.
O pagamento representa a parcela aplicada, não as cédulas entregues ao caixa.
Não é uma mudança no cálculo de troco da venda original.

Depois da conferência: construir XML, conferir somas, validar XSD/assinatura,
persistir requisição, reconferir autoridade antes da rede, fazer uma tentativa
mTLS e associar o retorno à chave, ambiente e digest exatos. O transporte e o
journal existentes continuam responsáveis pelo registro anterior ao POST.

Estados retornados: `not_sent` para candidato que não iniciou reserva/envio;
`uncertain` para tentativa com resultado incerto; `response_received` para
resposta que não prova protocolo válido; `protocol_received_homologation`
somente quando a associação do protocolo de homologação passou. Repetição
retorna `ErrRecorded` e a evidência original recuperável; nunca faz novo POST.
Se a permissão de recuperação faltar, retorna `recovery_required`, sem exibir
um candidato reconstruído como se fosse o original.

`RecoverHomologationNFe` exige leitura autorizada do journal, verifica hashes,
envelope SOAP e integridade criptográfica da assinatura do XML arquivado.
Não precisa de rede, chave privada nem certificado ainda vigente. O certificado
público vem do documento originalmente validado e persistido. Essa verificação
arquivística NÃO cria confiança nova em cadeia ICP-Brasil, não verifica revogação
atual e não serve para aceitar XML arbitrário enviado pelo usuário.
O envelope SOAP arquivado admite até2MiB e8200 nós para acomodar o invólucro;
o XML fiscal extraído mantém o limite original de1MiB e8192 nós. Uma regressão
confere a recuperação de envelope maior que1MiB sem ampliar o limite da nota.
O método público de assinatura/verificação de operações novas continua exigindo
credencial confiável e vigente; o helper de arquivo é privado.

## Limites explícitos

- É componente do host; nenhuma rota HTTP ou tela de emissão foi habilitada.
- O número ainda é informado pelo host. O índice durável existente impede
  reutilização já registrada neste banco, mas não faz alocação automática/global.
- NCM, CFOP, unidade fiscal, descrição e grupos tributários precisam de fonte
  fiscal aprovada; não são determinados pelo snapshot comercial atual.
- Não é motor tributário nacional, aprovação contábil, DANFE, nfeProc completo,
  contingência integral, ciclo de eventos, consulta/reconciliação oficial,
  SPED/PVA ou NFS-e de produção.
- Não se alterou a capacidade indisponível das APIs de produção. O protocolo
  sintético dos testes não é autorização da SEFAZ nem homologação real.
- Teste positivo de transporte usa journal de arquivos e mTLS real em loopback.
  A conferência SQLite é testada separadamente com venda/API reais em laboratório.
  NÃO foi executado um E2E SQLite + SEFAZ; não somar os dois ensaios como se fosse.

## Referência oficial

Manual de Orientação do Contribuinte e anexos publicados pelo Portal Nacional:
https://www.nfe.fazenda.gov.br/portal/listaConteudo.aspx?tipoConteudo=ndIjl+iEFdE%3D
O código não transforma recebimento HTTP200 em autorização fiscal.

## Verificação desta unidade

Resultados finais e hashes dos fontes: `evidence/fiscal-issuance-20261027/verification.json`.
Logs JSONL ficam fora da árvore Git, em `work/integration-20261027/`.
A primeira rodada do laboratório foi recusada por permissão do diretório do
FileRecorder (`invalid fiscal input`); o diretório descartável foi ajustado para
0700. Um teste tentou uma rota de logout inexistente e recebeu404; passou a usar
`localauth.Logout`, já existente. Uma chamada Go na raiz falhou com
`go.mod file not found`; foi reexecutada em backend/. Não houve bypass do produto.
Os resultados finais só se referem às execuções posteriores às correções.

Os sete checks verdes anteriores dos PRs4/5 pertencem aos seus respectivos
commits, não a estes fontes novos. Exigir CI desta revisão antes de aceitá-la.
O bloqueio de merge por credenciais antigas ainda não trocadas permanece.

Resultado local final:42 funções principais/98 resultados incluindo subtestes,
zero falhas e zero skips nas suítes fiscais selecionadas, com detector de corridas.
São36 resultados no núcleo/store e62 na API;33 comandos reais de backup.
Análise estática dos pacotes afetados, build de titan-local/titan-fiscal e mapa
de537rotas passaram. A suíte geral de todo o repositório não foi repetida
localmente nesta unidade; o workflow remoto continua exigindo-a.
