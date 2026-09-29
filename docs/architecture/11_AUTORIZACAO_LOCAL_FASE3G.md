# Fase 3G — autorização conjunta para operações locais

`CanOperate` consulta em uma leitura transacional a identidade, o vínculo ativo, o papel, a loja e o estado aprovado do dispositivo antes de liberar uma permissão. `CanOperateReview` também recusa revisão da própria solicitação. Os testes cobrem caixa, gerente, lojas/empresas cruzadas e revogação de vínculo ou aparelho.

**A origem das identidades é indispensável:** o futuro backend local deve fornecer `actor` a partir de desbloqueio humano verificado e `device` a partir do desafio Ed25519 concluído; jamais confiar nos campos do corpo HTTP. A verificação e a operação de escrita precisam ser acopladas de modo transacional para evitar revogação entre a autorização e a venda. Esta função isolada não cria sessão, não faz desbloqueio offline e ainda não é chamada por PDV, mobile ou API PostgreSQL. Não apresentar as vendas como protegidas por esta função até haver integração e testes de fluxo.

Nenhum arquivo de banco real é aberto nesta etapa. Rodar testes em `t.TempDir()`, `go test ./...` e `go vet ./...` antes do commit.
