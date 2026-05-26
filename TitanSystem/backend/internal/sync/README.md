# `internal/sync/` (Modo Apagão e sincronização)

Esta pasta contém a base do **Modo Apagão**: registrar vendas localmente sem rede e sincronizar depois.

## Objetivo

- Permitir que o PDV (aplicativo de computador) e o aplicativo do vendedor (mobile) continuem vendendo sem internet.
- Garantir que, ao voltar a conectividade (rede local ou nuvem), os dados sejam sincronizados com segurança e consistência.

## Como os dados transitam (visão prática)

1. **Criação local da venda** (sem rede)
   - O cliente (computador/mobile) registra a venda em um armazenamento local, com um identificador estável e dados completos do item vendido.
2. **Fila de operações**
   - Cada mudança relevante vira uma operação na fila: criar venda, adicionar item, cancelar, etc.
3. **Sincronização**
   - Quando houver rede, o cliente envia as operações pendentes ao backend.
4. **Consolidação no SQLite central**
   - O backend valida, grava no `titan_pos.db` e registra auditoria.
5. **Resolução de conflitos**
   - Quando duas origens alterarem o mesmo registro, regras de conflito devem ser aplicadas (ex.: prioridade por carimbo de tempo + política de negócio).

## Estrutura recomendada (o que deve ser programado aqui)

- **Modelos de sincronização**: “operação pendente”, “versão” e “estado de sincronização”.
- **Estratégia de identidade**: identificadores que não colidem entre dispositivos.
- **Regras de conflito**:
  - por registro (venda, item, estoque)
  - por campo (status, total, quantidade)
- **Protocolos de envio**: endpoints para “enviar lote”, “confirmar lote”, “buscar pendências”.

## Observação importante

Antes de adotar qualquer técnica avançada de consistência, a regra é: **não perder venda**. A base de sincronização precisa priorizar durabilidade local, idempotência e auditoria.

