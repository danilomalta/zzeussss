# Parte 1 — Entrega 02: administração e recuperação local

Aplicar sobre a entrega Fundação 01 já registrada. Nenhuma tela foi alterada.
A parte 1 continua em andamento; esta entrega fecha os fluxos locais descritos
aqui, não a autenticação online nem permissões delegadas por departamento.

## Administração de acesso

| Rota | Corpo exato | Efeito |
| --- | --- | --- |
| `POST /local/v1/staff/:id/password-reset` | operation_id, current_password, new_password, reason | Troca senha de outro membro autorizado, revoga suas sessões e audita |
| `POST /local/v1/staff/:id/sessions/revoke` | operation_id, current_password, reason | Revoga sessões de outro membro autorizado e audita |
| `POST /local/v1/account/recovery/revoke` | current_password | Dono revoga suas chaves de emergência; mantém sua sessão atual |
| `POST /local/v1/account/recover` | identity_id, recovery_key, new_password | Recupera dono com chave preparada, sem criar sessão automaticamente |

- Três primeiras rotas exigem sessão atual; recuperação exige a chave de
  emergência válida e o aparelho comprovado pelo processo da API.
- Limite de cinco tentativas por minuto por IP e por rota. Rotas administrativas
  também exigem a senha atual do administrador, não a senha do funcionário.
- Dono administra outro membro não-owner vinculado à loja atual. Gerente somente
  employee/cashier/stock da loja atual. Gerente não administra gerente, contador,
  produção, fornecedor ou dono. Nenhum administrador redefine outro dono.
- Autorizações e sessões são revalidadas na mesma transação da escrita. Revogação
  de vínculo/aparelho impede novos pedidos e repetições.
- Sessões revogadas são do alvo na empresa, neste banco, inclusive em outras lojas.
  Credenciais da mesma identidade em outra empresa permanecem intactas.
- Estes recursos de segurança não dependem de licença Staff ativa; eles não criam
  funcionário, não concedem módulo e não mudam papéis. Cadastro Staff mantém seu
  contrato existente. Segurança de conta não deve ser bloqueada por inadimplência.
- operation_id é obrigatório nas duas operações administrativas. Repetição devolve
  o resultado original sem reaplicar e sem encerrar sessões abertas depois.
- Mesmo ID com ação, alvo, motivo, ator ou loja diferentes retorna 409.
- Repetição de reset com nova senha diferente, ou depois de a senha do alvo ser
  alterada, retorna 409. Nunca restaurar uma senha antiga devido a resposta perdida.
- Metadados de operação NÃO incluem senha, hash de senha, token ou chave de recuperação.
  O bcrypt fica somente na tabela de credenciais atual. Não colocar segredos no motivo.
- Senha, revogação e auditoria commitam juntos; escrita ignorada por trigger também
  é detectada. Falha de auditoria desfaz toda a operação.
- IDs até 128 bytes; motivo não vazio até 255 bytes; senha segue política de 12–72
  bytes sem espaços nas extremidades. JSON exato rejeita duplicatas, extras, null e
  conteúdo após o objeto. Não aceitar escopo da empresa/aparelho em campos HTTP.
- Reset de si próprio usa a rota anterior de troca da própria senha. O helper legado
  SetPassword não é exposto por estas APIs; não usá-lo em novas rotas administrativas.

## Recuperação do dono

Preparar ANTES de perder a senha. É uma credencial de emergência, não uma
recuperação por e-mail/SMS nem uma ferramenta que ignora a identidade do dono.

- CLI `titan-access recovery-init` exige banco/estação existentes, dono ativo e
  senha atual, lida de arquivo regular privado 0600. A chave aleatória tem 256 bits.
- Arquivo JSON privado 0600, exclusivo, contém versão e contexto. Chave nunca é
  mostrada no terminal nem fornecida por argumento/variável de ambiente.
- SQLite guarda só SHA-256 da chave e metadados de vínculo/validade/consumo.
- A chave fica vinculada ao dono, à empresa, loja e aparelho; validade de 365 dias,
  uma utilização. Gerente e dono revogado não podem emitir ou usar uma chave.
- Preparar uma nova chave substitui a anterior daquele dono/aparelho neste banco.
- Recuperação consome todas as chaves desse dono presentes neste banco, troca a
  senha, revoga suas sessões e audita na mesma transação. Novo login obrigatório.
- Troca normal da própria senha também invalida chaves de recuperação existentes.
  Depois de trocar/recuperar a senha, preparar uma nova chave se desejar recuperação.
- Se não houve preparação prévia, ou se perderam estação e chave, não há bypass
  automático. Recuperação assistida externa continuará exigindo procedimento próprio.
- Resposta perdida: tentar login com a nova senha. Não repetir recuperação
  automaticamente. Chave consumida retorna 401 inclusive em replay idêntico.
- Sem consulta pública que revele se a identidade/chave existe.
- Nenhuma informação de e-mail, CPF ou nome permite recuperar acesso por si só.
- Relógio anterior à emissão ou posterior à validade recusa a chave; não alegar
  proteção completa contra adulteração de relógio ou administrador do sistema operacional.

## CLI de manutenção

Compilar em backend: `go build -o /tmp/titan-access ./cmd/titan-access`.
Os caminhos abaixo são exemplos, não comandos de demonstração para o banco real:

```text
titan-access recovery-init --db /instalacao/store.sqlite --station /instalacao/station.json --owner ID --password-file /privado/senha-atual --out /privado/recuperacao.json
titan-access recover --db /instalacao/store.sqlite --station /instalacao/station.json --recovery-file /privado/recuperacao.json --password-file /privado/senha-nova
```

Arquivos de senha são entrada temporária de manutenção, 0600, no máximo 72 bytes
mais uma quebra de linha opcional. Não versionar, anexar, mostrar ou armazenar em
pastas sincronizadas públicas. A ferramenta não os apaga; o operador deve gerenciar
essas credenciais. Interface de entrada secreta interativa ainda não implementada.
Não passar senha como argumento de shell nem imprimir o arquivo de recuperação.

Guardar a chave de emergência separadamente do aparelho e do backup. Nunca
enviá-la ao serviço Titan. Posse dessa chave e acesso ao aparelho aprovado permitem
redefinir a senha do dono. Ela não substitui chave de backup ou chave de estação.

Publicação de arquivo e commit SQLite não são uma única transação. O arquivo é
salvo e sincronizado antes de gravar a nova chave no banco. Se o banco/auditoria
falhar, o arquivo pode existir sem estar ativo; a chave anterior permanece ativa.
Nada é sobrescrito ou apagado automaticamente. Investigar falha e usar outro caminho
para nova preparação; não considerar arquivo criado como prova de ativação.

## Compatibilidade e backup

- Migração 0026 adiciona estruturas; não modifica scripts aplicados.
- Backup desta versão aceita snapshots 25 e 26 verificados. Restore migra somente
  a cópia descartável de 25 para 26, preservando o arquivo original.
- Sessões E chaves de recuperação presentes na cópia restaurada são consumidas.
  Isso reduz reaproveitamento de credenciais antigas; não reconcilia revogações
  externas ou eventos comerciais posteriores ao backup.
- Depois de uma recuperação por backup, fazer login autorizado na instalação
  validada e preparar outra chave. Nunca ativar duas cópias do mesmo aparelho.
- Backups 26 não podem ser abertos pela aplicação antiga com máximo 25.
- A política de proteção, limite de 128 MiB, arquivos privados, restauração em
  destino novo e demais limitações da entrega 01 permanecem.

## Aceite observado e pendências

Testes: reset/replay/conflito, senha alterada posteriormente, loja/perfil/aparelho,
mesma identidade em outra empresa, auditoria ABORT/IGNORE, consumo único e
concorrente, chave errada/expirada/revogada, dono/estação revogados, emissão com
senha errada/falha de arquivo, CLI sem vazamento/sobrescrita, limiter e JSON ambíguo,
backup 25 migrado e chaves invalidadas apenas na cópia restaurada.

F06 avança para recuperação local preparada e administração local restrita.
Seguem pendentes: recuperação online, entrada secreta interativa, sincronização de
credenciais/revogações entre bancos, sessões individuais paginadas/gerenciáveis,
permissões delegáveis por departamento e consulta autorizada da auditoria.
Próxima entrega da parte 1: permissões delegáveis e auditoria administrativa.
