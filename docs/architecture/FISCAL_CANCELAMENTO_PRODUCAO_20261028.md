# Cancelamento de produção e preparação do ensaio oficial

Base: `49ddbb508b8aa97f38e6c817c1e70957919236f0`, branch fiscal existente.
Os sete checks dessa base passaram, inclusive o aceite HTTP/PostgreSQL/SQLite/
pontes/recuperação no run40777738365. Isso não certifica o novo delta.

## Código acrescentado

Cancelamento de produção NF-e55/NFC-e65 por evento110111/sequência1, vinculado
a uma consulta autorizada recente do documento original. A decisão explícita
contém operação, identificador da consulta, justificativa e referência de
aprovação. O processo deriva chave, CNPJ, protocolo, ambiente e data; não aceita
esses dados, certificado, endpoint ou XML arbitrário pelo navegador.

O proprietário/gerente precisa ter sessão/contexto e permissão contratada atuais.
Acesso contábil não concede cancelamento. Certificado e CRLs atuais, identidade
pública da instalação e evidência de autorização são conferidos antes da reserva
e imediatamente antes do transporte. Endpoint opcional `cancellation_url` fica
somente na configuração privada e precisa ser HTTPS governamental, distinto dos
serviços de autorização e consulta. Ausente, o recurso permanece indisponível.
A ausência do novo campo conserva a serialização/hash dos bindings anteriores.

Novas rotas autenticadas, todas sem cache:

- `POST /local/v1/fiscal/intents/{id}/production/cancel`;
- `GET /local/v1/fiscal/intents/{id}/production/cancellation`;
- `GET /local/v1/fiscal/intents/{id}/production/cancellation/xml`.

Contrato: `docs/api/fiscal-production.openapi.json`; mapa total547rotas. A última
rota só entrega `procEventoNFe` quando o registro correspondente estiver
comprovado. HTTP200,136,573, redirecionamento, XML inválido ou perda de resposta
não se tornam cancelamento registrado.135/155 precisam corresponder exatamente
a chave, ambiente, tipo, sequência e data/protocolo válidos. O serviço não decide
se um pedido é legalmente tempestivo em qualquer UF: a decisão humana e a
resposta da autoridade continuam necessárias.

A migração74 acrescenta requisição e resultado imutáveis, operação única e um
evento por intenção. Usa a trava SQLite já existente. Reserva falha não envia;
reserva incerta não desaparece e não autoriza outro POST. Repetição idêntica lê
o recibo original; alteração de justificativa/consulta/aprovação com o mesmo
identificador é conflito. O XML ativo fica bloqueado a partir da reserva, mesmo
se o processo cair antes da resposta. Resultado comercial e resultado fiscal
continuam separados: não estorna dinheiro, não repõe estoque, não altera venda
nem libera número. Rejeição exige análise; não há nova tentativa automática.

Uma resposta recebida depois do logout ainda é gravada. Leitura com sessão nova
recupera os mesmos identificadores e XML, sem assinatura ou transmissão nova.
Backup cifrado aceita schemas25–74, preserva o evento e a barreira de restauração;
credenciais privadas não entram no backup comercial. Nenhuma migração antiga
foi alterada.

## Recuperação por consulta

Depois de uma resposta perdida, a consulta do documento pode indicar que ele
está cancelado. Isso sozinho não fabrica um recibo do nosso evento. A recuperação
exige o processo de evento na consulta, com os mesmos dados, certificado,
assinatura e digest do evento originalmente enviado, e o registro correspondente.
Evento diferente, ainda que assinado e com o mesmo ID/chave, não recompõe o nosso
recibo. A troca do estado do documento é distinta da recuperação do protocolo do
evento específico.

Quando a prova exata chega, o recibo informa `recovery_query_operation_id`. Seu
`exchange` conserva o resultado incerto da tentativa original; não é substituído
pela resposta da consulta nem recebe um HTTP200 fictício. A consulta é arquivada
no journal próprio, com seu hash e identificador. A leitura não altera nenhum
registro anterior nem retransmite.

O leiaute de consulta arquivado contém uma restrição antiga de ID de evento
somente numérico. Os43XSDs anteriores foram conservados byte a byte. A validação
composta verifica o envelope pelo leiaute existente e cada processo de
cancelamento pelo schema oficial atualizado já incorporado ao componente de
eventos, sem relaxar atributos, assinatura exigida ou filhos desconhecidos.
Formatos de evento não suportados são recusados. O limite50 nessa composição é
uma proteção local de recursos, não regra jurídica nacional. Testes demonstram
recusa de envelope/filhos extras, namespace incorreto, excesso e evento sem
assinatura. Uma tentativa de baixar novamente o pacote oficial para conferência
falhou com `HTTP Error 302: The HTTP server returned a redirect error that would
lead to an infinite loop.` Nenhum pacote diferente foi aplicado nem schema
oficial editado; a composição usa os arquivos oficiais já versionados.

## Homologação oficial: estado real

**Não executada.** O registro anterior do usuário informa ausência de emitente
piloto/certificado. Foi solicitada atualização apenas de UF/modelos/caminho de
configuração privada, sem envio de segredo pela conversa. Não foram procuradas
chaves em arquivos pessoais, utilizados certificados sintéticos na SEFAZ,
inventados CNPJ/inscrição/credenciamento ou acessados serviços comerciais.

O requisito de acesso precisa ser verificado na UF e no modelo escolhidos.
O Portal Nacional relaciona credenciamento e certificação ICP-Brasil. Paraná
informa acesso ao ambiente de homologação por ofício para contribuintes ativos
nas condições do seu cadastro; Pernambuco orienta solicitar credenciamento nos
dois ambientes. Portanto, não existe nesta entrega uma afirmação de que todos
os estados têm o mesmo procedimento ou de que a software house está homologada
para qualquer cliente.

O próximo ensaio oficial depende de um estabelecimento autorizado e dos dados
fiscais revisados, certificado válido/configuração segura, ambiente2 e endpoints
verificados, inscrição/credenciamento aplicáveis e, para NFC-e conforme o perfil,
configuração QR/CSC do ambiente correto. A configuração de produção ambiente1
**não deve ser reutilizada como se fosse homologação**, nem ter banco comercial
selecionado para o ensaio. O provisionamento nativo atual de produção não é um
aplicador automático de credenciais ou um comando de homologação oficial.

A evidência de ensaio deverá identificar versão, UF/modelo/ambiente, casos,
requisição original, respostas/protocolos reais, rejeições e recuperação;
publicação apenas de dados saneados, sem chaves/CSC/banco/dados pessoais.
Autorização de documentos em ambiente2 não equivale a aptidão de todos os
cenários nem habilita automaticamente produção em todas as UFs.

Fontes primárias consultadas em28/10/2026:

- Portal Nacional, perguntas frequentes sobre credenciamento/certificação e
  ambiente de homologação:
  https://www.nfe.fazenda.gov.br/PORTal/perguntasFrequentes.aspx?AspxAutoDetectCookieSupport=1&tipoConteudo=4figqHYhYho%3D
- Receita/PR, credenciamento de emissores:
  https://sped.fazenda.pr.gov.br/NFCe/Pagina/Credenciamento-de-emissores
- SEFAZ/PE, credenciamento:
  https://www.sefaz.pe.gov.br/Servicos/Nota-Fiscal-de-Consumidor-Eletronica/Paginas/Credenciamento-de-Contribuintes.aspx

## Verificação e limites

Resultados finais, skips e hashes em
`evidence/fiscal-production-cancellation-20261028/verification.json`.
Assinatura, TLS mútuo, SQLite e executável de backup são reais nos ensaios; a
autoridade fiscal e os dados são sintéticos, isolados em loopback. Não chamar
isso de homologação oficial. As primeiras rodadas corrigiram fixtures de papel
contábil, restrição antiga de ID no schema de consulta e sincronização do mapa
de rotas; somente a repetição integral final comprova o delta congelado.

Continuam pendentes o ensaio SEFAZ real, DANFE de produção, contingência,
inutilização e outros eventos de produção integrados ao novo journal, motor
tributário nacional completo/RTC, cenários econômicos adicionais, SPED/PVA e
NFS-e de produção. A chave atual é PEM com CNPJ do emitente exato; PFX/A3/HSM,
armazenamento nativo Windows e uso de certificado da matriz para filiais exigem
trabalho/aceite próprios. Certificado, UF/modelo ou CNPJ não configurados não
são inferidos. Não há implantação comercial ou promessa de adequação nacional
completa nesta unidade.

Nenhum executável instalado, serviço8182/8282 reiniciado, banco comercial lido
ou alterado, frontend/Electron modificado ou merge na main. Árvores originais
preservadas. Credenciais históricas ainda precisam ser trocadas pelo proprietário.
