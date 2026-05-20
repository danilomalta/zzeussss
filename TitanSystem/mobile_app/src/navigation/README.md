# TitanSystem: Rotas e Autenticação (Mobile)

**O que você vai codar aqui**: 
Árvore de navegação do React Navigation. (Qual botão leva pra qual tela e validação de token).

**Linguagem**: TypeScript.

**Processo e Regras**:
1. Verificação se há internet; Se houver e o JWT estiver expirado -> Tentar Refresh silencioso.
2. Proteção brutal contra Deep Links invasivos (exemplo: bloquear roteamento de apps maliciosos para a tela de relatórios financeiros sem biometria).

**Barreira de Segurança**:
* Tela de "Relatório" não existe livre na memória; exige Biometria / Senha Master via FaceID / TouchID antes de renderizar e buscar no Backend Go central.
