# `components/kanban/` (Funil logístico em colunas)

Esta pasta contém os componentes de interface do **kanban de logística**, com colunas e cartões.

## Objetivo

- Visualizar o funil logístico (etapas) de pedidos/entregas/produção.
- Permitir movimentar cartões entre colunas com feedback imediato.
- Reagir a atualizações em tempo real vindas do backend.

## Arrastar e soltar

O kanban será construído com **arrastar e soltar**. A biblioteca prevista é:

- **dnd-kit**: para movimentação de cartões/colunas com acessibilidade e boa performance.

## Como isso se conecta ao resto do sistema

- Movimentações (ex.: troca de etapa) geram eventos:
  - localmente, atualizam a interface
  - remotamente, são persistidas no backend (status do funil) e propagadas por WebSocket

## O que deve morar aqui

- Componentes de coluna, cartão e cabeçalho
- Estado do kanban (lista por etapa) e funções de transição
- Integração com eventos em tempo real (entrada/saída) e controle de conflitos visuais

