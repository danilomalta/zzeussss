# Fundação 04 — Auditoria administrativa e isolamento local

Base do usuário: `37fa35c`. Backend local; nenhuma nova migração, alteração de
frontend ou acesso a banco real. A versão de esquema continua 27.

## Consulta de oito fontes administrativas

`GET /local/v1/access/audit` mantém o formato de resposta da entrega 03 e
acrescenta o filtro opcional `source`. Sem filtro, o dono vê os registros de sua
loja nas oito fontes abaixo. Nenhum parâmetro permite escolher outra empresa,
loja, aparelho ou ator: o contexto vem da sessão comprovada.

| source | Registros lidos | Dados omitidos |
| --- | --- | --- |
| policy | access_policy_events | Senha e request_hash |
| account | account_access_operations | Credenciais, chave de recuperação e request_hash |
| security | account_security_events | Tokens e hashes de credenciais |
| staff | staff_registrations | Senha, hash de senha e request_hash |
| invite | membership_invite_events + membership_invites | Código/hash do convite |
| pairing | device_pairing_events + device_pairings | Chave pública, desafio e prova |
| peer | sync_peer_audit | Chave pública e conteúdo das mensagens |
| encryption | device_encryption_key_audit | Chaves, assinaturas e fingerprints |

Os joins acrescentados vinculam eventos de convite/aparelho à empresa e loja
corretas. Nas fontes peer e encryption, o filtro usa as colunas de escopo já
persistidas. Não lê incoming_events.payload_json, metadata_json, recibos ou
conteúdo comercial. Exibe somente referências, tipo de evento, ator/alvo,
revisão, horário e os metadados administrativos já previstos na entrega anterior.
No evento de comprovação do aparelho, actor_id identifica o aparelho; não
representa uma aprovação humana. Na fonte peer, permission informa o tipo de
evento autorizado no protocolo, e não uma nova permissão humana.

## Autorização e paginação

- Dono: oito fontes da loja atual; source pode filtrar uma delas.
- Delegado: apenas policy do departamento explicitamente autorizado. Nenhum
  papel de gerente/RH dá acesso às demais fontes por si só.
- Com department_id informado, source só pode ser omitido ou policy. Outra
  fonte retorna 403: eventos de aparelho/convite não possuem escopo departamental.
- Source desconhecido, parâmetro extra/duplicado ou cursor com fonte/escopo
  diferente retorna 400. Cada página revalida sessão, aparelho e delegação.
- Limit padrão 50, intervalo 1–100. Ordenação decrescente por horário/referência.
  Duas linhas com o mesmo horário não são confundidas. Referência inclui o
  prefixo da fonte e o identificador do evento.
- Fonte omitida mantém a estrutura do cursor anterior. A função Audit anterior
  continua disponível para chamadores de biblioteca. AuditFiltered acrescenta
  somente o filtro explícito; a resposta não ganhou campos incompatíveis.

Exemplos de rotas, sem credenciais e sem comandos contra banco real:

```text
GET /local/v1/access/audit?source=invite&limit=50
GET /local/v1/access/audit?source=pairing&limit=50
GET /local/v1/access/audit?source=peer&limit=50
GET /local/v1/access/audit?source=encryption&limit=50
GET /local/v1/access/audit?department_id=ID&source=policy
```

OpenAPI atualizado: `docs/api/access-policy.openapi.json`, versão documental
1.1.0. Não altera `/local/v1` nem os campos das respostas existentes.

## Inventário da barreira de autenticação

O teste `TestEveryProtectedLocalRouteRejectsForeignOrRevokedSession` enumera as
rotas REAIS registradas no Fiber, excluindo HEAD e as três rotas públicas
health/login/account-recover. Novas rotas entram automaticamente no teste.

| Grupo protegido | Operações atuais |
| --- | ---: |
| me, capabilities, logout | 3 |
| access/policy e access/audit | 3 |
| staff, password-reset e sessions/revoke | 4 |
| account/sessions, password, revoke-others e recovery/revoke | 4 |
| module-contracts | 1 |
| products, catalog/search e locations | 5 |
| stock | 2 |
| cash | 4 |
| sales | 4 |
| replenishment | 6 |
| purchases | 6 |
| comparison | 3 |
| Total de operações protegidas, sem HEAD | 45 |

O teste solicita cada operação com ausência de sessão, sessão de empresa
estrangeira, outra loja/aparelho, outro aparelho da mesma loja, usuário revogado,
sessão revogada, aparelho revogado e vínculo de loja removido. Espera 401 e
confere que tabelas comerciais/administrativas não ganharam registros. Somente
SQLite descartável e credenciais fictícias são usados.

Isso verifica a barreira de sessão e escopo do processo; NÃO prova sozinho o
isolamento por registro quando a sessão é válida. Os testes específicos de
catálogo, estoque, venda, caixa, compras, reposição e comparação continuam
necessários para validar referências estrangeiras e permissões dos casos de uso.
Revalidação transacional de todos os handlers antigos, trabalhadores, exports,
integrações e APIs PostgreSQL ainda precisa de inventário e aceite próprios.

## Verificação e limites

Testes novos exercitam as quatro fontes acrescentadas, exclusão de outra loja,
omissão de hashes/chaves, filtros conhecidos/desconhecidos, empate de horários,
cursor vinculado à fonte e recusa das fontes administrativas ao delegado.

Esta entrega não cria registros retroativos nem cobre alterações que os módulos
antigos não auditavam. Auditoria comercial, contratos de licença, fornecedores,
retenção/exportação, integridade contra administrador do sistema operacional e
consulta online continuam pendentes. Volume elevado precisa de ensaio de
desempenho e índices apropriados antes do piloto; limite da resposta não é uma
garantia de custo constante da consulta.

Próximo bloco da Parte 1: instalação/atualização e recuperação da instalação,
mantendo o banco e os arquivos privados do cliente preservados.

Resultados observados no ambiente de desenvolvimento: go test -count=1 ./... passou; go vet ./... passou; go test -race das novas consultas e da barreira das rotas passou; titan-local compilou com CGO_ENABLED=0 e -buildvcs=false; git diff --check passou. Os testes da barreira executaram as 45 operações em oito cenários (360 solicitações).
