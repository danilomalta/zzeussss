# Comparador: sites da empresa e conferência manual

## Entrega e limite da integração

Nova área `/local/prices`, acessível pelo tile Comparador de preços para quem tem acesso ao catálogo e módulo de estoque no contrato. Esta é a primeira entrega do comparador: configurações persistentes de sites e comparação de valores conferidos pela pessoa. A coleta automática de preço, frete e disponibilidade ainda não está conectada.

O backend não faz requisições aos sites. A interface prepara links públicos com o nome do produto e a variante informada; a pessoa abre cada site e confere o produto. O cadastro de uma URL não é uma autorização de API, uma conexão comercial nem uma garantia de disponibilidade de dados.

Foram consultadas fontes oficiais em 06/10/2026:
- Documentação Netshoes: https://developers.netshoes.com.br/api-portal/content/entenda-api — descreve integração do lojista ao marketplace, com cadastro de seus produtos, preços, estoque e pedidos. Não foi validada como interface pública para consultar ofertas de outros lojistas.
- Busca Centauro: https://www.centauro.com.br/busca/tenis
- Busca Netshoes: https://www.netshoes.com.br/busca?nsCat=Natural&q=tenis
- O endereço de busca da Adidas não foi confirmado nessa verificação. O preenchimento sugerido usa somente https://www.adidas.com.br; o operador pesquisa no site. Um endereço de busca conferido pode ser configurado posteriormente.

Esses caminhos públicos podem mudar. O comparador não extrai ou interpreta o HTML, não utiliza APIs internas não documentadas e não anuncia site conectado. Os valores nunca são simulados nem preenchidos automaticamente.

## Cadastro e autorização

GET `/local/v1/comparison-sites`: sites da empresa da sessão; sem parâmetro de empresa; verifica usuário, loja e aparelho, com permissão view_catalog. Consultas continuam disponíveis após expiração do contrato, como nos outros registros locais.

POST `/local/v1/comparison-sites`: criação ou edição com operation_id, id, expected_revision, nome, origem HTTPS, modelo de URL de busca opcional e estado ativo/inativo. Exige manage_stock e módulo inventory vigente. Papéis existentes owner, manager e stock podem gerenciar. Nenhuma senha ou chave de terceiros é aceita ou armazenada.

Só origens HTTPS em nomes de domínio, sem porta, credenciais, caminho, query ou fragmento são aceitas. Use a origem sem barra final. IPs literais e sufixos locais/especiais conhecidos são recusados. O modelo de busca precisa permanecer no mesmo domínio, sem credenciais/fragmento e com uma única ocorrência de `{query}` no caminho ou query. O termo é codificado antes de formar o link; os links usam noopener/noreferrer. Isso valida destinos de navegação, não comprova o DNS ou a titularidade do site.

A migração 0024 adiciona comparison_sites e comparison_site_operations. Os sites são da empresa, compartilhados pelas lojas autorizadas neste banco. Cada endereço principal exato só pode ser cadastrado uma vez. Limite inicial: 32 registros incluindo inativos. Inativação preserva o registro; não há exclusão nesta entrega.

Criação, atualização, avanço do relógio contratual e registro da operação são transacionais. Uma revisão desatualizada gera conflito em vez de sobrescrever outra alteração. A trilha de operações mantém entrada, resultado estável, autor, loja, dispositivo e horário. Repetição com mesma operação e conteúdo recupera o mesmo snapshot; reaproveitamento de ID com outros dados é recusado. Esse histórico está no banco; a interface ainda não oferece um painel completo de auditoria.

GET `/local/v1/comparison-site-operations/:id`: resultado da operação, restrito ao usuário, loja e aparelho originais com permissão de gestão. O navegador persiste entrada e estado antes de enviar, usa Web Locks e consulta o resultado antes de qualquer repetição explícita. Erro de consulta não dispara POST. Recusa posterior não apaga incerteza anterior. Uma primeira recusa definitiva só libera correção depois de consultar e confirmar ausência da operação.

## Fluxo na tela

1. Abrir o menu e Comparador de preços. Em Gerenciar sites, preencher Centauro/Netshoes/Adidas ou outro domínio e salvar. Os preenchimentos sugeridos não são gravados até confirmar.
2. Buscar o produto no catálogo inteiro. São exibidos até 50 resultados; refinar a busca quando necessário. Escolher o produto real, indicar variante exata e condição de pagamento, e preparar as consultas.
3. Abrir os links dos sites ativos. Conferir modelo, tamanho, cor, quantidade/unidade, preço, vendedor e condição. Registrar os valores na tela como conferência manual, com disponibilidade declarada e equivalência confirmada pela pessoa.
4. O preço mais frete é calculado em centavos inteiros com soma segura. Frete vazio significa desconhecido; zero precisa ser informado quando não houver cobrança. Somente observações com equivalência confirmada, disponibilidade conferida como disponível e frete conhecido entram no destaque de menor total.

O destaque diz “Menor total entre as conferências manuais completas”. Ele não afirma menor preço de todo o mercado ou preço vigente garantido. A disponibilidade é declarada pela pessoa, não pelo servidor externo. Datas indicam registro na tela. O preço do catálogo é apenas referência na unidade do produto, não uma oferta externa.

Conferências manuais ficam em memória: sair, recarregar, mudar produto/condições ou atualizar sites limpa os valores. Não há histórico persistente de cotações nesta etapa. Sites e operações de configuração sobrevivem ao reinício do banco. Configurações ainda não são sincronizadas comercialmente entre dispositivos; não geram eventos na outbox de estoque, caixa ou vendas.

Nenhuma ação altera preço de venda, estoque, caixa, pedido ou pagamento. Não há comparação automática de tamanhos em estoque local: variantes persistentes também precisam de uma entrega própria no catálogo.

## Aceite e próximas etapas

Testes: autorização e licença, URLs inseguras/JSON ambíguo, isolamento por empresa, revisão, replay, limite, concorrência, rollback por falha/ignore na trilha, migração repetida e reabertura. Cliente: navegação permitida, validação dos links e respostas, perda de resposta, payload estável, recusa versus incerteza, armazenamento bloqueado e cálculo exato com critérios de equivalência/frete.

Verificação: testes locais do frontend, build TypeScript/Vite, todos os testes Go, go vet e git diff --check. O aceite visual nos temas claro/escuro, smartphone e navegador do usuário permanece necessário. Não foi efetuada compra nem consulta automática de ofertas reais.

A próxima integração exige um provedor/feed/API autorizado e validado para cada site. Definir identificador de modelo/variante, acesso, limites, atualização, frete por destino, vendedor e condição de pagamento. Credenciais ficam na configuração do backend, nunca no navegador. Resultado automático deverá mostrar fonte, horário real, equivalência e falhas independentes por site; timeout ou bloqueio não pode virar preço zero ou disponibilidade confirmada.
