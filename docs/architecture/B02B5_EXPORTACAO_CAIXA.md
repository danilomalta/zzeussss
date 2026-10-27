# B02B5 — exportação dos lançamentos operacionais de caixa

Base no PC: B02B4 `e8ee0d2`, branch `feat/backend-b02a-compat`. Saída enviada
confirmou testes Go, race, frontend, build, commit e árvore limpa. Esta entrega
fica na mesma worktree isolada `zzeus-frontend-backend-b02a`.

## Entrega e uso pela futura integração

`GET /local/v1/cash/sessions/{id}/journal.csv`

Download autenticado dos lançamentos do turno encerrado. Mesma autorização da
B02B4: permissão Sell, operador original, empresa/loja/aparelho da sessão.
Não exige licença POS vigente para consultar histórico. Autenticação e
revogações continuam nos guardas existentes; outro operador não acessa o turno.
Não envia arquivo a contador ou terceiro: somente retorna o download ao usuário
autorizado. Sem botão novo no frontend nem integração visual declarada.

O CSV contém os mesmos lançamentos e resumo conferido do journal JSON. Usa uma
única transação de leitura para conferir fontes, fechamento, somatórios e obter
todas as entradas. Não junta páginas consultadas em momentos diferentes. Só
depois de validar e codificar o arquivo completo define headers de download e
envia bytes. Falha não deixa um CSV parcial com valores utilizáveis.

## Formato versão1

UTF-8 com BOM, separador `;`, linhas CRLF, nome fixo
`titan-caixa-lancamentos.csv`. Cabeçalho fixo com22 colunas:

`format_version;row_type;scope;session_id;entry_id;source_kind;source_id;category;operation_id;actor_id;occurred_at;cash_delta_cents;debit_account;credit_account;amount_cents;total_entries;expected_cents;declared_cents;difference_cents;total_debit_cents;total_credit_cents;unclassified_count`

Primeira linha de dados: `row_type=summary`, `format_version=1`,
`scope=cash_operational`, sessão e totais do turno inteiro. Colunas de lançamento
ficam vazias. Depois, uma linha `row_type=entry` por lançamento, em ordem
persistida. Colunas exclusivas do resumo ficam vazias nessas linhas. Assim os
totais não são repetidos em cada entrada e cada valor do caixa aparece uma vez.

Entradas incluem ID, origem, categoria, operação, autor e instante originais,
delta assinado do caixa e contas de débito/crédito com o mesmo valor absoluto.
Valores monetários são centavos inteiros exatos, sem arredondamento ou conversão
para reais. Estorno preserva venda e acrescenta delta negativo. Recebido/troco
não entra como receita duplicada. Diferença da contagem fica no resumo, não gera
despesa automaticamente. Abertura zero continua identificável.

Autor/operação desconhecidos ficam vazios no CSV; no JSON continuam `null`.
Categoria `unclassified` e contador correspondente são preservados: não há
responsável, venda ou conta contábil inventados.

## Limites e células textuais

Até10000 lançamentos incluindo abertura; máximo16MiB de arquivo codificado.
Acima disso retorna413, sem truncamento e sem fingir que a primeira página é
o arquivo inteiro. Não aceita query, paginação ou corpo GET; retorna400.
Journal JSON continua com limite100 por página e não recebe o limite de exportação.
Caixa aberto/inconsistente409; fora do escopo404; precisão ou texto incompatível422.

O CSV preserva os identificadores; não acrescenta apóstrofos ou tabulações para
modificar células. Texto não vazio deve começar com letra Unicode, dígito ou `_`,
ter UTF-8 válido, não ter controles e ocupar até4096 bytes por célula. Texto que
não atende retorna422. Por exemplo, operação `=1+1` permanece consultável no JSON,
mas não é exportada como uma fórmula nem transformada em outro ID. Aspas,
separadores e Unicode no restante de um campo são codificados pelo encoder CSV.

Essa escolha evita exportar os prefixos de fórmula mais comuns, incluindo as
variantes de largura completa, sem alterar o histórico. Não é promessa universal
de comportamento de todo editor de planilha. Referência técnica consultada:
https://community.owasp.org/attacks/CSV_Injection . Não foi testado abrir,
salvar e reabrir em Excel/LibreOffice reais. CSV não declara tipos de célula:
IDs, datas e números grandes devem ser importados como texto quando necessário,
para o editor não remover zeros, converter datas ou perder precisão.

## Dependências e coordenação

Sem migração nova: esquema47 e suporte de backup25–47 permanecem inalterados.
Sem edição em autenticação, sessões, PostgreSQL, backup ou código frontend.
Arquivos compartilhados alterados: `localapi/cash.go`, `cash_journal.go`,
`cashjournal/journal.go`, inventários de rotas e matriz de progresso. Ao integrar
branches, preservar a captura atômica e as proteções de histórico da B02B4.
Aplicador exige B02B4 `e8ee0d2`, confere hashes e recusa alterações fora da entrega.

O teste de10000 entradas encontrou uma junção reversa lenta sobre a view de
fontes. A conferência de entradas órfãs agora consulta diretamente sessão e
movimento pelos índices existentes, preservando a verificação de origem e os
erros. Nenhum índice/migração histórico foi alterado. Há teste de fonte removida
de propósito em banco temporário para verificar essa condição, além do teste
de fonte sem lançamento da B02B4. Não aumenta timeout para esconder o problema.

## Aceite observado e pendências

Testes HTTP exercitam o ciclo real de abertura/venda/suprimento/sangria/estorno,
exportação idêntica ao JSON, saldo1200, declarado1202, diferença2 e totais8196 de
débito/crédito. Testam mais de uma página, limite exato10000 e recusa10001,
reenvio sem escrita, novo turno sem contaminação do histórico, escopo, revogação,
licença, erros sem anexo parcial, precisão, fonte divergente/órfã e texto inseguro.
Testes de formato exercitam aspas/Unicode, proveniência desconhecida, limite de
bytes e cancelamento. Backup, reabertura e concorrência preservam os bytes.
Fixtures de volume são dados sintéticos auditados; não provam10000 vendas reais.
Resultados executados e comandos constam de `EVIDENCIAS.txt` no pacote.

Continua sendo um extrato operacional de dinheiro, não escrituração contábil,
SPED, emissão fiscal, extrato bancário ou confirmação da adquirente. Financeiro
completo, pagamento misto R$315, conectores eletrônicos, malotes remotos e adapter
visual permanecem pendentes. Não há envio automático nem conector simulado.

Frontend completo não está disponível no ambiente de preparação; o aplicador
executa testes e build no PC antes do commit. Sem merge/push, servidor iniciado
ou acesso ao banco da demonstração.
