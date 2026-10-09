# Limites HTTP online — entrega 22

Base main 8714110. Configuração do executável cmd/api, sem migração, mudança de endereço padrão, TLS, frontend, produção ou servidor SQLite.

## Controles efetivos

| Controle | Valor |
| --- | --- |
| Corpo HTTP global | Até 65536 bytes (64 KiB) |
| Buffer de leitura de linha/cabeçalhos | 8192 bytes |
| Concorrência de conexões do servidor Fiber/fasthttp | 256 |
| Tempo de leitura | 10 segundos |
| Tempo de escrita | 30 segundos |
| Espera em conexão ociosa | 60 segundos |
| Host padrão | 0.0.0.0 |
| Porta padrão | 8080 |

Limites menores de cada handler continuam valendo: cadastro de produto aceita corpo de até 8192 bytes. Cabeçalhos/linha maiores que o buffer podem receber 431. Corpo acima do limite global recebe 413 antes de entrar nos handlers. Leitura parcial não mantém conexão indefinidamente. Concorrência é limite do servidor, não uma autorização ou quota comercial por empresa.

PORT aceita somente dígitos e valor 1–65535. TITAN_API_HOST opcional aceita IP literal IPv4/IPv6; para loopback use 127.0.0.1 ou ::1. Nome de host, espaços e porta embutida são recusados antes da inicialização do banco. Configuração padrão continua servindo LAN em 0.0.0.0:8080. Logs informam o endereço HTTP escolhido; não anunciam HTTPS inexistente. Nenhum arquivo .env é modificado por esta entrega.

X-Forwarded-For/X-Real-IP não passam a controlar c.IP(). Os limites existentes por IP continuam usando o peer. Atrás de proxy, clientes podem compartilhar o IP do proxy; política explícita de proxies confiáveis e admissão de rede continua pendente. Não basta confiar em um cabeçalho recebido pela internet.

## Erros e saúde

Erros de framework do servidor online usam mensagem genérica, no-store e Vary: X-Titan-Error-Format. Quando o cabeçalho v1 pode ser lido, usa o formato negociado existente. Sem negociação, retorna objeto JSON com error textual genérico. Esse fallback substitui o texto padrão do Fiber apenas para erros retornados ao framework; handlers com respostas próprias mantêm seu formato legado.

Não reflete diagnóstico do parser, SQL, senha, corpo ou cabeçalho enviados. Conexão encerrada ou falha anterior ao parsing pode não produzir resposta negociada; o cliente precisa conservar incerteza de uma escrita sem confirmação, em vez de repetir automaticamente.

GET /api/v1/saude continua público e retorna status=ativo. É liveness, não readiness: não consulta PostgreSQL nem comprova integrações comerciais. O limitador geral por IP também alcança essa rota. Contrato em online-health.openapi.json.

## Limites desta proteção

Os tempos de leitura/escrita limitam I/O HTTP, não garantem cancelamento de um handler ou rollback de uma consulta SQL lenta. Deadline por operação e desligamento gracioso continuam pendentes. Uma resposta perdida por timeout não prova que a operação falhou.

São controles de recursos numa instância. Não provam resistência a DDoS volumétrico, não criam páginas falsas, failover, WAF, filtragem de operadora, TLS público ou proteção distribuída. Esses componentes exigem infraestrutura e validação próprias. Não existe garantia de invulnerabilidade.

## Verificação

Testes usam TCP loopback com porta atribuída pelo sistema: corpo excessivo e cabeçalho excessivo são recusados antes do handler, sem refletir entrada; request parcial é encerrado pelo timeout curto de fixture. Outras verificações: limite exato de corpo aceito, erro arbitrário não revela diagnóstico, IP não é controlado por cabeçalhos forjados, host/porta inválidos recusados e saúde real preservada.

Suite Go, vet, race, contratos e diff conferidos. Testes não abrem a porta comercial, não inicializam PostgreSQL ou SQLite e não leem dados do cliente. Não são teste de carga, pentest ou simulação de DDoS.
