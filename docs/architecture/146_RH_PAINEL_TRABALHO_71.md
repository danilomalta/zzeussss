# Entrega 71 — painel de trabalho de RH

## Implementação

A rota `/local/hr` deixa de abrir um índice com formulários de rascunhos voláteis. A visão geral passa a reunir busca, seis indicadores, equipe, alertas, quatro áreas prioritárias e outras áreas recolhidas. O botão de cadastro leva ao formulário existente `/local/staff#staff-form`. Só há um botão visível Voltar ao menu, no cabeçalho global; o logo mantém sua navegação existente ao menu.

A equipe usa o cliente existente `localClient.staff`, com `/local/v1/staff`. Este contrato fornece somente identidade interna, nome, perfil de acesso e status de acesso. A interface não inventa cargo, setor, aniversário, vencimento, presença ou vínculo trabalhista. `active`/`revoked` aparecem como Acesso ativo/Acesso revogado, não como presença ou férias. As outras informações aparecem como “—”, com explicação. Busca e filtros operam sobre a lista recebida, paginada no cliente em 25 linhas. A ordenação pode usar nome, cargo ou setor quando fornecidos. Mais de 10.000 registros geram erro de fonte; não são truncados e apresentados como total.

As oito fontes têm contratos e adaptadores separados: funcionários/acessos, ponto/jornada, escalas, férias/afastamentos, documentos/marcos, totais de folha, custo da folha e alertas. Sem contrato conhecido, retornam indisponível. Nenhuma URL nova foi inventada. Cada fonte resolve separadamente e uma falha não impede as demais. Atualização ao abrir e a cada 1, 2, 5 ou 10 minutos enquanto a aba estiver visível; período padrão de cinco minutos. Há atualização manual e horário de atualização da equipe.

Os indicadores só exibem números conhecidos. Na operação real, ponto, faltas, férias, contratos de experiência, aniversários e aprovações permanecem como “—” até seus adaptadores terem uma fonte autorizada. Botões indisponíveis não levam a uma página fictícia. Na demonstração, o clique filtra a lista correspondente e mostra um breadcrumb RH › indicador.

## Cinco áreas de entrega

1. Visão de trabalho: cabeçalho, ações, busca Ctrl+K ou `/`, indicadores úteis e painel lateral de alertas.
2. Equipe: nomes/iniciais, campos conhecidos, filtros, ordenação, seleção, paginação, ficha de leitura e CSV/XLSX.
3. Navegação: cartões prioritários e outras áreas recolhidas, estados claros, sem duplicidade de abas/cartões e sem formulários sem persistência.
4. Integração: oito fontes independentes, atualização, estados vazio/erro/sem permissão/não contratado/indisponível, demonstração opcional e integração condicional com as planilhas70.
5. Proteção e apresentação: permissões existentes, ausência de caches pessoais novos, proteção de saída no cadastro existente, temas neutros, responsividade e cinco idiomas.

## Permissões e dados sensíveis

- A entrada respeita `allowedAreas` e a licença `staff` com `manage_staff`. Não há papel `rh` nos contratos atuais. Uma conta de gestão com `manage_staff` representa a gestão operacional de RH nesta interface; não recebe acesso salarial por inferência.
- Funcionário sem essa permissão retorna ao menu, sem buscar a equipe. Nenhuma permissão de backend foi alterada.
- Folha e benefícios são ocultados da gestão comum. Totais salariais e custo só aparecem para titular autorizado a entrar no RH, e custos também respeitam o módulo financeiro/contábil. Até haver uma permissão específica de salários, não se atribui esse acesso a qualquer gerente que possa criar usuários.
- Para RH com acesso a salários, o backend precisa de autorização explícita por campo/fonte, como leitura da folha e benefícios sensíveis. Esses nomes são propostas de contrato, não permissões criadas nesta entrega.
- Dados carregados pelo novo painel ficam somente na memória da sessão, sem IndexedDB, localStorage, logs ou persistência salarial. Logout limpa o estado e respostas atrasadas não são aplicadas a outra sessão. Exportações são ações explícitas do usuário, com nova conferência de sessão ao concluir a geração.
- Não se apagou o mecanismo legado de cadastro pendente de `/local/staff`: ele conserva metadados para recuperar uma operação idempotente e não armazena senha. Esta entrega não afirma que todo o aplicativo foi auditado quanto a caches; a nova visão de RH não acrescenta cache pessoal.

## Áreas em breve

Ponto e jornada; férias e afastamentos; escalas e turnos; recrutamento; admissão e desligamento; folha e remuneração; benefícios; obrigações/documentos; desempenho; relatórios; portal da equipe; segurança/auditoria. Não há rota de trabalho validada para essas áreas nos arquivos inspecionados; por isso os cartões são não clicáveis. Funcionários e acessos continua ativo na rota existente.

Registrar ocorrência, solicitar documento e enviar comunicado estão desabilitados. Importação por planilha está desabilitada, mas o modelo XLSX pode ser baixado no estado vazio e na orientação de primeiros passos. Não há formulário que aceite informações como se tivesse salvado sem persistência. A ordenação por arrastar dos cartões não foi incluída, por ser opcional. Preferências de busca, filtros e intervalo são da sessão, sem dados de funcionários gravados no navegador.

A confirmação de saída em `/local/staff` cobre nome/senha ainda não enviados e operação em andamento. Um cadastro já registrado como pendente continua com sua recuperação existente. A visão geral, sem formulários de edição empresarial, não exibe confirmação ao alterar apenas filtros.

## Planilhas

O adaptador `staffSheetLink` detecta a presença do cliente da entrega70 e a permissão/área de planilhas. Sem ele, fica desabilitado. Quando habilitado, abre `/local/sheets?source=staff`, sem inserir nomes pessoais na URL. Abre a fonte completa; para a seleção e os filtros desta tabela, o usuário exporta CSV/XLSX. A demonstração não transfere dados fictícios para uma planilha que consultaria o servidor real. O cliente da entrega69, sozinho, não é anunciado como integração de dados.

## Demonstração

A flag `VITE_HR_MOCKS=true` habilita dados fictícios, identificados em destaque. Sem essa flag, nenhum zero é inventado e só a equipe usa o cliente real. `VITE_HR_MOCK_EMPTY=true` mostra a equipe vazia somente quando a demonstração também está habilitada. As flags nunca concedem permissões nem substituem uma licença válida. O cadastro existente continua sendo um formulário conectado ao backend; o painel de demonstração não transforma o restante do sistema em sandbox.

## Contratos/endpoint necessários (propostos, não implementados)

| Área | Contrato necessário |
|---|---|
| Indicadores | snapshot do dia/semana e timezone da loja; presentes, faltas/atrasos, afastados, experiência em 30 dias, aniversários, aprovações; contagem, IDs autorizados, comparação e indicação de completude |
| Equipe e busca | lista paginada e busca autorizada por pessoa, cargo, setor, status trabalhista e documentos; identificação separada de usuário e vínculo empregatício; imagem autorizada opcional; controle de campos sensíveis |
| Ponto/jornada | registros e justificativas, perfil próprio do funcionário, resumo e lista filtrada, rotas de trabalho e autorização para registros/ajustes |
| Escalas | jornada, turnos, pendências, alterações com revisão e auditoria |
| Férias/afastamentos | períodos, solicitações, aprovações, saldo e vencimentos; dados médicos restritos |
| Documentos/marcos | experiência, exames e vencimentos; acesso restrito a documentos e trilha de consulta; aniversários apenas quando permitidos |
| Folha e financeiro | totais em centavos, competência, visão autorizada por campo, permissão salarial explícita e resumo de custo; sem cálculo ou pagamento inferidos no cliente |
| Alertas | lista priorizada, pessoa, motivo, vencimento, severidade, completude e ações autorizadas com recibo |
| Ocorrências/comunicados | gravação idempotente, validação, revisão esperada e resultado recuperável; envio explícito, sem disparos automáticos nesta entrega |
| Importação | modelo versionado, prévia e mapeamento no servidor, erros por linha, confirmação explícita, aplicação atômica/idempotente, recibo e recuperação; convites sem senha na planilha |
| Preferências | ordem de cartões e intervalo por empresa/usuário; versão e persistência somente de preferências não pessoais |
| Portal do funcionário | endpoints de dados próprios, folha/ponto pessoal e solicitações; não expor a lista completa da equipe a um perfil comum |

A empresa, loja e permissões devem ser derivadas e verificadas pelo backend a partir da sessão. Frontend não é barreira de segurança.

## Idiomas, acessibilidade e verificação

pt-BR, en, es, fr e zh-CN têm o mesmo conjunto de chaves novas. Chaves novas sem tradução: nenhuma. en/es/fr/zh-CN estão marcados para revisão por falante nativo. Nomes e dados fornecidos pela empresa não são traduzidos artificialmente. Datas, contagens e valores usam Intl. Só os novos textos e a nova confirmação de saída foram cobertos; não se anunciou uma retradução completa de telas legadas.

Tema claro/escuro, fonte de 13px em auxiliares e 14–16px nos dados, foco âmbar, ícones com texto nos status, áreas clicáveis de 44px, redução de movimento e tabela em cartões no celular. Não há cores azuis novas. Menu global e logo foram reutilizados, sem alteração da identidade.

Verificação: TypeScript passou; 122 testes Node da base disponível passaram, incluindo 10 específicos de RH. Ensaio isolado Chromium com respostas HTTP controladas, sem dados comerciais: titular/gestão/funcionário, fonte real do cliente `/staff`, demonstração, vazio, seis indicadores, Ctrl+K, busca, CSV/XLSX, temas, cinco idiomas, larguras 390/768/1440/2880 e limpeza no logout. O roteador real foi utilizado para validar a confirmação de saída do cadastro. Largura 2880 simula espaço CSS equivalente à ampliação de área em 50%; não certifica todo navegador a todo nível de zoom. Firefox e integração com dados empresariais precisam de conferência no computador do usuário.

O instalador confere os dois arquivos-base, preserva outras entregas, configuração npm e backend, faz backup e roda o build e todos os testes `.test.mjs` do frontend real antes do commit. Não realiza push nem executa a branch de produção.
