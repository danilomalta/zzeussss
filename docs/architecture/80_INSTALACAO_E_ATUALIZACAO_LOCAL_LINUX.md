# Entrega 05 — Instalação gerenciada do backend local em Linux

Base do PC: `79b3890`. Sem alteração do frontend ou do banco da demonstração.
Sem nova migração: o esquema permanece 27. Este é um fluxo local executável,
não uma conclusão dos modos servidor/nuvem/híbrido nem um instalador de todo o ERP.

## Componentes

`tools/titan_install.py` usa Python 3, biblioteca padrão e flock do Linux.
Suporta Linux amd64/arm64; compilação e ensaios desta entrega em amd64.
Opera com o usuário proprietário, sem sudo e sem download automático.

Pacote: diretório privado 0700 contendo manifest.json 0600 e exatamente seis
executáveis ELF 0700: titan-local, titan-sync, titan-receive, titan-peer,
titan-access e titan-backup. São os comandos do cliente; titan-license não é
distribuído neste pacote. Cada executável tem SHA-256, versão/revisão e
arquitetura local no manifesto. Caminhos, duplicatas JSON, arquivos extras,
links simbólicos e permissões incorretas são recusados. A revisão é um inteiro
monotônico de distribuição, distinto do número da migração SQL.

**Hashes detectam alteração dos arquivos, mas não autenticam a origem.**
O operador deve confiar no código e no pacote local antes de executá-lo.
Assinatura com chave pública fixada e canal remoto de atualização seguem pendentes.
Não usar este mecanismo para executar arquivos recebidos de origem desconhecida.
O processo e o Python do gerenciador também precisam ser confiáveis.

Estrutura da instalação (raiz nova, pais existentes, sem links simbólicos):

| Caminho | Conteúdo |
| --- | --- |
| releases/VERSAO | Cópia dos seis executáveis e manifesto; nunca substituída por outra versão |
| data/store.sqlite | Banco exclusivo da instalação |
| data/station.json | Chave privada e identidade do aparelho, arquivo 0600 |
| private/backup.key | Chave aleatória 32 bytes 0600 criada pelo titan-backup |
| backups | Backups cifrados novos, sem sobrescrita ou remoção automática |
| checks | Diretórios 0700 de verificação temporária; cópia descriptografada removida ao encerrar normalmente |
| active.json | Versão ativa e estado habilitado; não contém credenciais |
| pending.json | Transição interrompida; somente versões, nome do backup e habilitação |
| process.lock | Bloqueio permanente; não substituir/remover enquanto houver processos |

Todos os diretórios gerenciados são 0700 e pertencem ao usuário. O gerenciador
não importa nem copia uma instalação antiga ou demonstração para a raiz nova.
`install` prepara executáveis e chave de backup; `run titan-local init` cria
uma empresa/loja/aparelho novos usando a inicialização existente. A senha entra
por stdin, nunca como argumento. IDs não secretos do dono/aparelho aparecem
na inicialização. Licença, cadastro comercial, pairing e conexão entre aparelhos
continuam exigindo os comandos/autorização existentes.

## Processo de atualização

1. Bloqueio exclusivo: recusa manutenção enquanto houver comando gerenciado ativo.
2. Valida ambos os pacotes e exige revisão candidata maior que a ativa.
3. Copia somente os executáveis listados para uma versão nova, sem sobrescrever.
4. A **versão antiga** cria backup cifrado consistente do banco usando sua própria
   interpretação do esquema. Backup inclui WAL confirmado, conforme biblioteca existente.
5. A versão antiga restaura esse backup em uma cópia descartável; a candidata
   executa `titan-local check` nessa cópia e comprova compatibilidade, integridade
   e identidade aprovada. A restauração revoga sessões somente na cópia de teste.
6. Grava e sincroniza pending.json ANTES de executar qualquer migração candidata
   no banco da instalação. O banco e as chaves da instalação não são substituídos.
7. Executa check com a candidata no banco real da **instalação gerenciada**.
8. Publica active.json por rename atômico e sincronização do diretório.
9. Remove somente o registro pending.json e sincroniza. Backups/versões permanecem.

`check` é manutenção, não um health check somente leitura: abre banco existente,
valida/migra com as regras existentes, comprova o aparelho e executa
integrity_check/foreign_key_check. Recusa banco ausente, histórico incompatível
ou aparelho não aprovado; não imprime chaves, tokens ou consultas do cliente.
Prova de aparelho pode gravar desafios de autenticação. Não executar em um banco
em produção fora de uma janela de manutenção.

Se o processo morrer após preparar a transição, `run`, update e mudança de
habilitação recusam acesso até `recover`. Recover valida versões/backup novamente,
retoma a candidata e somente então ativa. Se a candidata ou o backup estiverem
corrompidos, permanece bloqueado. **Nunca há downgrade automático ou restauração
do backup por cima do banco**: já pode haver migração confirmada ou dados posteriores.
A recuperação do banco em si continua sendo restauração em arquivo novo e
investigação manual. Atualização repetida/revisão menor é recusada.

Uma falha antes de pending.json mantém a versão antiga ativa; o backup e a versão
candidata já copiados podem permanecer. Escolher outra identificação/revisão após
investigar uma tentativa recusada, pois versões existentes não são sobrescritas.
Uma instalação inicial incompleta também preserva seus arquivos e recusa repetir
install na mesma raiz. Ainda não existe recuperação automática do init incompleto.

## Execução e bloqueios

`run` força os caminhos do banco/aparelho desta raiz e rejeita tentativas de
substituí-los nos argumentos, inclusive `-db=...`. Comandos aceitos:

| Executável | Comandos gerenciados |
| --- | --- |
| titan-local | init, check, serve |
| titan-sync | send (inclusive --once) |
| titan-receive | serve |
| titan-peer | key-init, export, trust, approve, revoke, pair-start, pair-finish |
| titan-access | recovery-init, recover |
| titan-backup | create, watch; --key/--out definidos pela instalação |

Comandos comuns mantêm bloqueio compartilhado, permitindo servidor e worker juntos.
Init/check e manutenção mantêm exclusivo. O filho herda o descritor: matar apenas
o Python não libera o bloqueio enquanto o executável continua usando o banco.
Não existe encerramento automático de processos, nem escolha automática de portas.
Os comandos ainda validam suas próprias flags e autorizações. Arquivos adicionais
de credenciais, certificados ou configuração devem continuar privados e preservados;
o gerenciador não os cria ou os envia. Executáveis chamados diretamente fora de
`run` não participam do bloqueio. Não misturar execução direta com atualização
gerenciada; processos não gerenciados são um risco aberto, não proteção comprovada.

`disable` interrompe a possibilidade de novas execuções, sob bloqueio exclusivo;
preserva dados, chaves, backups e versões. `enable` reativa sem recriar identidade.
Isso é desativação reversível, **não desinstalação/removal de pacote**. Remoção de
executáveis, serviço systemd, retenção e desinstalação formal seguem pendentes.

## Demonstração isolada

Em diretório descartável, compilar os seis comandos com CGO_ENABLED=0,
`go build -buildvcs=false -o DIRETORIO/titan-NOME ./cmd/titan-NOME`, chmod0700.
Depois, a partir da raiz do repositório, estes são formatos de comandos
(substituir os caminhos; não apontar para titansystem-demo):

```text
python3 -B tools/titan_install.py pack --source /tmp/binarios-privados --out /tmp/pacote-v1 --version local-v1 --revision 1
python3 -B tools/titan_install.py install --root /tmp/instalacao-nova --bundle /tmp/pacote-v1
python3 -B tools/titan_install.py run --root /tmp/instalacao-nova titan-local init --empresa Teste --loja Teste --dono Teste
python3 -B tools/titan_install.py run --root /tmp/instalacao-nova titan-local check
python3 -B tools/titan_install.py run --root /tmp/instalacao-nova titan-local serve --port 8199
python3 -B tools/titan_install.py status --root /tmp/instalacao-nova
python3 -B tools/titan_install.py update --root /tmp/instalacao-nova --bundle /tmp/pacote-v2
python3 -B tools/titan_install.py recover --root /tmp/instalacao-nova
python3 -B tools/titan_install.py disable --root /tmp/instalacao-nova
python3 -B tools/titan_install.py enable --root /tmp/instalacao-nova
```

Init lê a senha de stdin; para uso manual, fornecer via entrada privada sem eco
(o comando antigo não desabilita eco do terminal). Não escrever a senha no comando
ou guardar no histórico. Os testes passam senha fictícia por arquivo temporário
anônimo. Entrada interativa integrada sem eco permanece pendente.

Para avaliar atualização sem mudar o esquema, empacotar os mesmos executáveis
com outra versão/revisão. Isso testa ativação/backup/recuperação, não demonstra
compatibilidade com uma futura migração ainda não escrita.

## Aceite e limites

`python3 -B -m unittest discover -s tools/tests -v` compila seis executáveis reais
uma vez para os testes do instalador (Go no PATH, GOTOOLCHAIN compatível).
Também pode receber TITAN_INSTALL_BIN_DIR apontando a esses seis binários 0700.
Não substitui por mocks silenciosamente se Go estiver ausente.

Testes: pacote alterado, arquitetura errada, modos/links/JSON ambíguos,
sobrescrita, downgrade, estação revogada, bloqueios, filho herdando bloqueio,
init/update/check reais, chave/aparelho/empresa preservados, candidata recusada,
desativação/reativação e saída real do processo em prepared/migrated/activated.
Recover nessas fronteiras deve manter dados e chegar à versão candidata;
candidata corrompida deve manter bloqueio e não restaurar por cima do banco.
Migração DDL interrompida já é testada pela suíte de migrate_safety; estes ensaios
de versão usam o mesmo esquema 27. Ensaios com falta de energia, disco cheio,
hardware real e uma futura mudança de esquema continuam necessários.

Limites herdados: backup até 128MiB em memória, sem retenção/cópia externa; chaves
fora do arquivo cifrado; arquivos temporários após SIGKILL podem permanecer em
checks privado e exigem limpeza autorizada após investigação. Sem promessa de
apagamento físico seguro. Sem Windows/macOS/mobile, frontend empacotado, hardware,
serviço de inicialização automática, assinatura remota ou recuperação física do disco.

F01/F04/F09/F10 avançam, mas permanecem abertos na matriz da Parte 1.

Verificação observada nesta entrega: `go test -count=1 ./...`, `go vet ./...`
e `go test -race -count=1 ./cmd/titan-local` passaram. A suíte Python completa
passou 28 testes; após acrescentar a consulta HTTP real do health ao ensaio do
filho com bloqueio herdado, os 14 testes do instalador passaram novamente.
Os seis executáveis usados nesses ensaios foram compilados com CGO_ENABLED=0.
Nenhum teste dependeu do banco da demonstração ou de uma licença real.
