# `internal/auth/` (Autenticação, autorização e defesa)

Esta pasta concentra o que protege o acesso ao TitanSystem:

- autenticação (login, sessão, token)
- autorização (perfis: Dono, Vendedor, Cliente, Contador)
- middlewares de proteção (limites, bloqueios, auditoria)

## Princípios e regras obrigatórias

- **Falhas de login** devem ser observadas e limitadas para evitar força bruta.
- Logs de auditoria devem registrar tentativas críticas: login falho, acesso negado, mudanças de dados.
- A autorização por perfil deve ser **centralizada** (evitar regras espalhadas em handlers).

## O que deve morar aqui

- `middleware_tentativas_login.go`: middleware para bloquear/atenuar tentativas repetidas.
- (futuro) `perfis.go`: enumeração/constantes dos perfis e regras de acesso.
- (futuro) `tokens.go`: geração/validação de tokens, quando o método for escolhido.

## Como se conecta ao resto do sistema

- `cmd/api/main.go` registra middlewares globais daqui.
- `internal/routes` aplica proteção por grupo de rotas quando necessário.

