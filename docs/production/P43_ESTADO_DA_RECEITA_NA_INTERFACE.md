# P43 — Estado da receita e recibos de execução
Base decd314, P42; interface /local/production.

Consultar estado, informar motivo, ativar ou inativar explicitamente. Revisão do estado inicia em zero e é independente da publicação. Inativar bloqueia novos planos; versões e ordens existentes são preservadas. Publicar versão não reativa.

Contrato compartilhado: GET /production/operations/{kind}/{id} aceita também recipe_state, reserve, materials e result. Exige manage_production e aparelho aprovado, com escopo original empresa/loja/aparelho/operador. Consulta histórica independente de contrato vigente. Entradas canônicas e resultados originais conferidos; mudança posterior não substitui recibo. Corrupção retorna conflito, ausência ou outro operador não expõem dados. Sem retry implícito.

A fila preserva IDs/payload antes do POST; sucesso exige confirmação GET. Resultado incerto não é descartável e repetição explícita conserva dados. Tipos materiais/resultado preparados para P44/P45; ações ainda não exibidas.

Schema 44 mantido, sem migração ou mudança de backup. Testes novos percorrem estado da receita, reserva/liberação/nova reserva/consumo/conclusão, restauração dos mesmos recibos em SQLite temporário, isolamento e corrupção. Frontend verifica requests exatos, zero explícito e resposta perdida de consumo sem duplicação.

aplicar.sh executa testes HTTP específicos, suíte backend, vet, race específico, testes frontend e build antes do commit. Nenhum servidor ou banco comercial. Sem merge/push.
