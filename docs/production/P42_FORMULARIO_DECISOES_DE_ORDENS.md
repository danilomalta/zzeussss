# P42 — Decisao visual de aprovar/cancelar ordem

Consulta porID retorna estado atual e revisao da ordem, com receita/local/
responsavel/quantidade. Planejada admite aprovar ou cancelar; aprovada somente
cancelar; concluida/cancelada nao admitem transicao. Revisao maxima bloqueia UI.
Motivo obrigatorio com limite255bytesUTF8. A decisao preserva expected_revision
consultada; conflito nao troca revisao ou reenvia automaticamente.

POST orders/state registra autoria/auditoria preexistentes. Aprovar autoriza plano,
nao reservas/consumo. Cancelar nao libera materiais; backend recusa se reservas/
consumo impedirem. Exige permissao e contrato vigentes. Fila P40 compartilhada,
mesmos IDs/payload; consulta retorna decisao original mesmo apos transicao futura.
Ao confirmar, detalhe e limpo; nova decisao exige consultar estado novamente.

Sem migracao, schema44; backend, backup, auth e infra identicos aP40.
Compartilhados: LocalProduction inclui formulario; package.json novo teste.
Testesfrontend104 e build aprovados, estados terminais, revisao esperada, motivo,
nenhuma liberacao implicita, e conflito sem revisao nova. Suite backend/vet e
race dos novos endpoints P40 aprovados no lote. Nao executado aceite visual.

Ainda pendentes: estado ativo/inativo de receitas, reservas/consumo/resultados,
etapas/perdas/lotes/qualidade, operacoes catalogo/estoque e recebimento visual.
Sem main, merge, push ou bancos reais.
