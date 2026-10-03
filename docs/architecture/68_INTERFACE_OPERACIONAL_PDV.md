# Interface operacional — catálogo, PDV e fechamento

Base: integração do PDV do commit 3ed2457. Compatível com a demonstração isolada, sem alterar seus arquivos.

## Disponível
- Navegação lateral com tiles arredondados; menu completo no hambúrguer e três atalhos configuráveis. Preferência visual armazenada sem dados comerciais.
- Catálogo: cadastro real de produto (SKU, nome, código, unidade, preço e custo), local e entrada de estoque. Empresa e permissões derivam da sessão; contrato continua obrigatório.
- Entrada de estoque guarda o identificador e conteúdo antes do POST. Repetição usa a mesma operação. Armazenamento indisponível impede envio; operação pendente não pode ser apagada nesta tela.
- Consulta do turno dá feedback visível de aberto/fechado e data de abertura. Sem saldo esperado antes do fechamento.
- Fechamento cego com contagem exata por notas/moedas e resultado confirmado pela API. Resultado de fechamento da sessão fica apenas em memória.
- Histórico paginado via GET /local/v1/sales. Somente vendas do operador autenticado na empresa, loja e aparelho atuais, seguindo a consulta individual existente. Não é histórico consolidado do dono nem documento fiscal.
- Login e tiles têm estados ativos e transições suaves, respeitando redução de movimento.

## Ainda pendente
Cadastro comercial de empresa/planos e recuperação de senha não foram implementados por esta etapa. A área de login continua exibindo orientação real. Cadastro de produtos está no tile de catálogo.
SOS, leitor de comprovantes no smartphone, cartão, Pix, delivery, comparação de preços, RH, contabilidade, produção e frota aparecem no menu como em preparação. Tiles não concedem direitos nem representam contratação.
Não existe conciliação automática de comprovantes nesta entrega. Histórico consolidado por empresa precisa de permissão própria e consultas específicas.

## Validação
Build TypeScript/Vite e testes Node verificam o cliente. Go não está instalado no ambiente de preparação: executar os testes do backend no PC antes de registrar a entrega. Não houve inspeção visual automatizada em navegador real.
Verificação manual: entrar na demonstração, cadastrar produto e local, registrar estoque, abrir PDV, consultar turno, vender, ler histórico, contar gaveta e fechar. Testar também usuário sem permissão e resposta perdida.
Cadastro de produto/local não usa chave de idempotência: em caso de resposta perdida, consultar o cadastro antes de repetir. Não há reenvio automático desses cadastros.
