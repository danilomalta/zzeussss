# Contratos HTTP do TitanSystem

Base desta revisão: entrega 11 (`fde43f8`). A documentação separa presença de rota, contrato de payload e demonstração do fluxo. Um endpoint montado não comprova que todas as integrações comerciais estejam prontas.

## Documentos e cobertura

| Arquivo | Cobertura |
| --- | --- |
| `route-inventory.json` | Métodos, caminhos, fronteira pública/sessão e origem das rotas locais e online da base. Testes comparam o inventário com o Fiber real. |
| `routes.openapi.json` | Mapa OpenAPI dessas rotas, com parâmetros de caminho e classificação. Operações marcadas `x-payload-contract: pending` ainda precisam de schemas detalhados. |
| `account-security.openapi.json` | Segurança e administração de conta local. |
| `access-policy.openapi.json` | Política de acesso e auditoria local. |
| `online-auth.openapi.json` | Login, refresh, sessões, troca de senha e chave pessoal de recuperação online. |
| `cash-sales.openapi.json` | Oito operações locais de caixa/venda/histórico/cancelamento, com schemas conferidos no ciclo HTTP. |
| `catalog-stock.openapi.json` | Sete operações locais de catálogo/busca/locais/saldo/movimentos, com schemas conferidos contra respostas HTTP. |
| `orders-replenishment-comparison.openapi.json` | Quinze operações locais de fornecedores, pedidos, reposição e sites; schemas conferidos contra HTTP. |
| `http-errors.openapi.json` | Negociação opcional de erros, compartilhada pelas duas APIs. |

Os documentos de produção pertencem à branch separada e não são incorporados nesta entrega. Compras, comparação e reposição locais possuem contrato detalhado nesta revisão; o catálogo online ainda requer contrato próprio. O mapa de rotas não finge completar esses contratos.

## Endereços e segurança

- Local: `http://127.0.0.1:8181/local/v1`, no perfil local. O servidor de loja usa HTTPS e sua configuração explícita; não se presume esse mesmo endereço para outros aparelhos.
- Online: `/api/v1` no servidor configurado, porta de desenvolvimento 8080. TLS de publicação não é demonstrado por esse endereço de desenvolvimento.
- Sessão local: token opaco no `Authorization: Bearer ...`. Sessão online: token de acesso, com revogação persistida verificada pelo backend. Refresh usa seu contrato próprio de cookie/origem.
- `public` no inventário significa ausência do guarda de sessão; origem, limitação de tentativas e validações próprias continuam aplicáveis. Publicamente acessível não significa operação irrestrita.
- O cliente não envia empresa, loja ou operador para obter acesso a recursos de terceiros. O servidor deriva o contexto autenticado e verifica registros/permissões.

## Erros e compatibilidade

Para solicitar o novo formato, envie `X-Titan-Error-Format: v1`. Erros retornam `{"error":{"code":"unauthorized","message":"Autenticação necessária ou inválida."}}`, por exemplo em HTTP 401. HTTP é a autoridade sobre o resultado; não interpretar uma mensagem como sucesso.

Sem esse header, o corpo legado permanece. Outros valores também preservam o legado. Respostas de sucesso, payloads, cookies e headers de retry/autenticação permanecem. O `Vary` inclui o header de negociação; erros negociados recebem `Cache-Control: no-store`. Mensagens não incluem request, SQL, tokens ou conteúdo do erro interno.

Os códigos são estáveis: `invalid_request` (400), `unauthorized` (401), `forbidden` (403), `not_found` (404), `method_not_allowed` (405), `conflict` (409), `payload_too_large` (413), `unsupported_media_type` (415), `unprocessable_request` (422), `rate_limited` (429), `not_implemented` (501), `unavailable` (503). Outros 5xx usam `internal_error`; outros 4xx usam `request_rejected`. Os detalhes de negócios dos erros legados não são carregados para esse formato genérico.

O middleware cobre handlers e guardas após sua instalação. Recusas do parser HTTP, handshake TLS, tamanho de corpo antes do middleware ou infraestrutura externa podem manter o formato da camada correspondente. Não se promete JSON para falhas de transporte.

## Paginação efetiva

| Endpoint | Entrada | Limites e retorno |
| --- | --- | --- |
| `GET /local/v1/products` | `limit=50`, `offset=0` | Limit 1–100; offset não negativo; `{items}`. Não retorna total. |
| `GET /local/v1/sales` | `limit=20`, `offset=0` | Limit 1–50; offset não negativo; `{items}`. Ordenação por data e ID decrescentes. |
| `GET /local/v1/catalog/search` | `offset=0`, `q`, `unit`, `pending` | Página fixa de 50; offset até 1.000.000.000; retorna `items`, `total`, `offset`, `limit`, `cost_visible`. Não aceita `limit`. |
| `GET /local/v1/access/audit` | `limit=50`, `cursor`, `department_id`, `source` | Limit 1–100; cursor opaco validado; retorno inclui próximo cursor conforme contrato de acesso. |

Esta entrega verifica os limites existentes sem mudar parsers/payloads. Offset não oferece snapshot consistente entre páginas se os dados mudarem. Não inventar total ou `has_more` em coleções que não retornam esses campos. Não aplicar a tabela a outras listas: fornecedores/pedidos/etc. precisam de revisão individual antes de uma política universal.

## Repetição, versionamento e disponibilidade

Uma resposta perdida em operação de escrita é resultado incerto. Consultar o estado durável; quando o contrato permitir retry explícito, reutilizar identificadores e payload originais. Não repetir venda, pagamento, estoque ou compra com novos IDs por causa de 409/503, timeout ou animação.

Adicionar campo opcional não deve mudar o significado dos campos existentes. Mudanças incompatíveis de payload, autorização, unidade monetária ou comportamento exigem uma versão de API nova e migração de clientes. Esta revisão não cria `/v2`, não desativa `/v1` e não decide uma data de descontinuação.

Rotas online marcadas `unavailable` continuam bloqueadas. Saúde do processo não comprova integração fiscal, pagamento, sincronização comercial ou envio de e-mail. Recuperação por chave pessoal exige chave preparada previamente; recuperação por e-mail verificado continua pendente.

## Exemplos sem credenciais

`curl -i -H 'X-Titan-Error-Format: v1' http://127.0.0.1:8181/local/v1/products` deve retornar 401 sem token. `curl -i http://127.0.0.1:8181/local/v1/health` verifica apenas o processo. Os testes exercitam erros, compatibilidade, inventário real e limites em bancos temporários; não usar dados reais para demonstração.
