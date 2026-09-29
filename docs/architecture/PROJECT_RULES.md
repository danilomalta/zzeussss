# TitanSystem — Regras do Projeto

## Escopo da primeira entrega
Público: mercados e empresas fornecedoras/produtoras.
Fluxo: PDV e estoque → sugestão de reposição → aprovação humana → confirmação do fornecedor → reserva de horário de descarga → recebimento conferido → financeiro e relatório para contador.
- PC e smartphone vendem; o smartphone tem banco próprio e continua se o PC falhar.
- A rede local funciona sem internet.
- O cliente controla seus dados. O serviço Titan opera licenças e troca de mensagens autorizadas, sem acesso livre a vendas, estoque e documentos.
- Funcionários usam o celular para ponto, tarefas e metas; o gerente pode registrar avaliação privada.
- O fornecedor controla a produção por etapa.
- Fora da primeira entrega: consumidor, salão, oficina e marketplace de promoções.

## Regras obrigatórias
1. Trabalhar só na fase solicitada. Ler este arquivo e o código real antes de editar. Não inventar arquivos, rotas, números, sucesso de testes ou funcionalidades.
2. Antes de editar, mostrar `git status --short`, o commit atual e os arquivos afetados. Preservar alterações não publicadas.
3. Nunca imprimir, copiar, versionar ou pôr em imagem Docker o conteúdo de `.env`, senhas, chaves, tokens ou dados reais. Não alterar o banco real. Migrações incrementais, reversíveis quando possível, testadas em banco descartável.
4. Editar arquivos no editor, em mudanças pequenas, conferindo o diff. Não apagar `TitanSystem/` nem código duplicado sem inventário e comparação.
5. Separar empresa (tenant), loja, dispositivo e usuário. A API deriva empresa e permissões da sessão autenticada e valida todo acesso. O frontend não é barreira de segurança.
6. Dinheiro em centavos inteiros ou decimal exato. Transações, movimentos de estoque, outbox e idempotência protegem toda operação crítica. Sem "última gravação vence" para vendas, pagamentos, estoque ou horários de doca.
7. A tela mostra só estados reais. Rotular planejado, simulado ou indisponível. UI limpa, adaptável, com navegação por áreas e sem rolagem da página inteira.
8. Ao terminar cada fase: arquivos alterados, decisões, comandos e resultados, testes não executados (com motivo), riscos abertos, `git diff --check`, `git status --short` e demonstração curta. Sem push nem publicação automáticos.

## Comandos proibidos
`git reset --hard`, `git clean`, `rm -rf`, cópia indiscriminada de diretórios, `DROP TABLE`, `TRUNCATE`, reescrita total para resolver erro localizado.
Não executar `000001_init.sql` (contém `DROP TABLE`).
Não compilar com `go build -o api` em caminho versionado; usar `go build -o /tmp/titan-api ./cmd/api`.

## Estrutura efetiva
- Backend oficial: `backend/` (Go, Fiber, módulos em `internal/modules`, porta 8080). É o que `backend/Makefile` e `docker-compose.yml` usam.
- Frontend oficial: `frontend-web/` (Vite, TypeScript). É o que `docker-compose.yml` usa.
- `TitanSystem/backend` e `TitanSystem/frontend`: árvores duplicadas, menores. **Não são usadas** por Makefile nem Compose. Não apagar até haver inventário e comparação.
- CI: `.github/workflows/ci.yml` usa `backend/` para Go e `frontend-web/` para typecheck e build.
- Banco da API atual: PostgreSQL via pgx/GORM. O banco local offline do smartphone está planejado.
- `backend/.env` foi retirado do índice Git e permanece no PC. Credenciais anteriormente publicadas precisam ser trocadas pelo proprietário.
- O frontend usa `http://localhost:8080/api/v1`; acesso por outro PC ou celular ainda não está configurado.

## Estado das funcionalidades
Implementado e com teste observado:
- Login com JWT e cookie (teste de usecase de auth).
- Guarda de autenticação por tipo de token (teste de middleware).
- Health check HTTP (teste de integração).

Existe código, sem teste ou sem verificação nesta fase:
- Módulos `catalog`, `financial`, `pos`, `tenant` (sem arquivos de teste).
- Frontend: build passou a compilar na Fase 0; sem teste de comportamento.

Planejado (não implementado):
- Sugestão de reposição, aprovação humana, confirmação do fornecedor, reserva de doca, recebimento conferido, relatório para contador.
- Banco próprio no smartphone e operação offline; rede local sem internet.
- Ponto, tarefas, metas e avaliação de funcionários.
- Produção por etapa do fornecedor.

Simulado: nenhum item confirmado. Registrar aqui qualquer tela ou dado simulado que for encontrado.
