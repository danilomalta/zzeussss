# Simples Nacional e ST — ampliação de 29/10/2026

Base: `0b91281ab2055b46e7d0532ab0ef21af3ad1aed2`. Esta entrega amplia os cálculos, memórias de cotação e grupos XML dos CSOSN201/202/203. Não constitui cobertura tributária nacional completa, apuração de DAS ou homologação de cliente.

## Implementação

- CRT1, NF-e55, destinatário contribuinte, não consumidor final, finalidade de revenda. A seleção precisa ser aprovada pelo cliente; não é inferida de NCM, CEST ou UF. Operações fora do recorte são recusadas.
- `ICMSST.own_operation_deduction` exige alíquota, redução de base, método `approved_net_operation` e referência legal. A base líquida aprovada inclui os valores declarados de mercadoria, desconto, frete, seguro e outras despesas; o motor não adivinha a responsabilidade pelo frete ou sua incidência.
- A base da dedução é reduzida e arredondada a centavos. Seu produto pela alíquota é arredondado a centavos. Essa dedução teórica é subtraída do ICMS calculado sobre a base ST. Diferença negativa é recusada, sem truncamento silencioso a zero.
- A dedução não é o crédito do destinatário, não é destaque de ICMS próprio e não é o imposto devido no DAS. Tem memória própria em `AppliedTax.own_operation_deduction`.
- No CSOSN201, o crédito segue a política mensal já existente: percentual efetivo aprovado, competência anterior, regime normal declarado do destinatário. A memória do crédito permanece separada da dedução da ST. Não há crédito inventado nos CSOSN202/203.
- Geração de `ICMSSN201` ou `ICMSSN202` (CSOSN202/203), com base, MVA, redução, alíquota e valor de ST. FCP-ST tem base, alíquota e valor separados e deve usar a mesma base ST no recorte suportado.
- Percentuais opcionais de MVA/redução iguais a zero são omitidos nos novos grupos, respeitando os tipos opcionais dos XSDs embarcados. A ordem e serialização dos grupos do regime normal anteriores foram preservadas.
- O imposto próprio do Simples continua sem destaque. Isso não significa isenção do DAS ou dispensa de apuração.
- Combinação com MVA ajustada permanece recusada nesta dedução teórica. Não há seleção automática de MVA original, acordo/protocolo ST, cBenef, crédito, regime especial, pautas ou PMPF.
- Sem migração de banco ou mudança dos XSDs históricos. Os campos novos são opcionais e omitidos nas estruturas antigas, preservando seus hashes/serialização.

Exemplo exclusivamente aritmético: base líquida100,00, MVA40%, alíquota ST18%, dedução aprovada18% sobre100,00: base ST140,00; imposto calculado25,20; dedução18,00; ST7,20. FCP-ST2% sobre140,00 acrescenta2,80. Crédito aprovado2,45% continua sendo2,45, não a dedução18,00. Estes percentuais de teste não são uma tabela legal universal nem classificam um produto real.

## Integração e limite comercial preservado

A API real de políticas e cotações publica a regra revisada e conserva os resultados imutáveis. O ensaio novo verifica releitura, reabertura SQLite e restauração criptografada de uma cotação com ST. A restauração específica usa a biblioteca normal de backup; não é um novo ensaio do executável de backup.

**Cotações com acréscimos tributários positivos não autorizam aumentar uma venda ou pagamento já concluídos.** O contrato comercial atual do emissor exige que venda, itens e pagamentos originais fechem sem novos acréscimos de ST/FCP-ST/IPI. A preparação que viola esse contrato continua recusada antes de consumir número ou transmitir. Não foi removida essa proteção para aparentar integração completa.

A emissão com os grupos novos foi ensaiada para o caso aprovado em que não existe ST adicional (MVA0 e dedução igual ao imposto calculado). Esses testes usam assinatura, SQLite e transporte mTLS reais para autoridade descartável. Não provam emissão oficial com ST adicional, homologação SEFAZ, recolhimento, GNRE ou apuração de DAS.

Para completar o fluxo comercial com ST adicional, a cotação tributária precisa participar do preço aprovado e do pagamento antes de concluir a venda, e esse vínculo deve permanecer no snapshot imutável. Esse contrato ainda requer implementação; o XML sozinho não a substitui.

## Verificação

- Núcleo: grupos201/202/203 com/sem FCP-ST, dedução separada do crédito, memória adulterada, centavos, redução, percentuais inválidos, dupla dedução, contexto e grupos indevidos. XML validado pelos XSDs embarcados e assinatura verificada.
- API: cotação persistida, recusas por acréscimo não contemplado na venda, rollback da numeração, ausência de transmissão, reabertura/restauração, emissão e reenvio no caso sem ST adicional.
- Gate do CI:15integrações obrigatórias. Os testes novos estão sob os regex de regressão fiscal já existentes; skip de integração obrigatória não é aceito.
- Fontes congeladas, comandos/resultados executados e hashes em `evidence/fiscal-simple-st-20261029/verification.json`.

## Fontes primárias consultadas

- [MOC7 — AnexoI, gruposN10e eN10f](https://www.confaz.fazenda.gov.br/legislacao/arquivo-manuais/moc7-anexo-i-leiaute-e-rv.pdf). Manual de estrutura; as alterações estruturais mais recentes são verificadas pelos XSDs já embarcados. Não é catálogo de incidência tributária.
- [SefazSP RC30427/2024, publicada em12/06/2025](https://legislacao.fazenda.sp.gov.br/Paginas/RC30427_2024.aspx): separação do ICMS-ST e da operação própria, composição com frete e dedução no contexto específico da consulta. Não foi generalizada como enquadramento nacional.
- [Resolução CGSN140/2018](https://normas.receita.fazenda.gov.br/sijut2consulta/link.action?idAto=92278): fontes oficiais indexadas sobre operação própria/ST e documentos do Simples. A página direta não forneceu texto completo à ferramenta nesta sessão; não se declara leitura integral nem aplicação automática de todas as alterações.

Continuam pendentes, além do contrato comercial acima: retenção anterior (CST60/CSOSN500), outras situações CST/CSOSN, RTC IBS/CBS/IS, benefícios e regimes especiais, regras nacionais atualizadas por produto/operação, apuração Simples/DAS, demais ST/DIFAL, ISS/NFS-e e obrigações acessórias. Nenhuma ausência é tratada como imposto zero presumido.
