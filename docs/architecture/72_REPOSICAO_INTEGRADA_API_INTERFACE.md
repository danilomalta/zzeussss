# Reposição local integrada à API e aos pedidos

Base de aplicação: commit 4700705. Esta entrega integra o núcleo de reposição existente à tela `/local/orders`, preservando a identidade visual.

## Fluxo disponível

1. Selecione um produto da loja e confira o saldo real.
2. Dono/gerente com `manage_replenishment` e contrato `orders` vigente configura mínimo e alvo.
3. Operador com `manage_stock` e contrato vigente calcula explicitamente a sugestão. Se o saldo está abaixo do mínimo, quantidade = alvo − saldo; caso contrário, registra `not_needed`.
4. Dono/gerente aprova ou rejeita com motivo. A aprovação reconsulta política e saldo dentro da transação; mudança exige rejeição/recalculo.
5. A aprovação aparece no formulário de pedido local já existente. Criar o pedido preserva itens aprovados, mas não envia ao fornecedor, não movimenta estoque e não gera pagamento.

Saldo: soma local dos movimentos de prateleira, depósito e recebimento da mesma empresa/loja, incluindo seus aparelhos registrados. Não representa disponibilidade reconciliada de uma instalação remota.

## APIs protegidas

- GET `/local/v1/replenishment/products?offset=0`: produtos, saldo, mínimo/alvo e revisão. Revisão zero significa **sem política**; os zeros não são uma política cadastrada.
- GET `/local/v1/replenishment/suggestions?offset=0`: sugestões, decisão, motivo, indicação de desatualização e pedido local associado. Paginação de 50.
- POST `/local/v1/replenishment/policies`: `operation_id`, `product_id`, `minimum_milli`, `target_milli`.
- POST `/local/v1/replenishment/suggestions`: `operation_id`, `product_id`.
- POST `/local/v1/replenishment/reviews`: `operation_id`, `suggestion_id`, `decision` (`approved`/`rejected`), `reason`.
- GET `/local/v1/replenishment/operations/:kind/:id`: resultado durável de `policy`, `suggest` ou `review`, com a entrada original, limitado à identidade/dispositivo/empresa/loja solicitantes.

A sessão determina contexto e permissões. As gravações usam `RequireTx` com `orders`, assinatura confiável e aparelho ativo na mesma transação da política/sugestão/revisão e outbox. O caminho legado de biblioteca permanece para seus consumidores anteriores; nenhuma rota HTTP nova utiliza esse caminho sem contrato.

Quantidades inteiras em milésimos, limitadas ao intervalo exato do JavaScript. Produtos `unit` não aceitam mínimo/alvo fracionário. Demais unidades mantêm a unidade cadastrada; não há conversão automática entre kg/g ou liter/ml. Mínimo pode ser zero e alvo deve ser maior. Motivo obrigatório, máximo 255 bytes. JSON ambíguo ou campos adicionais são recusados.

Toda escrita esperada exige uma linha afetada, incluindo registros de histórico e outbox. Falha SQL ou trigger `RAISE(IGNORE)` provoca rollback, incluindo observação do relógio do contrato.

## Recuperação no navegador

Antes do POST, entrada e identificador são persistidos no navegador, separados por empresa, loja, aparelho e operador. Web Locks impede duas abas de iniciarem operações independentes de reposição no mesmo contexto.

Consulta do resultado precede qualquer repetição explícita. Falha na consulta nunca provoca POST. Resultado confirmado precisa corresponder à entrada original. Perda de resposta, erro inesperado ou falha na confirmação preserva a operação como incerta; não permite descartá-la. Apenas uma primeira recusa definitiva, ou uma operação ainda não enviada, pode ser liberada para correção após uma consulta retornar 404. Uma recusa posterior nunca apaga uma incerteza anterior.

Consultas de registros próprios continuam disponíveis com contrato vencido, desde que sessão, papel e aparelho permaneçam autorizados; novas gravações e repetições exigem contrato vigente.

## Limites atuais

- Uma aprovação por produto permanece pendente até existir fluxo de recebimento. Criar o pedido não encerra artificialmente essa aprovação; outra aprovação do mesmo produto é bloqueada.
- Uma sugestão desatualizada pode ser rejeitada. O cálculo novo exige ação humana e outro identificador.
- Não há envio comercial entre empresas, negociação de preços, confirmação de fornecedor, agendamento, recebimento ou financeiro nesta entrega.
- Não há execução automática de compras nem aprovação automática.
- Dados locais de operação incerta precisam ser preservados; limpeza de dados do navegador não é recuperação segura.

## Validação e demonstração

Testes HTTP cobrem política → sugestão → aprovação → pedido, resultado durável, desatualização por política/estoque, concorrência, papel/contrato/revogação, entradas inválidas, rollback com ABORT/IGNORE, relógio e reabertura do banco. Testes do cliente cobrem cálculo decimal exato, resultado incompatível, perda de resposta, repetição, descarte seguro e falha de persistência.

Executar `npm --prefix frontend-web run test:local`, `npm --prefix frontend-web run build`, e no backend `GOTOOLCHAIN=go1.25.0 go test -count=1 ./...` e `go vet ./...`.

Para demonstração isolada: `python3 -B tools/demo_pdv.py --profile completo --api-port 8185 --web-port 3004`. Escolha portas livres, use o ID e a senha da nova demonstração e abra **Pedidos**. O auxiliar mantém os bancos reais intactos. Configure mínimo maior que o saldo e alvo maior que o mínimo, calcule, selecione a sugestão, informe motivo e aprove; em seguida cadastre fornecedor e crie o pedido local.

Build e testes HTTP/cliente foram executados no ambiente de preparação. Validação visual em navegador e teste manual no computador do usuário permanecem como aceite da demonstração; não foram anunciados como concluídos.
