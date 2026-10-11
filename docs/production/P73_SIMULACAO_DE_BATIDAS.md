# P73 — Cinco funções de simulação da alternativa

Base: `810cbe2`; branch `feat/producao-receitas`.

No catálogo local, após consultar alternativas de produção independentes, cada alternativa recebe uma simulação local da quantidade desejada:

1. Informar número exato de batidas completas desejadas, positivo e inteiro. Rejeita decimais, expoentes, sinais, zeros à esquerda e valores acima do limite seguro.
2. Calcular saída planejada na unidade explícita da receita, preservando versão, produto resultante, local e momento da consulta do estoque.
3. Calcular ingredientes necessários para a quantidade desejada, a partir da necessidade por batida da versão preservada.
4. Mostrar falta por ingrediente na unidade da receita e indicar se todos os ingredientes cobrem a escolha isolada. A quantidade desejada pode superar a capacidade consultada para permitir avaliar faltas.
5. Mostrar sobra matemática estimada por ingrediente suficiente na unidade de estoque; ingrediente insuficiente tem sobra indisponível, sem inventar valor negativo ou afirmar produção viável.

## Precisão e independência

Usa o objeto já retornado por `POST /local/v1/production/capacity/alternatives`, operação de consulta existente. A simulação não faz HTTP adicional, não escreve no banco, não cria ordem e não reserva ou baixa estoque. A consulta original continua autenticada e autorizada pelo backend.

Cada cálculo revalida o objeto completo das alternativas, incluindo os marcadores de independência, saldos compartilhados, conversões e ingredientes limitantes. Não soma saídas nem sobras de alternativas concorrentes. O saldo disponível considerado já desconta reservas no momento da consulta. Mudanças posteriores de estoque exigem nova consulta; a simulação não é confirmação de saldo futuro.

Quantidade planejada = quantidade por batida × batidas desejadas, com BigInt e verificação do limite seguro em cada quantidade planejada de produto e ingrediente. As unidades contáveis mantêm granularidade inteira de 1000 milésimos. Gramas são uma unidade explícita, independente da escala inteira.

Para ingrediente, `demanda` é em milésimos da receita. A quantidade disponível convertida é `saldo_estoque × numerador / denominador`, conforme a razão fornecida e validada pelo contrato. A falta usa a unidade da receita; a sobra usa a unidade do estoque. Intermediários podem superar o limite seguro do JavaScript e permanecem em BigInt/strings. Frações não divisíveis permanecem como razão exata na tela; não são arredondadas para simular movimentações de estoque. Massa não se converte em volume.

Quando algum ingrediente falta, as sobras dos outros são apenas estimativas matemáticas para a quantidade desejada; não representam saldo após uma produção executável ou confirmada. O ingrediente limitante exibido se refere à capacidade da alternativa, não a uma escolha simultânea de receitas.

## Arquivos compartilhados

`CatalogPlanning.tsx`: acrescenta uma simulação independente em cada alternativa já carregada. Não modifica seleção de versões, consulta de vínculos, requisições ou formulários de gravação. Alterar a quantidade limpa a simulação anterior; uma nova consulta remove os objetos anteriores durante o carregamento e monta novos simuladores.

`frontend-web/package.json`: acrescenta o novo arquivo de testes ao `test:local`; sem dependências ou lockfile novos.

Não há API, migração ou compatibilidade de backup nova. SQLite continua no schema 44, última migração `0044_production_receipt_voids.sql`.

## Verificações

Testes de entrada inteira, quantidades exatas, conversões nas duas direções, frações abaixo de um milésimo, intermediários acima do limite seguro, overflow das quantidades planejadas, unidades contáveis, saldo zero, múltiplos ingredientes, versões estrangeiras, alternativas independentes e preservação do objeto original.

Aplicador: base limpa, SHA256, patch, testes específicos `TestHTTPProductionCapacity`, suíte Go completa uma vez, vet, detector de corrida, todos os testes locais da interface e build. Confere os sete arquivos e registra somente após sucesso. Reconhece entrega já registrada. Sem merge, push, servidores ou bancos comerciais/demonstração. Não houve aceite visual em navegador.
