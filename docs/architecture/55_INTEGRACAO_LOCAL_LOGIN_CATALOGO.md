# Integração local 01 — login e consulta real do catálogo

Esta entrega pertence aos requisitos de autenticação e catálogo das fases
originais 3 e 4. Os patches históricos 9A–9L foram infraestrutura de comunicação;
não concluíram agenda de descarga e recebimento da fase original 9.

## Mudanças

- `/local/login`: ID do operador e senha → `/local/v1/login` → confirmação em
  `/local/v1/me`. Somente após confirmar o contexto a interface aceita a sessão.
- `/local/catalog`: consulta real de `/products`, 50 registros por página,
  atualização manual, estados de carregamento/erro/vazio e valores em centavos.
- Tokens locais opacos ficam em memória, separados do JWT da API online.
  Atualizar a página exige login novamente. Não há refresh offline implementado.
- Guardas de rota redirecionam visitantes sem sessão. Não substituem autorização
  na API. O catálogo elimina dados anteriores ao atualizar e invalida sessão
  quando recebe 401. Falha ao sair é informada sem fingir revogação no servidor.
- Vite encaminha `/local/v1` para a API local existente em loopback 8181.
  A porta 3000 é estrita para evitar troca silenciosa de porta no desenvolvimento.
- Login online deixa de registrar erro Axios completo e de afirmar HTTPS sem
  evidência. Mobile passa a indicar que venda e sincronização ainda não existem.
- `PROJECT_RULES.md` registra o estado atual e diferencia infraestrutura de
  comunicação das fases funcionais originais.

## Limites

Esta entrega integra consulta, não cria a interface completa de venda. Não
inventa usuário, função, empresa, produtos, estoque ou saldo. `/me` não retorna
nome/papel, por isso a interface não os presume. Cadastro, caixa, venda,
pagamentos, dispositivos móveis e produção ainda precisam de suas interfaces.

O proxy existe no servidor de desenvolvimento Vite. Não presumir que
`npm run preview`, nginx, Compose ou Electron empacotado publicam essa conexão.
Nenhum serviço, banco ou contrato real é iniciado ou modificado pelo aplicador.
Instalação, endereço e credenciais do cenário de demonstração precisam ser
confirmados antes de executá-la. Acesso pelo celular à API hospedada no PC não
constitui contingência quando esse PC falha.

Preços fora do intervalo inteiro seguro do JavaScript são rejeitados em vez
de exibidos com perda de precisão. Formatação usa dígitos dos centavos; não
introduz cálculo financeiro de venda no frontend. Não há leitura de chaves
privadas, persistência em localStorage ou envio de cookies da sessão online.

Desktop observado: inicia `/pos` no Vite; o preload citado e o processo de
build/empacotamento precisam ser verificados. Mobile observado: tela estática
e dependências sem implementação SQLite. Não declarar plataformas prontas.

## Validação e aceite

`node --test tests/localClient.test.mjs` verifica o contrato HTTP usando
respostas controladas e dados fictícios exclusivamente no teste: login,
confirmação de identidade, falhas, logout, paginação, catálogo vazio e preços.
Não é demonstração de uma API ou dispositivo real.

Executar `npm ci`, os testes e `npm run build` no repositório do proprietário.
Não considerar build aprovado sem seu resultado. Não modificar package.json,
lockfile ou dependências nesta entrega.

Após definir um cenário local descartável e iniciar os serviços, demonstrar:
1. Abrir `/local/catalog` sem sessão: redireciona para login local.
2. Senha inválida: erro, sem acesso ao catálogo e sem senha em logs.
3. ID e senha válidos: API confirma empresa, loja, identidade e aparelho.
4. Catálogo vazio: mensagem real; catálogo preenchido: dados reais e paginação.
5. Sair, expirar/revogar sessão e interromper a API: comportamento claro, sem
   inventar sucesso ou deixar novos dados aparecerem após invalidar a sessão.

O aceite integrado fica pendente até essa demonstração. A próxima integração
deve aproveitar esta sessão para operações reais de catálogo/estoque/caixa/venda,
e manter requisitos do mobile explícitos no roteiro original.
