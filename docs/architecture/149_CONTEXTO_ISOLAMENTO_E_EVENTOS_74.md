# Entrega74 — Contexto visível e central de integrações

## Escopo entregue
Frontend e contratos de contexto no cliente. NÃO aplica migração nem muda consultas do backend. Não certifica isolamento de todas as tabelas existentes. Não cria troca de empresa, portal externo, notificações de eventos comerciais, backup ou sincronização completos.

1. Cabeçalho com empresa e loja do contexto da sessão. Usa comparação rigorosa dos IDs tenant/store/identity/device entre sessão e capacidades e rejeita sessão expirada. Nomes empresariais não vêm no contrato atual: mostra Empresa atual / Loja atual, sem inventar nome/CNPJ.
2. Ajuda funcional em diálogo nativo, fechamento ESC/X e foco devolvido. Notificações explicam fonte indisponível, sem zero fictício ou alerta de exemplo.
3. Um novo card Integrações, somente titular, em Administração. Nenhuma nova permissão é concedida.
4. Central com teste real GET /local/v1/health, exige status local e prazo de 5 segundos. Comprova somente a resposta HTTP; não comprova banco, migrações, autorização, sincronização, pagamentos ou prontidão do ERP. Não envia token a esse health público já existente. Demais cartões têm estado sem evidência consultada porque o painel não possui fonte real desses serviços.
5. Modelo reutilizável para chaves de contexto sem colisão, registros de alcance loja/empresa, envelope de eventos sem dados pessoais e critério de operacional com seis evidências booleanas. São funções de validação no cliente, não serviço de eventos ou auditoria durável.

Não há seletor que permita escolher outro ID. O contexto real autorizado não é alterado. Os dois seletores têm apenas a opção atual e explicação. A sessão é a origem; o navegador não cria vínculo com empresa.
Não altera arquivos 70/71/73 ou seus formatos, nem faz retrofit silencioso de dados. Os novos contratos são a base para os próximos consumidores; não estão injetados em todas as bibliotecas históricas.

## Modelo a implementar/verificar no servidor
- Identidade autenticada possui vínculos de empresa/loja, permissões e prazo. Permissão não é derivada do tamanho/plano da empresa nem de um ID fornecido pelo navegador.
- Registro de empresa: tenant_id obrigatório, store_id nulo somente quando escopo empresarial for explicitamente definido. Registro de loja: tenant_id e store_id obrigatórios.
- Autor/operador e aparelho de origem são metadados de proveniência. Dispositivo não substitui autorização; uma troca de dispositivo não muda a propriedade do dado.
- IDs comerciais isolados por tenant, FKs compostas para empresa/produto/loja e restrições para referências cruzadas. Não migrar registros existentes com tenant padrão sem classificação verificada.
- Toda consulta/alteração deriva contexto da sessão. Nunca confiar em tenant/store na URL/corpo sem confronto com autorização. Consolidação recebe lista de vínculos autorizados, não acesso global.
- Troca de contexto deverá: confirmar alterações pendentes; pedir sessão nova ao servidor; retornar contexto/capacidades verificados; cancelar requisições anteriores; limpar caches e fechar recursos; recarregar. Não modificar apenas Zustand/store_id.
- Catálogo/serviços compartilhados e estoque/caixa por loja precisam ter alcance explícito; não adicionar store_id obrigatório indiscriminadamente a todo cadastro.

## Operacional: critérios obrigatórios
Fluxo integrado demonstrado; dado sobreviver a reinício; permissões respeitadas; isolamento entre empresas/lojas verificado; trilha de auditoria durável; restauração testada. O painel não marca nenhum módulo como operacional porque não recebeu evidências de um servidor autorizado. Booleanos de um arquivo local não certificam produção.
Teste de perda de resposta/idempotência e repetição de eventos também obrigatório nas operações críticas.

## Eventos e auditoria — especificação para backend
Envelope proposto: event_id, operation_id, type, version, occurred_at, tenant_id, store_id, identity_id, device_id. Tipos iniciais stock.low, cash.difference, fiscal.rejected, backup.restore_result. A função cliente não é usada para produzir prova oficial.
O servidor deriva contexto/ator; grava operação, auditoria e outbox na mesma transação. Entrega posterior pelo menos uma vez, recibos e deduplicação por evento/consumidor. Consumidores autorizados rejeitam tenant/store incompatíveis e versões desconhecidas. Não guardar salários, documentos ou tokens no envelope genérico. Retenção e visibilidade variam por evento.
O código não implementa transporte, fila comercial ou worker novos. Outros componentes existentes de outbox são preservados; sua cobertura completa não foi auditada nesta etapa.

## Backup e restauração
Contrato necessário com identificação do backup, alcance empresa/loja, versão de schema, digest, criação, criptografia e autorização. Restauração em ambiente isolado, integridade/referências e fluxos conferidos, resultado registrado com data e responsável. Não anunciar backup recente pela mera existência do executável titan-backup.
Restaurar não deve executar sobre banco comercial sem um fluxo autorizado e revisão do resultado. Esta entrega não faz operação de banco ou exclusão.

## Contratos ainda necessários (não endpoints existentes)
- Contextos autorizados: nomes/CNPJ/lojas, vínculo, capacidades, versão e sessão/credencial por contexto. Consulta não deve enumerar empresas sem vínculo.
- Evidências operacionais: módulo, versão implantada, datas/resultados de testes e responsáveis; fonte autorizada, sem confiar em marcações do frontend.
- Status de integração: provedor, ambiente, conexão, última execução, falhas e referência de configuração segura (nunca devolver certificado/senha).
- Notificações: eventos autorizados, cursor, severidade, ação e confirmação idempotente; contagem apenas da fonte real.
- Backup/restauração e sync: status/resultado real, filas, conflitos e parceiros autorizados.
Sem essas fontes, os cartões permanecem sem evidência; ações de configurar são desabilitadas.

## Compatibilidade
Instalador faz inserções localizadas no menu72/73, workspace e router, preservando planilhas/RH/PDV. Aceita main; faz backup; recusa duplicidade e mudanças não reconhecidas. Sem push, novas dependências, backend ou outra branch. O cabeçalho de contexto some junto do cabeçalho no modo de foco do PDV para não ocupar a operação.
5 idiomas, temas existentes e acento limitado ao foco/seleção. Traduções estrangeiras precisam de revisão nativa. Diálogo nativo mantém navegação por teclado.

## Validação
Testes de modelo/contexto, permissão e regressão do cliente; TypeScript e build isolado. Demonstração no Firefox/servidor do usuário continua necessária. Teste HTTP não prova isolamento de banco. Não houve migração ou teste de duas empresas em PostgreSQL real nesta etapa.
Referência SAT SP: https://portal.fazenda.sp.gov.br/servicos/nfce — NFC-e obrigatória no varejo paulista desde 01/01/2026. Trata SAT como legado, sem nova emissão.
