# P02 — correcao de compatibilidade de backup

A execucao no PC confirmou aplicacao da P02 sobre o commit `b12a7ff`, na branch
`feat/producao-receitas`. `go test ./...` aprovou production, localapi, localdb
e os demais pacotes exceto backup/CLI de backup. Nao houve commit da P02:
o script parou na primeira verificacao, antes de race, vet e build.

Causa confirmada no codigo: `backup.validate` aceitava somente os schemas 25,
26 e 27, enquanto a P02 acrescentou a migracao 28. A correcao acrescenta somente
28 a lista. Mantem verificacao de checksums/historico, integridade, chaves
estrangeiras, identidade de aparelho e rejeicao de versoes futuras.

Acrescenta `TestProductionRecipeSnapshotRestoresVersionAuditAndOutbox`:
publica receita pelo servico com contrato de teste assinado, faz snapshot
cifrado com banco aberto, verifica, restaura em caminho novo e compara a
versao completa, ingrediente exato, auditoria e evento pending.

| Etapa | Estado | Verificacoes | Dependencias |
| --- | --- | --- | --- |
| P01 | Confirmada no PC pelo HEAD b12a7ff | Documento commitado | — |
| P02 inicial | Aplicada, sem commit; bloqueada no backup | Testes production e localapi passaram no PC; suite completa falhou no backup | Aplicar esta correcao |
| P02 correcao | Entregue para aplicacao | Patch e sintaxe do script conferidos; Go indisponivel no ambiente gerador | Testes backup, suite completa, race, vet, build no PC |
| P03 | Pendente | — | P02 confirmada |

Arquivo compartilhado adicional para coordenacao com Chat A:
`backend/internal/localdb/backup/backup.go`, duas linhas da lista explicita de
schemas suportados. Nenhuma nova migracao alem de SQLite 0028. Nao altera
criptografia, politica de recuperacao, sessoes online nem bancos comerciais.

O script de retomada aplica apenas a correcao sobre a P02 ja existente,
preserva o formato produzido por gofmt, verifica a branch/base e recusa
alteracoes fora da lista da entrega. Todos os testes devem passar antes de
criar um unico commit com P02 e correcao. Nao executar novamente o aplicar.sh
original sobre a pasta que ja recebeu P02.
