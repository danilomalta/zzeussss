# B02B11 — documentos de fornecedores e agenda de vencimentos

Base de aplicação: B02B10 `446319a`, limpa, na worktree
`zzeus-frontend-backend-b02a`, branch `feat/backend-b02a-compat`.
O log do proprietário confirma Go, race, 89 testes locais do frontend, build e
commit da B02B10. Isso não demonstra integração visual desta nova entrega.

## Dois fluxos implementados

1. Registrar obrigação manual em BRL de fornecedor ativo já cadastrado, com
   referência documental, descrição, data de emissão e até120 parcelas com IDs
   estáveis. A soma deve coincidir exatamente com o total informado. Não divide,
   arredonda ou completa valores automaticamente. Aprovação/rejeição por outra
   pessoa autorizada, cancelamento motivado e recibos originais recuperáveis.
2. Consultar documentos e agenda por vencimento, separando estados e datas;
   exportar a mesma agenda integral em CSV, numa única leitura transacional.

Não há execução ou confirmação de pagamento. `payment_execution_available=false`
e `settled_cents=null` em documento/agenda, inclusive depois da aprovação.
As quantias aprovadas são documentos programados, não saldo bancário, dinheiro
reservado, passivo contábil homologado ou saldo devedor conciliado com banco.
Uma operação externa pode existir fora deste módulo: cancelar o registro não
prova estorno ou quitação externa. Nenhuma ação cria venda, baixa estoque,
suprimento/sangria, journal de caixa ou lançamento em cofre.

## Dados e transições

Estado inicial `pending`. Outro responsável com ManageCash pode `approve` ou
`reject`; o criador não aprova/rejeita o próprio documento. Enquanto pendente,
somente o criador pode `cancel`. Depois de aprovado, outro responsável com
ManageCash pode cancelar. Rejeitado/cancelado são terminais. Motivo obrigatório.
Não se altera valor, prazo, fornecedor ou parcela de um documento existente.

Documento e operações/eventos preservam autor, data, valores, histórico e nome
do fornecedor no momento do cadastro. Editar/inativar o fornecedor posteriormente
não reescreve documentos, nem bloqueia a leitura ou replay autorizado de cadastro.
Fornecedor ausente no escopo retorna404; inativo não recebe novo documento409.
Referência documental usa ASCII seguro de1–128 caracteres, iniciando em letra ou
dígito e permitindo `_ . : -`. Mesmo fornecedor/referência, sem diferença entre
maiúsculas/minúsculas ASCII, não pode gerar outro documento na loja, inclusive
depois de rejeitar/cancelar. Substituição versionada de documento permanece
pendente; não sugerir outra referência apenas para contornar essa proteção.
Não lê XML, verifica chave fiscal, conecta fornecedor ou deduz valores do pedido.

Parcelas:1–120, IDs distintos, valor positivo, datas civis ISO `YYYY-MM-DD`, ano
a partir de1900, sem horário/fuso. Prazo não anterior à emissão; entrada ordenada
por vencimento, podendo repetir a data. Inteiros1–9007199254740991 centavos;
soma incremental limitada antes de adicionar, sem ponto flutuante. BRL somente.

Operação possui chave própria e hash da requisição. Reenvio idêntico do mesmo
ator retorna o recibo original (`repeated=true`), mesmo após transições; conteúdo
diferente na mesma chave retorna409. Chave de outro ator não revela recibo404.
Consulta de operação retorna snapshot original, consulta de documento retorna
estado atual. Documento, parcelas, operação e evento são uma só transação;
falha reverte todos. Restrições únicas também protegem documento/IDs concorrentes.
Os ensaios concorrentes desta entrega usam duas requisições no mesmo processo;
não demonstram concorrência comercial entre bancos de aparelhos independentes.

## APIs e autorização

Contrato real: `docs/api/supplier-payables.openapi.json`. Nove operações:

| Método | Caminho em /local/v1 | Efeito |
| --- | --- | --- |
| POST | /finance/payables | Criar documento pendente |
| GET | /finance/payables | Lista por estado, limit/offset |
| GET | /finance/payables/{id} | Documento e histórico atuais |
| GET | /finance/payables/operations/{id} | Recibo do próprio ator |
| POST | /finance/payables/{id}/approve | Aprovar por outro responsável |
| POST | /finance/payables/{id}/reject | Rejeitar por outro responsável |
| POST | /finance/payables/{id}/cancel | Cancelar conforme estado/autor |
| GET | /finance/payables/report | Agenda integral por período |
| GET | /finance/payables/report.csv | CSV integral da agenda |

Escritas exigem sessão válida, ManageCash efetivo e contrato Finance + Core
verificado dentro da transação. Finance não é habilitado automaticamente.
Não se acrescenta permissão nova nem se modifica autenticação/sessões. Leituras
exigem ViewAccounting efetivo e preservam acesso histórico sem exigir licença
Finance atual. As permissões podem ser negadas pela política existente.
Empresa, loja, aparelho e usuário vêm da sessão; cliente não escolhe escopo.
Registro/lista/agenda têm escopo do aparelho local. Não consolidam outros bancos.
O cadastro de fornecedores continua usando seu serviço e permissões existentes.

JSON exige os campos declarados, sem extras/duplicatas/nulos, inclusive dentro de
cada parcela; valores fracionários são recusados. Criação até32KiB; ações4KiB.
GET não aceita corpo. Query extra, vazia ou duplicada é recusada. Lista aceita
`state`, `limit`1–100 (padrão50) e `offset`0–1000000000. Toda a fonte local é
validada antes de paginar, inclusive página vazia; histórico/recibo/evento que
não confere retorna409 sem reparar o passado.

## Agenda e CSV

Requer `from`, `to`, `as_of`, todos datas civis. Vencimentos em `[from,to]`, ambas
as datas incluídas, até366datas. `as_of` explícito classifica cada vencimento em
`overdue`, `today` ou `future` por comparação de data, não pela hora da máquina.
O resumo separa `pending_cents`, `approved_cents`, `rejected_cents`,
`cancelled_cents`; somente aprovado tem somatórios por faixa de data. Não somar
aprovado novamente com suas três faixas. A classificação cronológica de uma
parcela cancelada/rejeitada não a torna obrigação ativa.

`complete=true` significa que os documentos do escopo local foram validados e
as parcelas daquele período estão completas; não significa banco ou outras lojas
conciliadas. Até1000 documentos locais, incluindo terminais; acima disso lista e
agenda retornam422, sem página parcial. Até10000 parcelas no período. Cada soma
do resumo deve caber no inteiro exato de JavaScript; estouro retorna422.

CSV formato1, UTF8 BOM, separador `;`, CRLF, até4MiB, nome fixo
`titan-contas-a-pagar.csv`, `no-store`, `nosniff`. Linhas `summary` e `installment`
são seções distintas; não somar entre elas. Valores em centavos inteiros. Texto
com início compatível com fórmula ou caracteres de controle é recusado409 sem
reescrever IDs/nome; JSON conserva o valor original. Não entrega CSV parcial.

## Migração, dependências compartilhadas e limites

Nova `0052_supplier_payables.sql`: quatro tabelas próprias, restrições de estado,
FKs e triggers de preservação. Nenhum SQL0001–0051 foi alterado. Não há downgrade
automático: apagar tabelas apagaria histórico. Backup passa a aceitar25–52 e o
teste restaura também um snapshot real do esquema51 para52 em arquivo novo.
Fixtures de migração futura passam a53; as verificações de checksum, integridade,
FKs, aparelho e revogação de sessões na restauração são mantidas.

Compartilhados alterados: montagem de rotas em server.go, inventário/índice de
contratos, suporte de versão no backup e expectativas dos testes de migração.
`purchase_suppliers` é somente consultada; suas migrações e serviços não mudam.
Antes de aplicar/commitar, o instalador verifica o snapshot da produção28–44,
migrações pendentes em outras worktrees e colisões de52 em outras branches.
Colisão interrompe e preserva os arquivos para coordenação; não renumera nada.

Eventos duráveis privados `supplier_payable_events` não são enviados pela outbox
comercial antiga, que não possui este contrato. Transporte remoto pendente.
Backup/reinício recuperam os dados locais; não demonstram sincronização móvel.

Pendentes: pagamentos/baixas e estornos, juros/descontos/taxas, recorrência,
substituição versionada do documento, vínculo com XML/pedido/recebimento, conciliação
bancária, contabilidade estatutária e interface. Cartão/Pix e venda mista da B02
continuam pendentes. Nenhum provedor, banco real ou frontend novo foi utilizado.
Evidências executadas estão no arquivo EVIDENCIAS.txt do pacote; frontend completo
desta revisão somente pode ser verificado pelo aplicador no PC do proprietário.
