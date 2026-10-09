# P16 — origem das reservas que comprometem o estoque

Base efetiva: **7c7943a**, branch **feat/producao-receitas**, P15 registrada
com árvore limpa. Consulta de estoque/produção, sem interface nova.

**Sem nova migração.** SQLite permanece **35**, sem editar migrações, backup,
formato, criptografia ou recuperação.

## Consulta e permissões

GET /local/v1/stock/reservations?product_id=flour&location_id=production-room

Lista apenas reservas P05 active daquele produto/local e informa saldo físico,
reservado e livre da P15 na mesma transação. Não inclui released/consumed nem
cria uma nova reserva. Histórico continua nas APIs de materiais e ordens.

Exige sessão, aparelho autorizado, **manage_stock e manage_production** atuais.
Responsável, autor, ordem e versão são dados de produção, portanto a permissão
de estoque isolada não basta. Empresa/loja vêm da sessão. Não amplia papéis,
altera autenticação ou reduz a autorização das APIs anteriores.

Somente product_id e location_id obrigatórios, e offset opcional, padrão 0.
IDs até 128 bytes UTF-8, sem espaços nas bordas, NUL, CR ou LF. offset aceita
somente dígitos decimais 0..9007199254740991, sem sinal ou fração. Queries
desconhecidas, vazias ou duplicadas são recusadas; não aceita status, empresa,
loja ou limit do cliente. Referências ausentes no escopo retornam 404.

```text
GET /local/v1/stock/reservations?product_id=flour&location_id=production-room
GET /local/v1/stock/reservations?product_id=flour&location_id=production-room&offset=50
GET /local/v1/production/orders/order-1/trace
```

Contrato autocontido: docs/api/stock-reservations.openapi.json.

## Resposta e significado dos itens

| Campo | Significado |
| --- | --- |
| balance | Resposta Availability P15: produto, local, unidade, físico, reservado e livre |
| total_count | Todas as reservas active desse produto/local |
| offset, limit, has_more | Posição, limite fixo 50 e existência de continuação |
| items | Página das origens das reservas ativas |

Cada item informa reservation_id, order_id, version_id, responsible_id,
created_by, created_at, unit e quantity_milli. Quantidade é o compromisso
efetivo desse produto na reserva P05, em milésimos da unidade explícita.
Não informa nome pessoal, custo, preço ou avaliação privada do responsável.
created_by é quem registrou a reserva; responsible_id é o responsável
preservado na ordem. Não presume que são a mesma pessoa.

Origem na página deve corresponder ao local planejado e a uma ordem approved
sem resultado registrado. Divergência retorna 409; não apresenta uma reserva
ativa de ordem cancelada/concluída como compromisso normal. Quantidade deve
ser positiva, na unidade do saldo, e a soma da página não pode superar o total
reservado global. Não altera registros para corrigir inconsistência.

Ordena por created_at,reservation_id crescentes, com desempate estável. Página
tem até 50 origens; total_count e balance continuam globais. Não substituir
reserved_milli global pela soma da página. Página vazia mantém totais e saldo.
Referências existentes sem reservas retornam items [], total_count 0 e saldo
físico/livre reais. Liberação ou consumo retiram a reserva da lista ativa.

## Snapshot comum e efeitos

Saldo, unidades, contagem, ordens e itens são lidos na mesma transação SQLite.
Reutiliza a leitura interna P15 e HeldTx da regra P05. Não chama a API pública
AvailableBalance dentro da transação: o pool local tem uma conexão e perderia
o snapshot comum. Cursores são fechados antes do commit.

Consumo concorrente não pode retornar saldo físico posterior com itens ativos
anteriores. Requisições distintas podem observar novas reservas/liberações/
consumos; offset não garante exportação imutável de várias páginas.

Consulta não cria, libera, consome, altera ou cancela reservas/ordens; não
movimenta estoque, escreve auditoria/outbox ou atualiza relógio do contrato.
Histórico autorizado permanece legível após expiração do contrato, mantendo
as duas permissões atuais. Não recebe operation_id e não promete saldo futuro.
Uma escrita posterior deve revalidar autorização, contrato, saldo e reservas
em sua própria transação.

Mantém precisão e limites da P15: saldo físico exato, reserva não superior ao
físico, resultados inteiros seguros. Unidades preservadas conhecidas não são
reinterpretadas. Limite existente permanece: movimentos genéricos não guardam
unidade original; a consulta não reconcilia alteração externa sem snapshot.
Não converte volume em massa nem decide disponibilidade por lote ou FEFO.

400 query inválida; 401 sem sessão; 403 sem ambas as permissões/aparelho;
404 produto/local ausente no escopo; 409 saldo, unidade ou origem da página
inconsistentes nas validações; 500 erro interno. Não substitui validação
integral de dados fora da página ou verificação de integridade do banco.

## Mudanças compartilhadas e verificação

server.go monta uma rota GET protegida separada. stock/availability.go extrai
availableBalanceTx privada, que recebe transação já autorizada. A função
pública AvailableBalance conserva assinatura, validação inicial, manage_stock,
resposta e commit. P15 continua acessível a quem tem apenas manage_stock;
P16 exige adicionalmente manage_production para os detalhes.

Não modifica escritores de estoque, regra de reservas, contratos P05/P15,
autenticação, sessões, infraestrutura, PostgreSQL ou backup. Sem merge/push.

Testes com SQLite temporário e Fiber App.Test cobrem metadados de origem,
reserva/liberação/consumo, ausência de escrita, saldo/contagem globais em
53 reservas e páginas vazias, desempate e outro local; queries, escopos,
permissões separadas, expiração sem alterar relógio; ordem/local/unidade
divergentes; consumo concorrente; backup/restauração de origens ativas.
Testes P15 são repetidos para validar a extração compartilhada.

aplicar.sh confere base 7c7943a, branch, árvore limpa e checksum; aplica apenas
P16; testa backup/CLI, stock/stockreservation, APIs P15/P16, suíte completa
uma vez, vet, race e build sem executar binário. Registra apenas os oito
arquivos listados após sucesso, com visualizador desativado.

Limites: reservas ativas de um produto/local. Sem painel frontend, nova
reserva, ação automática, histórico paginado nessa rota, exportação, venda,
saldo por lote ou alteração de unidade.
