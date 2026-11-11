# P28 — ensaio opt-in do cliente (v1)

Base aprovada: `bd7502c3a11e298eeb179ee9d6fd3a87fd8ea7aa`.
**V1-09 PENDENTE — validação real pendente.** O criador do ERP não é o emitente.
O procedimento é repetido separadamente em cada instalação/empresa/UF; não
transfere dados, token, CSC ou A1 ao provedor do ERP. O ensaio atual cobre
exclusivamente NFC-e 65/A1/homologação. Não promete cobertura de outros modelos
ou municípios, nem homologação universal por ter passado para um cliente.

## Preparação privada pelo cliente e seu contador

1. Confirmar contrato Focus, atendimento/credenciamento da UF, emitente, regime,
   A1 válido e CSC configurados na conta de homologação do próprio cliente.
   Cadastrar manualmente no onboarding existente; não exportar segredos.
2. O contador revisa os dados da operação e os campos/impostos do JSON de emissão.
   O teste não escolhe CFOP, NCM, CSOSN/CST, alíquota, valor ou regime.
   Usar apenas uma operação de ensaio autorizada pelo cliente, sem dados pessoais
   de consumidor. Arquivo privado absoluto, regular, sem symlink, permissão 0600,
   no máximo 1 MiB. É um pedido da API Focus existente, não um novo leiaute fiscal.
3. Na sessão privada do terminal, configurar `FOCUS_SANDBOX_CNPJ`,
   `FOCUS_SANDBOX_TOKEN`, `FOCUS_SANDBOX_PAYLOAD_FILE`,
   `FOCUS_SANDBOX_SALE_ID` e `FOCUS_SANDBOX_ATTEMPT_FILE`.
   O token deve ser carregado pelo mecanismo privado do cliente, sem digitá-lo
   em comando/histórico, sem `set -x`, sem compartilhar ambiente/log bruto.
   O último caminho é absoluto, em pasta privada e persistente do cliente,
   inicialmente inexistente. O ID identifica esta operação de ensaio; não é
   uma venda automaticamente criada no banco do checkout.
4. Guardar a mesma solicitação, ID e arquivo de tentativa. O arquivo grava só
   referência derivada e digest, com sync antes do POST. Não apagá-lo para
   tentar novamente. Se o processo parar antes/depois de enviar, a repetição
   **só consulta a mesma referência**. Alteração de arquivo/empresa/ID ou marcador
   incompleto é recusada; investigar com a Focus, sem novo POST automático.
   A perda deliberada desse arquivo fica fora da garantia deste harness; o fluxo
   operacional continua usando recuperação persistida do servidor existente.

## Comando exclusivo para o cliente autorizado

Dentro de `backend/`, depois da preparação privada acima:

```bash
go test -timeout 120s -count=1 ./internal/fiscalproduction -run '^TestFocusSandboxRealNFCe$' -args -focus-sandbox
```

Este comando pode emitir **uma NFC-e real de homologação**. Não foi executado
pelo agente. Sem variáveis ou sem flag explícita, o teste é pulado. Não usar
`go test ./...` como autorização para emitir. O host permanece fixo no provedor
existente: homologação, sem produção, A3 ou endereço informado pelo operador.

## Diagnóstico e evidência

- Falta de credencial/opt-in: SKIP, não autorização fiscal.
- Arquivo inseguro/incompleto, origem divergente ou tentativa inconsistente:
  recusa antes de transmitir; mensagens não mostram valores.
- Recusa/timeout/resposta incerta: nenhum reenvio automático; consulta apenas
  a referência original. Rever o motivo privadamente na instalação/Focus.
- Aceitação: exige autorização, consulta com a mesma chave, XML completo
  obtido pelo download restrito existente, XSD/ambiente/chave e protocolo.
  XML/token não são impressos nem salvos pelo harness. O arquivo de tentativa
  não é arquivo fiscal nem backup: o aceite operacional deve ainda comprovar
  arquivo cifrado/cupom/backup pelo checkout instalado.
- Cancelamento é ação separada e explícita do proprietário, pelo fluxo já
  existente. Após autorização específica, conferir consulta, protocolo e XML
  do evento arquivado. O ensaio não cancela automaticamente, não apaga a venda
  e não movimenta caixa/estoque. Sem esse ensaio, não atestar evento real.

Registro preenchível sob controle do cliente: commit do programa; instalação
(pseudônimo); modelo/UF; responsável/data da autorização; contador/data da revisão;
status do ensaio; consulta da mesma referência; protocolo presente; hash do XML
obtido pela exportação local; evento autorizado e seu hash quando aplicável.
Manter CNPJ, chave integral, XML, token e configurações fora do Git e de conversas.
Publicar, se o cliente autorizar, só resultado sanitizado sem identificação fiscal.
Um PASS sintético não preenche nenhum campo de aceitação oficial.

Preparação técnica comprovada por `TestCustomerSandboxPreparation`,
`TestCustomerSandboxInterruptedAttemptNeverPermitsNewSend` e
`TestCustomerSandboxRejectsUnsafeFilesAndMissingData`: sem rede e sem credenciais.
O ensaio opt-in real continua separado em `TestFocusSandboxRealNFCe`.

itens fora do v1: ver o documento
