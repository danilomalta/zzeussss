# `src/pos/` (Frente de caixa)

Esta pasta contém a implementação da **frente de caixa** do aplicativo de computador.

## Objetivo

- Operação rápida: leitura de itens, carrinho, finalização e emissão.
- Funcionar com e sem rede (Modo Apagão).

## Como os dados transitam no Modo Apagão

- Quando houver rede: a frente de caixa envia a venda ao backend imediatamente.
- Quando não houver rede:
  - a venda é persistida localmente
  - operações são enfileiradas para envio posterior
  - a interface indica estado “pendente de sincronização”

## Armazenamento local temporário (bibliotecas)

- **IndexedDB**: armazenamento local no contexto de interface web (incluindo quando empacotado no Electron).
- Alternativas, dependendo da evolução:
  - **bibliotecas de fila e cache local** sobre IndexedDB para garantir durabilidade e reenvio

## O que deve ser programado aqui

- Fluxo de venda (itens, descontos, totalização)
- Camada offline (persistência local + fila + reenvio)
- Telas de estado de sincronização e resolução de pendências

