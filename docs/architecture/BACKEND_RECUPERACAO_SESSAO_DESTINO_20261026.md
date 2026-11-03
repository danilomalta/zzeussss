# Recuperação com sessão nova no destino e origem desligada

26/10/2026. Complemento do transporte/fila e do backup cifrado; base `64eff3d27498329e87a0336f47b2d34adcc9409f`. Não altera rotas HTTP, schema65, política comercial, frontend, estações ou serviços existentes.

## Problema e comportamento

A recuperação do motor já consultava recibos sem acessar a origem. Porém, depois de expirar/substituir a sessão do destinatário, a reautorização da fila exigia as duas sessões e a origem online. Além disso, o worker verificava os dois arquivos de credencial mesmo para uma consulta de recibo. Isso impedia recuperar uma entrega anterior com a origem desligada ou sem seu arquivo de sessão.

O comando explícito abaixo autoriza SOMENTE leitura do recibo da operação original:

```bash
titan-trade queue recover-session --queue /PASTA/FILA --id ID_ORIGINAL_DO_ITEM --target-token /ARQUIVO/SESSAO_PRIVADA_DO_DESTINO
titan-trade queue verify --queue /PASTA/FILA --id ID_ORIGINAL_DO_ITEM
titan-trade queue status --queue /PASTA/FILA
```

`verify --id` consulta somente esse item. `run`/`watch` continuam processando o lote e podem enviar OUTROS itens previamente autorizados; não use um lote para representar uma consulta isolada. A limitação de sessão é individual e não revoga nem amplia a autorização dos demais itens.

Não aceita `--source-token`, token bruto, outro ID de operação, outra mensagem ou outra identidade. Requer prova original do motor já persistida, com job/envelope/fingerprint e chave pública da origem conferidos. Sem essa prova, recusa antes de consultar a API ou alterar a autorização; não cria uma primeira tentativa. `recover-session` confere no destinatário empresa, loja, aparelho, usuário e fingerprint da estação, por sua API real e sessão nova. Não consulta a origem nem exige a presença de seu arquivo de credencial.

A fila passa a mostrar `receipt_only_session:true`. O worker vincula o hash à sessão nova do destinatário, exige a prova original e executa exclusivamente `recover`. A consulta pode confirmar recibo original e persistir esse resultado; não cria pedido, decisão ou efeito comercial remoto. Troca do arquivo de sessão posterior à autorização pausa antes de qualquer HTTP. Expiração/revogação remota continua sujeita à validação da API, com pausa e preservação das provas. Não amplia permissões nem aceita outro usuário por ser dono da mesma empresa.

404 permanece pendência, com pausa e zero POST. `queue retry` não é permitido nesse modo. Se a prova desaparecer depois da reautorização, o worker pausa; não converte a ausência em primeiro envio. Nenhuma opção force foi adicionada.

## Relação com retomada completa e restauração

`queue resume` continua sendo a reautorização COMPLETA das duas sessões, partes, fingerprints e envelope exato. Só depois de passar nela o modo limitado de sessão é removido; estado incerto ainda obriga consultar o recibo e eventual retry continua sendo outra ação explícita, com os IDs originais. Uma reautorização de leitura não é usada silenciosamente para obter essa permissão.

A barreira `recovery_only:true` dos backups restaurados é independente e permanente: nem `recover-session` nem `resume` a removem. Com prova original no backup, é possível trocar apenas a sessão do destino e recuperar mesmo com origem desligada. Backup anterior a qualquer tentativa continua sem prova e não pode usar esse atalho: exige conciliação e nunca autoriza envio automático. Arquivos de credenciais/chaves privadas continuam fora do backup.

Itens antigos sem o campo novo mantêm o comportamento anterior. Ferramenta antiga com JSON estrito recusa o campo novo quando ativo, em vez de ignorar a limitação e enviar. Essa compatibilidade não autoriza reescrever arquivos para rebaixar a versão.

## Evidências

Testes com duas APIs Go/TCP e SQLite descartáveis e CLI compilada: entrega incerta depois de commit, entrega confirmada e fila restaurada; logout e novo login no destinatário, arquivo de credencial sintético da origem ausente e origem efetivamente desligada ANTES do novo comando. Prova não modificada pela reautorização; recuperação com três GETs no destino, zero HTTP na origem, zero POST e um único efeito remoto. Nenhum mock de sucesso comercial; proxy usado apenas para perder resposta depois do commit real.

Regressões cobrem falta de prova, prova inválida, sessão de outra empresa, nova substituição da sessão depois da autorização, desaparecimento da prova entre autorização e execução,404 sem reenvio e retomada completa sem retry automático. A restauração exercitada especificamente neste teste é da FILA cifrada, não uma restauração nova do banco do destinatário. Pilotos completos de banco/reinício/restauração continuam na suíte integral.

Resultados completos, CI, binários instalados e limites são registrados com as evidências reais no registro único de continuidade. Não instalar monitor automaticamente, trocar os serviços8182/8282, selecionar chaves comerciais ou apresentar este comando como comunicação pública/Windows/ERP inteiro concluídos.
