# TitanSystem — aplicativo para computador

`desktop/` é o cliente Windows/Linux. Ele abre as telas React compiladas de `../frontend-web/` em uma janela própria do Electron. Não depende de Vite ou de aba do navegador em produção. Não existe uma segunda implementação do ERP nesta pasta.

## Responsabilidades

- `main.cjs`: inicialização, instância única, janela, menus, permissões e sessões.
- `src/assets.cjs`: origem estável `titan://app`, arquivos compilados e rotas SPA.
- `src/config.cjs`: configuração explícita de conexão, validação e perfis separados.
- `src/transport.cjs`: encaminhamento limitado das duas APIs aos destinos configurados.
- `preload.cjs`: somente leitura/gravação da conexão pela janela local de configuração. Nenhuma execução de comando ou acesso arbitrário a arquivo pelo renderer.
- `setup/`: configuração de conexão; não contém telas comerciais do ERP.
- `scripts/build.mjs`: prepara aplicação em `.build/app-…`, sem `.env`, backend, banco ou dependências copiadas. Só o estágio atual entra numa futura distribuição.
- `scripts/start.cjs`: prepara/reutiliza a compilação e abre o Electron existente. Não instala nada.

`main.ts` permanece como entrada de compatibilidade para `main.cjs`; não é outra implementação. A árvore `../TitanSystem/desktop/` é histórica e permanece preservada, sem ser usada pelo aplicativo atual.

## Conexão local e pela internet

A escolha é do responsável pela empresa, na janela **Aplicativo → Conexão**:

1. **Local:** serviço operacional no próprio computador, com endereço literal loopback e caminho `/local/v1`.
2. **Servidor HTTPS:** serviço operacional autorizado em HTTPS com o MESMO contrato `/local/v1`. Pode ser servidor da loja; acesso pela internet exige backend/perímetro compatíveis e homologados.
3. **Local e comunicação pela internet:** autoridade operacional local mais destino online HTTPS `/api/v1`. A configuração separa os destinos; NÃO implementa sincronização comercial, nuvem, cotas offline ou replicação de banco.

O cliente nunca traduz `/local/v1` para `/api/v1`, nunca aponta para outro banco quando uma rede falha e não inicia processo de backend escolhendo banco/estação/chaves. O executável de backend atual tem restrições de implantação que devem ser respeitadas. HTTPS configurado não prova compatibilidade, versão ou serviço ativo.

Cada combinação de conexão recebe partição persistente própria. Trocar servidor exige novo contexto/login e não transfere tokens, cookies, IndexedDB ou tentativas pendentes para outro destino. A partição anterior é preservada; voltar à mesma configuração permite consultá-la. As pendências do navegador de desenvolvimento também permanecem separadas; não são migradas silenciosamente para o Electron.

Configuração fora da instalação: `connection-v1.json` na pasta de dados do aplicativo. Contém somente modo e URLs, sem credenciais. Mudanças preservam cópia anterior. Não edite configuração enquanto outro operador usa o mesmo perfil.

## Comandos executáveis no terminal

Na pasta `desktop/`, com Node e as dependências já disponíveis:

```bash
npm test
npm run build
npm start
```

Se Electron ainda não estiver nesta pasta, `npm start` informa a ausência. É possível usar uma instalação existente informando `TITAN_ELECTRON_BINARY` com caminho absoluto. Nenhum comando acima instala dependências, seleciona backend, migra banco ou cria instalador.

O build não lê `.env`, não carrega a configuração de Vite de desenvolvimento e não abre portas. Mantém os arquivos de estágios anteriores para revisão; não inclui a árvore `.build/` inteira em pacote. Um estágio com as mesmas fontes e arquivos íntegros é reutilizado.

A configuração `electron-builder.yml` descreve destinos Linux/Windows. Deve ser usada sobre o estágio identificado, com Electron e builder instalados/aprovados. Nesta entrega não foi gerado instalador nem validado Windows, assinatura, atualização automática, impressora, TEF ou câmera física. A validação observada está no registro único `../docs/architecture/CONTINUIDADE_TITANSYSTEM.md`.

## Atalho no Linux

Com o estágio compilado e uma instalação existente do Electron:

```bash
node scripts/shortcut.cjs /caminho/absoluto/electron "$HOME/Downloads/TitanSystem.desktop"
```

O gerador confere os hashes do estágio, cria arquivo novo e usa sufixo se houver outro atalho. Não instala pacote nem inicia serviço. No Ubuntu, o gerenciador poderá exigir **Permitir lançamento**. Feche versões anteriores antes de abrir a atualização. O atalho aponta para o estágio identificado; gere outro quando decidir trocar de compilação.

Teste nativo repetível, em perfil e servidor HTTP descartáveis, sem Vite ou banco:

```bash
TITAN_ELECTRON_BINARY=/caminho/absoluto/electron npm run test:native
```

Não demonstra Windows, autenticação online por cookies ou comunicação com servidor comercial na internet.
