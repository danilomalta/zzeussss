# Parte 1 — Fundação operacional do backend

Base da entrega 01: commit do usuário `294a544`. A matriz abaixo foi atualizada
pela entrega 09 de troca de senha online; NÃO encerra os onze
requisitos. Frontend inalterado.

## Matriz de conclusão

| ID | Requisito | Implementação existente / desta entrega | Trabalho em aberto e aceite |
| --- | --- | --- | --- |
| F01 | Banco local e atualização | Migrações transacionais/checksums; recusa de versão futura/lacunas; backup pré-upgrade, teste em cópia, ativação atômica e recuperação em fronteiras persistentes | Distribuição assinada/compatibilidade entre aparelhos; upgrade com mudança futura de esquema, queda de energia e disco cheio em equipamento real |
| F02 | Backend online | PostgreSQL com credenciais explícitas e TLS remoto verificado; access HS256 ligado à sessão; refresh opaco de uso único e revogação; catálogo e parte de descontos | Serviços online completos; reconciliação com dados locais e testes isolados de cada operação |
| F03 | Isolamento | Contextos tenant/loja/aparelho/ator; novas APIs revalidam sessão na transação; teste da barreira nas 45 rotas locais e quatro operações comerciais online; consultas online por empresa | Inventário e testes de isolamento de todas as APIs, workers, exportações e integrações |
| F04 | Instalação | Gerenciador Linux por usuário, pacote local verificado, raiz separada de dados, init/check, bloqueios herdados, atualização/recuperação e desativação preservando dados | Distribuição assinada, recuperação do init incompleto, serviço automático, desinstalação formal, desktop/mobile e Windows/macOS |
| F05 | Modos | Perfis estritos, responsabilidades descritas; local loopback e servidor de loja SQLite via HTTPS com restrição de rede | Cliente LAN/piloto, identificação de cada terminal, nuvem comercial e reconciliação híbrida; cloud/hybrid recusados neste núcleo |
| F06 | Autenticação | Login/logout/revogação; troca da própria senha; consulta de sessões; reset/revogação administrativos restritos; recuperação local do dono com chave preparada de uso único; sessões online próprias com rotação, logout e revogação; troca da própria senha online com revogação transacional | Recuperação online; administração de outras contas; sincronização de credenciais e revogações; sessões individuais e entrada secreta interativa |
| F07 | Autorizações | Papéis, contratos, regras allow/deny/inherit por ação/loja/pessoa/grupo; departamentos e delegação limitada auditada | Interfaces de administração; propagação e reconciliação entre aparelhos; granularidade de registros dos futuros módulos de RH/produção; políticas online |
| F08 | Auditoria | Eventos críticos; políticas e acesso atômicos; consulta autorizada por loja/departamento com paginação para políticas, contas, funcionários, convites, pareamento, permissões de sincronização e aprovação de chaves públicas | Cobertura/consulta das demais fontes administrativas e comerciais; retenção e exportação |
| F09 | Proteção de dados | Transporte cifrado; backup AES-256-GCM, chave separada 0600 e autenticação do arquivo | Backups remotos, proteção dos demais arquivos/certificados, rotação e recuperação de todas as chaves |
| F10 | Backup/restauração | Snapshot SQLite consistente, restauração em arquivo novo; sessões e chaves de recuperação da cópia revogadas; snapshots 25/26/27; agendamento CLI watch | Retenção, cópia externa, serviço instalado, monitoramento, PostgreSQL e ensaio de desastre completo |
| F11 | Documentação API | Contratos detalhados de acesso/segurança e auth online; inventário de 69 rotas comparado ao Fiber; mapa OpenAPI; erro JSON opcional compatível; limites atuais de paginação testados | Schemas completos dos demais domínios; parsers e paginação de todas as coleções; aceite de clientes e evolução por versão |

## Segurança de conta local

- `GET /local/v1/account/sessions`: apenas sessões da identidade autenticada,
  neste banco, máximo 100, mais recentes primeiro. Não retorna tokens nem hashes.
- `POST /local/v1/account/password`: JSON exato com `current_password` e
  `new_password`. Exige senha atual. Nova senha segue regra existente de 12–72
  bytes, sem espaços nas extremidades. Troca hash, revoga todas as sessões da
  identidade na empresa e grava auditoria na mesma transação. Requer novo login.
- `POST /local/v1/account/sessions/revoke-others`: JSON exato com
  `current_password`. Mantém a sessão atual e revoga as demais da identidade
  na empresa. Não é um reset de senha nem recuperação do dono.
- As duas mutações têm limite de cinco solicitações por minuto por IP,
  por rota. A sessão e o aparelho são revalidados dentro da transação.
- Sem campo para selecionar outra empresa, loja, identidade ou aparelho.
- Corpo JSON máximo 2048 bytes; rejeita duplicatas, campos extras, null,
  tipos errados e conteúdo após o objeto. Nenhuma resposta inclui a senha.
- Senha incorreta ou política de senha recusada retorna 401; ausência de sessão
  segue o middleware existente. Erros de parser/limiter ainda seguem Fiber.
- A revogação alcança somente este banco; não anunciar logout global em todos
  os aparelhos até haver sincronização implementada para isso.

## Inventário de APIs

| Serviço | Grupos existentes | Limitações |
| --- | --- | --- |
| Local `/local/v1` | health, login, me, logout, capabilities, staff, module-contracts | Recuperação local preparada e APIs de acesso existentes; recuperação online pendente |
| Local `/local/v1` | products, catalog/search, locations, stock | Edição completa de catálogo e inventário operacional pendentes |
| Local `/local/v1` | cash, sales | Dinheiro, cancelamento integral, sangria/suprimento; sem integração real Pix/cartão |
| Local `/local/v1` | replenishment, purchase-suppliers, purchase-approvals, purchase-orders | Pedido somente local, sem envio/confirmação do fornecedor |
| Local `/local/v1` | comparison-sites, comparison-site-operations | Configuração e links externos; preços automáticos não implementados |
| Local `/local/v1` | account/sessions, account/password, account/sessions/revoke-others | Novas rotas nesta entrega; descritas em OpenAPI |
| Online `/api/v1` | auth/login, auth/refresh, auth/logout, auth/password, auth/sessions e revogações, saude, produtos, discounts/suggest, discounts/suggestions | Não equivale aos serviços locais completos |
| Online `/api/v1` | analises/produtos-parados, recompensas, accounting/sped, discounts/suggestions/:id/review | Rotas explicitamente indisponíveis; não contar como recurso pronto |

Contrato OpenAPI incremental: `docs/api/account-security.openapi.json`.
Ele cobre SOMENTE segurança e acesso locais, não todo o ERP. Política: manter contratos
antigos compatíveis na v1; alterações incompatíveis requerem nova versão e
migração explicitamente documentada. Isso não implementa compatibilidade dos
eventos comerciais entre aparelhos.

## Backup local cifrado

Executável: `go build -o /tmp/titan-backup ./cmd/titan-backup` em `backend/`.
Comandos (nomes de arquivos de exemplo, não executar contra a demonstração
para testar):

```text
titan-backup key-init --key /diretorio-privado/backup.key
titan-backup create --db /instalacao/store.sqlite --station /instalacao/station.json --key /diretorio-privado/backup.key --out /backups/copia.tytbak
titan-backup verify --archive /backups/copia.tytbak --key /diretorio-privado/backup.key --tenant ID --store ID --device ID
titan-backup restore --archive /backups/copia.tytbak --key /diretorio-privado/backup.key --tenant ID --store ID --device ID --out /recuperacao/store-novo.sqlite
titan-backup watch --db /instalacao/store.sqlite --station /instalacao/station.json --key /diretorio-privado/backup.key --out /backups/privados --interval 1h
```

- `watch` recebe em `--out` um diretório existente 0700, cria o primeiro backup
  imediatamente e depois no intervalo de 1 minuto a 24 horas. Novos nomes UUID
  impedem sobrescrita. SIGINT/SIGTERM cancelam. Cada tentativa revalida a estação.
  Falha interrompe o trabalhador com código não zero, sem remover backups.
  Ainda não há retenção automática ou instalação de serviço systemd.
- Manutenção por operador do sistema operacional com acesso à estação e à chave,
  não uma API disponível a usuários do ERP. A chave autoriza descriptografar o
  banco inteiro. Não enviar ao serviço Titan. Sem endpoint HTTP de backup.
- `VACUUM INTO` obtém snapshot consistente incluindo dados confirmados em WAL.
  Escritas posteriores podem não pertencer ao snapshot. Nenhuma leitura de arquivo
  `.sqlite` vivo por cópia comum é usada.
- GCM autentica o arquivo e identifica alterações/chave errada. Nonce aleatório
  novo em cada backup; versão do formato autenticada. Não equivale a assinatura
  que prove origem perante terceiros que não possuem a chave.
- Verificação: integrity_check, foreign_key_check, checksums de migrações,
  esquema completo atual, uma empresa e aparelho esperado aprovado na cópia.
- Arquivos novos 0600 e publicação exclusiva. Restore rejeita destino e sidecars
  (`-wal`, `-shm`, `-journal`) existentes. Nunca substitui um banco em uso.
- Chaves privadas de estação/cifragem são arquivos separados, NÃO incluídos.
  Preservá-las e preservar a chave de backup separadamente é parte da recuperação.
  Sem a chave de backup não há recuperação do conteúdo deste formato.
- Dados temporários descriptografados ficam em diretório privado e são removidos
  ao encerrar; não há promessa de apagamento físico seguro em SSD.
- Limite atual de snapshot de 128 MiB, processado em memória. Para bases maiores
  falta formato cifrado em blocos/streaming e ensaio de carga. Monitorar espaço.
- Restauração revoga sessões humanas na cópia, mas NÃO revalida revogações externas
  posteriores ao backup. Primeiro verificar estação, licença, sincronização e
  continuidade dos eventos em ambiente isolado. Não executar duas cópias da mesma
  identidade de aparelho ao mesmo tempo. Não ativar automaticamente a restauração.
- Migrações futuras são recusadas. A entrega 02 aceita snapshots verificados em
  esquema 25 e 26, migrando somente a cópia de recuperação. Versões anteriores
  a 25 continuam sem suporte neste formato.
- Backups PostgreSQL, cópia externa, rotação, retenção e recuperação de desastre
  seguem pendentes. Nenhum banco real foi usado para os testes desta entrega.

## Verificação e próximas entregas

Testes cobrem histórico futuro/adulterado/com lacunas, rollback de DDL e saída abrupta do processo,
mudança de senha/revogação, auditoria interrompida, isolamento de usuário e
aparelho, arquivos alterados, chave errada, contexto errado, WAL confirmado,
sessões revogadas na restauração e recusa de sobrescrita.

Próxima sequência dentro da parte 1: permissões delegáveis e auditoria consultável → backup/instalação/atualizações →
modos de armazenamento e serviços online → OpenAPI completa e aceite integrado.
Não fechar F01–F11 somente por compilar esta entrega.

Entrega 03: docs/architecture/78_PERMISSOES_DEPARTAMENTOS_E_AUDITORIA.md e docs/api/access-policy.openapi.json. Três operações novas: GET/POST access/policy e GET access/audit. Nenhuma interface alterada.

Entrega 04: docs/architecture/79_AUDITORIA_ADMINISTRATIVA_E_ISOLAMENTO_LOCAL.md. Sem nova migração ou interface. Isolamento por registro e serviços online/trabalhadores continuam no inventário de pendências.

Entrega 05: docs/architecture/80_INSTALACAO_E_ATUALIZACAO_LOCAL_LINUX.md. Gerenciador Python local e titan-local check. Sem nova migração/interface; dados da demonstração inalterados. Hash de pacote não autentica origem; assinatura de distribuição pendente.

Entrega 06: docs/architecture/81_MODOS_LOCAL_E_SERVIDOR_TLS.md. Servidor de loja com HTTPS e admissão por rede; cloud/hybrid descritos e bloqueados. Sem nova migração ou interface. Próximo trabalho: serviços online e isolamento de registros/terminais, antes de reconciliação comercial.

Entrega 07: docs/architecture/82_CONEXAO_E_AUTENTICACAO_ONLINE.md. Sem migração/interface. PostgreSQL sem senha padrão, TLS remoto verificado e validação compartilhada de JWT. Testes SQL mock das rotas online; PostgreSQL real e rotação persistida de refresh ainda pendentes.

Entrega 08: docs/architecture/83_SESSOES_ONLINE_PERSISTIDAS.md. Migração incremental PostgreSQL 5, sessões online próprias, refresh de uso único, auditoria e ferramenta de manutenção explícita. Teste PostgreSQL real opt-in não executado; frontend/SQLite inalterados.

Entrega 09: docs/architecture/84_TROCA_SENHA_ONLINE_E_COORDENACAO.md. Migração PostgreSQL 6 aditiva; nenhuma migração SQLite ou interface. Bloqueio por conta coordena login/refresh/revogação/senha. Teste PostgreSQL real permanece opt-in; produção em branch separada reserva SQLite 0028 após conferência.

Entrega 10: docs/architecture/85_VALIDACAO_POSTGRESQL_ISOLADA.md. Ferramenta para preparar banco novo de teste e exigir execução/aprovação do teste PostgreSQL das entregas 08/09. Sem alteração de API, migração ou interface. Testes da ferramenta não comprovam PostgreSQL real; registrar separadamente o resultado no PC. Recursos de teste preservados, credenciais temporárias e relatório privado.

Entrega 11: docs/architecture/86_RECUPERACAO_ONLINE_COM_CHAVE_PESSOAL.md. Migração PostgreSQL 7 aditiva; emissão autenticada com senha atual e recuperação por chave pessoal previamente guardada. Consumo, senha, revogação e auditoria transacionais. Não usa e-mail não verificado; recuperação por e-mail e interface permanecem pendentes. Teste PostgreSQL isolado ampliado, exigindo nova execução no PC. Produção e SQLite inalterados.

Entrega 12: docs/architecture/87_CONTRATOS_HTTP_ERROS_E_INVENTARIO.md. Negociação opcional de erros, inventário real, mapa OpenAPI e documentação de paginação existente. Sem migração nem integração da branch de produção. Contratos de payload pendentes estão explicitamente marcados.

Entrega 13: docs/architecture/88_CONTRATOS_CATALOGO_ESTOQUE_LOCAL.md. Sete operações locais com schemas detalhados e testes de payload HTTP/idempotência; nenhum handler ou schema de banco alterado.

Entrega 14: docs/architecture/89_CONTRATOS_CAIXA_VENDAS_LOCAL.md. Oito operações locais com contrato detalhado e teste do ciclo cego, venda, cancelamento e repetição. Nenhuma funcionalidade de pagamento/fiscal ou migração acrescentada.

Entrega 15: contratos de quinze operações locais de reposição, fornecedores, pedidos e sites, com teste HTTP de aprovação, replay e snapshots. Ver documento 90. Não implementa integrações externas nem incorpora produção.

Entrega 16: contratos HTTP de funcionários, capacidades e instalação de licença local, com testes de vigência e ausência de autorização implícita. Ver documento 91. Faturas, cobrança e RH completo permanecem pendentes.

Entrega 17: login local com JSON estrito e limite de corpo, no-store em login/contexto e contratos/testes de saúde, sessão e logout. Ver documento 92.

Entrega 18: paginação limitada nas consultas online de produtos e sugestões, mantendo isolamento por empresa e papéis, com contrato HTTP e testes SQL simulados. Ver documento 93.

Entrega 19: cadastro online de produto com JSON estrito, limites do esquema PostgreSQL e validação decimal antes da gravação. Ver documento 94. Sem migração, integração de produção ou alteração de interface. Modelo monetário legado, auditoria e idempotência de cadastro continuam pendentes.

Entrega 20: geração online de sugestões com limite síncrono de 1000 produtos, conflito explícito apenas para sugestão pendente e testes de rollback/commit/isolamento. Ver documento 95. Sem migração, aprovação, aplicação de desconto ou execução em lotes maiores.

Entrega 21: suíte PostgreSQL isolada para cadastro e geração de sugestões, com índice parcial concorrente, FK por empresa e rollback real. Ver documento 96. Aceite real exige execução no PC; preparo sem servidor não comprova aprovação.
