# Fase 9C — Chaves de criptografia persistentes e aprovadas

## Entrega

A 9B cifrou mensagens, mas usou chaves geradas nos testes. Esta fase adiciona
funções reutilizáveis para criar/carregar uma chave privada X25519 por aparelho
e para aprovar seu vínculo público no SQLite. Ainda não adiciona comandos à CLI,
endpoints, serviço de transmissão, sincronização automática ou aplicação dos
eventos recebidos ao estoque/financeiro. Não permite comunicação comercial entre
empresas diferentes: o escopo continua sendo empresa e loja iguais.

## Arquivo privado

`CreateEncryptionKey` cria um arquivo separado, com exclusão de criação (não
sobrescreve), permissão 0600 e conteúdo associado à empresa, loja e aparelho.
`LoadEncryptionKey` valida arquivo regular, permissão, tamanho, JSON sem campos
extras/duplicados e identidade esperada. Não modifica `station.json`, não troca a
chave Ed25519 já usada para assinaturas e não regenera chaves quando a leitura
falha. O processo chamador deve provar o aparelho antes de usar essas funções.

O arquivo contém segredo. Não imprimir, enviar à nuvem, versionar, incluir em
imagem Docker nem anexar em chamados. Permissões de arquivo não são criptografia
em repouso e não protegem contra um administrador do computador. A proteção foi
projetada para o ambiente Linux atual; ACLs/armazenamento seguro e empacotamento
para Windows, macOS e smartphones ainda precisam de implementação e validação.

Falha durante a criação pode deixar um arquivo incompleto. A aplicação deve
reportar a falha e exigir recuperação, sem apagar ou substituir silenciosamente.
São necessários backup protegido e procedimento de recuperação antes de uso
comercial. Perder a chave privada impede abrir mensagens destinadas a ela.

## Confiança na chave pública

O destinatário assina um vínculo com versão, empresa, loja, aparelho, revisão
positiva e chave pública X25519. A assinatura usa a chave Ed25519 que já está
aprovada para aquele aparelho. O dono aprova explicitamente esse vínculo no
aparelho de origem. Cargo de gerente, isoladamente, não permite essa aprovação.

A migração 0018 guarda apenas chaves públicas, assinatura, revisão e aprovador;
registra auditoria com impressão SHA-256 da chave. Instalação e auditoria usam a
mesma transação. Repetição idêntica não duplica auditoria; revisão antiga ou chave
conflitante na mesma revisão é rejeitada. Uma revisão maior exige assinatura e
aprovação novamente. Um novo dono pode reapresentar e aprovar explicitamente o
mesmo vínculo, com nova auditoria.

`TrustedEncryptionPublic` verifica os pareamentos atuais dos dois aparelhos,
assinatura persistida e vínculo ativo do dono aprovador. Revogação, alteração da
chave Ed25519 pareada ou adulteração da assinatura bloqueia a consulta. O código
nunca considera uma chave enviada livremente por um intermediário como confiável.

Essa confiança não substitui as permissões de recebimento da 9A nem autoriza
compartilhamento de dados entre empresas. O recibo continua confirmando apenas
persistência durável da mensagem, sem declarar sua aplicação no domínio.

## Limites de revisão e rotação

Controle de revisão impede reapresentar vínculos antigos sobre o estado atual.
Não impede um administrador de restaurar uma cópia antiga de todo o banco.
As funções desta fase não oferecem procedimento completo de rotação: arquivos
privados antigos devem ser preservados com segurança até tratar mensagens
pendentes. Não substituir arquivos de chave em produção por tentativa e erro.
Não distribuir uma mesma chave privada a vários aparelhos.

## Verificação

Testes adicionados: arquivo privado persistente e sem sobrescrita; rejeição de
arquivo inseguro ou JSON ambíguo; envio cifrado com chave recarregada e aprovação
persistida após reabrir SQLite; idempotência e revisões; rollback de auditoria;
assinante falso, escopo estrangeiro e gerente; revogações e adulteração no banco.
Todos usam diretórios e bancos descartáveis.

Os comandos de teste, vet e compilação devem ser executados no PC e seus resultados
observados antes de declarar esta fase validada. O pacote não representa uma
auditoria criptográfica independente nem comprova transmissão real pela rede.
