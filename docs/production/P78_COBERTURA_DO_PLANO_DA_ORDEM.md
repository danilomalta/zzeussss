# P78 — Cinco funções de conferência da cobertura da ordem

Base `087f0a6`, branch `feat/producao-receitas`. SQLite permanece 44.

1. Consulta plano imutável: versão/revisão, local, responsável, batidas e saída planejada.
2. Consulta capacidade no local, mostra batidas possíveis, excesso do plano e ingredientes limitantes.
3. Mostra demanda total dos ingredientes preservados pela ordem.
4. Calcula faltas exatas na unidade da receita, com saldo livre na unidade do estoque.
5. Calcula sobras hipotéticas por ingrediente na unidade do estoque, sem apresentá-las como movimentação.

Sem entrada de batidas ou local alternativos: usa os valores da ordem existente. Aceita somente ordem planejada/aprovada sem reserva ativa, consumo ou resultado. Reservas liberadas anteriormente são permitidas e contadas. Ordem com reserva própria não recebe simulação que duplique a disponibilidade: consulte os materiais da ordem nesse caso.

## Leitura e consistência

Sequência de três GETs existentes, com Bearer, sem cookies/cache/corpo: rastro da ordem com `lot_offset=0`, capacidade para versão/local encontrados e nova leitura do rastro. As leituras têm autorização e isolamento conferidos pelo backend. Não usa POST de alternativas nem qualquer escrita.

Valida rastro e receita, identidade da capacidade, revisão, produto/unidade/rendimento resultantes e todos os ingredientes com quantidade/unidade por batida. Demanda calculada deve ser idêntica à preservada pelo plano. Capacidade não pode trazer ingrediente extra, omitir ingrediente ou usar outra receita/versão/local. Reutiliza `validAlternatives` e `planAlternative`, incluindo conversões, limites e flags de alternativas independentes.

Nova leitura verifica elegibilidade e fingerprint de ordem, plano, identidade e contagem de reservas liberadas. Aprovação/cancelamento/revisão, plano diferente e ciclos de reserva/liberação detectáveis durante as leituras recusam o resultado. Falha em qualquer leitura não publica dados parciais. A consulta não é uma transação única e não bloqueia escritores. Nenhuma garantia de disponibilidade futura: mudanças após a capacidade ou a última leitura permanecem possíveis e serão conferidas pelo escritor transacional.

## Exatidão

Demanda, saída, faltas e sobras seguem P73, com intermediários BigInt e frações em milésimos. Sem floats, arredondamento ou conversão massa-volume. Quantidades e resultados armazenáveis mantêm limite seguro; unidade unit mantém a granularidade dos planos. Batidas faltantes = máximo de 0 e batidas planejadas − capacidade atual. Limitante descreve a capacidade consultada; faltas descrevem o plano solicitado.

Sobra aparece somente para ingrediente cujo saldo cobre a demanda; em ingrediente insuficiente a interface informa insuficiência, não saldo negativo ou sobra inventada. Cobertura significa apenas saldo matemático para esta ordem isolada. Não significa autorização, aprovação, reserva, produção concluída ou estoque acumulável com outras ordens. Ordens planejadas ainda exigem aprovação humana independente.

## Contratos compartilhados e validação

`LocalProduction.tsx` acrescenta o painel; `package.json` acrescenta o teste. Clientes/validadores de escrita não são alterados. Novo leitor compõe contratos GET existentes, sem API nova, migração ou backup.

Testes: plano preservado, capacidade suficiente/insuficiente, conversões/frações/limites, referências/revisões trocadas, reservas próprias, estado alterado entre leituras, reserva liberada no intervalo, transporte/autorização e ausência de escrita. Frontend completo/build. Backend testes HTTP de capacidade/rastreabilidade, suíte completa uma vez, vet e corrida dos testes específicos. Patch/script verificados sobre cópia limpa, base errada e etapa já registrada.

Inputs/nova consulta/sessão limpam resultados via `useReadTask`; respostas ultrapassadas são descartadas. Sem aceite visual em navegador. Sem main, merge/push, bancos comerciais ou banco de demonstração.
