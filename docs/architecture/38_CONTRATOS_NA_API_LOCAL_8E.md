# Fase 8E — Chaves confiáveis e contratos na API local

## Comportamento implementado nesta etapa

O processo `titan-local serve` aceita `--issuer-keys CAMINHO.json`.
O arquivo local contém um objeto JSON com IDs de emissor como chaves e
chaves públicas Ed25519 de 32 bytes em base64 padrão com padding como valores.
Não contém chaves privadas, senhas nem dados de vendas.

As chaves públicas devem ser provisionadas por um canal confiável e o arquivo
protegido contra alteração não autorizada. Receber uma chave de qualquer pessoa
e colocá-la nesse arquivo significa confiar nessa pessoa como emissora.
Não usar chaves do aparelho como chaves emissoras.

`entitlements.ReadTrustedKeys` rejeita mapa vazio, duplicidades, IDs inválidos,
base64 inválido, tamanho incorreto de chave, valores não textuais, conteúdo
extra, mais de 64 emissores ou arquivo maior que 16 KiB. Chaves são copiadas
pelo verificador. Configuração informada mas inválida interrompe startup antes
de abrir o SQLite. Ausência de configuração não concede módulos por padrão.

## API

- `POST /local/v1/module-contracts`: exige sessão humana e aparelho ativos.
  Recebe somente `key_id`, `payload` e `signature` do Envelope. Os campos de
  bytes usam base64 padrão produzido por `encoding/json`. Empresa, loja e
  operador vêm da sessão. Somente o dono ativo pode instalar. Gerente não pode.
  Instalação repetida do mesmo contrato retorna `repeated: true`, sem duplicar.
- `POST /local/v1/products` e `POST /local/v1/locations`: usam obrigatoriamente
  as funções com contrato da 8D, exigindo `ManageStock` e `inventory` na
  mesma transação SQLite do cadastro. As rotas não chamam o caminho legado.
- Sem emissor configurado: login, logout e consultas autorizadas continuam;
  cadastros e instalação retornam 503. Com emissor mas sem contrato, cadastros
  retornam 403. Contrato expirado ou módulo ausente também retorna 403.
- Sem sessão: 401. Papel sem permissão: 403. Contrato enviado inválido: 400.
  Revisão antiga, conflitante ou retrocesso observado do relógio: 409.
  Falha inesperada de persistência: 500, sem divulgar SQL, senha ou assinatura.
- Consulta de catálogo mantém vínculo, loja e aparelho como requisitos, mas
  não exige licença vigente. Expiração não apaga o histórico nem esses dados.

`localapi.NewWithVerifier` recebe o verificador da configuração do processo.
O construtor antigo `New` permanece para compatibilidade, porém SEM liberar
cadastros: usa o mesmo caminho com verificador ausente. Não existe fallback
para cadastro sem licença através das rotas desta etapa.

O servidor continua em `127.0.0.1:8181`. Isto não libera LAN, smartphone
autônomo, Internet ou acesso por qualquer interface de rede.

## O que ainda não foi entregue

Não há ferramenta operacional de emissão de contratos, portal de cobrança,
rotação remota de chaves, política de carência, revogação instantânea offline,
proteção contra restauração integral de backup antigo ou relógio confiável.
Chaves privadas emissoras não são criadas por startup nem pelos comandos de
teste. Chaves usadas em fixtures existem apenas nos testes.

O arquivo de configuração pode listar várias chaves confiáveis para transição
planejada. O operador precisa manter as necessárias para verificar contratos
já instalados; removê-las torna esses contratos não verificáveis.

As funções legadas de domínio ainda existem para migração gradual, conforme
8D, mas a API de catálogo não as usa para cadastro. Licenciamento de outros
casos de uso, integração HTTP de estoque/caixa/vendas e seus fluxos ainda devem
ser integrados. Não tratar todos os módulos como produtos operacionais prontos.

## Verificação e aplicação

Doze testes novos: dois para parsing de chaves, dois para configuração CLI e
oito de integração HTTP. Fixtures geram chaves em memória e bancos descartáveis.
Cobrem instalação e repetição, ausência de emissor, módulo e papel, assinatura
inválida/empresa estrangeira/emissor desconhecido, JSON ambíguo, revisão antiga,
expiração com leitura preservada e revogação de sessão/aparelho.

O ambiente de preparação não possui Go. A validação do patch não substitui
compilação, testes e vet no PC. A etapa só deve ser registrada pronta após
esses comandos passarem. Não iniciar servidor nem abrir banco real para testar.

Quando existir configuração e contrato reais, o formato do comando de startup
será `titan-local serve --db CAMINHO.sqlite --station CAMINHO.station
--issuer-keys CAMINHO.json`. Este documento descreve o formato; não cria
arquivos de confiança nem emite contrato de produção.
