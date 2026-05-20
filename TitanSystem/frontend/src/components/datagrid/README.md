# `components/datagrid/` (Grade de dados de altíssima performance)

Esta pasta contém a “grade de dados” do TitanSystem: uma experiência tipo planilha para gestão (produtos, vendas, clientes, estoque).

## Objetivo

- Substituir o fluxo de “planilha externa” por uma grade dentro do sistema, com:
  - milhares de linhas com desempenho excelente
  - filtros, ordenação, seleção, edição e atalhos
  - exportação e auditoria quando necessário

## Biblioteca prevista

- **AG Grid**: escolhida por altíssima performance, virtualização e recursos avançados.

## Regras de arquitetura

- Separar:
  - **modelo de dados** (colunas, validações, formatações)
  - **camada de consulta** (paginação, filtros e ordenação vindos do backend)
  - **camada de edição** (mudanças locais, confirmação e tratamento de conflito)

## Como se conecta ao resto do sistema

- Consome endpoints do backend para listagem paginada e atualização.
- Mudanças relevantes podem gerar `AuditLog`.

