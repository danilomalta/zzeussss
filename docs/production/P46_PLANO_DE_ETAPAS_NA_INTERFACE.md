# P46 — Plano de etapas na interface
Base do usuario 6f7fd0e, P45. Rota /local/production. Define uma sequencia imutavel de 1 a 20 etapas antes da reserva ou consumo. Consulta trace da ordem para conferir estado, versao e local. Plano existente e apresentado sem edicao. Ordem planejada ou aprovada sem reserva ativa/consumida pode definir; reservas liberadas nao impedem. Nomes obrigatorios ate 120 bytes UTF8, responsavel existente com manage_production, motivo ate 255 bytes. IDs gerados uma vez dentro do bloqueio da fila antes de POST.

Contrato compartilhado ampliado: GET /production/operations/{kind}/{id} aceita stage_plan e stage_state. Usa eventos existentes scoped por empresa/loja/aparelho/operador, readTx e permissao manage_production. Historico nao exige contrato vigente; gravacoes continuam exigindo. Resposta guarda estado/revisao original, sem substituir pelo progresso atual. Planos e referencias conferidos, JSON canonico, corrupcao gera conflito, evento de outro tipo/operador nao retorna dados. POST stage-plans e stages/state existentes nao mudam.

Definir plano nao altera revisao da ordem nem estoque. A ordem nao conclui com etapas pendentes; iniciar uma etapa requer consumo previamente registrado. Publicar receita nao altera este plano.

Fila persistida inclui as novas operacoes, valida campos estritos, conserva ordem dos itens e payload/IDs. Falha de rede exige consultar ou repetir mesma operacao. GET confirmado limpa fila; nenhum envio automatico.

Schema SQLite 44 mantido. Sem migracao ou alteracao do backup. Testes HTTP percorrem configuracao, consumo, progresso, conclusao, recibos originais e backup/restauracao em banco temporario; isolamento e corrupcao em fixtures descartaveis. Frontend testa limites, sequencia, payload e resposta perdida. Fixture frontend separada do arquivo de testes para evitar execucao duplicada.

aplicar.sh: testes especificos, suite backend completa, vet, race de recibos, testes frontend e build. Nao abre servidores/bancos reais, merge ou push.
