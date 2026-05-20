# TitanSystem: Processo Principal do Computador (Windows/Mac/Linux)

**O que você vai codar aqui**: 
A lógica primária do Electron. É este código que abre janelas, lida com atalhos de teclado (ex: F2 pra vender, F4 pra encerrar) e se conecta ao SQLite no "Modo Apagão".

**Linguagem**: TypeScript (Node.js API).

**Processo e Regras**:
1. Criar o `BrowserWindow` com `contextIsolation: true`.
2. Habilitar o modo de Tela Cheia (Kiosk) para pontos de PDV e Caixa de Supermercado (Windows e Mac).

**Barreira de Segurança Contras Vazamentos**:
* **ESTRITAMENTE OBRIGATÓRIO**: `nodeIntegration: false`. Isso fará com que se o frontend sofrer um ataque por javascript forçado, o hacker NÃO consegue ler arquivos confidenciais do Windows ou formatar o disco.
* Comunicar-se com o frontend através de Criptografia IPC/ContextBridge rigorosa (*preload scripts*). As variáveis globais/ambiente jamais estarão visíveis no `window`.
