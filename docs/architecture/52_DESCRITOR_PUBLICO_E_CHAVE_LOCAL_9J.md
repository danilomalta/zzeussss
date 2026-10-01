# 9J — Chave local e descritor público assinado

## Escopo

O comando administrativo `cmd/titan-peer` cria a primeira chave X25519 do
aparelho, registra seu vínculo público mediante autenticação do dono e exporta
um descritor público assinado pela chave Ed25519 da instalação existente.
Não instala serviço, não abre porta, não aprova parceiro remoto e não concede
permissão para receber eventos. Não adiciona migrações nem modifica o frontend.

## Comandos

- `key-init --db CAMINHO --station CAMINHO --encryption NOVO_CAMINHO --owner ID`
  exige banco e station existentes, prova criptográfica do aparelho, senha do
  dono pelo stdin, sessão válida e papel atual exatamente `owner`.
  Cria arquivo privado 0600 sem sobrescrever e registra vínculo público de
  revisão 1 e auditoria. Gerente não pode fazer esta operação.
- `export --db CAMINHO --station CAMINHO --encryption CAMINHO --out NOVO_CAMINHO`
  prova o aparelho, carrega sua chave X25519 e escreve um arquivo novo 0600.
  A revisão padrão é 1; `--revision N` assina uma proposta, mas não altera a
  aprovação persistida. Esta etapa não oferece rotação operacional de chaves.
- `inspect --in CAMINHO_PUBLICO` verifica formato estrito, assinatura própria,
  versões, tamanhos e chave X25519 utilizável. Mostra identidade declarada e
  SHA-256 das duas chaves públicas. Não acessa o banco nem altera permissões.

Os caminhos e IDs devem vir da instalação confirmada, não de exemplos
inventados. O comando não cria um SQLite ausente. Para fornecer a senha em
terminal Bash, leia sem eco com `read -r -s`, encaminhe por `printf` no stdin e
remova a variável depois. Não coloque senha em argumentos, arquivos versionados
ou histórico. O stdin deve terminar após a senha (uma linha seguida de EOF);
o programa lê no máximo 74 bytes e a senha aceita tem no máximo 72 bytes.
Não cole a saída de arquivos privados na conversa.

## O que pode ser compartilhado

Somente o descritor exportado: versão, chave pública Ed25519, escopo declarado,
revisão, chave pública X25519 e assinatura. Nunca compartilhar station.json,
arquivo de chave X25519 privada, chave TLS privada, senha ou banco real.
Os identificadores de empresa, loja e aparelho também são metadados: envie
o descritor apenas ao administrador autorizado, embora não contenha segredos.

Uma assinatura própria prova posse da chave anunciada. Não comprova identidade
externa da empresa nem aprovação pelo dono do receptor. Antes de aprovar um
parceiro, compare ambas as impressões por outro canal autenticado; comparar
valores enviados junto com o mesmo arquivo não autentica sua origem.
A autorização de pares e tipos de evento permanece separada para a próxima
etapa. O transporte atual continua limitado à mesma empresa e loja.

## Falhas e limites

Arquivo e SQLite não têm commit atômico conjunto. Se a criação do arquivo
privado ocorrer e a aprovação posterior falhar, a operação retorna erro,
preserva o arquivo e a transação do banco é revertida. Não apagar ou substituir
essa chave, não declarar a instalação concluída e não repetir com outro caminho
antes de investigar. Esta etapa não inclui ferramenta de recuperação ou rotação.
Exportar um descritor também não resolve uma aprovação local pendente.

O comando tenta encerrar sua sessão temporária em até dois segundos, mesmo se
o contexto original tiver sido cancelado. Falha no encerramento não é exibida
com detalhes sensíveis; expiração e revogação continuam sendo aplicadas pelo
módulo de autenticação. Não há promessa de limpeza segura de memória nem de
proteção contra administrador malicioso do próprio sistema operacional.
Erros da CLI são genéricos para não imprimir credenciais ou conteúdo privado.

## Verificação

Testes adicionados usam bancos e arquivos em `t.TempDir()`: exportação e
inspeção, assinaturas adulteradas, JSON ambíguo, chaves de baixa ordem, dono
revogado, senha incorreta, gerente, proibição de sobrescrita e falha forçada na
auditoria com preservação do arquivo e rollback do banco. Não são executados
contra banco de produção. A validação Go deve ser confirmada no computador do
proprietário; gerar o patch não constitui evidência de testes Go aprovados.
