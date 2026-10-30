# ICMS diferido e crédito do Simples — 29/10/2026

Base: `3b745541cfc232bde78ec7020ce9dd8334c64c3f`. Ampliação do motor e do vínculo entre cotação persistida e emissor. Não altera frontend, instalações existentes, serviços ou dados comerciais. Sem migração nova.

## Diferimento CST 51

- Entrada explícita `Component.deferment`: percentual e referência legal revisada pelo cliente. Não inferida da UF/NCM.
- Recorte: NF-e55, CRT2/3, operação interna, sem redução de base, ST ou diferimento de FCP. Casos fora do recorte são recusados.
- O valor da operação de ICMS é arredondado a centavos; o valor diferido é esse valor multiplicado pelo percentual e arredondado half-up; o devido é a diferença exata entre os dois valores monetários.
- A memória imutável grava percentual, imposto da operação, diferido e devido. O componente ICMS representa o devido. Componentes posteriores só descontam esse valor se a política explicitamente declarar a dependência.
- O XML contém `vBC`, `pICMS`, `vICMSOp`, `pDif`, `vICMSDif`, `vICMS`. Totais de ICMS usam o devido; diferimento não é isenção e não altera automaticamente o valor da venda.
- Recusa percentual ausente/zero/negativo/acima de100, precisão não representável, referência vazia, associação a outro componente/CST, desconto de tributo sobre o próprio ICMS, acréscimo ao total e memória adulterada.
- Código cBenef deve ser escolhido e aprovado conforme a tabela aplicável. Os códigos sintéticos dos testes não são classificações legais do produto. Não há validação automática de enquadramento ou catálogo de benefícios.

Exemplo aritmético: operação de ICMS180,00 e diferimento33,33% resultam em59,99 diferidos e120,01 devidos. O motor não reproduz arredondamentos aproximados de exemplos como se fossem exatos.

## Crédito CSOSN 101

- Entrada explícita `GoodsTaxClassification.simple_credit`: percentual efetivo aprovado, competência de receita, regime declarado do destinatário, método de base e referência legal.
- Recorte: CRT1, NF-e55, destinatário contribuinte de regime normal declarado, não consumidor final, finalidade de revenda/industrialização. Não contempla início de atividade, crédito do CSOSN201/900, apuração DAS ou comprovação automática da elegibilidade.
- A vigência da política deve estar contida em um único mês de operação. A competência de receita é obrigatoriamente o mês anterior. Outra competência requer outra política/revisão aprovada; não se reutiliza percentual indefinidamente.
- `approved_net_operation` usa o valor líquido da operação já calculado pela política. O responsável revisa a composição e aplicabilidade dessa base. Não é uma base inferida pelo software.
- A memória `GoodsXML.simple_credit` registra base, percentual, valor e competência. O XML utiliza `ICMSSN101`, `pCredSN` e `vCredICMSSN`.
- O crédito não aumenta imposto próprio, total da nota, dinheiro ou estoque. Não é devolução nem imposto a pagar. O motor não adivinha faixa de receita, repartição de ICMS, impedimentos ou percentual efetivo do cliente.

## Integração e proteção

Ambos os recortes usam as políticas/cotações versionadas já existentes. O emissor consulta a mesma cotação aprovada e constrói os grupos; conserva número, revisão e identidade no reenvio. Retirada de política continua bloqueando aprovação/envio conforme a proteção existente. O resultado de teste com autoridade descartável não é autorização SEFAZ.

Testes novos cobrem centavos e arredondamento, diferimento total/parcial, recusas de enquadramento, competência e destinatário do crédito, XSD embarcado, assinatura real, API real/SQLite, mTLS para autoridade descartável, reenvio sem duplicação e releitura da memória persistida. A validação de reinício/restauração dos contratos compartilhados é reexecutada na suíte existente; os novos testes específicos não constituem ensaio oficial do cliente.

## Fontes primárias

- [Orientação NF-e, campos CST51 e CSOSN101](https://www.nfe.fazenda.gov.br/portal/exibirArquivo.aspx?conteudo=5W9aeeSeghM%3D). Localizada pelo índice oficial; acesso direto ao PDF apresentou ciclo de redirecionamento nesta sessão. A estrutura foi conferida nos XSDs oficiais já embarcados, sem alterá-los.
- [Receita RS: diferimento parcial e escrituração](https://atendimento.receita.rs.gov.br/como-deve-ser-preenchida-a-nfe-no-caso-de-diferimento-parcial-e-como-esta-deve-ser-escriturada-na-efd). A base para escrituração EFD tem tratamento próprio; esta entrega não gera EFD/PVA.
- [Sefaz SP: tabela cBenef](https://portal.fazenda.sp.gov.br/servicos/nfe/Paginas/cBenef.aspx). A exigência e o código dependem da operação e da regra aplicável; não são resolvidos só pelo CST.
- [Resolução CGSN140/2018 consolidada](https://normas.receita.fazenda.gov.br/sijut2consulta/link.action?idAto=92278). Percentual efetivo e informação do crédito dependem do regime e das condições legais; não se adota percentual fixo universal.
- [Sefaz SP RC29823/2024](https://legislacao.fazenda.sp.gov.br/Paginas/RC29823_2024.aspx): campos próprios do CSOSN101. Consulta específica não equivale a regra de aplicação nacional.

## O que continua fora da cobertura

O motor não contém todas as regras tributárias brasileiras. Permanecem lacunas de classificação e apuração, incluindo RTC IBS/CBS/IS, ICMS monofásico, retenções anteriores, créditos/diferimentos adicionais, benefícios e regimes especiais por produto/operação, ST em todos os acordos, outras bases DIFAL, Simples/DAS e início de atividade, ISS/NFS-e e obrigações acessórias. Também não houve homologação oficial, nova instalação, DANFE/contingência ou SPED/PVA nesta ampliação. Ausência de cobertura continua gerando indisponibilidade/erro explícito, nunca imposto zero presumido.

Resultados executados e hashes estão em `evidence/fiscal-deferment-credit-20261029/verification.json`.

## Ajuste do CI

O job de aceite do commit-base terminou cancelado em40minutos, durante a última etapa de pilotos com detector de corrida. Os passos de regressão completa e race fiscal haviam passado. O limite agregado foi ampliado para90minutos para comportar a soma dos limites individuais, sem remover testes ou aceitar skips obrigatórios. As duas novas integrações fiscais passaram a integrar o gate:13integrações obrigatórias. O novo HEAD precisa executar seu próprio CI; resultados da base não o certificam.
