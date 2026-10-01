# 9L — Desafio e prova pública para pareamento

## Escopo

Acrescentar a `titan-peer` os comandos `pair-start`, `pair-answer` e
`pair-finish`, usando as tabelas de aparelhos e auditoria já existentes.
Não há migração nova, serviço de rede, cópia de chave privada, instalação
automática de segundo SQLite, nem replicação de dados comerciais.

Pré-requisitos: instalação administrativa válida e aparelho local aprovado;
no parceiro, arquivo station privado e descritor público correspondentes,
com a mesma empresa e loja e um ID de aparelho diferente. Esta etapa não gera
uma identidade de instalação adicional nem distribui credenciais humanas para
outro SQLite. Inicializar duas empresas independentes com `titan-local init`
não cria uma loja compartilhada. Ainda falta o fluxo completo de instalação
do segundo aparelho; não modificar IDs ou copiar bancos manualmente.

## Três ações separadas

1. No aparelho administrativo, o dono autentica `pair-start`: fornece
   `--db`, `--station`, `--owner`, `--in DESCRITOR`, `--out NOVO_DESAFIO`,
   `--name NOME`, `--signing-sha256` e `--encryption-sha256` conferidos por
   canal autenticado independente. A senha vai pelo stdin, como na 9J/9K.
   A operação cria somente um pedido pendente, com desafio aleatório de
   32 bytes e validade de cinco minutos. O arquivo público também é assinado
   pelo aparelho administrativo, incluindo origem, destino, chaves e prazo.
2. No parceiro, `pair-answer --station PRIVADO_LOCAL --in DESAFIO_PUBLICO
   --out NOVA_PROVA --requester-sha256 IMPRESSAO_CONFERIDA` verifica formato,
   assinatura, origem autenticada pela impressão, prazo, IDs e chave pública
   do próprio aparelho. Assina apenas a prova de posse do desafio existente.
   Não abre banco nem aprova aparelho. A chave privada permanece no parceiro.
3. O dono recebe a prova pública e autentica `pair-finish --db CAMINHO
   --station CAMINHO --owner ID --in PROVA_PUBLICA --signing-sha256 IMPRESSAO
   --encryption-sha256 IMPRESSAO`. A operação verifica ambas as assinaturas,
   correspondência ao aparelho solicitante atual e ao pedido persistido,
   vínculo do dono, prova, chave e prazo; registra prova e aprovação na mesma
   transação. Não concede recepção de eventos nem aprova o vínculo X25519.
   Essas decisões continuam separadas, pelos comandos da 9K.

Compartilhar somente descritor, desafio e prova públicos. Nunca compartilhar
station privado, chave X25519 privada, chave TLS privada, senha ou banco real.
Os arquivos públicos têm metadados de empresa, loja, aparelhos e impressões;
enviar somente aos administradores autorizados. Comparar impressões que chegam
junto com o próprio arquivo não autentica seu emissor.

## Persistência e falhas

Concluir o pareamento exige papel atual exatamente `owner`, aparelho local
aprovado e solicitante original ainda dono ativo. Não ressuscita pareamento
revogado, não substitui chave existente, não admite empresa ou loja diferente.
O pareamento que já estiver aprovado com o mesmo desafio e chave admite
repetição sem auditoria duplicada. Pedidos ainda pendentes/verified expiram;
repetir uma aprovação já gravada não concede uma nova autorização.

Auditorias de prova e aprovação e alteração de estado compartilham transação.
Falha na auditoria reverte ambas, inclusive quando a prova estava pendente.
Os eventos existentes continuam consultando as permissões e estados atuais.

Arquivos públicos são criados com 0600 e O_EXCL, sem sobrescrever. O commit do
pedido no SQLite e a gravação do desafio no arquivo não são atomicamente
combinados. Se a gravação falhar, a CLI retorna erro e o pedido pode permanecer
pendente; não considerar concluído, não trocar IDs e não forçar repetição.
Também pode restar arquivo parcial se escrita ou sincronização falhar. Esta
etapa não oferece cancelamento, renovação de pedido, recuperação de arquivo,
rotação de chaves ou nova instalação do parceiro. Investigar antes de continuar.

O parceiro precisa de relógio razoavelmente alinhado: rejeita desafio expirado
ou prazo mais de cinco minutos à frente de seu relógio. O instante exato do
desafio vem do banco do solicitante e é conferido novamente na aprovação.

## Verificação

Testes usam bancos e chaves descartáveis. Cobrem troca completa por CLI,
solicitação pendente após resposta, segunda autenticação do dono, repetição,
origem não conferida, station de outro aparelho, revogações, gerente, expiração,
auditoria com falha, assinaturas e JSON adulterados, proibição de sobrescrita
e ausência de segredos em arquivos públicos. O station do candidato criado
pelo teste é preparação de cenário, não um instalador de segundo aparelho.
A suíte Go, vet e compilação devem passar no PC antes do commit. Isso ainda
não demonstra instalação multiplataforma nem continuidade real após queda do PC.
