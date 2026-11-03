# Credenciais Focus administradas pelo cliente

Esta entrega configura token e CSC local opcional de NFC-e Focus. Não envia
certificado A1 à Focus, não credencia emitentes, não transmite uma nota e não
habilita produção. O cadastro da empresa e as credenciais são etapas posteriores
ao pré-cadastro mínimo do produto. A empresa cliente é o emitente.

## Ativação por instalação

O administrador da infraestrutura habilita `customer_managed: true` somente para
as empresas/lojas/aparelhos autorizados no arquivo privado passado a
`titan-local serve --fiscal-providers`. O arquivo continua privado (0600), e
`FISCAL_MASTER_KEY` continua vindo da configuração privada do processo: base64 de
32 bytes aleatórios. Nunca colocar essa chave no frontend, no Git ou em logs.

Exemplo estrutural SEM segredos; substituir identificadores e caminho pelos da
instalação, sem criar outro banco comercial:

```json
{
  "version": 1,
  "vault_path": "/caminho/privado/fiscal-vault.sqlite",
  "tenants": [{
    "customer_managed": true,
    "provider": "external",
    "environment": "homologation",
    "crt": "1",
    "certificate_type": "A1",
    "binding": {
      "tenant_id": "empresa-autorizada",
      "store_id": "loja-autorizada",
      "device_id": "aparelho-autorizado",
      "profile_id": "customer-setup",
      "profile_revision": 1
    }
  }]
}
```

O perfil inicial do binding reserva a entrada de configuração. Não cria nem
homologa emitente. Ao salvar pela tela, perfil, revisão, CRT e ambiente são
obtidos do perfil fiscal persistido da própria empresa; não são inferidos dos
valores iniciais do arquivo. O registro suporta um binding fiscal por tenant,
como o registry existente; não amplia esta entrega para múltiplas lojas em um
único binding.

Provedores próprios e configurações antigas sem `customer_managed` mantêm seu
comportamento. Não há ativação automática, endereço de API enviado pelo cliente,
seleção de cofre pelo HTTP nem chave mestra padrão. A3 e URLs diferentes da
homologação Focus são recusados nesta ativação.

## Operação pelo cliente

Na configuração fiscal, o proprietário ativo salva ou consulta seu perfil
modelo 65. O painel **Credenciais Focus da empresa** primeiro consulta a revisão
atual. O cliente preenche token e, opcionalmente, CSC local em campos ocultos.
Certificado A1 e CSC do emitente devem estar configurados no ambiente Focus;
presença no cofre local não comprova essa configuração externa.

Salvar substitui as credenciais anteriores. CSC vazio remove o CSC local.
Os campos são limpos após o envio e na mudança de sessão/contexto; não há
localStorage, exportação ou devolução dos valores pela API. Em resposta perdida,
o painel retém apenas o identificador da tentativa e usa GET para reconciliar;
não repete o POST automaticamente. Se outra revisão interveio, mostra conflito.

- `GET /local/v1/fiscal/onboarding/:id/provider`: somente status permitido.
- `POST /local/v1/fiscal/onboarding/:id/provider`: proprietário ativo, sessão,
  contexto e contrato fiscal; corpo estrito e comparação de revisões.
- Persistência: registro único AES-GCM por tenant/purpose; troca atômica com CAS
  do ciphertext protege contra sobrescrita concorrente.
- Após reinício, o serviço recompõe o provedor pelo mesmo registro criptografado.
- Ambiente production pode ser salvo no perfil, mas o checkout recusa emitir.

## Custódia e recuperação

Servidor próprio e nuvem devem usar o transporte TLS/restrições já exigidos pelo
perfil de implantação. O cliente ou administrador de infraestrutura controla a
chave mestra. Quem controla processo e chave pode tecnicamente descriptografar o
cofre; não há garantia de inacessibilidade ao operador da infraestrutura apenas
por usar AES-GCM. Não foi criado acesso de suporte remoto para o fornecedor.

O cofre externo e sua chave exigem cópias seguras separadas do banco operacional.
O backup comercial existente não foi ampliado para incluir automaticamente o
cofre nesta entrega. Não apagar ou trocar a chave antiga sem um procedimento de
migração e restauração comprovado. Perder a chave impede ler as credenciais.

## Evidência e limite

Os testes exercitam banco descartável, sessão real da API local, isolamento,
reinício, comparação de revisões, substituição de ciphertext e campos de presença.
Não são evidência de autorização Focus/SEFAZ. O teste oficial depende de emitente
cliente autorizado e credenciais privadas; não foi executado nesta entrega.
