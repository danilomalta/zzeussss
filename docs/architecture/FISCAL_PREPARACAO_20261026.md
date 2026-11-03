# Fiscal — preparação durável e limitações reais

Base de desenvolvimento: `327002939a517a314864cb2d42cfb984ca09f79e`,
branch `feat/backend-b02a-compat`, 26/10/2026. Registro principal:
[BACKEND_PROGRESS.md](BACKEND_PROGRESS.md).

## Resultado implementado

- Migração SQLite67, incremental, com histórico e chaves estrangeiras.
- Perfis fiscais por empresa/loja com revisão imutável e compare-and-swap.
  Somente owner ativo, com permissão contábil e contrato Fiscal assinado válido.
  Não modifica cadastro central nem comprova credenciamento do CNPJ.
- CNPJ numérico/alfanumérico com DV segundo a Receita; CPF produtor ainda não
  implementado no perfil. Município verifica formato/prefixo da UF, não existência
  cadastral. IE/CRT precisam de validação tributária antes da emissão.
- Solicitação fiscal de venda existente e registrada; Sell + módulo Fiscal.
  Itens, pagamentos e total vêm do banco, nunca de valor fornecido pelo navegador.
  Snapshot imutável, SHA-256 e vínculo com a revisão original do perfil.
- Mesmo operation_id com contexto/corpo originais devolve o mesmo resultado.
  Outra solicitação para a mesma venda/modelo/ambiente é recusada, inclusive por
  outro aparelho. Não é licença para emitir55 e65 simultaneamente: a seleção
  fiscal definitiva ainda depende do motor de emissão e das regras regionais.
- Recuperação pelo ator/aparelho/loja originais; logout não apaga operação.
  ViewAccounting pode consultar a loja, Sell somente suas próprias solicitações.
  Histórico consultável após licença expirar, com sessão/permissão atuais.
- Listagem com limite/offset/has_more; não anuncia recorte como total fiscal.
- Catálogo documental datado das27UFs, fontes oficiais e lacunas explícitas.
  É referência de análise, não configuração ativa de Web Services ou alíquotas.
- Arredondamento decimal exato, sem float64, com escala explícita e testes.
  É utilitário de base; não calcula ICMS/ISS/IBS/CBS ou enquadramento tributário.
- Backup criptografado existente estendido para aceitar esquema67. Perfis e
  solicitações são preservados em snapshot/restauração, com IDs e hashes originais.
  Não há numeração fiscal alocada nesta entrega; prevenção de reutilização de
  numeração após restaurar backup antigo ainda exige mecanismo próprio.
- SPED legado retorna indisponibilidade antes de tocar banco, sem atraso aleatório,
  worker fictício, resultado concluído ou URL fabricada. Rotas antigas continuam
  indisponíveis, preservando seu contrato público.

Oito operações em `docs/api/fiscal-preparation.openapi.json`. Sem instalação de
dependências, mudança em frontend/Electron, parada de serviços ou banco comercial.

## Estados e limites

Todo resultado de preparação é `blocked_configuration`, nunca autorizado.
`issuance_available`, `official_authorization`, `number_allocated`,
`transmission_available`, `nfse_available` e `sped_generation_available` são falsos.
Não existe endpoint HTTP que aceite protocolo/autorização/XML de um navegador.
Não foi criado worker ou reenvio automático. Troca de sessão não retoma envio.
Os registros são imutáveis inclusive contra UPDATE/DELETE acidental; o ciclo de
vida futuro deve usar migração revisada, sem liberar remoção dos originais.

Esta é uma preparação de dados, não uma fila pronta para transmissão. Não atende
sozinha ao pedido de fiscal completo; não deve ser instalada para prometer emissão
num piloto fiscal. As APIs online legadas e suas interfaces NFEEmitter não foram
declaradas operacionais. Nenhuma SEFAZ foi chamada durante os ensaios.

## Pendências para emissão oficial

1. Definir cliente/UF/atividade/regime e parâmetros aprovados pela contabilidade,
   incluindo produtos, endereço, destinatário, natureza da operação e tributos.
2. Implementar/provar motor tributário e numeração fiscal transacional, com
   exclusividade por aparelho e proteção de restauração de backups antigos.
3. Fixar NT/XSD e tabelas vigentes, gerar XML completo, validar e assinar.
4. Cofre e acesso autorizado a certificado/CSC/CSRT; não receber segredos nestas
   rotas de preparação nem pôr em exportações, logs ou configurações públicas.
5. Adaptadores oficiais de autorização/consulta/eventos, TLS, timeouts e conciliação
   de resposta incerta; validar protocolo/chave/XML antes de afirmar autorização.
6. Credenciamento e obrigações da desenvolvedora por UF; só habilitar após prova.
7. Contingência por UF, DANFE/QR/guarda/exportação e eventos permitidos por modelo,
   incluindo cancelamento, inutilização e devolução sem apagar originais.
8. NFS-e da assinatura Titan em fluxo próprio, SPED real e documentos setoriais
   conforme atividade; não deduzir obrigação apenas por ter módulo de frota.
9. Integrar telas existentes aos contratos aprovados, cinco idiomas/permissões,
   e executar homologação oficial e E2E real com reinício/restauração.

## Fontes de validação usadas na implementação

- [RFB: manual DV CNPJ](https://www.gov.br/receitafederal/pt-br/centrais-de-conteudo/publicacoes/documentos-tecnicos/cnpj/manual-dv-cnpj.pdf).
- [NF-e: catálogo oficial](https://www.nfe.fazenda.gov.br/portal/webServices.aspx?AspxAutoDetectCookieSupport=1&tipoConteudo=OUC%2FYVNWZfo%3D).
- [NFC-e: catálogo SVRS](https://dfe-portal.svrs.rs.gov.br/Nfce/Servicos),
  [MG: catálogo próprio](https://portalsped.fazenda.mg.gov.br/spedmg/nfce/web-services/).
- As demais fontes e data da consulta estão no JSON incorporado
  `backend/internal/localdb/fiscal/uf_requirements.json`. Lacunas não são dispensas.

## Verificações executadas

Verificações concluídas sobre os fontes aplicados, com bancos e identidades
exclusivamente descartáveis:

- `go test ./...`: exit0,56pacotes aprovados (cache Go em pacotes inalterados).
  O pacote localapi executou em481,362s; duração da suíte não é latência da API.
-14novas funções de teste: perfis/CNPJ/decimais/27UFs, snapshot, idempotência,
  oito solicitações simultâneas, entradas estritas, permissões, licença, sessão,
  migração66→67, reinício, restauração e recusa do SPED fictício.
- `go test -race ./internal/localapi ./internal/localdb/fiscal
  ./internal/modules/financial/usecase -run
  'Fiscal|CNPJ|ExactDecimal|ProfileValidation|DocumentedRequirements|SpedNever'
  -count=1`: exit0; sem condições de corrida detectadas.
- Ensaio `TestFiscalRealHTTPPreparationRestartAndRestore`, com
  `-pilot-backup-cli /tmp/titan-fiscal-backup-20261026`: exit0; oito chamadas
  HTTP reais, três processos, reinício e backup/restauração pelo executável
  preservaram ID e SHA-256 originais. Chave errada, empresa errada e sobrescrita
  foram recusadas. O hash agregado pilot_cash_v1 não inclui tabelas fiscais;
  estas foram verificadas separadamente pela leitura HTTP do registro restaurado.
- Compatibilidade de backup das versões25–66 e upgrade até67 aprovada.
- `go vet` nos pacotes alterados e `go build ./...`: exit0.
  `titan-local` e `titan-backup` compilados em `/tmp`, sem instalar/substituir os
  executáveis existentes. `git diff --check`: sem erros.
- Conferência de hashes dos fontes aplicados: sem edição concorrente detectada.
  Migrações1–66 preservadas byte a byte contra a base Git.
- Health dos serviços8182 e8282: HTTP200; nenhuma ação de parada, atualização
  de executável ou migração dos bancos desses serviços foi executada.

O primeiro ensaio amplo encontrou expectativas antigas de esquema66, ausência
no inventário de rotas e um teste esperando403 onde contrato ausente deve retornar
503. Foi interrompido (`signal: terminated`) para correção; não é evidência de
aprovação. Todos foram corrigidos antes do ensaio final aprovado. Os testes de
rollback usavam67 como versão de sondagem e passaram a usar9998, preservando o
mesmo teste de falha/rollback sem colisão com a nova migração real.

Evidências sanitizadas: [evidence/fiscal-20261026](evidence/fiscal-20261026).
Nenhuma aprovação de SEFAZ, assinatura real, certificado comercial ou emissão em
produção foi executada. Frontend, Electron e instalador não foram alterados nem
homologados nesta tarefa. Não houve commit, push, pacoteZIP ou instalação.

## Incremento posterior no mesmo dia

Componentes locais foram acrescentados sem habilitar emissão na API. Ver
[FISCAL_COMPONENTES_20261026.md](FISCAL_COMPONENTES_20261026.md). As provas
anteriores de preparação67 permanecem válidas somente para aquele recorte.
