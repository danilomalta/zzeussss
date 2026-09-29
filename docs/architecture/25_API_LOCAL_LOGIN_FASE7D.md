# Fase 7D — API local com login

`localapi.New` cria rotas locais de saúde, login, sessão e saída. O processo deve passar apenas um aparelho já provado; IDs enviados pelo navegador não definem o tenant nem o aparelho. Cada requisição protegida revalida token, vínculo e dispositivo. O token é entregue uma única vez para o cliente conservar em memória; nenhuma rota de negócios está registrada ainda.

O próximo passo liga catálogo, estoque e caixa. O servidor local será iniciado exclusivamente em 127.0.0.1, e o frontend de desenvolvimento usará proxy de mesma origem. Não abrir esta API à rede local antes de definir autenticação e transporte para outros aparelhos.
