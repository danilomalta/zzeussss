# Fase 3 interrompida — contrato de inutilização Focus

Base: `feat/cadastro-fase2`, `b75af68`. Branch: `feat/cadastro-fase3`.
Somente registro do primeiro bloqueio; nenhuma implementação fiscal nova nesta fase.

## Bloqueio documentado

A referência oficial https://doc.focusnfe.com.br/reference/inutilizar_numeracao
informa POST `/v2/nfe/inutilizacao` com `cnpj`, `serie`, `numero_inicial`,
`numero_final`, `justificativa`. A resposta 200 descrita e seu exemplo incluem
status, série, faixa, modelo, CNPJ, caminho XML e protocolo. Não definem seleção
nem semântica do ano a inutilizar. A consulta oficial
https://doc.focusnfe.com.br/reference/consultar_inutilizacoes também não oferece
filtro documentado de ano ou série, apenas CNPJ/CPF, datas e faixa.

O mecanismo existente em `backend/internal/fiscalcore/invalidation.go:35`
exige Year, inclui esse ano no ID e no XML e confere a identidade original.
`backend/internal/localdb/fiscal/invalidation.go` reaproveita essa identidade na
reserva durável. Sem correspondência documentada para o ano, não há como afirmar
que o pedido à Focus reserva/inutiliza exatamente a mesma identidade aprovada.
Assumir ano corrente, enviar campo não documentado ou remover a verificação não
foi autorizado. É necessária documentação oficial que resolva essa correspondência
antes de concluir a unidade de NF-e/cancelamento/inutilização/DANFE pelo adapter.

Foram consultadas somente fontes oficiais Focus: introdução, NFe, emitir_nfe,
consultar_nfe, cancelar_nfe, inutilizar_numeracao, consultar_inutilizacoes e a
referência de campos de homologação ligada pela própria página de emissão
(https://campos.focusnfe.com.br/nfe_homologacao/NotaFiscalXML.html).
Alguns URLs tentados não existiam; a página correta de inutilização foi localizada
pelo menu visível oficial. A consulta não enviou credenciais nem emitiu documentos.

A emissão NF-e documenta respostas 201 síncrona e 202 em processamento; 202 não
comprova autorização. Isso foi identificado, mas o adapter NF-e 55, cancelamento,
inutilização e DANFE não foram implementados nesta fase, pois o solicitante
mandou parar no primeiro bloqueio. O adapter NFC-e já existente permanece.

Bloqueios/exclusões externos do escopo: SPED/PVA, NFS-e, A3, validação tributária
por contador, produção fiscal e ensaio Focus real dependente de token/emitente do
cliente. Nenhuma nova alíquota/regra tributária inferida; nenhuma integração com
servidor falso foi apresentada como homologação oficial. Segredos .env históricos
continuam com revogação não comprovada, sem leitura dos valores nesta tarefa.
