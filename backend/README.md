# Serviços do TitanSystem

A fonte oficial dos serviços Go está em backend/ na raiz do monorepo. TitanSystem/backend/ é uma cópia histórica preservada; não mover nem substituir a fonte atual com ela.

O backend não executa dentro de página HTML. Ele aplica autorização, contratos, regras comerciais, transações e persistência, recebendo requisições do aplicativo desktop e de outros clientes autorizados.

- Núcleo operacional SQLite: cmd/titan-local, contrato /local/v1, em serviço local ou servidor da loja conforme perfil suportado.
- API online PostgreSQL: cmd/api, contrato /api/v1, autenticação e escopo próprios. Não assumir equivalência de funções, licença ou dados com o núcleo local.
- O desktop em ../desktop/ usa a UI compilada de ../frontend-web/ e endereços explícitos; não inicia banco/estação/chaves automaticamente.
- Conexão HTTPS não libera modo cloud/hybrid bloqueado no executável, não replica dados e não implementa cotas offline.

A referência de backend em desenvolvimento pode estar numa worktree separada. Verifique branch, HEAD, contratos e versão de serviço no registro único ../docs/architecture/CONTINUIDADE_TITANSYSTEM.md. Esta organização não altera fonte Go, migrações ou banco comercial.
