# Backup cifrado e restauração segura da fila de transporte

26/10/2026. Base da implementação: `246691c6d68ff5d64920224c411a11f5d04f7ca5`. Complementa a fila de documentos previamente autorizados; não altera as 527 rotas, schema65, regras comerciais, frontend ou serviços8182/8282.

## O que é protegido

O backup SQLite oficial não contém os arquivos externos da fila. `titan-trade queue backup` protege separadamente o snapshot de `queue.json` e as provas do motor referenciadas pelos IDs desses itens. A captura mantém o lock da fila e os locks das tentativas até terminar a leitura. Worker concorrente ou tentativa avulsa com lock ocupado recusa a captura; não produz um snapshot inconsistente para aparentar sucesso.

Não abre banco, arquivos de token, chaves de estação, `.env` ou arquivos não referenciados. A fila guarda caminhos de arquivos privados e hashes de sessão, mas não o token bruto. Esses metadados e as chaves PÚBLICAS das provas ficam dentro da cifra; a saída JSON mostra somente escopo, contagens e SHA-256 do snapshot original. Não publique o arquivo de chave nem o backup privado.

Formato próprio `TYTQBAK1`, AES-256-GCM, nonce aleatório e domínio autenticado separado do backup do banco. Chave privada regular0600 com32bytes; não é senha informada na linha de comando. Use uma chave dedicada à fila, criada explicitamente por `titan-backup key-init`; a ferramenta não escolhe nem lê chaves comerciais. Arquivo cifrado0600 publicado sem substituir arquivo existente; sem compressão ou expansão de arquivos arbitrários. Snapshot limitado a8MiB, fila a4MiB/500itens, estados somente dos itens válidos. Versão, JSON estrito, identidade do job, hash do envelope e fingerprint da chave pública são verificados antes da restauração.

## Comandos executáveis no terminal

Os caminhos abaixo são exemplos que o operador deve escolher; não são instruções para iniciar os serviços existentes.

```bash
titan-backup key-init --key /PASTA/CHAVE_NOVA_PRIVADA
titan-trade queue help
titan-trade queue backup --queue /PASTA/FILA --key /PASTA/CHAVE_PRIVADA --out /PASTA/BACKUP_NOVO.enc
titan-trade queue verify-backup --archive /PASTA/BACKUP_NOVO.enc --key /PASTA/CHAVE_PRIVADA
titan-trade queue restore --archive /PASTA/BACKUP_NOVO.enc --key /PASTA/CHAVE_PRIVADA --queue /PASTA/FILA_RESTAURADA_NOVA
titan-trade queue status --queue /PASTA/FILA_RESTAURADA_NOVA
```

`verify-backup` autentica, decifra e valida sem API, criação de fila ou mudança do backup. Chave errada, adulteração, truncamento, arquivo público ou link simbólico são recusados. Não use o status do processo como confirmação comercial; cada recibo continua precisando ser validado.

Restauração exige destino novo, prepara diretório privado0700 e publica com `renameat2(RENAME_NOREPLACE)` no Linux; nem uma pasta vazia existente é sobrescrita. Não remove dados antigos. Eventual preparação incompleta fica privada, sem ser ativada. Sistemas sem essa publicação/lock equivalente são recusados; esta entrega comprova Linux, não Windows.

## Por que uma cópia antiga não pode reenviar

Uma fila copiada antes da primeira tentativa pode omitir uma entrega que aconteceu depois do backup. Ausência de arquivo de tentativa ou404 não prova que nenhuma obrigação foi criada. Todos os itens restaurados ficam `paused`, com `recovery_only:true`, sem agendamento. Job, mensagem, operação, hash e provas existentes são preservados.

`resume` reautoriza explicitamente as mesmas partes e envelope, mas NÃO remove a barreira `recovery_only`. Depois da retomada, o worker só consulta o recibo da operação original. Sem prova local, consulta as identidades e o envelope da origem antes de procurar o recibo no destino. Com prova local válida, recupera no destino mesmo com a origem desligada. Um recibo original válido confirma o item sem POST.404 mantém a pausa com `restored_receipt_missing`; `retry` é recusado para todo item restaurado, inclusive após nova sessão. Não há opção force.

Essa proteção pode bloquear uma mensagem que realmente nunca foi enviada: é uma pendência de conciliação, não autorização para inventar outro ID ou reabrir um envio. Compare os registros das duas empresas e o estado mais recente da fila original antes de definir uma nova operação comercial autorizada. Não apague a prova para contornar a barreira.

A barreira protege o fluxo das ferramentas cooperantes; um administrador malicioso que reescreva os arquivos ou crie deliberadamente outro job está fora dessa garantia. Os servidores continuam validando sessão, vínculo, assinatura e confiança após restauração. Restaurar a fila não restaura o banco, sessão, chave privada ou confiança da estação. Preserve e teste também o backup oficial SQLite e a recuperação controlada de credenciais; não há snapshot transacional conjunto entre duas empresas.

## Evidências e limites

Regressões isoladas cobrem cifra, autenticação, privacidade, locks, consistência de provas, publicação sem substituição e estado restaurado sem execução automática. O executável compilado é testado com duas APIs Go reais e SQLite descartáveis em quatro cenários: mensagem não enviada, entrega posterior ao backup, resposta perdida após commit e entrega confirmada antes do backup. Nos três últimos, a restauração encontra o recibo original sem novo POST; no primeiro, mantém a pendência e não cria efeito remoto. Nos cenários com prova persistida, a origem é desligada antes da recuperação.

Suíte completa, CI, hashes instalados e resultados reais constarão do registro único de continuidade após a conclusão das verificações. Não confundir esses testes com Internet pública, configuração comercial, UI completa do fornecedor, Windows ou ERP inteiro concluídos. Esta entrega não instala daemon nem reinicia os dois serviços preservados.
