# Entrega 06 — Responsabilidade dos modos e servidor de loja com TLS

Base do PC: `b839d85`. Backend oficial: `backend/`. Nenhuma migração nova;
esquema 27. Nenhuma conversão SQLite/PostgreSQL, mudança de frontend ou do banco
da demonstração. Esta entrega implementa o transporte servidor para o núcleo
comercial existente, não quatro ERPs completos por selecionar uma opção.

## Matriz dos modos

| Modo | Dados comerciais e autoridade | Operação sem internet | Falha do servidor/rede | Estado |
| --- | --- | --- | --- | --- |
| Local | SQLite no disco do aparelho; operações confirmadas nele são a fonte do registro local | Sim, respeitando contrato e permissões vigentes | Continua neste aparelho; não comprova outro aparelho independente | Executável existente, agora com perfil validado e admissão loopback |
| Servidor | SQLite no disco local do servidor da loja; terminais chamam sua API HTTPS | Sim, se LAN/servidor estiverem disponíveis | Terminais não vendem de forma independente; sem fallback para outro banco | Implementado nesta entrega no backend; cliente LAN e piloto em equipamentos pendentes |
| Nuvem | Planejado: PostgreSQL do serviço comercial contratado pelo cliente, separado do serviço Titan de licença/troca autorizada | Operação central depende da conexão | Continuidade exige operações locais reconciliáveis, ainda não implementadas | Reconhecido para descrição, mas bloqueado neste executável |
| Híbrido | Planejado: bancos locais por aparelho e autoridade definida por tipo de operação; serviço remoto recebe eventos autorizados | Meta: confirmar localmente e reconciliar depois | Meta: smartphone continua se PC/servidor falhar | Bloqueado até reconciliação comercial e instalação do segundo aparelho |

`InternetRequired` e `TerminalOffline` não são sinônimos. O servidor da loja
funciona sem internet, mas o terminal remoto depende desse servidor/LAN.
Descrição de modo é capacidade do código, não prova de serviço iniciado,
certificado instalado ou licença comercial vigente.

A API PostgreSQL existente em cmd/api não é automaticamente ativada por um
perfil cloud. Ainda possui serviços incompletos, autenticação própria e esquemas
distintos. Nada replica dados comerciais para ela nesta entrega. Também não há
upload para o serviço Titan, envio de DSN ou concessão de acesso a dados do cliente.

## Perfil privado validado

Novo pacote `internal/deployment`: leitura limitada a 16KiB, arquivo regular
0600, caminho absoluto, sem links simbólicos inclusive nos pais; compara inode
aberto. Seis campos obrigatórios, sem duplicatas, desconhecidos, null, tipos
errados ou JSON adicional. Versão do formato: 1.

Exemplo local (conteúdo de arquivo que deve ser salvo com permissão 0600):

```json
{
  "version": 1,
  "mode": "local",
  "listen": "127.0.0.1:8181",
  "tls_cert": "",
  "tls_key": "",
  "allowed_networks": []
}
```

Exemplo servidor (IP, rede e caminhos de exemplo; substituir pelos reais):

```json
{
  "version": 1,
  "mode": "server",
  "listen": "192.168.1.10:8443",
  "tls_cert": "/diretorio-privado/server-cert.pem",
  "tls_key": "/diretorio-privado/server-key.pem",
  "allowed_networks": ["192.168.1.0/24"]
}
```

O servidor exige IP explícito privado ou loopback; recusa hostname,
wildcard 0.0.0.0/::, IP público e porta fora de 1–65535. Não seleciona interface
ou porta automaticamente. Para teste TLS descartável permite loopback, mas não
considerar esse teste uma demonstração em duas máquinas da loja.

De uma a dezesseis redes distintas, CIDR canônico. Somente sub-redes contidas
em RFC1918, ULA IPv6 e loopback. Recusa prefixos amplos que incluam endereços
públicos ou bits de host não zerados. Restrição de rede não substitui login,
controle de aparelho, contrato, papéis ou autorização de registros.

Certificado e chave: PEMs regulares 0600, caminhos absolutos sem links, máximo
1MiB cada. Verifica par de chaves, validade temporal, SAN para o IP de listen e
uso ServerAuth quando o certificado declara usos. TLS mínimo 1.2; sem opção
InsecureSkipVerify. O cliente precisa confiar na CA correta e verificar o
certificado; o servidor não instala confiança no browser/PC/smartphone.
Emissão, renovação, distribuição de CA, revogação de certificados e mTLS ainda
exigem uma entrega própria. Chave TLS nunca entra no manifesto ou resposta.

Perfis cloud/hybrid: version1, mode correspondente, listen/tls_cert/tls_key
vazios e allowed_networks[]. Podem ser descritos, porém serve retorna erro
de modo indisponível ANTES de abrir/migrar banco. Não há fallback silencioso,
nem dual-write ou conversão de banco baseada na seleção.

## Comandos e compatibilidade

Sem --deployment, `serve --port 8181` mantém o modo local HTTP loopback.
Com --deployment, listen vem somente do perfil; combinar --port é recusado.
Nomes do banco/aparelho continuam explicitamente fornecidos e existentes.
Certificado, perfil, flags e modo são validados antes de abrir o banco.
Servidor usa o mesmo catálogo, estoque, caixa, venda, compras e autorização
SQLite, sem implementar uma segunda lógica comercial concorrente.

```text
titan-local mode --deployment /perfil-privado/operation.json
titan-local serve --db /instalacao/data/store.sqlite --station /instalacao/data/station.json --deployment /perfil-privado/operation.json --issuer-keys /chaves-publicas/issuers.json
```

`mode` não abre banco. JSON de saída contém somente modo, disponibilidade da
implementação, armazenamento/autoridade e capacidades. Não contém caminhos,
chaves, IDs do dono ou tokens. Nenhum endpoint novo de configuração pelo usuário.

Instalação gerenciada da entrega05: usar `run titan-local serve --deployment
CAMINHO`, mantendo seu bloqueio herdado. Não executar duas versões do núcleo
sobre o banco durante manutenção. Parar o serviço para mudar o perfil. `mode`
é uma consulta CLI fora de run; não precisa acessar o banco da instalação.

Servidor: banco sempre em disco local, não abrir SQLite por compartilhamento
SMB/NFS entre terminais. Os terminais enviam requisições HTTPS. Uma estação do
servidor representa uma empresa/loja no processo; sessões humanas são vinculadas
a essa estação. Isto **não prova uma identidade de aparelho distinta para cada
terminal remoto**, nem autoriza uso fora da loja. Cada terminal usa login humano
separado. Identificação/enrollment individual de terminais, múltiplas lojas,
limites de instalações simultâneas e sincronização de permissões seguem pendentes.

## Barreiras de transporte e encerramento

Gate de admissão instalado antes de health, login, recuperação e rotas protegidas.
Usa o IP real da conexão; não configura ProxyHeader, não confia em X-Forwarded-For
ou Forwarded. Rede recusada retorna403. Local permite somente loopback.
No máximo32 requisições simultâneas no gate e120 inícios/segundo por IP;
limites específicos de login e ações sensíveis continuam aplicados.
Cache-Control:no-store, corpo máximo1MiB; leitura/escrita15s, idle1min,
concorrência de conexões128. Não é garantia de proteção DDoS na rede.
Diagnósticos brutos de parsing/conexão do fasthttp são suprimidos para evitar
registrar fragmentos de requisições; telemetria estruturada sem segredos pendente.
Firewall, perímetro, limite de banda, proteção do host e monitoramento pendentes.
Não usar proxy reverso nesta configuração sem desenvolver confiança nele.

Listener TLS não atende a API em HTTP simples na mesma porta. Interrupção/SIGTERM
fecha admissão e solicita encerramento com prazo5s, esperando a goroutine do
servidor antes de fechar o banco. Não mata outros processos nem resolve porta
ocupada encerrando o dono anterior. Alteração de modo não migra credenciais,
sessões ou dados entre serviços. Sessões expiram/revogam pelas regras existentes.

Sem chaves emissoras confiáveis, login/consultas autorizadas continuam disponíveis,
mas mutações contratadas ficam bloqueadas; HTTPS não cria uma licença válida.
URI `/local/v1` foi mantida para compatibilidade, inclusive no modo servidor.
Health conserva a resposta antiga; não confundir status local com sincronização.

## Verificação e aceite pendente

Testes descartáveis de perfis, IPv4/IPv6, CIDRs, JSON ambíguo, links/modos,
certificado expirado/SAN errado/chave inválida, modo indisponível sem criar banco,
CLI em HTTPS real, CA não confiável, recusa de HTTP simples, login/consulta/
revogação via HTTPS, IP real contra cabeçalhos falsos e encerramento.
Os testes usam certificado e senha fictícios gerados em diretórios temporários.
Sem conexão ao PostgreSQL real, ao banco da demonstração ou a outra empresa.

Aceites seguintes: terminais físicos na LAN e cliente integrado, identificação
de cada terminal, carga sustentada/queda de rede, serviço instalado, PKI/renovação,
reconciliação comercial, aparelho móvel independente e núcleo online completo.
Não marcar F02/F03/F05 como concluídos por esta entrega.

Verificação observada: suíte Go completa e go vet passaram; testes com detector
de concorrência em cmd/titan-local e internal/deployment passaram. A suíte Python
passou28 testes, incluindo compilação CGO_ENABLED=0 dos seis executáveis reais;
os14 testes de instalação foram repetidos após corrigir o encerramento e passaram.
Os testes HTTPS executam listener TCP real com confiança na CA descartável,
sem desabilitar verificação do certificado. Após suprimir diagnóstico bruto,
testes de transporte/encerramento com race e vet da API passaram novamente.
Frontend, PostgreSQL real e equipamentos físicos não foram exercitados.
