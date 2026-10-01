# 9K — Aprovação administrativa de parceiros já pareados

## Objetivo e pré-requisitos

Estender `titan-peer` com decisões autenticadas pelo dono: confiar no vínculo
de criptografia do parceiro, autorizar eventos de entrada e revogar tipos
selecionados. A chave pública de assinatura do descritor precisa corresponder
à chave de um aparelho já pareado e aprovado no SQLite local. A assinatura
própria do descritor nunca substitui desafio, prova e aprovação do pareamento.

Esta etapa não cria aparelhos ou usuários, não altera contratos de módulos,
não replica tabelas e não oferece pareamento completo por CLI. Para aparelhos
novos, ainda falta expor o fluxo de desafio existente com troca pública segura.
Não inserir registros manualmente no banco para contornar esse pré-requisito.

## Interface administrativa

Os três comandos exigem `--db`, `--station`, `--owner`, `--in`,
`--signing-sha256` e `--encryption-sha256`. A senha vai pelo stdin, nunca por
argumento. As duas impressões devem ter sido conferidas por canal autenticado
independente do arquivo recebido; copiar as impressões do próprio descritor
sem autenticar seu emissor não demonstra identidade.

- `trust`: aprova somente o vínculo público X25519. Não concede recepção de
  eventos. É útil para confiar na chave pública de um destino no emissor.
- `approve --events LISTA`: aprova o vínculo e concede tipos de evento de
  entrada, em uma única transação. A direção é parceiro → aparelho local.
- `revoke --events LISTA`: revoga apenas os tipos selecionados nessa direção.
  Mantém histórico, eventos recebidos, pareamento e vínculo de criptografia.
  Não equivale a revogar completamente o aparelho ou sua chave.

A lista aceita somente `sale.committed`, `stock.operation`, `cash.open` e
`cash.close`, separados por vírgula, sem espaços e sem duplicatas. Não há
curinga nem autorização implícita de todos os eventos. Eventos não selecionados
permanecem com suas permissões atuais. Uma lista de concessões não é uma
substituição completa das permissões anteriormente aprovadas.

O escopo continua restrito à mesma empresa e loja e a aparelhos distintos.
Mercado e fornecedor, por serem empresas diferentes, ainda não usam este fluxo.
Uma mensagem recebida continua comprovando armazenamento durável, não aplicação
automática de venda, estoque ou financeiro no receptor.

## Segurança e persistência

O comando verifica arquivo público regular, limite de 16 KiB, JSON estrito,
assinatura própria, impressões e prova do aparelho local; autentica a pessoa e
resolve a sessão antes de chamar a operação. A transação revalida vínculo ativo,
papel exatamente `owner`, escopo e aprovação do aparelho local. Gerente não pode
conceder esta confiança. O pareamento do parceiro deve estar aprovado para
`trust` e `approve`, e sua chave Ed25519 deve ser exatamente a anunciada.

Chave aprovada, concessões e respectivas auditorias são gravadas na mesma
transação. Falha em qualquer gravação reverte todas. Repetir a mesma decisão
pelo mesmo dono não duplica auditorias. Outro dono ativo pode assumir a aprovação
do mesmo material de chave, com nova auditoria. Revisão ou material diferente
de um vínculo previamente gravado é conflito; não há rotação silenciosa.

`revoke` admite um parceiro com pareamento já revogado para retirar permissões
remanescentes, mas ainda exige a mesma chave pública de assinatura. Falha na
auditoria também reverte a revogação. A chave de criptografia não é alterada.
A política de recepção existente consulta aprovação e revogação atuais mesmo
para tentativas repetidas de entrega.

O encerramento da sessão administrativa é tentado em até dois segundos; falhas
de limpeza não imprimem segredos. Expiração e revogação continuam aplicadas pelo
módulo existente. Nenhum comando transmite chaves privadas, abre serviço de rede
ou altera arquivos privados do parceiro. Não instalar contra dados reais antes
de validar provisionamento completo em cenário descartável.

## Verificação

Testes de domínio cobrem concessão e revogação por tipo, idempotência, confiança
sem concessão, chave já pareada divergente, pareamento pendente, empresa diferente,
dono revogado, aparelho local revogado, gerente, listas inválidas, conflito de
chave e rollback total quando a auditoria falha. Testes da CLI percorrem
pareamento por desafio real em banco temporário, autenticação do dono, conferência
de impressões e comandos trust/approve/revoke. A suíte Go, vet e compilação devem
ser confirmados no computador do proprietário antes do commit.
