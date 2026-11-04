# Retenção anterior de ICMS-ST — 29/10/2026

Base: `bf1ac5b4e6e9944c1cf49bfb8ce9c64ac735ac99`, branch fiscal existente. Os sete checks desse commit passaram antes desta ampliação. O novo commit exige seus próprios checks.

## Comportamento implementado

Acrescentado CST 60 para CRT 2/3 e CSOSN 500 para CRT 1. O recorte suportado é NF-e 55, operação interna, CFOP 5405, revenda a contribuinte não consumidor final e CEST explícito na política. Outros CFOPs, operações interestaduais, consumidor final, NFC-e e ICMS efetivo não são inferidos desse suporte e continuam recusados.

A política continua aprovada pelo proprietário. O item da cotação pode conter `retained_st`, com chave NF-e 55 e item da compra, referência de aprovação, quantidade/unidade exatas e os valores de base, alíquota suportada, ICMS próprio do substituto e ST retido atribuídos àquela linha. FCP anterior exige grupo completo e `fcp_scope:retained`. A alíquota suportada é informada com seu conteúdo legal, inclusive FCP quando aplicável; o servidor não escolhe uma alíquota interna pela UF.

A nova declaração de valores de origem exige proprietário ativo e sessão/contrato válidos dentro da transação. Um contador com permissão de consulta não pode publicar essa declaração. Quantidade precisa coincidir com a linha; unidade precisa coincidir com o catálogo observado no servidor. Chave tem formato/modelo, dígito e mês conferidos; mês futuro é recusado. Valores decimais têm limites exatos de leiaute. FCP precisa fechar com base × percentual, arredondado para centavos. Referência vazia, origem incompleta, dados duplicados/desconhecidos, valores negativos, excesso de precisão ou contexto não suportado não geram cotação parcial.

**A origem é declarada e aprovada pelo proprietário.** O sistema não consulta a existência/autorização da compra na SEFAZ, importa/valida automaticamente seu XML nem vincula essa declaração aos lotes recebidos nesta unidade. Também não rateia a compra, consome um saldo fiscal de origem ou prova que a mesma origem não foi alocada em excesso por outras declarações. Os valores devem estar previamente atribuídos à quantidade da linha; mudança de quantidade requer nova declaração e cotação. Não apresentar esses limites como rastreabilidade automática concluída.

## Dinheiro, XML e recuperação

Retenção anterior não é nova incidência: não aumenta `vST`, `vFCPST`, `vICMS`, preço, pagamento ou movimento de caixa. O grupo XML será `ICMS60` ou `ICMSSN500`, com `vBCSTRet`, `pST`, `vICMSSubstituto`, `vICMSSTRet` e FCP retido quando declarado. `vFCPSTRet` soma no campo próprio dos totais, sem aumentar `vNF`.

O método interno de cálculo sem débito próprio continua usando o marcador aritmético `exempt` no componente ICMS. Isso não muda o enquadramento jurídico: o XML conserva CST 60/CSOSN 500, e não uma classificação de isenção.

Cotação, memória e hashes preservam a declaração original. O vínculo imutável da venda aponta para essa cotação. O caminho tributado já existente valida novamente preço/quantidade/unidade, escopo, perfil/política, idade e sessão. Não recalcula silenciosamente venda paga. Replay com valores diferentes é recusado; replay exato recupera os mesmos IDs e não duplica dinheiro ou transmissão. Reinício e restauração criptografada conservam fonte, recibo e XML processado.

Nenhuma migração nova: campos opcionais são omitidos em requisições/resultados legados, preservando formato e hashes anteriores. Permanecem as 76 migrações já publicadas, sem modificação. Frontend/Electron, instalação, serviços e banco comercial não são alterados.

## Validação e evidências

Testes de núcleo cobrem CST 60 (CRT 2/3) e CSOSN 500, com/sem FCP, assinatura e XSD embarcado. Testes da API cobrem preço sem cobrança duplicada, venda, emissão, replay, proprietário/permissão, unidade, rejeições de origem, reabertura SQLite e backup/restauração. Um ensaio percorre dez chamadas HTTP TCP reais e uma transmissão mTLS assinada. Autoridade, identidades, certificado e dados são descartáveis, sem homologação oficial.

A CI passa a exigir 25 integrações; os quatro testes novos da API não podem ser pulados. As rodadas com detector de concorrência incluem o núcleo e as novas integrações. Resultados finais, comandos, hashes e skips são registrados em `evidence/fiscal-retained-st-20261029/verification.json` após a conclusão dos testes, com linhas controladas do ensaio nativo.

## Fontes e limites fiscais

- [MOC 7, Anexo I — grupos N08/N10g e total W06b](https://www.confaz.fazenda.gov.br/legislacao/arquivo-manuais/moc7-anexo-i-leiaute-e-rv.pdf).
- [NT 2018.005 v1.52 — campos da retenção anterior e regras de validação](https://www.nfe.fazenda.gov.br/PORTal/exibirArquivo.aspx?conteudo=vZguLua3oPM%3D).
- XSD oficial já embarcado em `backend/internal/fiscalcore/schemas/nfe400/`; nenhum schema foi editado para fazer o teste passar.

O catálogo oficial consultado lista a NT 2018.005 v1.52; a abertura direta desse PDF nesta sessão retornou `Redirect loop detected`. Os campos foram conferidos no MOC aberto e no XSD já embarcado; não se declara leitura integral da revisão mais recente da NT nesta sessão.

Essas fontes descrevem estrutura e validações. Não são um catálogo atualizado de incidência, benefícios ou todas as exigências estaduais de cada cliente. A aplicação das regras pode depender da UF e da operação. Regras/taxas e proveniência continuam exigindo aprovação explícita, sem valores nacionais presumidos.

Permanecem lacunas do fiscal completo: RTC IBS/CBS/IS, regimes especiais e benefícios, desoneração, outras retenções/créditos, apurações e obrigações, outros ST/DIFAL, Simples/DAS, ISS/NFS-e de produção, SPED/PVA, contingência e DANFE de produção nos recortes pendentes, integração visual do checkout e homologação oficial por emitente. Esta entrega acrescenta retenção anterior no recorte documentado, não promete cobrir todos esses requisitos.
