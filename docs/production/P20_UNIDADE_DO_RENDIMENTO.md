# P20 — Unidade do rendimento na capacidade

Depende de P19, na branch feat/producao-receitas. Sem migracao: SQLite 35.

GET /production/capacity confere tambem o produto resultante: deve existir na
empresa autenticada e sua unidade atual deve coincidir com a versao da receita.
Antes, essa verificacao abrangia apenas ingredientes. Divergencia retorna 409
sem publicar uma capacidade aparentemente executavel com unidade incompatível.

Compartilhado: production/capacity.go, contrato da consulta de capacidade.
Calculo inteiro, alternativas independentes, ingrediente limitante, saldos e
reservas permanecem iguais. Nenhuma conversao implicita, reserva ou baixa.
A receita historica permanece consultavel com seus dados preservados.

Teste compara resposta antes/depois de divergencia no resultado, leitura
historica, retorno apos restaurar unidade e ausencia de escrita/relogio.
Backup, migracoes, autenticacao e infraestrutura permanecem intactos.
