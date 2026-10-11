# P67 — cinco exportações locais CSV

Base: `d9c5176`, branch `feat/producao-receitas`.

## Funcionalidades

1. **Receitas versionadas**, na consulta de versões de `/local/production`:
   uma linha por ingrediente, identificadores imutáveis, revisão, rendimento
   resultante e quantidade por batida com suas respectivas unidades. O rendimento
   repete-se nas linhas dos ingredientes; não somar como novas produções.
2. **Ordens de produção**, na consulta de ordens da mesma tela: estado efetivo,
   versão/revisão, responsável, local, batidas e quantidade planejada preservados.
3. **Reservas ativas**, em `/local/stock`: apenas a página consultada, ordem,
   versão, responsável, produto, local e quantidade. O arquivo não transforma a
   página em total global e não conta reserva como estoque livre.
4. **Pedidos de compra**, na consulta filtrada de `/local/receiving`: fornecedor,
   estado comercial separado do recebimento e quantidade planejada do item.
5. **Eventos de recebimento**, na consulta do histórico da mesma tela:
   recebimentos originais, recusas e anulações em linhas distintas. Original
   anulado permanece registrado. Quantidades entregues, aceitas e anuladas são
   campos separados; não somar linhas como entradas efetivas de mercadoria.

Cada seção com dados oferece “Baixar CSV desta página”. Páginas vazias não
habilitam o botão. O nome fixa tipo e offset. O conteúdo inclui escopo e offset;
contagens globais existentes são rotuladas como total da **consulta**, sem
inventar quantidades de páginas não lidas. Nas receitas, sem total global
fornecido pelo contrato, inclui-se apenas a contagem de versões da página.

## Formato e contrato

Exportação exclusivamente local via Blob. Nenhum GET ou POST adicional; nenhum
novo endpoint, permissão, migração ou alteração de backup. SQLite permanece 44.
Os dados são as respostas já validadas e autorizadas pelas consultas existentes.
Se a sessão muda, o hook existente limpa essas respostas. Não exporta token,
senha, custo, saldo de caixa, identificador do aparelho ou dados não consultados.

CSV em UTF-8 com BOM, separador ponto e vírgula, linhas CRLF, todos os campos
entre aspas e aspas internas duplicadas. Textos preservam Unicode, delimitadores
e quebras de linha. Prefixos interpretáveis como fórmula, inclusive com espaços
ou controles iniciais, recebem apóstrofo de proteção. NUL e números inseguros
são recusados. A proteção não executa nem avalia qualquer fórmula.

Quantidades trazem os milésimos inteiros originais. Nas exportações de receitas,
ordens, reservas e pedidos, também há decimal com vírgula e três casas, sem
separador de milhares e sem ponto flutuante intermediário. A unidade fica em
coluna explícita. Não converte unidades nem atribui preço/total não negociado.
No histórico, os milésimos originais permanecem em colunas distintas por papel.

Ao importar em planilha, selecionar IDs, SKU e milésimos como texto para evitar
conversão automática de zeros iniciais ou inteiros grandes pelo aplicativo.
O arquivo CSV contém os valores completos; não promete controlar a importação
feita por outro programa.

Arquivos compartilhados de tela e `package.json` recebem apenas integração dos
botões e inclusão dos testes. Leitores e contratos HTTP não são alterados. Não
há coleta de páginas adicionais, reserva, baixa, envio a fornecedor ou gravação
de pedido. O link temporário é removido e sua URL liberada após o clique, mesmo
quando a tentativa de download falha.

## Verificação

Testes dos cinco formatos, identidade/estado, escopo de página, unidades,
quantidade no limite seguro, linhas originais/anuladas, caracteres especiais,
fórmulas, arquivo vazio, falha de download e liberação de recurso. Suíte local do
frontend e TypeScript/build. Backend: suíte completa, vet e corrida das consultas
existentes. Testes usam instalações temporárias, sem banco comercial/demonstração.
Sem aceite visual de navegador nesta entrega.
