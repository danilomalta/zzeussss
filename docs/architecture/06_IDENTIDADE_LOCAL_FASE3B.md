# TitanSystem — vínculos e permissões locais (Fase 3B)

Esta entrega acrescenta `0002_memberships.sql` ao SQLite de cada dispositivo. Preserva `0001_initial.sql` e todos os dados já registrados. Uma `identity` pode ter vínculos distintos em várias empresas; cada vínculo tem papel, estado e lojas explícitas. A autorização nega por padrão papel, empresa ou loja ausente; revogação local passa a negar novas verificações. O dono pode acessar lojas existentes da sua empresa e gerenciar vínculos no escopo da empresa.

`identity.Can` é uma função interna de política, ainda **não ligada** a login, rotas HTTP nem ao aplicativo mobile. O chamador precisa obter a identidade de uma sessão já validada e fornecer a empresa/loja escolhida entre seus vínculos; não aceitar `identity_id` ou `tenant_id` do corpo como autoridade. Para operações críticas, verificar autorização e gravar a operação na mesma transação ou tratar revogação concorrente. A regra de aprovação da própria solicitação usa `CanReviewDiscount` e precisa ser integrada à origem real do pedido antes de habilitar a rota.

O cadastro antigo `users` em `0001` e o login PostgreSQL ainda não estão ligados a `identities`. Nenhum usuário real é migrado automaticamente. `membership_stores` não se aplica ao dono, mas acesso a loja inexistente ou de outra empresa continua negado. Funcionário comum não recebe permissões de venda/estoque; ponto e tarefas virão na fase própria. Fornecedor e contador só recebem acesso onde houver vínculo explícito e limitado.

**Ainda faltam para concluir a Fase 3:** autenticação/desbloqueio offline, credenciais locais protegidas, escolha de empresa e loja na sessão, convites de uso único, pareamento e revogação de dispositivos, auditoria, autorização efetiva das rotas atuais e testes de ponta a ponta. Não expor essas funções ao público antes da integração e validação.

Verificar `GOTOOLCHAIN=go1.25.0 go test ./internal/localdb/...`, `go test ./...`, `go vet ./...`, `git diff --check` em ambiente sem dados reais. Testes usam `t.TempDir()` e verificam múltiplas empresas, lojas, papéis, revogação, aprovação própria e referências cruzadas.
