# Fase 8H — Caixa na API local

Base de aplicação: commit 184eea8. Sem alteração do frontend, PostgreSQL,
dados reais, migrações ou configuração de rede.

## Rotas

Todas ficam em /local/v1 e exigem sessão humana ativa vinculada ao aparelho
do servidor. Empresa, loja, operador e aparelho vêm dessa sessão.

- GET /cash/current: retorna session null ou session_id e opened_at do turno
  aberto do próprio operador no aparelho atual. Não retorna saldo esperado.
- POST /cash/open: recebe session_id e opening_cents inteiro não negativo.
  Criação retorna 201; repetição idêntica de turno ainda aberto retorna 200.
  Outro turno aberto no aparelho, dados divergentes ou turno já fechado: 409.
- POST /cash/close: recebe session_id, operation_id e declared_cents inteiro
  não negativo. Retorna 200 com resultado da conferência após commit.
  Repetição idêntica retorna repeated true sem novo evento.
  Dados divergentes, turno ausente ou operation_id reutilizado: 409.

Valores são centavos int64, sem conversão intermediária para float.
Objetos JSON devem conter exatamente os campos exigidos. Campos adicionais,
duplicados, nulos, ausentes, JSON concatenado e valores inexatos são rejeitados.
Corpos superiores a 4096 bytes recebem 413; IDs maiores que 128 bytes são inválidos.

## Autorização e transação

Abrir e fechar exigem Sell, aparelho aprovado e contrato POS válido,
com as dependências exigidas pelo verificador (core e inventory).
RequireTx usa a mesma transação de caixa, vínculos, fechamento e outbox.
Falha na gravação desfaz também a observação de relógio do contrato.
Sem verificador configurado, mutações retornam 503; sem módulo ou contrato
válido, 403. Consultas autorizadas continuam disponíveis.

Esta fase mantém a regra vigente de licença válida para mutações: expiração
bloqueia também fechar e repetir um fechamento pela rota POST. O turno não
é apagado nem fechado automaticamente. Uma futura política de tolerância
ou encerramento administrativo precisa ser definida e testada explicitamente.
Não existe renovação automática, fechamento emergencial ou bypass oculto.

Open e Close internos preservam compatibilidade para testes e chamadas
existentes; as novas rotas usam exclusivamente OpenWithContract e
CloseWithContract. Esse caminho interno não é uma fronteira de licenciamento
contra alguém que controla ou modifica o executável.

## Testes incluídos

Fechamento cego, repetição, conflito entre turnos, reutilização de operação,
ausência de licença/verificador, papéis e revogação, JSON ambíguo e dinheiro
inexato, rollback de abertura/fechamento/outbox/relógio, reabertura do SQLite,
abertura concorrente e consulta com licença expirada. Teste de domínio
assegura que contrato nil não recai no caminho interno sem licença.

Todos os cenários usam bancos descartáveis criados por t.TempDir.
O teste de conferência insere um movimento conhecido apenas nesse banco;
não simula integração de venda em produção.

## Limites

Não entrega venda HTTP, sangria, suprimento após abertura, cancelamento,
pagamento eletrônico, impressão fiscal, transporte da outbox nem celular
independente. O servidor continua em loopback.

O pacote foi preparado por leitura do código publicado e verificação do
patch. Go não está disponível no ambiente de preparação. Compilação, testes
e vet devem passar no PC antes de registrar esta fase como entregue.
