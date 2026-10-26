# Worker de mensagens vinculado ao contexto de origem

Data:26/10/2026. Base49136eab3e53e57d831ad7aae340ab3b6910629d, branch `feat/backend-b02a-compat`. Schema66 e529rotas preservados; não altera bancos, endpoints, Electron ou telas.

## Problema e entrega

Uma fila privada pode conter mensagens autorizadas de várias empresas ou operadores. Executar o lote inteiro ao trocar o contexto do desktop seria inadequado. A nova opção `--scope ARQUIVO` em `queue status`, `queue run` e `queue watch` limita a leitura e o processamento ao endpoint original exato: URL, empresa, loja, aparelho, operador e fingerprint da estação.

Não transforma o arquivo em credencial. Os jobs, envelopes, hashes de sessão, permissões atuais, aprovação de pares e barreiras de restauração continuam validados pelo transporte original. Não escolhe empresa, estação, banco, endereço ou sessão automaticamente. Não contém tokens, documentos, chaves privadas ou caminho de banco.

A implementação do backend eCLI é completa para esse filtro. A ligação no Electron ainda é uma etapa própria; não foi feita por esta unidade porque os arquivos desktop atuais têm trabalho local de outra frente a preservar. Esta entrega não anuncia a fila integrada ao menu do produto.

## Formato

O [schema do arquivo](../api/company-trade-queue-scope.schema.json) define versão1 e `source` com o mesmo formato do endpoint de origem do job. Arquivo absoluto, regular0600, limite4096bytes. Campos desconhecidos, duplicados, ausentes/nulos e links simbólicos são recusados. Fingerprint identifica a chave Ed25519 da estação aprovada, separada da validação TLS.

Um host desktop futuro deve montar esse contexto somente depois de verificar `/me` e a identidade da estação na API do perfil ativo. Nunca aceitar empresa/permissão só porque veio do renderer. Parar o worker ao encerrar ou trocar sessão/perfil; não reutilizar o arquivo de outro contexto. O usuário doCLI já precisa ter preparado explicitamente seus jobs e sessões autorizados.

## Comandos executáveis no terminal

Use caminhos próprios e verificados; os exemplos não selecionam uma instalação:

```bash
titan-trade queue status --queue /PASTA_PRIVADA_DA_FILA --scope /CONTEXTO_PRIVADO.json
titan-trade queue run --queue /PASTA_PRIVADA_DA_FILA --scope /CONTEXTO_PRIVADO.json --limit 10
titan-trade queue watch --queue /PASTA_PRIVADA_DA_FILA --scope /CONTEXTO_PRIVADO.json --limit 10 --interval 10s
```

`--scope` explicitamente vazio é erro; não cai no modo sem filtro. Sem a opção, oCLI antigo conserva seu comportamento, destinado à execução explicitamente autorizada do administrador. Não constitui fallback automático para o desktop.

`status` mostra somente itens do contexto, sem IDs, operações, tokens, hashes de sessão, caminhos privados ou contagem de outros contextos. Lote usa o limite somente nos itens correspondentes; os demais não têm token lido, tentativa incrementada, erro atribuído ou agendamento alterado.

## Troca de contexto e respostas incertas

`watch` fixa o conteúdo original para toda a vida do processo. Reescrever o arquivo com outra identidade não troca o contexto de um worker em execução. Remoção, invalidação ou mudança de permissão também encerram esse worker. Há conferência antes de cada tentativa e de cada chamada HTTP, incluindo a verificação anterior aoPOST. Contadores não contam uma chamada impedida pelo guard como HTTP enviado.

Uma requisição que já passou pelo envio pode terminar: preservar sua prova é obrigatório. O resultado original é gravado antes de encerrar o lote. Se a resposta for perdida após o efeito remoto, a operação fica incerta com seus IDs originais; a troca de contexto não a apaga nem faz novoPOST. Retomar no contexto original continua consultando o recibo, com retry comercial separado e explícito. Não há promessa de revogação instantânea de um pacote que já saiu pela rede.

Quando a troca acontece entre a gravação conservadora do estado incerto e o despacho, o guard pode impedir oPOST: a prova continua preservada e a tentativa pausa. Esse estado não autoriza reenviar automaticamente. A saída doCLI é erro de contexto, sem exibir sua identidade ou conteúdo.

## Provas e limites

Testes isolados cobrem seis dimensões do endpoint, fila mista, status sem vazamento, arquivos privados e JSON estrito, contexto removido/substituído e CLI sem fallback vazio. APIs Go/Fiber e SQLite reais comprovam: duas empresas isoladas na mesma fila; troca antes doPOST com zero efeito; troca depois doPOST confirmado; troca depois do efeito com perda da resposta e recuperação sem novoPOST. A fila restaurada também foi ensaiada com origem desligada e arquivo de sessão da origem ausente: recuperação de recibo preserva a barreira permanente e gera zeroPOST. Os comandos compilados `status/run/watch --scope` têm ensaios próprios, incluindo encerramento dowatch em vez de troca silenciosa de operador.

Resultados consolidados eCI do commit final ficam no registro único de continuidade da branch frontend e em `docs/evidence/backend-trade-scope-20261026/acceptance.json`. Não inferir aprovação por este texto; consultar o run correspondente e os logs.

Esta unidade não expõe banco na Internet, não cria sessão remota, não altera políticas de fornecedor nem remove a proteção `recovery_only` de fila restaurada. Fiscal, Pix/TEF, homologação Windows e ligação visual do transporte continuam fora desta prova. Serviços8182/8282 preservados; instalar o binário não os reinicia.
