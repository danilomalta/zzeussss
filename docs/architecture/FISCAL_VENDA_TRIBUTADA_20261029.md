# Venda com preço tributário aprovado — 29/10/2026

Base: `0506eaac06a1c8ac0c88ee4a9ddb31138a8867f3`, branch fiscal existente. Esta entrega fecha o vínculo comercial que faltava para os acréscimos suportados de ST, FCP-ST e IPI: a cotação integra o valor aprovado **antes** de concluir a venda em dinheiro. Não altera vendas já concluídas.

## Fluxo e API

1. Proprietário configura/revisa o perfil fiscal e seu fuso IANA. `Local` é recusado. O fuso não é inferido pela localização do computador. A base IANA da biblioteca Go é embarcada para portabilidade.
O proprietário pode exigir esse caminho com `require_tax_pricing:true`. A exigência é aplicada na transação de todos os escritores de venda, inclusive na finalização de planos antigos; esses planos deixam de anunciar `can_finalize:true` quando bloqueados. Revisões futuras precisam informar explicitamente true/false: omissão não desativa a proteção. Replays de configurações antigas recuperam sua revisão original sem mudar a revisão ativa. A ativação não é feita automaticamente na instalação nem pelas novas telas.

2. A API existente cria a política aprovada e a cotação tributária. A unidade do catálogo é observada no servidor e gravada na cotação, independentemente de campos enviados pelo cliente.
3. `/local/v1/sales/quote` aceita opcionalmente `tax_quote_operations`: uma referência única por item, na ordem dos itens. Até100itens nessa modalidade; o fluxo legado preserva seu limite anterior.
4. O servidor revalida sessão, contrato POS+Fiscal, caixa, estoque cumulativo, produto ativo, unidade, preço e quantidade. A política precisa estar ativa e na revisão vigente, assim como o perfil. A cotação precisa ter até10minutos e data igual ao dia atual no fuso do emitente. Todas as linhas compartilham perfil/revisão e contexto do destinatário/operação, respeitando origem do produto/CEST próprios de cada linha.
5. A resposta identifica `merchandise_cents`, `tax.additional_cents` e o `total_cents` final. Não reserva estoque nem confirma pagamento.
6. `/local/v1/sales` recebe as mesmas referências e o pagamento em dinheiro. Os preços e tributos não são aceitos como valores autoritativos da requisição. A transação repete as verificações e exige o pagamento do total completo. Qualquer divergência recusa a operação; não há recálculo silencioso de uma venda já paga.
7. Venda, pagamento, movimento de caixa, baixa de estoque, vínculo tributário e outbox são gravados na mesma transação. O replay exato recupera o resultado original; outro corpo/ator/dispositivo não adota uma venda preexistente.
8. O snapshot fiscal conserva o vínculo da cotação e os grupos/totais originais. O emissor compara as linhas e pagamentos com essa fonte imutável. Alterar/omitir o vínculo ou substituir os tributos não autoriza outra emissão.

O caminho opt-in permanece em dinheiro, sem descontos, frete, seguro ou outras despesas comerciais adicionais. Estes contratos não foram estendidos nesta entrega. O suporte depende também dos grupos tributários já implementados; não permite tributos ou enquadramentos que o motor recusa.

## Persistência e compatibilidade

- Migração76 incremental: `sale_tax_bindings`, com FKs e proteção contra alteração/exclusão, e unidade observada em `fiscal_tax_quotes`.
- As75migrações históricas são preservadas. Nenhuma abertura de banco comercial foi executada nesta tarefa.
- Cotações históricas sem evidência de unidade continuam legíveis. Não recebem unidade inferida retroativamente e não autorizam uma nova venda tributada; é necessária uma cotação nova.
- `sale_items.total_cents` permanece o valor da mercadoria. `sales.total_cents`, pagamentos e movimento de caixa incluem os acréscimos aprovados. O recibo expõe a separação e confere que a soma fecha.
- Inputs e snapshots legados omitem os campos opcionais novos e preservam os hashes antigos. O novo tipo de outbox `sale.committed` usa versão2 quando carrega evidência tributária; o caminho legado continua na versão1. Consumidores precisam reconhecer a versão2; esta entrega não comprova sincronização comercial desse evento entre dispositivos.
- Backup aceita esquemas25–76, valida os históricos e migra cópias antigas na restauração. A inspeção inclui os vínculos tributários; arquivos de esquemas antigos podem indicar essa tabela como indisponível até a migração, sem inventar contagem zero histórica.

## Recuperação, cancelamento e devolução

- A gravação incerta é recuperada pelos identificadores originais e pelo replay exato. Não é apagada ou refeita automaticamente depois de trocar sessão.
- A sessão é verificada novamente dentro da transação para o caminho opt-in, inclusive quando o middleware já tinha resolvido o ator antes do logout.
- Leitura do recibo e das evidências históricas não exige política atualmente ativa nem licença original ainda válida, mantendo a autorização atual de leitura.
- Cancelamento comercial integral reembolsa o total efetivamente pago, incluindo os acréscimos. A evidência original permanece.
- Devoluções parciais usam o total original de cada linha, incluindo os acréscimos, com o cálculo cumulativo já existente: duas metades reembolsam exatamente o total, sem centavo perdido ou devolução excessiva.
- Cancelar/reembolsar comercialmente não cancela automaticamente a NF-e nem conclui escrituração ou compensação tributária. As ações fiscais continuam separadas.

## Ensaios implementados

- CSOSN201/202/203: mercadoria31,98 + ST2,30 + FCP-ST0,90 = dinheiro35,18, com XML validado, assinatura verificada e uma transmissão mTLS para autoridade descartável.
- Regime normal: IPI1,60 incorporado antes do pagamento; total33,58 e XML com o mesmo valor.
- Nove chamadas TCP HTTP reais percorrem preço, venda, recibo, snapshot, revisão, aprovação, emissão, replay e consulta, usando a API e o SQLite reais.
- Falha na gravação dos vínculos reverte venda, pagamentos, caixa, estoque e evidências. Pagamento incompleto e fonte antiga/estranha são recusados.
- Preço/unidade/quantidade alterados, perfil atualizado, cotação envelhecida/futura, dispositivo alheio e referência repetida não autorizam nova venda.
- Logout depois de obter o escopo e antes da transação impede o POST antigo. Novo login recupera os registros originais.
- Reinício e restauração criptografada preservam valor, vínculos e hashes. A rodada habilitada com `titan-backup` usa o executável real, incluindo verificações negativas e recusa de sobrescrita, em estação de teste.
- Testes de cancelamento, devoluções parciais e concorrência verificam conservação de dinheiro e estoque e um único efeito financeiro.

Resultados realmente executados, hashes e distinções de ambiente em `evidence/fiscal-tax-priced-sale-20261029/verification.json`. A CI exige21integrações obrigatórias, incluindo o percurso TCP e o cenário de IPI. Skip de integração obrigatória não é aceito.

## Limites preservados

A autoridade e todos os dados dos ensaios são descartáveis. Não houve homologação oficial de cliente ou alteração da instalação. A API opt-in não está conectada ao checkout visual nesta entrega; o frontend permaneceu preservado. Não se anuncia o fluxo como homologado na interface instalada.

O motor tributário nacional continua incompleto: RTC IBS/CBS/IS, retenções anteriores, catálogo atualizado de benefícios/enquadramentos, regimes especiais e apurações adicionais, outras modalidades ST/DIFAL, Simples/DAS, ISS/NFS-e e obrigações acessórias ainda precisam de implementação e validação. Esta entrega resolve o vínculo comercial dos acréscimos suportados; não elimina essas lacunas nem representa homologação SEFAZ, recolhimento, GNRE ou apuração.
