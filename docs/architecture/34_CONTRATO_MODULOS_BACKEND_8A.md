# Backend 8A — Contrato inicial dos modulos

## Base e alcance

Base informada pelo proprietario: commit c43de1e, arvore limpa apos registrar
formatacao dos testes de catalogo. Testes Go sem cache e go vet passaram no PC.
Este pacote adiciona somente um contrato de dependencias e testes em memoria.
Nao modifica rotas, autorizacao existente, banco, migracoes ou frontend.

## Modulos e dependencias funcionais

| ID | Area | Dependencias obrigatorias |
|---|---|---|
| core | Identidade, empresa, loja, aparelho, catalogo e infraestrutura de eventos | Nenhuma |
| inventory | Estoque, entradas, saidas e inventario | core |
| pos | Vendas e caixa de mercadorias | core, inventory |
| orders | Pedidos, reposicao e recebimento entre empresas | core, inventory |
| logistics | Agendamento e recebimento logistico vinculado a pedidos | core, orders; inventory por dependencia de orders |
| finance | Contas e conciliacao | core |
| fiscal | Integracoes e documentos fiscais | core |
| accounting | Acesso autorizado e exportacoes para contador | core |
| staff | Funcionarios, tarefas, metas, ponto e avaliacao privada | core |
| production | Ordens, etapas, insumos, perdas e qualidade | core, inventory |

O PDV deste contrato e de mercadorias, por isso exige estoque. Atendimento de
servicos e consumidor ficam fora da entrega inicial. Uma industria pode selecionar
inventory, orders e production sem contratar pos. Contabilidade e staff nao
obrigam contratar PDV. Dependencia funcional nao fixa preco ou pacote comercial.
core e infraestrutura comum, nao um adicional vendido separadamente.

O transporte autorizado de eventos pertence ao core; a rotina comercial de
compras e vendas entre empresas pertence a orders. A comunicacao nao transmite
automaticamente todo estoque ou todas as vendas para outras empresas ou para o Titan.

## Separacao de responsabilidades

Uma operacao exige quatro decisoes distintas:

1. Sessao humana valida na empresa certa.
2. Permissao da pessoa para a acao e loja.
3. Aparelho autorizado quando a operacao exige dispositivo.
4. Modulo disponivel para aquela empresa, com dependencias atendidas.

O dono nao ganha modulos nao contratados por ter papel owner. Um modulo
contratado nao concede permissao a qualquer funcionario. A lista recebida de
um navegador nunca e prova de contratacao.

O pacote modules calcula dependencias e verifica uma lista fornecida por um
chamador confiavel. Ele ainda nao obtem nem valida essa lista de contratacoes.
Required apenas calcula; nao ativa, nao compra e nao verifica pagamento.
Validate verifica integridade da selecao; Require verifica um modulo e suas
dependencias. Nenhuma dessas funcoes autentica, valida tenant ou assinatura.

## Contratacao e offline — implementacao posterior

Ainda precisam ser definidos e implementados: emissor e chaves confiaveis,
vinculo do contrato ao tenant, vigencia, renovacao, cancelamento, auditoria,
politica de uso offline e comportamento na perda de conectividade. Nenhuma
assinatura, licenca ou bloqueio comercial e simulado por este pacote.

Durante a futura integracao, nao conceder todos os modulos automaticamente
ao migrar bancos existentes. Instalar e habilitar modulos serao acoes explicitas.
Desativacao deve preservar historico e exportacao autorizada. Uma alteracao de
contrato nao deve interromper uma venda em andamento sem uma politica definida.

## Plataformas e producao configuravel

Windows, Linux, macOS, Android, iOS/iPadOS e web sao alvos de entrega. Somente a
base local Go no Ubuntu teve execucao de testes informada. O contrato de modulos
e comum; ele nao entrega aplicativo, banco no smartphone ou empacotamento.

Cada aparelho vendedor precisa persistencia propria e sincronizacao real; uma
API no PC nao garante autonomia do smartphone. Ainda faltam reconciliacao,
politica de estoque entre aparelhos desconectados e tratamento de conflitos.

production identifica a area funcional. O motor configuravel de fases ainda
precisa de modelos versionados, ordens vinculadas a versoes, responsaveis,
entradas, saidas, perdas, qualidade e trilha de auditoria. Alterar um modelo
nao deve reescrever ordens ja executadas.

## Estado de entrega

- Implementado neste pacote: IDs estaveis, dependencias, calculo da selecao,
  verificacao de disponibilidade e testes de combinacoes de modulos.
- Nao integrado: gates das rotas e dos casos de uso.
- Nao implementado: armazenamento seguro de contratacoes, licenciamento,
  RH, producao, fiscal e comunicacao real entre empresas.
- Testes deste pacote devem ser executados no PC antes de declara-lo validado.

## Proximo passo

Definir a fonte confiavel de modulos por empresa e a politica offline antes de
conectar Require aos casos de uso. Depois integrar estoque, caixa e venda a API
local com testes de acesso, rollback, repeticao e persistencia. Frontend permanece
pausado. Nenhum modulo empresarial esta concluido apenas por ter um ID aqui.
