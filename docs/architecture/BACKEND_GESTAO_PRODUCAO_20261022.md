# Gestão, produção e reposição assistida — 22/10/2026

Complemento exclusivamente de backend. Requer Inventário/Custos R2. Nenhuma nova
migração, integração de tela, PSP, SEFAZ ou hardware está incluída.

## Unidades implementadas

1. **Vendas por produto**: períodos explícitos, empresa/loja da sessão,
   quantidade e receita registrada em itens de vendas `committed`, comparação
   com período anterior, paginação e CSV. Vendas canceladas não entram.
   Não é apuração de liquidação bancária, devoluções parciais, caixa ou lucro.
   Se o negócio registra devoluções fora do status de venda, esse relatório
   ainda não é receita líquida após devoluções; o contrato indica a base.
2. **ABC de receita dentro da própria loja/período**: ordenação decrescente por
   receita, desempate por product_id, grupo definido pelo acumulado ANTES da
   linha: A abaixo de80%, B abaixo de95%, C restante. Receita zero recebe
   categoria própria. Nunca compara estabelecimentos diferentes, estima porte,
   diz que baixo volume é prejuízo ou sugere retirar produto automaticamente.
3. **Posições/valor do estoque**: físico, reservado e livre por produto em um
   local, paginação estável e CSV. Valoração usa custo ATUAL do catálogo,
   racionais exatos e total SOMENTE DA PÁGINA. Custo zero exige revisão.
4. **Divergências do inventário**: linhas não contadas, iguais, faltas, sobras,
   valores e saldo líquido estimados pelo custo atual. Contagem desconhecida
   permanece null. Não aprova inventário nem faz lançamento financeiro.
5. **Planejamento simultâneo de receitas**: soma insumos compartilhados de até
   20 versões e suas bateladas, desconta reservas existentes uma única vez por
   insumo/local e mostra falta/saldo. Não soma capacidades independentes como
   se elas fossem simultaneamente disponíveis. Não reserva nem cria ordem.
6. **Cenário de fabricação**: materiais atuais + minutos, funcionários e custo
   por hora + despesas para a execução INTEIRA. Dados ausentes são desconhecidos,
   zero explícito é zero informado. Receita/contribuição são hipóteses, nunca
   custo histórico completo, DRE ou lucro. Contribuição só é mostrada se mão de
   obra E despesas forem fornecidas. Tributos, frete, energia, comissões e
   encargos só entram se o responsável os incluir nas hipóteses; não são inferidos.
7. **Reposição assistida**: um produto, vendas observadas da loja, estoque de
   todos os seus locais menos reservas, política humana existente, lead time,
   cobertura e segurança. Não envia pedidos, altera mínimos, dá garantia de
   previsão, agenda caminhão ou confirma fornecedor. Aprovação humana permanece.

## Segurança e leitura

Sessão, empresa, loja, usuário e aparelho vêm dos mecanismos existentes. Cada
unidade verifica permissão/aparelho na mesma transação de leitura. Relatórios
financeiros exigem `view_accounting`; posições exigem `manage_stock` e valoradas
também `view_accounting`; inventário idem; produção usa `manage_production`,
com custo adicionalmente `view_accounting`; reposição exige `manage_stock` e
`manage_replenishment`. Não há novo papel genérico ou bypass administrativo.

Leituras e simulações não renovam contrato, registram estoque, capturam
pagamento, alteram relógio de licença nem escrevem outbox. Não transferem dados
para fornecedor/Titan. Módulos já licenciados e operações de escrita oficiais
continuam regidos por seus contratos. Uma consulta não autoriza uma escrita futura.

Rotas retornam `Cache-Control: no-store`. Nenhum dado pessoal ou comercial é
registrado em logs. CSV neutraliza prefixos de fórmula nos campos textuais.
O cliente deve baixar CSV diretamente; não é solicitada uma pasta de destino.

## Exatidão e limites

- Valores/quantidades inteiros até9007199254740991, intermediários big.Int/Rat.
- Custo por unidade é racional; valores agregados arredondam half-up uma vez.
- Percentual é basis points, truncado em direção a zero; zero anterior vira
  `no_baseline`/`new_activity`, não infinito ou crescimento inventado.
- Datas RFC3339Nano, intervalo fechado no início e aberto no fim [from,to).
  A leitura SQL é alargada em1segundo; Go faz classificação precisa, inclusive
  fuso e nanossegundos. Datas armazenadas inválidas não viram sucesso vazio.
- Vendas: até366dias por janela e100000linhas consultadas. Acima disso erro422,
  sem truncamento silencioso. Janela comparativa diferente em duração mantém
  totais e suprime variação percentual. Sazonalidade depende da janela escolhida;
  não foi criado modelo estatístico ou comparação entre empresas.
- Catálogo e nomes/unidades são os ATUAIS; o esquema não guarda unidade
  histórica em todo item vendido. Não são conversões retroativas de unidade.
- Estoque: local obrigatório, limite100 por página, offset até1000000.
  Quantidades de unidades diferentes não são somadas em um número genérico.
- Receitas:1..20 versões únicas; faltas calculadas por produto e unidade.
  Resultado expressa viabilidade atual, sem produzir produtos intermediários
  para alimentar outra receita da mesma simulação.
- Cenário: minutos0..525600, funcionários1..10000. Mão de obra/despesas são da
  execução completa, não de cada batelada. Preço é por unidade comercial do
  produto acabado; preço0 não produz percentual de margem.
- Reposição: janela1h..366dias; lead0..365dias; cobertura1..365dias; até500locais
  da loja e100000itens de venda do produto. Sem política existente:404.
  mínimo sugerido=max(mínimo humano,ceil(média diária*lead+segurança)); alvo
  sugerido=max(alvo humano,ceil(média diária*(lead+cobertura)+segurança)).
  Só sugere pedido ao atingir mínimo, por alvo-livre. Produto unitário arredonda
  para unidades inteiras. Sem vendas no período mantém política explícita e
  identifica ausência de vendas observadas; não inventa previsão de demanda.
- Nenhuma simulação inclui entregas futuras, pedidos abertos ou estoque de
  aparelho offline ainda não reconciliado como se já estivessem disponíveis.

## Integração e pendências

Contrato: `docs/api/management-production.openapi.json`,10operações novas.
As rotas ficam sob `/local/v1/management`, montadas a partir do R2. No B13, os
mapas globais são ampliados preservando rotas existentes. Nenhuma migração nova.

Ainda faltam adaptar as telas, validar login/serviço/configuração integrados,
piloto real, mensurar memória/tempo em PC antigo, custo histórico industrial,
devoluções líquidas, previsão sazonal avançada, alertas persistentes e conexão
com fornecedor. Este complemento não os anuncia como concluídos.

## Aplicação e preservação

Aplicador exige R2 intacto e histórico de migrações conhecido, ensaia candidata
com os fontes locais, testa unidades/API/rotas e migração/backup, executa vet e
compara preimagens antes da gravação. Fontes originais em backup separado; não
abre banco comercial, não inicia serviço nem compila binário sobre instalação.
Mantém o registro R2 e acrescenta `BACKEND_GESTAO_PRODUCAO_APLICADO.json`.
Como um arquivo R2 recebe a montagem nova, reaplicar o instalador R2 antigo
posteriormente detectará mudança e parará: comportamento esperado. Use o
instalador DESTA entrega para verificar sua própria idempotência.
