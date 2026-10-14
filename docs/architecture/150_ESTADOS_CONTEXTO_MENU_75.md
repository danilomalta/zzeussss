# Entrega75 — Estados honestos de contexto e menu

## Cinco melhorias
1. Cabeçalho sem seletores falsos. Os nomes reais não existem no contrato atual, portanto mostra Nome da empresa/loja indisponível. Não inventa cadastro, CNPJ, lista de lojas ou troca de sessão. O painel explica esse limite.
2. Estados verificando, seleção necessária, acesso negado e sessão verificada. ContextGate75 não renderiza os módulos quando o contexto não está confirmado. SessionGuards e autorização do servidor continuam obrigatórios. A camada LocalAccess já impede entrar se a resposta de capacidades for inválida; nesses casos a tela de acesso pode aparecer antes do gate novo.
3. Notificações retiradas do cabeçalho até existir fonte de eventos reais. Ajuda mantida em diálogo com ESC/X/foco. Nenhuma notificação real foi implementada neste pacote.
4. Ícones SVG diferentes por área e PDV destacado, tipografia 15–16px no menu e 13px em selos. A legenda esclarece estados. Área local é âmbar, estado indisponível é vermelho suave, futura é cinza tracejado, validação pendente é neutra. Cor não é única informação. Operacional é verde somente na legenda; nenhum card recebe esse selo automaticamente.
5. Integrações mostra Estado indisponível no card. Não afirma nenhuma conectada nem conta falhas sem fonte real. O teste HTTP já existente continua distinto de uma integração operacional.

## Estados de menu
- Arquivo local: planilhas e ferramentas /local/workbench quando disponíveis; persistência real depende do arquivo/armazenamento descrito em suas entregas.
- Interface: RH, mapa e visão executiva; esse rótulo não certifica os subfluxos.
- Em breve: área planejada sem rota operacional.
- Estado indisponível: centro de integrações sem status dos provedores conectado ao menu.
- Validação pendente: áreas com integração existente cujo fluxo, reinício, isolamento, permissões, auditoria e restauração não foram certificados por uma fonte de evidências nesta etapa.
- Navegação: configurações e outros atalhos de navegação.
- Operacional: reservado para evidências verificadas; não aparece em card por mera existência de uma rota.

## Bloqueio de contexto
Mantém dimensões empresa, loja, usuário e aparelho e expiração da sessão. Ausência de empresa/loja => seleção necessária; capacidades carregando => verificando; divergência/expiração/identidade inválida => negado. Apenas o estado confirmado libera Outlet. Não cria endpoint para escolher a loja: obter outro contexto exige autorização e sessão do servidor.
Selecionar duas lojas e demonstrar alterações reais no Estoque/Caixa/Financeiro ainda depende de backend que liste vínculos e permita troca autorizada. Não houve esse teste e não se anuncia multiempresa operacional.
Não modifica tabelas, consultas, migrações, certificados ou dados comerciais. O isolamento no navegador não substitui autorização no servidor.

## Instalação
Compatível com menu72, 73 e 74 conhecido. Inclui os arquivos de contexto74 que faltarem, preserva os que já forem idênticos e modifica somente o cabeçalho/teste indicados. Inserções localizadas no workspace/router/menu. Nunca reescreve router inteiro. Preserva telas/rotas de RH, PDV, planilhas e workbench.
Dados não são copiados para outra empresa; não importa dados, não gera permissões, não faz push e não executa branch de produção. Backup e testes/build antes de commit. Arquivos alterados não reconhecidos são preservados, com parada antes das escritas.

## Validação e limites
153 testes Node disponíveis passaram, incluindo 8 novos testes de estados e UI, além dos testes de contexto74 e regressões. TypeScript passou. Build isolado das peças novas. Testes do instalador conferem preservação de rotas e bloqueio de duplicidade.
Sem navegador instalado para captura/demonstração visual real nesta execução. Não há captura anunciada como 100%. É necessário conferir Firefox em zoom 100% no computador do usuário, além de 50% e telas menores. Build não substitui essa demonstração.
5 idiomas completos para os textos novos, revisão nativa pendente para idiomas estrangeiros. Sem novas dependências; paleta neutra e estados de cor. SVGs usam currentColor, sem logo dourado.
