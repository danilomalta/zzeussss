# Módulos, perfis e cadastro comercial

## Separação obrigatória
Porte/enquadramento da empresa (MEI, micro etc.), atividade (varejo, produção, RH ou contabilidade), módulos contratados e perfil individual são conceitos separados.
O tipo de atividade sugere a seleção inicial de módulos, mas não concede direitos. Somente o contrato assinado confirma módulos; permissões vêm da sessão, vínculo, loja e dispositivo.
Produção não exige PDV. RH não concede acesso indiscriminado a qualquer funcionário. Ponto pessoal é área distinta da gestão de pessoas.

## Implementado nesta entrega
- GET /local/v1/capabilities, autenticado, deriva contexto da sessão e revalida vínculo/aparelho em transação. Retorna permissões reais e metadados do contrato verificado.
- StatusTx autentica assinatura/claims para exibir metadados inclusive de contrato expirado. Verificação em instante interno ao intervalo só autentica metadados; nunca autoriza operações. RequireTx permanece obrigatório nas escritas.
- Menu e navegação local por módulo e perfil; acesso direto a rotas incompatíveis redireciona à área pessoal. Esta guarda de interface não substitui segurança das APIs.
- Catálogo e estoque em rotas próprias. Operador de caixa não vê os formulários de alteração de estoque.
- Página comercial /register com enquadramento, atividade, armazenamento desejado, quantidade de acessos e seleção de módulos. Active State explícito nos cards e abas; foco suave nos campos.
- Valores mensais definidos pelo dono do projeto: PDV R$350; produção R$2.500; RH R$2.500; contabilidade R$999. Valores de apresentação da promoção: R$3.500 → R$2.500 nas duas opções de R$2.500, R$1.500 → R$999 na contabilidade.
- Esta página prepara configuração, SEM criar conta comercial, cobrança, assinatura ou liberar licença. Os valores ainda não vêm de catálogo comercial protegido no servidor.
- Funcionários locais: GET/POST /staff exigem módulo staff vigente e permissão manage_staff. Dono pode criar perfis existentes não owner; gerente limitado a employee/cashier/stock. Vínculo é criado apenas na loja da sessão. Password bcrypt, criação e auditoria atômicas, identificadores estáveis de repetição. Senhas nunca aparecem nas respostas, registros de auditoria ou armazenamento do navegador.
- Assinatura/faturas: dias restantes da licença assinada e data de expiração reais. Faturamento não conectado é rotulado explicitamente. Ausência de integração NÃO significa ausência de dívida.
- tools/demo_pdv.py --profile varejo|producao|rh|contabilidade prepara contrato específico em instalação descartável nova. RH/contabilidade não semeiam produtos nem consultam caixa. Não muda empresas reais.
- tools/titan_diagnostics.py cria relatório técnico privado (diretório 0700, arquivo 0600) no computador do desenvolvedor, com resultados de testes e anotações locais. Não coleta logs crus ou dados comerciais. Não é monitoramento remoto de clientes nem painel administrativo online.

## Próximas integrações ainda necessárias
1. Delegação auditável do dono ao RH e permissões individuais configuráveis, com revogação e prevenção de elevação de privilégio. Nesta etapa continuam os perfis fixos existentes, sem papel RH novo ou autonomia irrestrita do gerente.
2. Cadastro comercial real, limites contratados de usuários, modalidades de armazenamento e serviço de faturamento. Aviso sete dias antes e bloqueio após três dias precisam de vencimento e confirmação confiáveis de pagamento; data da licença não substitui vencimento da fatura.
3. Ponto pessoal, histórico e regras do ponto; esta navegação não marca ponto.
4. Portal administrativo exclusivo do operador Titan, com identidade administrativa separada dos donos de empresas clientes. MFA, trilha de ações, testes isolados e telemetria técnica mínima opt-in; sem documentos, vendas, estoque ou dados de pessoas. Nenhum acesso global foi concedido por esta entrega.
5. Cadastro inicial da empresa deve criar um dono autenticado e emitir licença apenas após autorização comercial, sem confiar em preço/plano/papel enviados pelo navegador.

## Validação e limitações
Build e testes Node/Python no ambiente de preparação. Go indisponível aqui: rodar a suíte completa no PC. Migração 0022 adiciona somente auditoria local de funcionários.
Os menus são consultados ao entrar e pelo botão de conferir licença; mudanças de permissões continuam sendo verificadas em cada API. Contador de vigência atualiza na consulta, sem cronômetro fictício.
Funcionários listados nesta loja até 100 registros; paginação administrativa, edição/revogação e troca segura de senha ainda pendentes.
Nenhuma integração de faturamento ou autenticação de portal Titan foi presumida.
