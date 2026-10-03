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
- Backend oficial: `backend/`. API online Go/Fiber/PostgreSQL na porta 8080, usada por Makefile/Compose; API local SQLite em `cmd/titan-local` na porta loopback 8181, iniciada separadamente.
- Frontend oficial: `frontend-web/` (Vite, TypeScript). É o que `docker-compose.yml` usa.
- `TitanSystem/backend` e `TitanSystem/frontend`: árvores duplicadas, menores. **Não são usadas** por Makefile nem Compose. Não apagar até haver inventário e comparação.
- CI: `.github/workflows/ci.yml` usa `backend/` para Go e `frontend-web/` para typecheck e build.
- Persistência online: PostgreSQL via pgx/GORM. Núcleo local Go: SQLite com migrações numeradas. Aplicativo móvel com banco próprio e venda sem PC ainda não demonstrado.
- `backend/.env` foi retirado do índice Git e permanece no PC. Credenciais anteriormente publicadas precisam ser trocadas pelo proprietário.
- Frontend online: `http://localhost:8080/api/v1`. Integração local em `/local/login` e `/local/catalog`, via proxy de desenvolvimento Vite para `127.0.0.1:8181/local/v1`. Publicação desse caminho e acesso por outros aparelhos ainda dependem de integração específica.

## Estado das funcionalidades
Implementado no backend, com testes observados até o commit c89430c:
- Autenticação online e local, contexto de empresa/loja, papéis, convites e pareamento auditado.
- Catálogo, locais, movimentos e contagem de estoque no núcleo local.
- Caixa e venda em dinheiro, com operações atômicas, idempotência e outbox; APIs locais correspondentes.
- Política, sugestão e aprovação humana de reposição local; pedido entre empresas ainda não demonstrado.
- Contratos assinados por empresa e verificações nas operações locais integradas.
- Transporte cifrado entre aparelhos da mesma empresa/loja, inbox durável, recibos assinados, retries persistentes e ferramentas de aprovação/pareamento.

Integração de interfaces em validação:
- Login local, confirmação de contexto e consulta paginada do catálogo foram adicionados. Aceite exige build e demonstração contra a API local.
- Rotas online e locais usam guardas de sessão na interface; a autorização continua obrigatória no backend.
- PDV local em `/local/pos` integra consulta/abertura de caixa, carrinho, venda em dinheiro, consulta do registro e fechamento cego. Build e testes de cliente não substituem demonstração contra o servidor. Descontos, cartão/Pix e demais operações visuais continuam pendentes.
- Desktop Electron aponta para o Vite; empacotamento, preload e integrações de hardware não foram comprovados.
- Mobile é uma tela inicial: não implementa venda, banco próprio, ponto ou sincronização offline.

Pendências do roteiro original:
- Instalação completa do segundo aparelho, reconciliação dos dados comerciais, catálogo no sentido inverso e backup/restauração.
- Smartphone vendendo e recuperando sua venda com PC desligado.
- Vínculo mercado–fornecedor, pedido confirmado, agenda de descarga e recebimento conferido.
- Produção configurável, ponto/tarefas/avaliações, financeiro e exportações autorizadas ao contador.
- Cancelamento/devolução, descontos e pagamentos além da venda em dinheiro demonstrada no núcleo; fiscal e piloto continuam pendentes.

## Referência de progresso
O roteiro original de fases 0–13 continua sendo a referência de requisitos e aceite.
Os nomes de patches 8A–8I e 9A–9L identificam entregas técnicas; não demonstram conclusão das fases originais 8 (mercado–fornecedor) e 9 (descarga/recebimento).
Backend, frontend, desktop e mobile devem ter seu estado registrado separadamente. Teste de biblioteca não substitui demonstração do fluxo integrado ou teste no aparelho.
Não anunciar percentuais de conclusão sem requisitos e critérios de aceite medidos.

## Estados simulados ou indisponíveis
A mensagem mobile que anunciava sincronização offline ativa era incompatível com sua implementação e foi corrigida para indicar pendência.
O login online não deve afirmar HTTPS sem verificação nem registrar objetos Axios que podem conter senha.
Outras telas devem ser inventariadas antes de afirmar ausência de simulações.
