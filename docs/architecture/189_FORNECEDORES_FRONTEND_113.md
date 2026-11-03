# Frontend 113 — fornecedores, parcelas e dinheiro

## Acesso e integração

Rota `/local/payables`, carregada sob demanda. O card Financeiro aponta para ela quando a sessão possui `view_accounting`; o arquivo de trabalho financeiro anterior continua em `/local/enterprise?area=finance` e inclui link para a nova consulta. Permissões e identidade vêm de LocalAccess/useLocalSession, com comparação de empresa, loja, aparelho e usuário por contextOf. Sem permissão ou contexto confiável, nenhuma consulta é feita. Não altera Go, migrações, B01, PDV, login ou cadastro112.

Consulta requer `view_accounting`, inclusive para consulta histórica sem licença Finance ativa. A interface permite comandos somente com `view_accounting`, `manage_cash` e licença ativa Core+Finance. O servidor continua validando autoria da fonte, licença, permissões e todos os limites. Um botão habilitado não concede autorização. Não cria novos perfis.

O adaptador usa o mesmo caminho local e autenticação Bearer em memória dos clientes existentes, credenciais omitidas, cache desativado e timeout de 15s. Nenhum token em URL, logs, arquivo ou armazenamento do navegador. Respostas e valores são validados antes de mostrar. Falha de consulta de pagamentos não quebra a lista de documentos. A troca de contexto/token desmonta a área e cancela requisições; respostas antigas não são exibidas em outro contexto.

## Funcionalidades desta entrega

- Lista de documentos carregados: fornecedor, referência, estado, principal e saldo de espécie conhecido. Busca literal sem distinção de acentos, estado, período de vencimentos inclusivo e paginação local de 15 linhas.
- Detalhe com parcelas, vencimentos e valores exatos. Pagamentos consultados no servidor em páginas de 20. Não confunde a soma da página com os totais da fonte.
- Saldo separado em dinheiro bruto registrado, devolvido, líquido aplicado e saldo documental. Não soma principal, aplicado e saldo como se fossem dinheiro diferente. Ausência de informação mostra traço, inclusive indicadores durante carregamento/erro.
- Vincular pagamento em dinheiro: escolher parcela, informar referência de sangria auditada existente, recebedor e referência de comprovante. O valor não vem do navegador. Confirmação obrigatória; não cria nova sangria nem envia pagamento bancário.
- Devolução integral de um pagamento: referência do suprimento existente e motivo. Confirmação obrigatória; não cria outro suprimento. Pagamento já devolvido não pode ser devolvido novamente pela interface.
- Recibo retornado pelo servidor, consulta do resultado por operação e reenvio com os mesmos identificadores e conteúdo após falha incerta. Sem gerar outra operação automaticamente. Campos ficam congelados enquanto o resultado for incerto; falha definitiva inicial permite corrigir. A autorização atual é respeitada; consulta de recibo pode continuar disponível em modo somente leitura.
- Trava de clique duplicado, confirmação invalidada ao editar, aviso antes de fechar aba ou navegar com campos/pedido pendentes. Não envia comandos automaticamente ao sair.
- CSV dos documentos filtrados e carregados, download por Blob e proteção contra fórmulas. Não procura pastas; preferências de download do navegador continuam sob controle do usuário.

## Contratos usados — já existentes

| Método | Caminho |
| --- | --- |
| GET | /local/v1/finance/payables |
| GET | /local/v1/finance/payables/{id}/cash-payments?limit=20&offset=N |
| POST | /local/v1/finance/payables/{id}/cash-payments |
| POST | /local/v1/finance/payable-cash/{id}/return |
| GET | /local/v1/finance/payable-cash/operations/{id} |

Os quatro contratos de espécie foram lidos integralmente no patch B02B12 enviado pelo usuário. Cópia de referência em `docs/api/frontend113-payable-cash.reference.json`, sem sobrescrever contratos mantidos pelo outro trabalho de backend. A consulta de documentos usa uma projeção conservadora dos campos verificados no patch: array ou envelope items, id, supplier_name, document_reference, state e installments(id, due_on, amount_cents), e cash quando disponível. Não inventa parâmetro de paginação/totalidade da listagem B02B11: filtros/exportação são explicitamente dos documentos carregados, sem presumir que representam toda a empresa. Contrato incompatível bloqueia a apresentação, em vez de substituir valores por zero.

Endpoints novos necessários para esta entrega: nenhum. É preciso executar o backend B02B12 com seus pré-requisitos e proxy local apontando para esse serviço. Instalar o ZIP frontend não aplica o backend, suas migrações, nem troca o servidor em execução.

## Limites / próximos passos

Esta entrega não cobre criação/aprovação/rejeição/cancelamento de documentos B02B11, edição versionada, agenda oficial consolidada/CSV do servidor, seleção assistida da fonte auditada, pagamento direto ou de cofre, devolução parcial, banco/Pix/cartão/TEF, emissão fiscal ou vínculo com pedido/recebimento. A lista de parcelas desta tela não é a agenda oficial consolidada. O financeiro89 continua arquivo local separado; seus lançamentos não são importados automaticamente como pagamentos reais. Sem migração de rascunhos locais para dados oficiais.

A referência da sangria/suprimento precisa existir previamente. O sistema não sabe se houve entrega física de dinheiro: isso é declarado pelo responsável e auditado pelo backend. A confirmação desta tela é do registro do vínculo, não de transferência ou recebimento do fornecedor.

Os comandos pendentes ficam apenas em memória. O aviso de saída não oferece garantia de recuperação após falha de energia/fechamento forçado. Guarde a referência da operação mostrada para conferência no servidor; não faça outro pagamento físico para tentar resolver uma resposta perdida. Persistência segura de fila de comandos e recuperação após reinício ficam pendentes de coordenação com a camada desktop/backend. Não há sincronização financeira entre aparelhos demonstrada.

As fontes contêm dados pessoais, portanto não há cache permanente novo nem uso de console com registros. Ao sair da rota/encerrar sessão, o estado é descartado. Exportar CSV é ação explícita do usuário e gera arquivo fora do cache.

## Validação e demonstração

32 testes novos de modelo e cliente; 607 testes totais aprovados. Typecheck, build e diff-check. Firefox: consulta, busca, paginação, CSV, ausência de escrita antes de confirmar, rejeição e correção, gravação com resposta perdida e recuperação do mesmo recibo, devolução integral, duplo clique, consulta sem permissão de escrita e estado vazio. Cinco idiomas × três larguras × dois temas, sem rolagem horizontal da página (tabelas têm rolagem interna). 79 chaves iguais e preenchidas nos cinco idiomas; revisão linguística humana pendente.

Prévia113 tem fonte fictícia explícita e isolada, usando o mesmo cliente/validador com transporte injetado. Não é importada pela aplicação. Não usa conta/servidor/dinheiro real. Não foi executada a combinação deste frontend com o backend completo B02B12 no computador do usuário; o ZIP recebido é incremental, sem toda a base necessária para subir essa versão aqui. Não declarar aceite integrado, restauração de frontend, funcionamento de banco/adquirente, emissão fiscal ou a Parte1 completa.

## Testar no aplicativo

1. Abrir `/local/payables` com sessão válida e `view_accounting`. Sem servidor compatível: erro claro e indicadores desconhecidos; sem permissão: bloqueio sem consulta.
2. Conferir documentos já registrados/aprovados no backend. Filtrar por fornecedor, referência, estado e vencimento. Abrir detalhes; conferir parcelas e pagamentos. CSV deve conter apenas os documentos carregados do filtro.
3. Com responsável autorizado e movimento de teste previamente auditado, revisar e vincular a sangria a uma parcela aprovada. Confirmar que o valor vem da fonte e que não há nova baixa de caixa. Repetir com resposta interrompida: consultar/reenviar a mesma operação, sem nova movimentação física.
4. Registrar devolução integral com suprimento compatível; conferir recibo, valor aplicado/saldo e histórico preservado. Origem reutilizada, valor incompatível ou documento não aprovado devem ser recusados pelo servidor.
5. Testar contador/leitura histórica, operador sem permissão, expiração de sessão, dois temas, cinco idiomas, tablet/celular, navegação com edição e encerramento de sessão. Não usar dinheiro comercial para testes.
