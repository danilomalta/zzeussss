# Perfil da empresa e revisão funcional — entrega87

## Correção entregue

- Cadastro da proposta e Configurações usam o mesmo editor de perfil: atividade, setores e recursos opcionais. Mercado, farmácia, alimentação, indústria e outro negócio não são tratados como restaurante por padrão.
- Configurações → Perfil da empresa é a entrada de manutenção. A rota anterior `/local/sectors` é preservada como destino desta configuração; deixou de ser mais um cartão de operação no menu.
- Receitas e Entregas abrem na área de trabalho. Nenhum módulo força o questionário de atividade. Alimentação não configurada orienta para Configurações, sem assumir cozinha/mesas. Recursos explicitamente desativados respeitam essa escolha.
- A configuração local é única por empresa/loja neste computador, compartilhada entre os usuários locais desse contexto. Leituras de outra empresa ou loja não reutilizam o perfil anterior. Alterações simultâneas usam revisão otimista; uma gravação desatualizada é recusada.
- Dados antigos de86 são preservados. O responsável abre Configurações, revisa o perfil existente e salva uma vez para torná-lo comum. Não há migração automática de dados comerciais nem ativação de plano.
- A proposta anônima guarda somente o perfil, em sessionStorage. Ela não é associada automaticamente a uma empresa autenticada. Aplicar exige entrar na empresa correta, abrir Configurações, escolher Usar perfil da proposta, revisar e confirmar. Nome/email não são copiados para esse armazenamento.
- Acesso continua condicionado à licença e às permissões originais. Escolher produção no perfil não concede `manage_production`, escolher RH não concede acesso a salários.
- Removida a barra que misturava Receitas, Entregas e Alimentação como abas da mesma operação. Cada rota mantém suas ferramentas internas.

## Estado auditado no código, sem equivalência a demonstração operacional

| Área | Presente na interface/código | Ainda falta |
|---|---|---|
| RH: funcionários/acessos | Lista pelo cliente local, busca, filtros disponíveis, perfis e estados | Validar fluxo integrado completo de admissão, cargo, setor e contrato |
| RH: ponto, jornada, escalas, férias | Adaptadores e estados indisponíveis; áreas registradas | APIs e persistência operacional; telas completas de cada fluxo |
| RH: folha, benefícios, obrigações | Áreas e restrições de acesso | Cálculos oficiais, integrações, documentos e fechamento |
| RH: recrutamento, desempenho, portal | Áreas previstas | Fluxos completos e integrações |
| Planilhas: edição | Univer com linhas/colunas, fórmulas, filtros, ordenação, congelar, formatação condicional, validação, localizar/substituir | Revisão visual e compatibilidade em hardware real; não é equivalência completa ao Excel |
| Planilhas: arquivos | Arquivo Titan, XLSX/CSV, modelos, IndexedDB, versões e impressão | Fidelidade de todos os arquivos Excel, servidor de arquivos/versões, colaboração e permissões remotas |
| Planilhas: fontes | Adaptadores reais de catálogo, vendas e funcionários, sujeitos às permissões e limites | Estoque, compras e financeiro dependem de endpoints; exemplos são sinalizados pela flag |
| Planilhas: tabela dinâmica | Botão indisponível | Implementação e testes |
| Planilhas: enviar/compartilhar | Camada de preparação/estado de indisponibilidade | Escritas reais em lote, compartilhamento e auditoria no servidor |
| Receitas/produção | Receita, ingredientes, capacidade, reserva e consumo do planejamento local | Integrar saldos, lotes, ordens, perdas e movimentos oficiais do estoque |
| Entregas | Fila genérica, entregador, etapas e comprovação local | Comunicação real, GPS, notificações, sincronização e baixa financeira |
| Cadastro | Proposta de plano e perfil de negócio | Provisionamento da empresa e gravação do perfil pelo backend |

Não adicionar cartões de funções inexistentes como se fossem operacionais. As áreas pendentes devem entrar dentro dos módulos pais, com rota de trabalho e evidência de persistência, permissão e auditoria antes do estado Ativo.

## Organização recomendada das próximas telas

1. RH: ficha do funcionário; ponto/jornada; escalas; férias/afastamentos; admissão/desligamento; recrutamento; folha/benefícios; documentos/obrigações; desempenho; portal; relatórios; segurança. Funcionários/acessos continuam separados da folha e dos dados trabalhistas.
2. Planilhas: manter Arquivo, Inserir, Dados, Fórmulas, Formatar, Revisão e Impressão dentro do editor; não adicionar um cartão ao menu principal para cada ferramenta. Priorizar tabela dinâmica, fidelidade XLSX e conexão real das fontes já registradas.
3. Compras/comercial, fiscal/financeiro, logística e produção mantêm módulos pais. Recebimento, conferência, devolução, cotações e aprovações são fluxos internos.

## Contratos necessários, não endpoints implementados

- Perfil autenticado da empresa/loja: leitura, revisão/versionamento, escrita autorizada e trilha de quem alterou; validação do perfil na conclusão do cadastro e retorno na sessão/contexto.
- Catálogo de módulos/recursos da empresa: distinguir contratado, autorizado, configurado, integrado e indisponível. Nenhum campo do navegador substitui esse contrato.
- RH: cadastro trabalhista, indicadores do dia, ponto/jornada, escalas, férias/afastamentos, documentos, pendências, folha, exportação/importação e auditoria, sempre com empresa/loja/perfil.
- Planilhas: arquivos, versões, ACL/compartilhamento, consultas de estoque/compras/financeiro e importações validadas com prévia, confirmação, idempotência e resultado por linha.
- Preferências entre dispositivos: sincronização com revisão, sem espalhar dados pessoais/salariais por armazenamento de configuração.

## Limites e verificação

Nenhum endpoint foi inventado ou chamado. Nenhum backend, banco comercial, emissão fiscal, pagamento ou plano foi alterado. Sem nova dependência. A configuração comum é local neste computador; não promete comunicação entre PCs/celulares.

Testes da entrega: regras, isolamento por empresa/loja, compartilhamento local por contexto, persistência, proposta sem associação automática, revisão concorrente, navegação sem questionário, restrição de funcionário e cinco idiomas. Build TypeScript/Vite e diff-check. Testes DOM virtual não substituem avaliação visual em navegador ou teste em equipamento real. O bundle grande de Planilhas permanece uma pendência de desempenho.

Os cinco idiomas têm as mesmas chaves novas; en/es/fr/zh-CN exigem revisão por falante nativo.
