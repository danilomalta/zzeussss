# Fase 9B — Conteúdo criptografado em trânsito

Base: 0beea08. Acrescenta Seal, OpenSealed e ReceiveSealed sem alterar
o protocolo de assinatura ou os recibos da 9A. Sem migração, dependência
nova, rota HTTP, worker, frontend ou acesso ao banco operacional.

## Proteção do envelope

X25519 com chave efêmera por envio, HKDF-SHA256 com salt aleatório de 32 bytes
e AES-256-GCM com nonce aleatório de 12 bytes. Implementações utilizadas:
crypto/ecdh, crypto/aes, crypto/cipher, crypto/rand da biblioteca padrão Go
e golang.org/x/crypto/hkdf, já presente na dependência v0.49.0.
Não há implementação própria das primitivas criptográficas.

Destino, versão, chave pública efêmera, fingerprint da chave receptora,
salt e nonce são autenticados como dados associados. Conteúdo, metadados
do evento e assinatura Ed25519 ficam dentro do envelope criptografado.
Mesmo evento reenviado tem criptografia nova e recibo de recepção estável.

O payload JSON original é codificado como bytes no conteúdo interno.
Assim, espaços e quebras de linha não são alterados por json.Marshal,
preservando o hash e a assinatura originais. Decodificação interna rejeita
campos extras, duplicados, ausentes, nulos e conteúdo JSON concatenado.

## Chaves e confiança

A chave X25519 do destinatário é separada da chave Ed25519 do aparelho.
A privada X25519 permanece com o destinatário. Seal deve receber a pública
por configuração confiável previamente autorizada, nunca diretamente de
um servidor intermediário não confiável.

Esta etapa NÃO provisiona chaves X25519, NÃO as salva no station.json e NÃO
vincula automaticamente a pública X25519 ao pareamento Ed25519 existente.
Esse vínculo, persistência privada, distribuição autenticada, rotação e
recuperação são requisitos antes de habilitar comunicação em produção.
Gerar uma chave nova a cada inicialização perderia acesso a mensagens
pendentes destinadas à chave anterior; isso não está autorizado.

Fingerprint identifica a chave, mas não torna uma chave desconhecida
confiável. Criptografar para a pública errada pode expor dados ao dono
daquela chave. A configuração precisa verificar o destinatário.

## Recepção

ReceiveSealed abre o envelope somente com a chave e o destino esperados.
Depois usa Receive da 9A, que verifica assinatura Ed25519, pareamento,
concessão por tipo, origem e destino antes de persistir.
Criptografia válida não substitui autorização do remetente.

Falha de abertura não permite fallback para conteúdo em texto aberto.
Recibo só é devolvido depois da persistência transacional da 9A e continua
compatível com outgoing.Confirm. Não representa aplicação dos dados.

## Limites de privacidade

Um intermediário que possua apenas o envelope vê destino, fingerprint,
versão, tamanhos e bytes criptografados. Não recebe a privada X25519.
Isso não elimina metadados de rede, IP, horário ou volume.

O JSON aberto continua armazenado no SQLite local autorizado. Não há
criptografia do banco nem do backup nesta etapa. Perda da chave privada
de criptografia não pode ser solucionada pelo envelope.

O protocolo mantém a restrição da 9A: mesma empresa e loja. Comunicação
com fornecedor continua exigindo vínculo comercial e escopo próprios.
Nenhuma API passou a transmitir dados e nenhum relay foi instalado.

Antes de HTTP/worker: provisão e vinculação das chaves, armazenamento
privado, decodificação estrita do envelope externo, limites do transporte,
TLS e configuração confiável dos pares. A API de objeto Go desta etapa
não deve ser exposta como aceitação indiscriminada de JSON pela rede.

## Testes

Testes cobrem preservação dos bytes assinados, ausência de conteúdo aberto
no envelope, recepção com confirmação real da outbox, aleatoriedade por
envio, recibo estável, chave/destino errados, cabeçalhos adulterados,
assinatura inválida, ausência de concessão, curvas incorretas, chaves de
baixa ordem, tamanhos inválidos e JSON interno ambíguo.

Os testes geram chaves descartáveis em memória e usam bancos temporários.
Não são criadas chaves reais, nem alterados arquivos privados da instalação.

Aplicação e diff do patch são verificados na preparação. Go está
indisponível nesse ambiente: testes, vet e compilação devem passar no PC.
Esta etapa não equivale a uma auditoria criptográfica independente.
