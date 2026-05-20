# TitanSystem: Componentes do Frontend Web

**O que você vai codar aqui**: 
Botões, Inputs, Tabelas, Modais, Toasts, usando Shadcn UI / Radix primitives adaptados ao layout visual estrito.

**Linguagem**: React (TSX).

**Processo e Regras**:
1. Todos os componentes precisam ser blindados contra múltiplos clics e não podem ficar carregando lixo eletrônico na DOM no "Fechamento de Caixa".

**Barreira de Segurança Contras Vazamentos**:
* NENHUM. O componente é visual. Mas validações contra entrada de código malicioso `SQL Injection` devem ser bloqueadas logo no input, pra evitar envio sujo de payload de rede.
