# P05 — reservas e consumo de ingredientes

Base de aplicação: `500fac3`, branch `feat/producao-receitas`, pasta isolada
`~/Downloads/zzeus-frontend-producao`. Sem merge, push, inicialização de servidor
ou acesso intencional a bancos comerciais/demonstração.

## Migração e compatibilidade

Nova migração SQLite **0030_production_materials.sql**. Na base P04 recebida,
0030 estava livre; `aplicar.sh` confere novamente antes de escrever. Nenhuma
migração antiga foi modificada. Criadas quatro tabelas: grupos de reserva,
itens exatos, eventos de auditoria/idempotência e vínculos com movimentos.
A autorização continuada do usuário cobre o ajuste mínimo de compatibilidade:
a lista explícita de backup passa de 25–29 para **25, 26, 27, 28, 29, 30**.
ValidateSchema, checksums, integrity_check, foreign_key_check, validação do
aparelho, criptografia e formato permanecem preservados. Os testes de versões
anteriores incluem schema 29 e os novos testes restauram reservas ativas e
consumidas, itens, eventos e vínculos de movimentos.

## Contrato e estados

Criar/aprovar uma ordem continua sem movimentar nem reservar estoque.
Uma reserva exige ordem `approved`, autorização atual do autor e do responsável,
contrato de produção válido e saldo livre para **todos** os ingredientes.
As quantidades vêm do snapshot da receita da ordem × lotes planejados, sem
aceitar quantidades alternativas na requisição e sem ponto flutuante.

Estados da reserva: `active` → `released` ou `consumed`; estados terminais.
Reserva/liberação não escrevem movimentos. Consumo grava uma saída exata por
produto e os vínculos correspondentes, muda o estado, registra auditoria e
outbox na mesma transação. Qualquer falha desfaz o conjunto inteiro.
Uma ordem só pode ter um grupo ativo, e só pode consumir seus ingredientes uma
vez. Após liberação é possível reservar novamente usando um ID novo. Uma
reserva consumida impede nova reserva da mesma ordem.

Nesta fase, consumir ingredientes **não registra produto acabado**, não fecha
a ordem e não representa automaticamente produção concluída. A ordem permanece
`approved`; o estado material é consultado no grupo. Cancelar ordem com grupo
ativo exige liberação anterior. Cancelar após consumo é recusado: compensação
explícita será uma entrega posterior, sem devolução fictícia de ingredientes.

Cada escrita exige `operation_id`, motivo e autorização atual, inclusive replay.
Replay com mesmo autor, aparelho, empresa, loja, ação e corpo normalizado retorna
o resultado original com `repeated=true`, sem nova auditoria/saída. Reutilizar
ID com corpo/ação diferentes retorna conflito. IDs de operação são definidos
por endpoint; não constituem uma chave global entre módulos.

## Arquivos compartilhados e contratos afetados

- `localapi/server.go`: três rotas novas protegidas pela sessão existente.
- `localapi/production.go`: conflito de materiais retorna HTTP 409, inclusive no
  cancelamento de ordem já existente.
- `production/orders.go`: acrescenta guarda contra cancelamento com materiais
  ativos/consumidos; preserva CAS de revisão e replay anterior à guarda.
- `production/capacity.go` e OpenAPI de capacidade: `stock_milli` passa a ser o
  **saldo livre após reservas ativas**, na unidade do catálogo. `basis` passa a
  `local_available_balance_after_reservations`. Alternativas continuam separadas
  e não devem ser somadas. Consulta não reserva nem consome.
- `stock/stock.go`: perda e transferência retiram somente saldo livre.
- `sale/sale.go`: venda verifica a soma das linhas do mesmo produto/local contra
  saldo livre, antes de gravar a venda.
- `inventory/count.go`: recusa contagem abaixo do total reservado; não abandona
  reservas silenciosamente. Libere/replaneje reservas antes desse ajuste.
- Novo `stockreservation`: regra única de leitura do total comprometido, sempre
  dentro da transação do chamador. Não concede autorização nem escreve dados.
- `backup/backup.go` e testes de migração: versão 30 explicitamente aceita;
  versões futuras continuam recusadas. Testes de interrupção usam versão 31.

`stock.Balance` continua mostrando saldo físico registrado. Esse saldo não deve
ser confundido com disponibilidade. Entradas e estornos de venda acrescentam
saldo e conservam seus contratos anteriores. Recepção online na base recebida
registra inbox, sem aplicar movimentos; nenhuma aplicação remota foi adicionada.
Os eventos `production.materials.changed` ficam na outbox existente; esta entrega
não cria aplicação remota desses eventos. Autenticação, sessões, infraestrutura
operacional e migrações PostgreSQL não foram alteradas.

## API

Base: `/local/v1/production/material-reservations`.

POST base, objeto exato:
```json
{"operation_id":"reserve-op","reservation_id":"materials-1","order_id":"order-1","reason":"Separar ingredientes"}
```

POST `/state`, objeto exato; `action` é `release` ou `consume`:
```json
{"operation_id":"consume-op","reservation_id":"materials-1","action":"consume","reason":"Executar ordem"}
```

GET `/{id}` devolve ordem/local, estado, autor, datas e ingredientes exatos.
Respostas de escrita: `reservation_id`, `order_id`, `status`, `repeated`.
Criação 201; replay 200; alteração/consulta 200. JSON incompleto, duplicado,
extra, null, query inesperada ou ação inválida: 400. Sem sessão: 401; sem
permissão/contrato: 403; referência fora do escopo/inexistente: 404;
estado, idempotência, unidade alterada ou insuficiência: 409. Falhas internas
não são tratadas como sucesso. OpenAPI: `docs/api/production-materials.openapi.json`.

Unidades seguem a milésima parte da unidade explícita do catálogo. Por exemplo,
500000 `g` representa 500 g; 500000 `kg` representa 500 kg. Não há conversão
volume/massa. Como os movimentos existentes não preservam a unidade histórica,
alterar a unidade do produto após publicar a receita bloqueia reserva/consumo,
inclusive quando a dimensão é compatível. Não reinterpretamos estoque antigo.
Limites de resultado preservam inteiros exatos de até 9007199254740991; somas
intermediárias de reservas usam inteiros arbitrários e rejeitam overflow.

## Verificação e execução

Executados e aprovados neste ambiente com **Go 1.25.0**: backup/CLI,
testes específicos de P05, uma execução da suíte completa `go test -count=1
./...`, `go vet ./...`, testes P05/regra de reservas com `-race` e build
`CGO_ENABLED=0` de `titan-local` sem executar o binário. A suíte completa não
teve acesso à variável opcional de testes PostgreSQL. Também foram verificadas
as migrações 1–30 com SQLite temporário, integridade/chaves estrangeiras,
a estrutura e referências OpenAPI, aplicação do patch e sintaxe Bash.
Esses resultados são da cópia de preparação; o script repete a validação na
branch do usuário antes de registrar qualquer commit.

O script no computador do usuário executa, antes de qualquer commit:
1. Backup/CLI: `go test -count=1 ./internal/localdb/backup ./cmd/titan-backup`.
2. Testes de P05 e proteção das saídas.
3. Uma execução da suíte completa; `go vet ./...`.
4. Detector de corrida nos testes P05/regra de reservas; build de `titan-local`
   em diretório temporário, sem execução do binário.
5. Verificação de diff e commit somente da lista explícita desta entrega.

Os testes cobrem fronteira de saldo livre, somas de linhas de venda, duas ordens
concorrentes pelo mesmo saldo, replay, estados inválidos, rejeição de unidade,
overflow, permissão/contrato, isolamento, falhas INSERT/UPDATE ignoradas e
backup/restauração de dados ativos e consumidos. Usam fixtures SQLite temporárias
e o harness HTTP em processo; não abrem servidor de rede. A variável
`TITAN_SESSION_TEST_DATABASE_URL` é removida no subshell de testes para manter a
validação PostgreSQL opcional sem acesso a banco real configurado por essa via.

Se alguma etapa falhar, o script para e conserva a alteração para diagnóstico;
não faz descarte automático nem commit parcial. Não reaplique nesse estado.
