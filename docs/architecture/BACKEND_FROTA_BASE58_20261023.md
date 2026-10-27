# Frota e conferência técnica sobre a base58

Adaptação autorizada pelo proprietário em23/10/2026. A base local foi preservada em `~/Downloads/zzeus-frontend-backend-b02a`, branch `feat/backend-b02a-compat`, HEAD original `58a26bbbdf08903d62f6dd1f0542417171b460dd` mais122fontes/documentos locais preexistentes. Esses trabalhos não foram recriados nem atribuídos a esta adaptação.

## Compatibilidade resolvida

O consolidado `TitanSystem_Backend_Consolidado_Frota_20261023.zip` (SHA256 `e2b166608071a6abb3b1719931376902e37753a1638d941ddab1bd292eb23ac6`) parte de Tratativas/56e não reconhece Ciclo de Compras/57e Coletas-Devoluções/58. Sua aplicação original foi corretamente recusada. A solução NÃO altera registros antigos nem a sequência1–58: acrescenta `0059_fleet_maintenance.sql` e `0060_fleet_workshop.sql`. Os conteúdos SQL novos são os mesmos dos dois pacotes recebidos; nenhum SQL instalado foi renomeado, removido ou reescrito.

Os dez registros anteriores e os fontes finais correspondentes foram conferidos antes de preparar a candidata. Instalação não foi liberada por alteração de hashes congelados ou criação de predecessor falso. O registro `BACKEND_FROTA_BASE58_APLICADO.json` documenta esta NOVA adaptação, com proveniência/preimagens/resultados, e não faz o aplicador antigo se tornar compatível. Não reaplicar os ZIPs antigos sobre fontes evoluídos.

## Alterações localizadas

- Fontes/testes/contratos de Frota e Workshop dos pacotes09/10, incluindo as duas atualizações de ledger/mutate, conferidas contra os hashes originais.
- Montagem das rotas a partir do ponto existente de Tratativas; erro422do Workshop e inventário/OpenAPI ampliados com80operações, preservando todas as rotas de compras/coletas/docas/recebimentos.
- Contadores de schema e testes de interrupção avançados para60/61. Backup reconhece schemas25a60, acrescentando59/60e mantendo os antigos; teste de restauração inclui58e59.
- Regressão específica58→60: SQLite novo temporário com schema58, dados/recibos de ciclo de compras e chaves estrangeiras ativas; preserva recibo/evento, esquema das8tabelas de compras/devoluções, checksums1–58e restrição de imutabilidade. Executa a migração duas vezes e confirma60e as tabelas novas. Também mantidos os testes de migração originais de Frota e Workshop.

## Semântica e limites

As APIs usam sessão/contexto/aparelho aprovado, permissões, contrato Logistics, revisão concorrente, operações idempotentes e histórico dos pacotes recebidos. Checklist exigido e inspeção válida são conferidos no backend antes de inspect/release. Horas declaradas/revisadas não constituem ponto, folha ou pagamento. Aviso consultável/ACK não é push nem som remoto.

As abas existentes do frontend ainda usam business85/IndexedDB. Esta adaptação NÃO envia automaticamente seus rascunhos nem transforma aprovação local em autorização do backend. Não foi iniciado serviço, aberto banco comercial, escolhido perfil/estação/chaves, alterado frontend ou Electron. O binário produzido é evidência de compilação; não foi instalado ou executado. Uso integrado exige configuração autorizada de ambiente descartável e adaptador real de frontend.

## Evidências

Os resultados finais executados e hashes constam no registro de aplicação e no registro único de continuidade do frontend. Dependências existentes foram reutilizadas, com Go1.25.8 já presente em cache, GOTOOLCHAIN=local/GOPROXY=off e ambiente de testes sem credenciais. O CSV de exemplo financeiro versionado foi incluído na candidata porque o filtro do aplicador de origem o omitia; nenhum resultado esperado foi alterado para esconder falha.

Pré-imagens, candidata, plano e logs preservados no laboratório da tarefa. Aplicação compara todas as fontes da base, grava somente os caminhos planejados e conserva backup privado. Recuperação deve comparar alterações posteriores; não restaurar banco nem remover SQL depois de uma migração operacional. O salvamento no GitHub contém apenas fontes/testes/contratos/documentação, sem bancos, chaves, caches, dependências ou binário.
