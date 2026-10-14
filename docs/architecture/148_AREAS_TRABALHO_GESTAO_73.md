# Entrega 73 — Áreas de trabalho locais

Esta entrega NÃO conclui o ERP nem a lista completa de integrações solicitadas. O backend e o PDV existentes não são modificados. Acrescenta ferramentas reais de cálculo, arquivos de trabalho e consultas usando os clientes existentes.

## Cinco grupos entregues
1. PDV: consulta real e paginada de vendas/recibos pelo cliente local, soma das vendas confirmadas DA PÁGINA e calculadora de divisão entre dinheiro, crédito, débito, Pix e crediário. A calculadora não efetua pagamentos nem cria recebíveis oficiais. Não é fechamento completo de turno.
2. Financeiro: arquivo de trabalho com contas previstas a pagar/receber, centro de custo, filtros por data e previsão de saldo. Marcação de conferência manual não confirma liquidação bancária. CSV com dinheiro em centavos.
3. Fiscal: leitura local de múltiplos XMLs NF-e/NFC-e, prévia de emitente/produtos/totais/CFOP/CST/CSOSN/NCM e protocolos informados. Não valida assinatura, destinatário contra CNPJ do cadastro, autorização na SEFAZ ou regras tributárias.
4. Contabilidade: plano de contas adaptável no arquivo, lançamento manual de duas partidas, validação de igualdade débito/crédito, diário, totais por conta/balancete, resultado de receitas e despesas do arquivo, estorno e bloqueio de períodos DO ARQUIVO. Não é livro contábil oficial, balanço completo, ECD ou ECF. O arquivo JSON importado pode ter partidas múltiplas validáveis. Estorno não altera original.
5. Contador: modo de leitura para o arquivo selecionado e ZIP com diário CSV, arquivo JSON e os XMLs escolhidos explicitamente. NÃO é portal externo com acesso próprio/carteira de clientes, envio automático ou formato específico Domínio, Alterdata ou Fortes.

## Armazenamento e segurança
Não há autosave nem cache persistente de dados no navegador. O usuário salva explicitamente um JSON de trabalho; ao abrir, o contexto empresa/loja/usuário deve corresponder à sessão. Arquivos salvos/exportados contêm informações financeiras/fiscais: devem ser guardados com o cuidado normal dos documentos empresariais. Não contêm token ou senha.
XMLs não são embutidos no JSON de trabalho; selecionar novamente após reabrir. Somente o ZIP solicitado inclui os XMLs selecionados. Recibos do servidor não são inseridos no arquivo local.
Eventos de atividade são notas em memória, limitados a 100. Não são auditoria oficial/imune a adulteração e não são persistidos. O bloqueio de período também é uma regra do editor, não uma garantia sobre arquivos alterados externamente.
Encerrar sessão desmonta as ferramentas e remove os dados da memória. Nenhum payload financeiro é registrado no console. Assinaturas e autorização de XML não podem ser inferidas apenas de um nProt presente.

## Permissões
Consulta PDV: licença pos ativa e sell. Financeiro: titular e finance ativa. Fiscal: titular e fiscal ativa. Contabilidade/pacote: accounting ativa e view_accounting. Sem licença ativa a ferramenta fica bloqueada; nenhum papel ou contrato é criado para contornar esse bloqueio. Cada endpoint continua validando a sessão no servidor.
Não implementa carteiras multiempresa; o arquivo pertence a empresa/loja/usuário da sessão. Um contador não ganha acesso a financeiro apenas por ter view_accounting.

## Dependências e limites
Reutiliza React, Router, i18n e JSZip existentes; não muda package.json ou lock. XML máximo 2MiB por arquivo, 20MiB carregados, 20 selecionados por vez e 100 documentos. JSON máximo 5MiB, 200 contas, 1000 lançamentos, 1000 contas financeiras, 240 períodos. Dinheiro em centavos inteiros; rejeita overflow.
Leitura de XML usa DOMParser, bloqueia DOCTYPE/ENTITY antes da análise. Não é validador XSD. Textos são renderizados pelo React, sem HTML oriundo do XML.

## Fiscal atual
Referência consultada em 14/10/2026: https://portal.fazenda.sp.gov.br/servicos/nfce — NFC-e obrigatória para o varejo paulista a partir de 01/01/2026, substituindo CF-e-SAT. Não se implementa emissão SAT nova neste pacote. Demais UFs precisam de conferência própria.
Pix: especificação primária https://github.com/bacen/pix-api . É necessária integração com PSP, autenticação/segurança e conferência do pagamento pelo backend; digitar Pix não confirma recebimento.

## Toda a lista solicitada: pendências reais
PDV: NFC-e assinada/autorizada, contingência por UF, CPF na nota, cancelamento/inutilização; Pix automático, TEF/POS, pagamento dividido efetivo e fiado; sangria/suprimento na tela, suspensão/retomada durável, troca/devolução, desconto autorizado, periféricos; sincronização comercial completa; auditoria oficial de todas as ações. Algumas bibliotecas antigas existem nas cópias locais, mas não se afirma compatibilidade com o servidor atual sem demonstração.
Empresa: persistência oficial de contas a pagar/receber e centros de custo, liquidação, bancos/OFX, boletos, DRE gerencial integrada; entrada XML com geração de estoque/contas, manifestação, cotações; ABC/validade/lotes/transferência entre filiais; comercial com orçamento, venda B2B, comissões/metas; multiempresa/BI. Estoque e reposição já têm componentes anteriores e são preservados, não reimplementados.
Contabilidade: plano de contas oficial versionado, escrituração/auditoria durável, razão analítico completo, balanço, apuração por regime/UF/data, calendário legal, SPED ECD/ECF/EFD, fechamento oficial, integração folha/eSocial e ingestão automática das origens. Arquivo local não substitui contador ou obrigações.
Portal: identidade externa, autorização por cliente, carteira multiempresa, downloads oficiais, classificação/pendências, colaboração/documentos e formatos validados para Domínio/Alterdata/Fortes.

## Contratos backend necessários — propostas, não rotas implantadas
- Pagamentos: criação idempotente por venda, métodos, centavos, txid/referência adquirente, estados pending/confirmed/refunded, webhook verificado, consulta de recuperação, conciliação e fechamento por turno/método. Nenhuma confirmação no cliente.
- Fiscal: perfil CNPJ/UF/regime/certificado seguro, número/série/ambiente, outbox fiscal, XML assinado, protocolo/autorização, DANFE, contingência e eventos idempotentes. Certificado/segredo não fica no frontend.
- Operação: movimentos de caixa, carrinhos suspensos versionados, aprovação de descontos, devoluções vinculadas às linhas originais, cancelamentos e restituições atômicas.
- Financeiro: título, parcela, vencimento, centro de custo, baixa e estorno auditados, previsão por período, conciliação por FITID/ID de provedor, boletos e classificação contábil.
- Compras/fiscal: ingestão XML com validação documental, chave única, destinatário, produtos/classificação, prévia e confirmação atômica; manifestação/reconsulta; cotações/reservas.
- Contabilidade: plano de contas por empresa/versão, partidas em centavos, documento/origem, idempotência, balanceamento obrigatório, períodos bloqueados no servidor, estornos auditados, relatórios e exports certificados.
- Contador: autorização por empresa/escopo com revogação, documentos e comentários, fechamento, exportação oficial e retenção/prazos legais verificados.

## Validação feita
138 testes Node disponíveis passaram; TypeScript passou; build Vite isolado das novas telas passou. Testes específicos cobrem números, permissões, igualdade de partidas, estorno, datas, períodos fechados, arquivo estrangeiro, fórmula CSV e pré-validação XML.
Não houve demonstração visual no Firefox, teste DOM completo de XML, conexão com SEFAZ/PSP/TEF ou servidor comercial. O instalador executa a suíte disponível no PC e o build completo antes do commit. Não afirma que todo o restante do projeto foi testado aqui.
Traduções em 5 idiomas com mesmas chaves; en/es/fr/zh-CN requerem revisão nativa.
