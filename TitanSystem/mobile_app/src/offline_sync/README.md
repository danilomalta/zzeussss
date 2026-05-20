# TitanSystem: Sincronização Local (Modo Apagão)

**O que você vai codar aqui**: 
A lógica que intercepta as chamadas de API quando a Internet cai.

**Linguagem**: TypeScript + SQLite.

**Processo e Regras**:
1. Criar fila de "Vendas Pendentes" e "Ações de Controle".
2. Quando a rede volta, ele espanca a fila de processamento até zerar, sem sobrecarregar o Go.
3. Tratamento de Idempotência: Se o usuário enviar 2 vezes por lag, o Go Backend descarta o segundo, guiado por `id_externo_uuid`.

**Barreira de Segurança Contras Vazamentos Nível Militar**:
* Apenas IDs genéricos das contas serão expostos se a criptografia do sqlite local do mobile quebrar por root do SO. O token Master nunca deve estar cru aí.
