# Cadastro fase 3b

Base: feat/cadastro-fase3, fb05901.

## Segurança do repositório

`frontend-web/.env` retirado apenas do índice, preservado no disco. Regras explícitas para `.env`, `.env.local`, `.env.*.local` e `frontend-web/.env`; `.env.example` permanece permitido. Nomes encontrados: `VITE_API_URL`, `VITE_APP_ENV`. Nenhum nome indica senha, token ou chave; valores não foram exibidos. Histórico não reescrito.

Verificações: arquivo local existente; ausente do índice; variantes locais ignoradas; exemplos não ignorados; git diff --check sem erros. Nenhum código Go alterado; testes Go não executados.

## NF-e 55: interrupção antes da implementação

Fontes consultadas: https://doc.focusnfe.com.br/reference/emitir_nfe, https://doc.focusnfe.com.br/reference/consultar_nfe e https://doc.focusnfe.com.br/reference/cancelar_nfe.

As páginas confirmam POST/GET/DELETE em /v2/nfe, envio assíncrono 202 ou síncrono 201, consulta pela referência e caminho de DANFE no retorno. A página de emissão remete o contrato completo dos campos a campos.focusnfe.com.br, domínio não incluído na autorização desta tarefa. A interface de itens da página permitida foi aberta e conferida. Não foi possível confirmar nessa documentação restrita o contrato completo de campos já usados pelo mapeamento NFC-e, como formas_pagamento e serie, para reutilizá-los na NF-e sem pressupor equivalência.

Pela instrução de parar se algum campo não estiver documentado, nenhum comportamento do adapter foi alterado. É necessária autorização para consultar a referência oficial completa vinculada pela Focus em campos.focusnfe.com.br.

Inutilização externa, emissão em produção e uso de A3 permanecem bloqueados. Não houve chamada à Focus com credenciais nem ensaio real. DANFE NF-e pelo adapter continua indisponível nesta entrega; conhecer o campo caminho_danfe não equivale a implementar seu download seguro. Nenhuma rota nova, nenhum texto de interface novo e nenhuma alteração em tools/ ou .github/.
