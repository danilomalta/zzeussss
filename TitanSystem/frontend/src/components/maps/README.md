# `components/maps/` (Rastreamento e mapas)

Esta pasta contém componentes relacionados a **rastreamento visual** e mapas (logística).

## Objetivo

- Exibir posições, rotas e estados de entrega/coleta.
- Integrar atualizações em tempo real (ex.: última posição conhecida).

## Armazenamento e privacidade

- Coordenadas e eventos de localização são dados sensíveis; devem ser minimizados, protegidos e auditados.
- No backend, a evolução prevista inclui suporte a dados geoespaciais (extensão geográfica no SQLite).

## Como isso se conecta ao resto do sistema

- A interface consome dados do backend (HTTP) e recebe atualizações (WebSocket).
- O backend persiste status e mensagens relacionadas ao funil logístico.

## O que deve morar aqui

- Componentes de mapa, marcadores e trilhas
- Camada de atualização em tempo real (assinaturas e reconexão)
- Camada de normalização de coordenadas e estados (para o kanban)

