# Frontend101 — Relatório oficial do caixa

Requer frontend100. Integra exclusivamente GET /local/v1/cash/sessions/{id}/report, contrato B02B2, na seção Relatórios do caixa. Não modifica backend, SQL, autenticação, pagamentos ou saldos.

O botão Consultar relatório oficial busca o turno selecionado. O relatório oficial tem CSV e impressão próprios; não mistura comprovantes e malotes locais aos totais oficiais. O relatório98 anterior continua disponível para a conferência local. Não há cache persistente nem leitura automática em segundo plano.

Mostra fundo inicial, movimentação líquida, esperado, contado, diferença, suprimentos, sangrias, vendas brutas/canceladas/líquidas e pagamentos por status gravado. Card permanece cartão sem tipo informado; confirmed gravado não vira providerConfirmed. Complete significa consulta local completa de caixa encerrado, não liquidação ou vários aparelhos.

Valida identidade do turno, datas, inteiros seguros, soma das vendas, pagamentos e diferença. Usa BigInt na conferência. Falhas limpam valores anteriores. HTTP401 encerra sessão pelo mecanismo existente;403 mostra sem acesso;404 pode significar versão antiga ou turno não encontrado;409 significa caixa aberto ou histórico inconsistente;422 é dado fora dos limites. Não consulta outros operadores/aparelhos, restrição atual do backend. Não há lista oficial de turnos: seletor usa os arquivos de fechamento já disponíveis no frontend98.

Sem B02B2, a consulta mostra indisponibilidade e o resto da tela permanece funcionando. Não instalar backend nessa pasta por causa deste pacote. A integração entre branches deve ser coordenada no chat de backend.

Arquivos en/es/fr/zh-CN precisam de revisão por falantes nativos antes de produção. Todas as33 chaves estão traduzidas nos cinco idiomas. Sem dependências novas.

## Testar
No frontend da pasta zzeus-frontend-backend, abrir PDV → Relatórios. Consultar um turno encerrado na mesma sessão/operador/aparelho do servidor B02B2. Comparar esperado, contado e diferença com fechamento persistido; baixar CSV e imprimir. Tentar caixa aberto (409), servidor antigo (404), usuário sem acesso, sessão expirada, mudança de turno e resposta inconsistente. Alternar cinco idiomas, claro/escuro e larguras1440/768/390.

Testes automatizados usam SQLite nenhum: apenas respostas HTTP interceptadas de exemplo. Não foram executados servidor Go, pagamento, SEFAZ, impressora física nem aceite visual no PC do proprietário. Disponibilidade real da rota depende da instalação B02B2 no ambiente integrado.
