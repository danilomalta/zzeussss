# `internal/websocket/` (Comunicação em tempo real)

Esta pasta contém a base de **comunicação em tempo real** do TitanSystem via WebSocket.

## Objetivo

- Permitir chat em tempo real (ex.: Vendedor ↔ Produtor).
- Suportar atualizações instantâneas do funil logístico (ex.: movimentação de cards no kanban).

## Como os pings serão tratados

WebSocket depende de manter a conexão viva e detectar clientes desconectados. As regras aqui são:

- **Servidor envia “ping”** periodicamente para garantir que a conexão está ativa.
- **Cliente responde com “pong”** (ou equivalente) automaticamente (dependendo da biblioteca).
- O servidor define:
  - **prazo de leitura** (encerra conexão se não houver resposta dentro do limite)
  - **tratador de “pong”** (renova o prazo quando o cliente responder)
- Ao detectar desconexão, o servidor:
  - remove o cliente do “hub” (sala/canal)
  - registra auditoria quando fizer sentido

## O que deve morar aqui

- “Hub” de conexões (salas/canais), registro/remoção de clientes, broadcast.
- Rotas de WebSocket (pontos de entrada) e validações iniciais.
- Regras de segurança: autenticação do usuário e autorização por perfil/canal (antes de aceitar o upgrade).

## Como se conecta ao resto do sistema

- Rotas HTTP em `internal/routes` expõem endpoints de WebSocket (ex.: `/api/v1/tempo-real/chat`).
- Os eventos persistidos ficam em `internal/models` (ex.: mensagens do chat, status do funil).

