# TitanSystem: Telas Mobile (App Stores)

**O que você vai codar aqui**: 
Componentes puros criados especificamente para gestos de toque (celular e tablet). Para garçons (lançar comandas, verificar liberação de pedidos da cozinha) e pro dono (App de Relatórios "Olho de Deus" minimalista).

**Linguagem**: React Native (TypeScript) + Expo / Flutter.

**Processo e Regras**:
1. Uso de bibliotecas de navegação rápida com feedback tátil.
2. Cada visão tem que funcionar e mostrar dados mesclando Zustand e SQLite nativo mobile para não dar erro "rede não encontrada".

**Barreira de Segurança Contras Vazamentos Nível Militar Mobile**:
* **Descompilação**: Se aplicarem engenharia reversa no APK do App, eles NÃO acharão Strings secretas das credenciais de banco. A autenticação é via Token/API Key rotativa. O JWT tem que ser estritamente armazenado via `SecureStore` interno que utiliza criptografia por base no hardware (Keystore/Keychain).
