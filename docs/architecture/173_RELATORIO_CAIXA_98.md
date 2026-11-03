# Frontend98 — relatório coerente com a conferência97

Requer frontend97. Mantém rotas e contrato de Report95, substituindo somente sua implementação. Não modifica arquivos da B01-0f6, APIs, permissões, banco, package.json ou migrações.

## Entrega
- Resultado oficial separado da conferência de vendas e dos registros locais de contagem/malote. Valor desconhecido é traço, não zero.
- Usa reviewClosing97: confirmação eletrônica e comprovante da mesma parcela não duplicam cobertura; venda+parcela compõem a identidade do vínculo; modo integrado exige confirmação da fonte.
- Total sem comprovação separado de valor ainda não verificado: associações provisórias não são confirmação de pagamento.
- Consulta incompleta conserva totais desconhecidos; fonte inválida não produz números utilizáveis; falha de uma fonte preserva as outras.
- Seleção de fechamento arquivado não reutiliza resposta do serviço nem vendas do caixa atual. Valores oficiais arquivados continuam sendo o registro local da resposta do encerramento, não uma nova consulta ao servidor.
- Imprimir/PDF pelo navegador e CSV usam as mesmas seções e valores. CSV baixa diretamente, sem pedir pasta através de seletor do aplicativo; o navegador pode pedir onde salvar conforme sua configuração.
- Atualização manual, indicação da hora da consulta, malotes paginados em10 registros; cinco idiomas, temas existentes, alvos44px. Tabela de pagamentos rola internamente no celular.

## Integração
Não há endpoint novo. Usa ClosingSourceContext92, closingStorage92, tillStorage94 e reportStore95 existentes. Somente leitura. O conector real continua sendo responsável pela autorização e completude da fonte. Não cria confirmação eletrônica, liquidação bancária, emissão fiscal, transferência entre aparelhos ou reconciliação oficial.
O registro de malotes ainda é local; não representa recebimento remoto pelo conferente. Continua dependente da futura integração do backend.
Na B01, vendas reais em dinheiro são consultadas pela fonte legada; exemplos de cartão/Pix da prévia são fictícios. Pagamentos mistos persistidos dependem da B02.

## Testar no sistema
1. Encerrar o caixa e conferir o relatório existente em /local/cash/register. A resposta oficial deve permanecer na primeira seção.
2. Comparar valores por meio com a conferência97. Desconhecidos permanecem como traço.
3. Imprimir/PDF e baixar CSV; verificar os mesmos valores, notas e seções.
4. Quando houver arquivos históricos disponíveis, escolher outro fechamento. Não pode exibir valores da sessão atual.
5. Desconectar uma fonte: mostrar aviso, preservar dados das demais, permitir exportar o relatório parcial com a observação.
6. Conferir claro/escuro, tablet/celular e pt-BR/en/es/fr/zh-CN.

A prévia isolada acompanha dados fictícios, não pede senha, não acessa banco ou provedor real e não demonstra B01 em operação. Traduções en/es/fr/zh-CN precisam de revisão nativa antes de produção. Nenhuma chave nova ausente nos cinco arquivos.

Não declara toda Parte1 concluída. Validação executada está em VALIDACAO.txt.
