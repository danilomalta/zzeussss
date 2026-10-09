# P40 — Publicacao visual de receitas versionadas

Base PC038ba17, branchfeat/producao-receitas. Nenhuma migracao, schema44.
/local/production recebe formulario: receita nova com revisao0; carregamento de
versao existente, campos editaveis e revisao esperada, nova version_id por envio.
Ingredientes IDs existentes, unidade explicita, rendimento/quantidades exatos com
ate3 decimais, sem milhar/expoente/arredondamento. Backend valida referencias,
produto ativo e unidade do catalogo. Publicacao nao reserva/consome/produz.

Fila unica de producao, em localStorage, chave tenant/store/device/operator.
Operacao salva antes do POST e antes de marcar resultado incerto. Web Lock entre
abas obrigatorio; navegador sem bloqueio nao grava. IDs e payload preservados.
Ao recarregar, ler fila nao envia. Consultar sempre GET; repetir explicitamente
consulta antes de POST. POST bem sucedido ainda exige recibo imutavel confirmado.
Falha de leitura/persistencia bloqueia; resultado incerto nunca descartado, mesmo
com404. Primeira recusa definitiva permite descartar apenas apos consulta404.
Erros401 invalidam sessao em memoria, sem apagar fila. Token/senha nao persistidos.

Novo contrato GET /local/v1/production/operations/{kind}/{id}, kinds recipe/order/
state. Retorna kind,input,result originais; consulta exige manage_production e
aparelho aprovado. SQL limita tenant/store/device/operator. Operacao estrangeira
ou inexistente404; tipo/query invalido400; inconsistencias409. Consulta nao avanca
relogio de licenca nem cria outbox. Resultados originais sobrevivem novas versoes
ou transicoes posteriores. Backup/criptografia/ValidateSchema sem mudancas.

Compartilhados: server.go monta a nova rota; productionRead exporta validadores;
LocalProduction inclui provider/fila e formulario; package.json inclui teste.
Contrato documentado em production-operation-receipts.openapi.json. Sem auth,
main, infra, merge, push ou banco real.

Testes locais: frontend96 e build; HTTPrecibo original/escopo/corrupcao/GET sem
escrita; suite backend/vet/race dos novos endpoints no lote. Fixtures descartaveis.
Ainda nao realizado aceite visual no navegador. Cadastro de estado da receita,
execucao de ingredientes, resultados e demais formularios ficam para proximos lotes.
