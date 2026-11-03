# Frontend pedidos autenticados entre empresas66

## O que funciona no código entregue

Nova rota /local/company-trade dentro das guardas existentes, com atalho condicionado a view_orders. Não altera autenticação, sessões, checkout ou os oito arquivos locais de carrinhos. Alterações compartilhadas limitadas ao router, menu e apêndice de continuidade. Sem mudanças no backend, migrações, CI, serviços ou dados.

- Consulta de identidade pública da instalação com conferência criptográfica SHA-256 do fingerprint. Download do JSON público; nenhuma chave privada é solicitada. Chave técnica não comprova identidade legal; vínculo exige conferência independente pelo dono.
- Vínculos paginados, ativos e revogados; cadastro de cliente ou fornecedor existente, revisão do fingerprint e revogação com revisão esperada. O seletor usa somente a página consultada; não apresenta página parcial como lista completa.
- Exportar pedido aprovado para fornecedor vinculado, baixar envelope original, importar no fornecedor, consultar a mensagem, confirmar quantidade e prazo ou recusar com motivo. Aplicar a decisão assinada ao pedido original no comércio. O payload base64 não é recodificado ou assinado pelo frontend; assinatura, validade, papéis, destino e transação continuam sob responsabilidade do backend.
- Histórico paginado com filtros de direção/tipo, recuperação individual e download explícito de envelope. Não envia a URL de terceiros, não usa token remoto e não chama relay/worker automaticamente.
- Consultar o bloqueio de confiança após restauração e registrar revisão pelo dono usando exatamente a geração consultada e referência independente.
- Revisão humana antes do envio. Quantidades em milésimos por parser decimal exato, sem arredondamento de ponto flutuante. Declínio sempre zera quantidade e prazo. Não mapeia IDs de produto entre catálogos.
- Fila local por empresa/loja/usuário/aparelho, sem credencial. Salva e relê corpo e operation_id antes de POST; Web Locks impede concorrência entre abas. GET do recibo antes de enviar e depois de responder. Falha de resposta preserva os mesmos bytes e ID; nenhuma recuperação é automática. Operação possivelmente enviada não pode ser descartada. Erros de armazenamento/corrupção/bloqueio/contexto impedem novos envios.
- Sete ações de mutação, seis rotas de leitura, cinco idiomas com as mesmas chaves, foco e campos rotulados, tabelas com rolagem interna. Revisão nativa dos idiomas e aceite com leitor de tela permanecem pendentes.

## Evidência e limites precisos

23 testes novos e suíte completa com 2465 testes passaram; TypeScript e build de produção passaram. Ponte opt-in Node→Go/Fiber via TCP loopback em duas instalações SQLite descartáveis passou, também com detector de corridas Go. Exercita as 13 rotas e sete ações, revisão da geração de restauração, vínculo, oferta, importação, decisão parcial, aplicação, consulta paginada/individual, recibos e revogação. O gate de restauração é semeado no fixture: este teste não faz backup/restore de arquivo.

A ponte usa o backend a5bbcdf/schema65. Os contratos usados foram comparados aos de 246691c e permanecem iguais; o worker/CLI adicionado depois não foi testado por esta interface. Nenhum endpoint/serviço instalado no PC foi atualizado. O código pode existir sem que o serviço em uso exponha as operações: erro 404/503 fica explícito. Leitura e autorização real dependem do serviço configurado.

Não foi executado aceite visual/nativo desta página, Windows, impressora, transações comerciais, transporte comercial entre empresas ou auditoria externa. O build mantém o aviso de chunk grande das planilhas. Nenhuma mensagem de sucesso declara liquidação, emissão fiscal, reserva de estoque ou produção automática. Consulta de recibo confirma somente a operação correspondente.

## Aplicação e desktop

aplicar.sh valida branch, ancestral público, marcador65 e preimagens dos arquivos; faz git apply --check, testa a candidata atual completa em pasta privada e revalida fontes/HEAD antes da escrita. Preserva alterações independentes, inclui backup dos fontes e rollback condicional de suas próprias escritas. Aplicação repetida da mesma entrega confere hashes e não reaplica. Não faz commit/push.

atualizar.sh reúne aplicação, testes desktop, compilação explícita, conferência/download do Electron quando necessário, novo atalho e abertura. É necessário fechar a janela anterior antes de executar: o perfil existente permite uma instância, e outra execução pode apenas focar a janela antiga. Não mata processos nem apaga perfis. O início não comprova a janela visível; erro de sistema/sandbox não é contornado. Nenhuma opção --no-sandbox é usada. O novo atalho preserva os anteriores com nome/sufixo distinto.

commit.sh é opcional após sucesso: confere marcador/HEAD e índice vazio, inclui lista explícita e apenas o apêndice desta entrega na continuidade. Trabalho independente continua fora do commit. Não faz push. Não reaplicar ZIP65 nem copiar indiscriminadamente a pasta fontes.

## Aceite visual de teste

1. Em serviço descartável com assinatura habilitada, abrir Pedidos entre empresas como dono com licença de pedidos; conferir a identidade pública em cada instalação. Sem view_orders a página/atalho não libera consultas. Gerente não cadastra/revoga/revisa confiança.
2. Trocar os JSONs públicos por canal autorizado, conferir empresa/fingerprint independentemente, vincular o fornecedor ao cadastro local e o cliente na instalação fornecedora. Consultar todas as páginas e revogar um vínculo de teste.
3. Exportar pedido de teste já aprovado, baixar envelope, importar na instalação do fornecedor. Consultar a mensagem e decidir quantidade/prazo. Transportar envelope da decisão manualmente e aplicá-lo ao pedido original. Conferir que estoque/dinheiro não mudam por aceitar o pedido.
4. Simular perda de resposta somente no ambiente de teste, recarregar, consultar o recibo original e recuperar sem criar outro ID. Testar duas abas, quota indisponível e logout antes da resposta.
5. Conferir gate de restauração e revisão da geração pelo dono, com referência real da verificação. Esta revisão não descobre revogações remotas automaticamente.
6. Verificar cinco idiomas, teclado, foco, contraste e larguras390/768/1280. Usar inventário66 para as demais pendências; não considerar o ERP inteiro encerrado por este fluxo.
