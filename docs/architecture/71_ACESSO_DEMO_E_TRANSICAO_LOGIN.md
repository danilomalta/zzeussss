# Demonstracao completa, painel tecnico e transicao de acesso

Base de aplicacao: bc6a097.

## Perfis de demonstracao

O perfil RH assina apenas Staff e Core. A restricao de menu e correta. Ser dono nao equivale a contratar todos os modulos.

`python3 -B tools/demo_pdv.py --profile completo --api-port 8185 --web-port 3004`

Cria outra instalacao descartavel para testes, com emissor proprio, senha escolhida no terminal e contrato de dois dias. Seleciona POS, Orders, Logistics, Finance, Fiscal, Accounting, Staff e Production com suas dependencias. Preserva instalacoes existentes e nao modifica licencas de clientes. Cada execucao gera outro ID de dono; usar o ID impresso nessa execucao e a senha escolhida nela.

O menu completo nao implementa funcao pendente. Ponto, producao, frota e demais tiles em preparacao continuam explicitamente indisponiveis. Comparador ainda nao tem capacidade propria integrada. Aprovar reposicao pela interface continua sendo a proxima entrega.

## Painel tecnico privado

`python3 -B tools/titan_diagnostics.py --open`

Executa os testes do codigo no computador e tenta abrir o relatorio HTML gerado no navegador. Se nao houver navegador disponivel, imprime o endereco local do arquivo. Nao usa conta de cliente, nao conecta bancos comerciais nem habilita administracao remota. Este e o painel inicial de testes/anotacoes; portal administrativo autenticado e telemetria autorizada ainda nao foram implementados.

## Transicao de login e cadastro

A navegacao Entrar/Criar conta usa viewTransition do roteador. Fade de 240ms, somente opacidade, sem escala, mudanca de largura ou altura, animacao continua ou atraso artificial na autenticacao. As dimensoes originais distintas de cada tela permanecem. Navegadores sem suporte mantem navegacao normal e, quando o CSS nao suporta View Transitions, efeito de entrada por opacidade. Respeita prefers-reduced-motion.

Validar build, testes locais do frontend, testes Python e git diff --check. Aparencia e suavidade precisam ser conferidas no navegador do usuario.
