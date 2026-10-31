# A1 protegido no emissor do backend — 29/10/2026

Base: `7223bac548f9c621f15f3964d82f2efc8edfb5ff`.

## Comportamento entregue

O mesmo `titan-local serve --fiscal-production ...` que carrega a
configuração privada do emissor passa a aceitar certificado/chave RSA em
PKCS#12 DER (`.pfx`/`.p12`), além do formato PEM já existente. Não há serviço
fiscal paralelo, mock de autorização ou alteração do fluxo de aprovação.
É código do backend, sem mudança de frontend/Electron nesta unidade.

Na configuração privada versão1, selecionar exatamente uma origem:

- PEM: `certificate_path` e `private_key_path` preenchidos;
- A1: `pkcs12_path` e `pkcs12_password_path` preenchidos, com os dois campos
  PEM vazios. A senha não é um campo JSON: o segundo caminho aponta para um
  arquivo privado separado.

Os caminhos são absolutos. Configuração, A1, senha, raízes e CRLs continuam
sujeitos à leitura de arquivo regular privado0600, sem link simbólico na
entrada final, com verificação de identidade entre lstat/open/stat e limite
de tamanho. O carregador recusa origens incompletas/misturadas. Não importa
certificado de um ZIP arbitrário nem procura automaticamente material de um
emitente no computador. Esta entrega não cria arquivo contendo senha do cliente.

A senha é lida literalmente, sem trim ou remoção de quebra de linha: UTF-8,
1–1024bytes, sem NUL. Não existe opção de senha na linha de comando, upload HTTP,
salvamento no SQLite ou exportação da chave PEM. Os buffers de entrada da senha,
PFX e chave PEM são limpos ao concluir a leitura; isso não constitui garantia
de zerar todas as cópias internas/temporárias da memória gerenciada pelo Go.

Depois da decodificação, a mesma verificação existente exige RSA compatível,
certificado válido, correspondência da chave, CNPJ exato do binding, cadeia
contra raízes confiáveis configuradas separadamente, fingerprint e vínculo
vigente de empresa/loja/aparelho/cadastro/modelo/série. Certificados CA embutidos
no PFX não passam a ser raízes confiáveis. A CRL continua sendo relida e validada
antes de cada novo envio: carregar um A1 uma vez não ignora revogação posterior.
O emissor continua exigindo sessão, contrato fiscal, revisão, aprovação e journal.

## Diagnóstico pelo executável

Comando de terminal, após o operador informar o caminho real de uma configuração
privada existente:

```bash
titan-fiscal check-production-config --in "$TITAN_FISCAL_CONFIG"
```

O comando não abre banco comercial, não migra esquema, não reserva numeração,
não transmite, não instala certificado e não escreve saída em arquivo. Devolve
JSON de configuração/cadeia/revogação válidas no instante, UF/modelo/série,
revisão, vigência do binding e hash público da configuração vinculada. Não
devolve CNPJ, senha, certificado/chave, caminhos privados ou URLs de serviço.
Configuração inválida devolve código de saída não zero e erro genérico.

`official_homologation_verified` e `transmission_performed` permanecem false.
O check é offline: não verifica situação cadastral/credenciamento na SEFAZ,
sessão ou contrato da instalação no banco, nem comprova teste oficial anterior.
Não confundir `configuration_valid` com autorização operacional para emitir.

## Dependência e interoperabilidade

`software.sslmate.com/src/go-pkcs12` está fixado em0.7.4, com apenas suas duas
entradas de checksum novas e sem atualização indireta de outras dependências.
Documentação primária:
[pacote](https://pkg.go.dev/software.sslmate.com/src/go-pkcs12@v0.7.4),
[código do fornecedor](https://github.com/SSLMate/go-pkcs12).
O fornecedor corrigiu a aceitação de PBMAC1 com chaves excessivamente curtas em
0.7.2; a versão escolhida inclui essa correção:
[GHSA-mpwr-8vm7-h73f](https://github.com/SSLMate/go-pkcs12/security/advisories/GHSA-mpwr-8vm7-h73f).
Testes locais cobrem Modern2026/PBMAC1, Modern2023/PBES2 e LegacyDES, com senha
Unicode, assinatura XML real verificável e recusa de senha incorreta.

## Limites e preservação

Arquivos de teste e identidades geradas são exclusivamente descartáveis; não
são distribuídos como configuração de cliente, credenciamento ou certificado
de produção. Nenhum ensaio desta unidade é anunciado como transmissão SEFAZ.
Nenhum dado/serviço8182/8282, banco comercial ou chave existente foi usado.

A1 por arquivo não implementa A3/PKCS#11/HSM, cofre nativo do Windows, rotação
automática, cadeia/certificado de matriz para filiais nem cadastro/upload de
segredos na interface. A política atual de arquivo0600 não é relaxada para
contornar ausência de ACL privada no Windows; compilar para Windows não prova
provisionamento seguro nessa plataforma. A configuração precisa ser preparada
por administrador autorizado de cada cliente, com suas raízes revisadas e CRLs
atuais, quando houver emitente real.

Este adaptador não conclui motor tributário nacional, DANFE de produção,
contingência, SPED/PVA ou NFS-e de produção. A homologação oficial não é feita
com certificado do proprietário do ERP em nome de todos os clientes, nem se
torna verdadeira pela presença de uma referência textual na configuração.

## Verificações

Resultados finais e hashes serão registrados no mesmo registro de continuidade
e em `evidence/fiscal-a1-backend-20261029/verification.json`. O CI passa a exigir
o executável compilado em `TestFiscalProductionConfigNativeCLI`; ausência ou
skip reprova o aceite integrado. As demais integrações obrigatórias permanecem.
O prazo da rodada fiscal de API com `-race` aumenta de10para20min, sem retirar
testes/assertivas: a execução40810819619 atingiu o limite durante um teste com
11s de execução, não uma espera individual de10min. O job segue limitado a40min.
