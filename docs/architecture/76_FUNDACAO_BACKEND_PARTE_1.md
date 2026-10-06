# Parte 1 — Fundação operacional do backend

Base de aplicação: commit do usuário `294a544`. Esta entrega é o primeiro
incremento da parte 1; NÃO encerra os onze requisitos. Frontend inalterado.

## Matriz de conclusão

| ID | Requisito | Implementação existente / desta entrega | Trabalho em aberto e aceite |
| --- | --- | --- | --- |
| F01 | Banco local e atualização | Transações por migração, checksums; validação integral antes de migrar; recusa de versões futuras/histórico com lacunas; arquivo regular 0600; teste de saída abrupta sem commit | Política de distribuição/compatibilidade entre aparelhos; ensaio de queda durante upgrade completo da instalação; backup anterior a upgrade automático |
| F02 | Backend online | PostgreSQL, login/refresh, catálogo e parte de descontos | Serviços online completos; reconciliação com dados locais e testes isolados de cada operação |
| F03 | Isolamento | Contextos tenant/loja/aparelho/ator, autorização local; novas APIs revalidam sessão na transação | Inventário e testes de isolamento de todas as APIs, workers, exportações e integrações |
| F04 | Instalação | Inicialização e comprovação de estação existentes; CLI de backup | Instalador, atualização, recuperação e desinstalação preservando dados |
| F05 | Modos | Núcleos SQLite e PostgreSQL existentes | Implementação e demonstração de local, servidor, nuvem e híbrido; nenhum seletor visual comprova o modo |
| F06 | Autenticação | Login/logout/revogação; APIs de troca da própria senha, consulta e revogação das outras sessões | Recuperação do dono sem sessão/senha; troca/recuperação online; gerenciamento administrativo; sincronização da política de credenciais |
| F07 | Autorizações | Papéis e contratos existentes | Permissões por ação/departamento/loja e delegação auditada; o RH não ganha acesso total implicitamente |
| F08 | Auditoria | Eventos em operações críticas; novas mudanças de segurança atômicas e sem segredos | Cobertura administrativa completa e consulta paginada autorizada |
| F09 | Proteção de dados | Transporte cifrado; backup AES-256-GCM, chave separada 0600 e autenticação do arquivo | Backups remotos, proteção dos demais arquivos/certificados, rotação e recuperação de todas as chaves |
| F10 | Backup/restauração | Snapshot SQLite vivo consistente, verificação, restauração somente em arquivo novo; sessões restauradas revogadas; agendamento via CLI watch | Retenção, cópia externa, serviço instalado, monitoramento, PostgreSQL e ensaio de desastre completo |
| F11 | Documentação API | OpenAPI das três novas rotas e inventário dos grupos atuais | OpenAPI de todas as rotas, erros padronizados antigos, paginação uniforme e compatibilidade entre versões |

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
| Local `/local/v1` | health, login, me, logout, capabilities, staff, module-contracts | Recuperação de acesso e administração detalhada pendentes |
| Local `/local/v1` | products, catalog/search, locations, stock | Edição completa de catálogo e inventário operacional pendentes |
| Local `/local/v1` | cash, sales | Dinheiro, cancelamento integral, sangria/suprimento; sem integração real Pix/cartão |
| Local `/local/v1` | replenishment, purchase-suppliers, purchase-approvals, purchase-orders | Pedido somente local, sem envio/confirmação do fornecedor |
| Local `/local/v1` | comparison-sites, comparison-site-operations | Configuração e links externos; preços automáticos não implementados |
| Local `/local/v1` | account/sessions, account/password, account/sessions/revoke-others | Novas rotas nesta entrega; descritas em OpenAPI |
| Online `/api/v1` | auth/login, auth/refresh, saude, produtos, discounts/suggest, discounts/suggestions | Não equivale aos serviços locais completos |
| Online `/api/v1` | analises/produtos-parados, recompensas, accounting/sped, discounts/suggestions/:id/review | Rotas explicitamente indisponíveis; não contar como recurso pronto |

Contrato OpenAPI incremental: `docs/api/account-security.openapi.json`.
Ele cobre SOMENTE as novas rotas, não todo o ERP. Política: manter contratos
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
- Migrações futuras e backups antigos sem esquema completo atual são recusados;
  recuperação de versões anteriores terá fluxo específico antes de ser anunciada.
- Backups PostgreSQL, cópia externa, rotação, retenção e recuperação de desastre
  seguem pendentes. Nenhum banco real foi usado para os testes desta entrega.

## Verificação e próximas entregas

Testes cobrem histórico futuro/adulterado/com lacunas, rollback de DDL e saída abrupta do processo,
mudança de senha/revogação, auditoria interrompida, isolamento de usuário e
aparelho, arquivos alterados, chave errada, contexto errado, WAL confirmado,
sessões revogadas na restauração e recusa de sobrescrita.

Próxima sequência dentro da parte 1: recuperação/administração de contas →
permissões delegáveis e auditoria consultável → backup/instalação/atualizações →
modos de armazenamento e serviços online → OpenAPI completa e aceite integrado.
Não fechar F01–F11 somente por compilar esta entrega.
