# Encerramento online e estado real — entrega 23

Base main b8b454f. Sem migração, alteração do banco comercial, frontend ou integração da branch de produção.

## Onde estamos

O mapa da entrega 22 registra 69 operações HTTP: 62 marcadas implemented/documented e 7 unavailable/pending. Isso representa aproximadamente 90% do inventário atual com implementação e contrato detalhado. Não representa 90% do produto nem 90% de todos os requisitos do backend: fluxos futuros podem precisar de novas rotas e critérios.

Os onze blocos F01–F11 da fundação possuem implementação, mas continuam com aceites abertos. O núcleo local possui caixa/venda em dinheiro, estoque, reposição e pedido local. A parte online possui sessões persistidas, senha/recuperação, catálogo e sugestões, com validação PostgreSQL real registrada pelo usuário na entrega 21. Produção evolui em branch separada e ainda exige integração/aceite conjunto.

Confirmação do fornecedor, descarga/recebimento, financeiro completo, Pix/cartão, fiscal, smartphone vendendo com banco próprio, RH operacional e reconciliação híbrida continuam fora desses 62 contratos comprovados. Não existe percentual geral confiável sem inventário completo de requisitos e aceite por fluxo. Não confundir quantidade de patches com avanço proporcional do produto.

## Encerramento do cmd/api

O processo captura Ctrl+C/SIGINT e SIGTERM. Após iniciar o listener, o servidor responde normalmente até receber cancelamento. No encerramento:

1. Fecha o listener, recusando novas conexões.
2. Aguarda conclusão das conexões/requisições em curso, até 45 segundos.
3. Aguarda a goroutine do servidor terminar.
4. Fecha o adaptador SQL e depois o pool PostgreSQL.
5. Registra mensagem de encerramento normal somente depois dessa sequência.

O primeiro Accept serve como barreira de inicialização: o fasthttp já registrou o listener quando o controlador tenta encerrá-lo. Cancelamento anterior ao início não cria um servidor e fecha os recursos já preparados. Erros de listener/banco não são refletidos como diagnóstico arbitrário no log final.

Se o drain não for confirmado no prazo, o controlador não chama o fechamento do banco sob um handler ativo e retorna erro. O executável sai com código diferente de zero, sem mensagem de sucesso. A saída do processo encerra conexões pelo sistema operacional; não é uma garantia de entrega da resposta ou de ausência de commit. Operação sem confirmação continua incerta: consultar antes de repetir, respeitando o protocolo de cada domínio.

O prazo limita a espera de drain HTTP, não cancela explicitamente todas as consultas SQL nem garante rollback. SIGKILL, falta de energia e reinício forçado não passam por esse fluxo. Supervisor/container precisa dar tempo suficiente para os 45 segundos de drain e fechamento de recursos; não há configuração automática de serviço nesta entrega. Não configura failover, alta disponibilidade ou atualização sem interrupção.

## Testes e compatibilidade

Testes TCP em loopback com portas aleatórias: requisição em curso recebe sua resposta antes do fechamento de recursos; novas conexões são recusadas durante drain; timeout retorna erro e não chama fechamento do banco; cancelamento antes do início libera listener/recursos. A fixture de corpo excessivo passa a anunciar Content-Length acima do limite sem enviar um corpo enorme, evitando corrida do cliente contra um reset TCP após a recusa antecipada.

Esses testes verificam a sequência de recursos com callbacks de fixture, sem abrir PostgreSQL real. Não comprovam todas as combinações de encerramento durante commit no banco; os testes transacionais PostgreSQL das entregas anteriores continuam sendo evidência separada. Não simula SIGKILL ou queda de energia.

Comandos: checker de contratos, suíte Go completa, go vet e race em cmd/api/rotas/apicontract. Nenhuma porta comercial ou banco do cliente é iniciado. O script de aplicação exige a base b8b454f limpa e só registra commit depois dos checks.
