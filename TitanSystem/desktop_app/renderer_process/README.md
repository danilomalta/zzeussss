# TitanSystem: Front Visual do Desktop

**O que você vai codar aqui**: 
A interface visual offline focada na digitação ultra-rápida do Operador de Caixa. O CSS "Titan Prime" com alto contraste. As telas para fechar o caixa e vender sem mouse.

**Linguagem**: TypeScript, React, Tailwind CSS/Vanilla CSS moderno.

**Processo e Regras**:
1. O Operador de caixa no modo Desktop nem sempre tem mouse, por isso focar 100% no fluxo de código de barras e preenchimento de input focado.

**Barreira de Segurança Contras Vazamentos**:
* A UI **NÃO ESCONDE** variáveis .env globais de bancos de dados da nuvem. O Desktop NUNCA se comunica direto com o banco da nuvem; ele se comunica com o Backend em Golang ou grava num `.sqlite` encriptado gerenciado pelo `main_process`.
* Impedir injeção de HTML no nome do cliente (Prevenir XSS React Injection).
