# Contratos locais de reposição, pedidos e comparação

Base: `4d77e34`. Entrega 15 acrescenta schemas de quinze operações existentes, sem alterar handlers, migrações, permissões ou frontend. A branch de produção permanece separada.

| Domínio | Operações | Resultado efetivo |
| --- | --- | --- |
| Fornecedores e pedidos | 6 | Referência comercial local, aprovação disponível e pedido com cópia estável dos itens. |
| Reposição | 6 | Política, sugestão, revisão humana e consulta durável da operação. |
| Sites | 3 | Configuração HTTPS e consulta da versão registrada; comparação manual. |

O contrato está em `docs/api/orders-replenishment-comparison.openapi.json`. Todos os campos obrigatórios, respostas de criação/repetição e parâmetros de caminho são documentados. IDs não selecionam empresa ou concedem autorização: o servidor deriva o contexto da sessão e da estação.

## Limites que o cliente deve respeitar

Pedidos e reposição têm páginas fixas de 50, com offset não negativo de até 10 caracteres; não retornam total ou snapshot de paginação. A lista de pedidos retorna `items: []` dentro de cada pedido; as linhas estão na consulta individual. Sites não aceitam parâmetros de consulta e têm limite de 32 incluindo inativos.

Escritas de pedidos/reposição aceitam JSON exato até 4096 bytes; sites até 8192. Valores de quantidade são inteiros em milésimos. Políticas de produtos unitários exigem múltiplos de 1000. Os detalhes de limites em bytes, revisão esperada e permissões constam nas operações.

Repetir com novos IDs após resposta perdida pode criar outra operação. Preservar payload e IDs originais e consultar o registro durável quando disponível. Resultado de reposição contém variantes policy/suggest/review; a consulta preserva input e result, com repeated=true. A consulta de cadastro de site mantém a versão original mesmo após outra atualização.

## Fronteiras comerciais

`local_not_sent` não significa compra enviada ou aceita. Criar pedido não movimenta estoque, emite nota, agenda descarga ou envia mensagem ao fornecedor. Cadastro de fornecedor não cria usuário externo. Sites retornam `external_link` e `automatic_state=not_configured`: não há coleta automática de preços, consulta de numeração ou garantia de disponibilidade.

Reposição agrega estoque shelf/backroom/receiving da loja; não calcula capacidade de receitas ou reserva ingredientes. O módulo de produção pertence à branch separada.

## Verificação

O teste HTTP percorre política, sugestão, aprovação, cadastro de fornecedor, pedido, replay e consulta dos itens. Também cria e revisa um site, verificando que a operação anterior conserva nome e revisão. Confere listas vazias e preenchidas, página sem resultados e ausência de itens detalhados na lista de pedidos. Utiliza banco temporário e nenhum acesso externo.

O validador de schemas de testes acrescenta suporte a oneOf com exatamente uma correspondência, mantendo as validações existentes. É um subconjunto explícito, não certificação geral OpenAPI: não verifica todos os formatos e limites descritivos. Os testes existentes de autorização, conflito e rollback permanecem necessários.

Executar o verificador JSON/referências, testes dos domínios e da API local, vet e detector de corrida do contrato. Não requer servidor, PostgreSQL, sudo, migração real ou push.
