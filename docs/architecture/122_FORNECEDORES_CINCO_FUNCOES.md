# Entrega47 — fornecedores internos: cinco funcionalidades

Base main ce3ec10. Migração incremental PostgreSQL000020, sem modificar SQLite ou a branch de produção.

1.Cadastro: nome, email/telefone opcionais, estado e motivo. UUID da operação é também ID do fornecedor. Persistência, auditoria e outbox na mesma transação.
2.Edição: PUT integral, expected_version obrigatório, motivo, ativação/inativação, incremento de versão e autor da alteração. Criação/autor original preservados. Conflitos409, nunca última gravação vence.
3.Consulta: contato atual por UUID dentro da empresa autenticada, inclusive inativo.
4.Listagem: busca literal em nome/email/telefone, active=all/true/false, paginação1..100, total e página no mesmo snapshot. Ordenação binária pelo nome e UUID. Chamadas entre páginas não compartilham um snapshot global.
5.Recuperação: recibo original pelo UUID da operação e pelo próprio ator, com before/after e versão. Replay só aceita mesmo ator/alvo/conteúdo normalizado; retorna o histórico original mesmo após edições posteriores.

## Escopo efetivo

É um contato declarado internamente, sem prova de identidade, aceite externo ou vínculo mercado–fornecedor. Nomes não são únicos. Email validado sintaticamente preserva caixa local; não envia email. Telefone aceita6..20 dígitos e separadores básicos, não prova titularidade. Email/telefone vazios são permitidos. Não gera compra, pedido, reserva, produção, movimento de estoque ou agenda de doca. Outbox gravada não significa evento enviado.

Acesso owner/admin/manager. Empresa/usuário/papel/sessão derivados do contexto autenticado; revalidação persistente dentro das transações. Recibos de outra pessoa ficam404 mesmo dentro da mesma empresa. Sessão revogada/empresa inativa recusadas. JSON estrito, até8KiB, campos obrigatórios, sem duplicados/extra/null; GET sem corpo. Query estrita, busca literal escapada, erros sanitizados.

## Migração e testes

CLI explícita: titan-online migrate-suppliers ou check-suppliers, após check-replenishment-proposals. Ledger20 SHA256, rejeição de versões desconhecidas/checksum diferente, DDL transacional com advisorylock. Nenhuma migração no startup da API. Migração depende de fornecedores/atores vinculados à empresa e histórico19 existente. Aplicada apenas no PostgreSQL novo isolado do teste durante instalação desta entrega.

Verificação local: parser/contatos/revisões, scanner/recibos, transações e rollback com SQLmock, migração fresh/replay/checksum/futuro, rotas/papéis/queries, contratos, suite/vet/race e builds. PostgreSQL real obrigatório no PC: idempotência/replay, alteração e stale revision, disputa concorrente, histórico original, páginas/busca literal com controle, inativação, isolamento, recibo por ator, outbox ignorada rollback integral, sessão revogada e catálogo/estoque preservados. Não confundir sucesso de testes mock com validação real; installer somente commita após PostgreSQL passar.

Sem PostgreSQL disponível no ambiente de preparação; execução real permanece no PC. Interface e aceite do fornecedor externo não implementados aqui.
