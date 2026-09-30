# Backend 8C — Armazenamento local de contratos

Base informada pelo proprietario: commit 87045d6, testes da 8B e suite do backend
passaram. Esta etapa adiciona somente armazenamento e testes. Os casos de uso e
rotas atuais ainda nao chamam o novo controle.

## Migracao 0016

module_contract_history registra cada revisao assinada com instalador e horario.
module_contract_state aponta a revisao vigente da empresa e a ultima observacao
de horario que foi commitada. FK composta impede apontar historico de outra
empresa. Nenhum contrato e gerado ou modulo ativado pela migracao.

A migracao so cria tabelas novas. O teste existente de reabertura passa a esperar
16 migracoes. Nao alterar conteudo das migracoes 0001 a 0015 nem seus checksums.
Aplicar codigo nao abre banco real; testes usam arquivos temporarios. Na futura
abertura de uma instalacao pelo servico local, Migrate aplicara 0016 como as
demais migracoes locais. Planejar backup antes de usar isso numa instalacao real.

## Instalacao

Install recebe contexto humano e aparelho ja autenticados pelo chamador. Verifica
vinculo ativo, loja, dispositivo aprovado e papel owner. Mesmo um gerente com
ManageStaff nao pode instalar. A assinatura deve ser de emissor confiavel e do
tenant do contexto humano. Nao existe endpoint publico ou licenca simulada.

Revisao maior substitui a vigente. Repetir os mesmos bytes na mesma revisao e
idempotente. Revisao menor e rejeitada. Mesma revisao com bytes diferentes e
conflito, mesmo com assinatura valida. Historico e ponteiro sao atomicos: se
qualquer gravacao falhar, o contrato anterior permanece vigente.

Historico e auditavel, mas nao imune a edicao por quem controla o arquivo SQLite.
O uso revalida a assinatura a cada decisao: edicao direta nao comprova contratacao.
Retornar uma revisao antiga via restauracao de backup completa ainda nao pode
ser detectado so pelo banco restaurado. Isso depende de ancoragem futura.

## Uso transacional

RequireTx recebe a mesma transacao da operacao de negocio. Verifica permissao,
empresa, loja, aparelho e contrato, incluindo assinatura, vigencia e dependencias.
Nao confiar em permission ou module enviados pelo frontend; o caso de uso escolhe
os dois. A transacao deve ser da conexao local usada pelo Store.

Uma consulta previa seguida de escrita em outra transacao nao tem a mesma
garantia. O chamador deve fazer rollback quando RequireTx retornar erro. Alterar
apenas o middleware nao protege casos de uso acessados por outros caminhos.

## Horario e offline

A verificacao e local. Uma decisao bem-sucedida, quando commitada, persiste o
horario observado. Horario inferior a essa observacao e rejeitado. Rollback da
operacao tambem desfaz essa observacao, preservando atomicidade.

Essa protecao nao e um relogio confiavel e nao registra horarios de tentativas
falhas. Restaurar banco antigo ou manipular o relogio sem cruzar uma observacao
commitada pode escapar dela. Nao prometer revogacao imediata offline nem licenca
inviolavel. A politica comercial de tolerancia, renovacao e caixa em andamento
ainda precisa ser concluida antes de ligar bloqueios nas rotas.

## Escopo e proxima etapa

Testes cobrem contrato ausente, instalacao repetida, revisao antiga ou conflitante,
gerente sem alçada, dono revogado, assinatura adulterada, vencimento, retrocesso
de horario observado, falha na atualizacao apos gravar historico e persistencia
apos reabertura. Contrato de outra empresa nao deixa registros parciais.

Os dados armazenados sao contratacao e identificador do instalador; nao incluem
vendas, estoque ou documentos. Nenhum dado e enviado para nuvem nesta etapa.

Proxima etapa: definir politica de operacoes em andamento e integrar o gate aos
casos de uso escolhidos, com configuracao explicita de chaves confiaveis. Sem
configuracao confiavel, nao conceder todos os modulos por padrao. Frontend pausado.
