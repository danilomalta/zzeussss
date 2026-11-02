# Fase 6 — implantação controlada pelo cliente

Base: feat/fase5-lacunas, d34dcd0. Sem acesso, conta mestra, chaves ou infraestrutura do fornecedor.

## Reuso comprovado
- Perfis: `backend/internal/deployment/profile.go`; `TestCustomerCloudVersion2RequiresRealTLSCertificate`, `TestServerHTTPSLoginReadRevocationAndNetworkAdmission` (cmd/titan-local). SQLite em `--db` escolhido pelo cliente; não há migração automática para banco central.
- Instaladores: `desktop/setup/`, `desktop/scripts/build.mjs`. Validação anterior Linux; Windows NÃO VALIDADO nesta fase.
- Backup: `localdb/backup/backup.go`; `TestEncryptedLiveSnapshotRestoreKeepsDataAndRevokesSessions`, `TestCurrentSchemaBackupRestoresDataAndRevokesSessions`. AES-GCM existente, snapshot consistente, esquema por LatestSchemaVersion(), restauração em caminho novo e sessões revogadas.
- Agendamento: `cmd/titan-backup/main.go`, comando watch; `TestBackupCLIAutomaticRunStopsOnCancellationAfterOneVerifiedSnapshot`.
- Chaves: `fiscalproduction/vault_lifecycle.go`, `cmd/titan-fiscal/vault.go`; `TestCustomerMasterKeyGenerationBackupRotationAndStaleWriter`. Nenhuma mudança no cofre ou na rotação.

## A — perfis e partida
`serve --db BANCO --station ESTACAO --deployment PERFIL` mantém destino explícito. Sem perfil, somente 127.0.0.1:8181. Os seis campos JSON são obrigatórios: version, mode, listen, tls_cert, tls_key, allowed_networks; arquivo privado 0600 sem links.
- Local: version 1, mode local, listen 127.0.0.1:8181, TLS vazio, redes [].
- Servidor: version 1, mode server, IP privado explícito, certificados absolutos privados do cliente, CIDRs privados explícitos.
- Nuvem do cliente: version 2, mode cloud, IP explícito do cliente, TLS próprio e CIDRs autorizados explícitos; nunca /0. É este serviço SQLite no host do cliente, não serviço do fornecedor.
Toda partida autentica a chave privada Ed25519 própria da estação contra o banco existente; exposição externa também confere sua estrutura. Login humano e permissões continuam obrigatórios; nenhum segredo padrão é criado. TLS direto exige certificado válido para o IP e TLS >=1.2 antes de abrir o banco. Ausência ou inconsistência recusa a partida sem imprimir chave.
Alternativa: proxy reverso TLS do cliente com backend exclusivamente loopback (perfil local) e `--reverse-proxy-tls`. A declaração não prova a configuração externa: o cliente deve bloquear acesso direto, configurar certificado e testar HTTPS no proxy. Declarar proxy com listen remoto é recusado. Não há confiança em X-Forwarded-For, nem bypass de autenticação.

## B — backup e ensaio de restauração
Destino exclusivo do cliente: `titan-backup watch --key CHAVE_PRIVADA --db BANCO --station ESTACAO --out DIRETORIO_PRIVADO --interval 1h`. Pasta 0700, chave privada própria de backup, diferente do arquivo da chave mestra fiscal. Um supervisor do próprio cliente mantém watch ativo; não há serviço remoto ou upload. Arquivo novo a cada execução; erros interrompem o agendamento, sem sobrescrita nem remoção automática.
`restore-test --key CHAVE --archive BACKUP --tenant EMPRESA --store LOJA --device APARELHO --out DIRETORIO_PRIVADO` testa um snapshot do esquema corrente. Compara, em memória, todas as tabelas de negócio ordenadas, não só o piloto. Confere integridade, histórico real das migrações, sessões revogadas, recuperação consumida e barreira fiscal; as tabelas de controle intencionalmente modificadas pela restauração ficam fora do digest. Não imprime registros ou hashes de dados. Todas as cópias desse ensaio ficam em subpasta privada no destino e são removidas ao terminar. Restauração histórica continua no comando restore original; o ensaio completo exige esquema atual para não comparar formatos diferentes.
A criação também coloca o snapshot temporário no diretório de destino escolhido. Preserve uma cópia externa da chave: o backup do banco não contém a chave nem o arquivo da estação. Nenhuma proteção de esquema foi reduzida. Evidência: `TestRestoreCommandChecksAllBusinessTablesAndCleansPrivateCopies` e testes de backup existentes, todos aprovados; cofre/rotação 20 vezes e frontend 2.547 aprovados.
