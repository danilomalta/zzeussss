# Emissor síncrono de produção NF-e55 / NFC-e65

Base de implementação: `f6115f972cb75f04c0cfdc6179976c330d756be1`, branch
`feat/fiscal-homolog-journal-20261027`. Código na worktree fiscal isolada;
nenhum executável instalado e nenhum serviço existente reiniciado nesta entrega.

## Entrega e alcance

O backend passa a ter orquestração real de emissão normal, ambiente1, de um
único documento por lote síncrono. A sequência é intenção de venda persistida
→ preparação/revisão → aprovação → XML/XSD/assinatura → reserva durável da
requisição → verificação atual de autoridade → um POST com TLS mútuo → protocolo
ou incerteza → consulta explícita e recuperação. Nada dessa sequência registra
uma segunda venda, movimentação de estoque ou pagamento.

O recorte econômico é o fluxo existente de venda em dinheiro, sem descontos,
frete, seguro ou acréscimos e sem tributos que alterem o total suportado pelo
vínculo econômico atual. Produtos, quantidades, preços, parcelas e total são
comparados com o snapshot imutável da venda usando inteiros/racionais, sem
ponto flutuante. Dados incompatíveis são recusados. Não é o motor nacional
completo de classificação tributária: NCM, CFOP, grupos, taxas e referências
precisam ser fornecidos e revisados pelo responsável fiscal. Aprovação não
substitui responsabilidade tributária ou validação legal do documento.

As duas modalidades55/65 têm testes com assinatura e TLS mútuo reais contra
**autoridade sintética, isolada em loopback**. Essa autoridade não é SEFAZ.
Não houve emissão oficial, credenciamento ou homologação real. Sem emitente
autorizado, certificado e aceite do ambiente aplicável, não declarar o recurso
operacional para um cliente.

## API e estados

Contrato tipado: `docs/api/fiscal-production.openapi.json`; mapa completo em
`docs/api/routes.openapi.json`, com544rotas. Todas as rotas abaixo recebem
sessão autenticada e contexto verificado pelo servidor, com `Cache-Control:
no-store`, inclusive quando a autenticação falha.

- `POST /local/v1/fiscal/intents/{id}/production/prepare`: identificador de
  operação, revisão esperada e dados do documento. Chave/número/data/cNF são
  atribuídos pelo servidor; o cliente não escolhe nem sobrescreve esses campos.
- `POST .../production/approve`: revisão, hash exato e referência de revisão
  fiscal; proprietário ou contador autorizado. Não concede autoridade de
  transmissão ao contador nem permite transmitir em nome de outro vendedor.
- `POST .../production/emit`: operação explícita, revisão e hash aprovado.
- `POST .../production/query`: operação separada e revisão emitida; consulta
  a situação da chave sem reenviar a nota.
- `GET .../production`: preparação, aprovação e evidência com revisão efetiva.
- `GET .../production/xml`: XML processado somente se a evidência mais recente
  confirmar autorização com chave, ambiente e digest correspondentes.

Somente100/150 com protocolo válido se tornam `authorized`. Rejeições certas
ficam `rejected_requires_review`/`batch_rejected_requires_review`; nova revisão
exige ação e aprovação explícitas e conserva o número original. Duplicidade,
processamento, denegação, fechamento e códigos desconhecidos permanecem
bloqueados para correção/reenvio automático. HTTP200 pode conter rejeição;
HTTP202 significa resultado incerto, jamais autorização.

Uma perda de resposta, XML/protocolo divergente, erro HTTP ou redirecionamento
não autoriza novo POST. A consulta é arquivada separadamente e a consulta mais
recente controla o resultado; uma consulta pendente/negativa não ressuscita uma
autorização antiga. Identificadores originais permanecem nos recibos, inclusive
após reinício e restauração. Nova sessão não revive uma tentativa antiga.
Antes do primeiro envio, troca de sessão exige preparação/revisão explícitas;
depois da reserva incerta, o caminho é recuperação, sem apagar a tentativa.

## Configuração privada do processo

Desativado por padrão. `titan-local serve` aceita a opção adicional
`--fiscal-production /caminho/absoluto/configuracao-privada.json`, combinada com
os argumentos já existentes de banco/aparelho e chaves públicas emissoras.
Isso não inicia ou configura outro banco. Configuração fiscal inválida é
recusada antes de abrir/migrar o banco. Não há upload HTTP de certificado,
endpoint, CSC ou chave privada.

O JSON privado é `FileConfiguration`, versão1, em
`backend/internal/fiscalproduction/config.go`:

- `binding`: empresa, loja, aparelho, perfil/revisão, CNPJ, UF, modelo, série,
  fuso brasileiro explícito, número inicial conciliado, fingerprint do
  certificado, vigência e referências de autoridade exclusiva de numeração,
  endpoints, homologação e desenvolvedor.
- `certificate_path`: cadeia PEM do e-CNPJ; `private_key_path`: RSA em PEM
  PKCS8 ou PKCS1; `issuer_roots_path`: raízes explicitamente confiáveis.
- `crl_paths`: listas completas, diretas, atuais e assinadas, cobrindo todos
  os certificados não raiz da cadeia. Recarregadas antes do envio. Listas
  ausentes, vencidas, futuras, adulteradas, delta/particionadas ou sem cobertura
  são recusadas; não há fallback para certificado presumidamente válido.
- Para65, `qr`: versão, URL pública, URL de consulta e, na versão2, identificador
  CSC e caminho privado de seu valor. Versão3 recusa CSC desnecessário.

Configuração e arquivos referenciados exigem caminho absoluto, arquivo regular,
permissão0600 e não podem ser links simbólicos. Erros não expõem conteúdos nem
caminhos privados. A configuração pública vinculada ao documento é arquivada
por hash; certificado privado/CSC não entram no journal, exportações ou logs.
Somente metadados públicos do suplemento participam do hash da preparação.

URLs de transmissão são HTTPS governamentais explícitas, separadas por serviço,
sem credencial, porta alternativa, query ou fragmento. Sem proxy herdado,
redirecionamento, TLS inseguro ou retry. A API não escolhe URLs fornecidas pelo
navegador. O carregador nativo usa a confiança TLS do sistema; injeção de raízes
ou dialer pertence exclusivamente ao host Go e é usada pelos testes isolados,
não é campo do JSON nem parâmetro HTTP.

O cadastro aceita as27UFs, mas isso **não comprova cobertura ou homologação das
27UFs**. A URL/modelo/ambiente/vigência devem ser verificados na autoridade do
emitente. Não foi fixada uma URL de um estado como se servisse ao país inteiro.
Referências textuais registram o ato de provisionamento; não provam por si só
credenciamento, exclusividade ou aprovação jurídica.

A série tem uma única autoridade neste banco. Antes de provisionar, a numeração
anterior e outros emissores precisam estar conciliados. Não há protocolo de
reserva distribuída entre instalações independentes nem liberação automática
por expiração. Não usar a mesma série simultaneamente em bancos separados.

## Persistência e recuperação

Migração73 acrescenta tabelas exclusivas de produção: autoridade/série,
alocação, preparação, aprovação, requisição e resposta. Trava compartilhada de
escrita e restrições únicas protegem concorrência inclusive entre conexões
SQLite independentes. Triggers recusam alteração/exclusão dos registros.
Falha na preparação reverte também a alocação; repetição da mesma operação
não avança a numeração. As72migrações anteriores permanecem idênticas.

A sessão/empresa/loja/operador/aparelho e a revisão do perfil são verificadas
antes da reserva e imediatamente antes de entregar a requisição ao transporte.
A validade do certificado e a revogação também são reavaliadas. Uma resposta
que chega depois do logout ainda pode ser persistida: revogar autoridade futura
não deve apagar o resultado de uma operação já entregue ao transporte.

Backup suporta schemas25–73. O journal integra o backup cifrado; as chaves
privadas do emissor não são incluídas. Restauração invalida sessões antigas e
mantém a barreira de nova emissão. Nova sessão autorizada pode ler XML histórico
sem credencial ativa e consultar explicitamente com credencial atual, mas isso
não remove a barreira ou reenvia o documento. Guardar/recuperar credenciais da
instalação tem procedimento próprio, externo ao backup comercial.

## Verificação e limites restantes

Resultados e hashes desta entrega:
`docs/architecture/evidence/fiscal-production-20261028/verification.json`.
Os testes cobrem modelos55/65, aprovação, sessão/contexto, concorrência,
rollback, revogação, resposta perdida, rejeição, duplicidade, protocolo inválido,
reinício, leitura histórica, restauração e ausência de reenvio. A rodada com
`-pilot-backup-cli` usa o executável real de backup/restauração.

Ainda exigem implementação ou aceite próprios: importação protegida dePFX,
adaptadores A3/HSM/armazenamento nativo Windows, cobertura adicional de perfis
CRL quando necessária, atualização automática segura de listas, regras
tributárias nacionais completas/RTC, cenários econômicos além do recorte,
DANFE de produção, contingência e eventos de produção integrados a este journal,
SPED/PVA e NFS-e produção. O DANFE técnico anterior continua exclusivamente de
homologação. Interface gráfica de provisionamento/revisão não foi acrescentada;
a entrega atual é backend/API, sem alterar frontend ou Electron.

Não foi acessado banco comercial, certificado real, CSC, senha ou token.
Nenhum executável foi instalado; serviços8182/8282 e as duas árvores originais
foram preservados. Não foi feito merge na main. Credenciais antigas divulgadas
no histórico continuam sendo uma pendência externa já registrada.

## Referências oficiais

- MOC7, visão geral e modelo de comunicação NF-e:
  https://www.confaz.fazenda.gov.br/legislacao/arquivo-manuais/moc7-visao-geral.pdf
- Endereços4.00 publicados pela Receita/PR, segregados por ambiente/serviço:
  https://sped.fazenda.pr.gov.br/NFe/Pagina/Enderecos-dos-ambientes-de-homologacao-e-producao-Versao-400

A páginaPR foi novamente conferida em28/10/2026; o novo acesso ao PDF MOC nessa
rodada teve timeout. Os schemas oficiais já versionados foram preservados;
apenas um wrapper declara a raiz de resposta usando o tipo oficial existente.
Consulta documental e ensaio sintético não substituem homologação oficial.
