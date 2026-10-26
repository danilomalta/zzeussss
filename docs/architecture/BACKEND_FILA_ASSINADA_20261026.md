# Fila operacional de documentos assinados entre empresas

26/10/2026. Base1d8b885f9a3d5b280d26968e6f472d9316740cc0. Complementa o transporte avulso. Mesmas527rotas/schema65, sem migração ou efeito comercial novo. Não instala serviço ou troca os backends8182/8282. A fila entrega somente mensagens já exportadas por usuários autorizados; não gera compra nem decisão do fornecedor.

## Operação

Comandos de terminal descritos por `titan-trade queue help`: init, add, status, run, watch, pause, resume, retry e verify. init cria fila nova ou usa pasta vazia0700, recusando substituir conteúdo. add captura o job e o hash exato do envelope assinado, confere sessões/identidades/fingerprints e bloqueios de restauração nas duas instalações. Persistência concluída antes de informar queued. Arquivos job/tokens devem ser privados0600 e fornecidos explicitamente.

run processa um lote limitado; watch repete lotes até receber interrupção. Intervalo1s–5min, padrão10s;1–100itens por lote, padrão10, com contexto2min na CLI. Entrega confirmada e status do monitor são coisas diferentes. Somente confirmed significa recibo persistido e validado no destino, não aceite comercial ou pagamento. Cada envio usa os IDs e a assinatura originais. A decisão comercial continua no fornecedor; importação não reserva estoque nem transfere dinheiro.

Estados: queued, retrying(pré-envio seguro), uncertain, needs_retry, paused e confirmed. Espera progressiva2s até5min. Antes de todo processamento, hashes das sessões devem coincidir com os autorizados; trocar o arquivo de token não troca a identidade do item. Logout/401, contexto divergente, arquivo inacessível ou mensagem alterada pausam sem apagar a prova original. resume exige reautorização explícita nas mesmas empresas/lojas/aparelhos/usuários e envelope exato. Retomar sessão NÃO autoriza reenviar uma operação incerta.

Antes de qualquer POST, o motor já gravou a prova incerta em arquivo separado com fsync/rename/fsync. Se essa prova existe, o worker faz SOMENTE recover, inclusive depois de cair entre commit remoto e gravação do status da fila. Após404, exige queue retry explícito; não deduz “não ocorreu” para apagar IDs ou confirmar entrega. retry consulta antes e usa o mesmo operation_id. Um recibo antes confirmado que desaparece bloqueia reenvio, inclusive se o operador tentar retomar/repetir: investigar restauração do destino e recuperar a prova. verify reconsulta recibo sem criar primeira tentativa ou POST.

A autorização fica vinculada ao conteúdo, não apenas à chave: substituir o job original no disco não altera o snapshot da fila; um hash de envelope divergente é recusado mesmo que a mensagem atual tenha assinatura válida. Uma mensagem/operação no mesmo destino não entra de novo com outro ID. Não há troca automática de endereço, chave ou sessão para “resolver” erro. Remover item local não cancela pedido remoto; pause não desfaz requisição que já chegou ao servidor.

## Persistência, limites e consulta

queue.json contém configuração/IDs/hash/chave de sessão em forma de hash, caminhos de arquivos privados e status. Não contém token bruto, chave privada, documento, produto ou motivo. A consulta status omite caminhos de credenciais e hashes de sessão. JSON estrito, sem campos desconhecidos/duplicados; profundidade máxima32, arquivo até4MiB e500itens por pasta. Limite recusa inclusão; não trunca totais nem apaga confirmados. Para outra fila, preserve a anterior e a política de backup; esta versão não executa limpeza/arquivamento automático.

Lock flock Linux impede workers simultâneos da mesma fila e tentativas simultâneas do mesmo item, liberando na morte do processo. Escritas atômicas e sincronizadas, incluindo o diretório. A consulta de estado lê o último snapshot atômico sem esperar o lock do worker de rede. Uma falha de sessão de uma empresa pausa aquele item, não os outros; lotes respeitam limite e ordem elegível. As mutações locais podem recusar lock ocupado; não anunciar pausa ou retomada se a operação foi recusada. Falha de persistência após efeito remoto preserva a prova do motor para a próxima recuperação.

Esse controle não protege de administrador/processo malicioso com acesso ao mesmo usuário. Também não descobre revogação remota por si só. Os servidores continuam validando autorização, vínculo e assinatura; a regra de revisão de confiança após restauração permanece ativa. Linux validado; não executar sem lock equivalente em outra plataforma.

## Evidências

Testes usam duas APIs Go/Fiber e SQLite independentes com contratos descartáveis válidos. O teste mata o executável worker depois do commit REAL no destinatário e antes da resposta. Outro processo encontra a prova original e recupera com zero POST; apenas um efeito no banco. Testados troca por outra sessão válida, logout, recuperação com origem desligada,404 antes do efeito, retry explícito, recibo confirmado que passa a responder404, hash congelado, lote/isolamento, arquivos/JSON/locks e interrupção de monitor.

O proxy só provoca falha/resposta de transporte, não sucesso comercial fictício. A prova de404 após confirmação é uma resposta inconsistente injetada sobre banco real; não é anunciada como restauração real desse banco. As restaurações REAIS são feitas pelos pilotos completos, com binário de backup no CI.

Piloto completo com fila compilada: comércio171HTTP/seis processos +fornecedor29HTTP/três processos =200chamadas/nove execuções. Estoque5000milésimos, caixa1574centavos, obrigação em aberto0; devolução parcial, recebimento parcial e tratativa ainda em andamento, como antes. Mesmos IDs/assinaturas após reinício/restauração, ação explícita do dono revisando confiança e recibo original conferido. Versão avulsa continua testada separadamente. CI exige queue nos pilotos por variável/flag, sem habilitar dados de demonstração nas rotas reais.

Resultados da suíte completa, CI, hashes e instalação ficam no registro único de continuidade; rodadas focadas não são confundidas com aceite integral. Backup de fila/arquivos privados é separado do backup SQLite; preservar o diretório inteiro sem tokens em pacotes compartilhados. Proteção cifrada e restauração da fila são continuidade desta mesma frente. Não declarar comprovados Internet pública, firewall/TLS comercial, UI do fornecedor, Windows, fiscal/Pix/TEF, escala ilimitada ou ERP inteiro.
