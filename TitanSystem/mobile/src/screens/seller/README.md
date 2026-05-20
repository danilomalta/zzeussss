# `src/screens/seller/` (Telas do vendedor)

Esta pasta contém as telas do fluxo do **vendedor** no aplicativo mobile.

## Objetivo

- Registrar vendas rapidamente no dispositivo móvel.
- Suportar operação sem rede (Modo Apagão) com sincronização posterior.

## Como os dados transitam no Modo Apagão

- Sem rede: a venda é salva localmente e marcada como “pendente de sincronização”.
- Com rede: o aplicativo tenta enviar as operações pendentes para o backend, mantendo idempotência (reenvio sem duplicar).

## Armazenamento local temporário (bibliotecas)

- Para mobile, a opção recomendada para dados locais estruturados é **WatermelonDB** (quando o volume e a necessidade de sincronização exigirem).
- Alternativas mais simples podem ser usadas no início, mas a regra é garantir durabilidade e reprocessamento seguro.

## O que deve ser programado aqui

- Tela de venda rápida (itens e finalização)
- Tela de pendências (operações locais aguardando envio)
- Tela de status de sincronização e conflitos

