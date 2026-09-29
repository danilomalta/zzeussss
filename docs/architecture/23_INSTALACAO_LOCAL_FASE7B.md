# Fase 7B — Provisionamento inicial local

`localsetup.Initialize` só opera em SQLite sem empresa cadastrada. Registra dono, empresa, loja, hash de senha e a chave pública do primeiro aparelho em uma transação. A chave privada é criada e guardada fora do banco pelo instalador, em arquivo local exclusivo, com permissão 0600. Uma instalação repetida é rejeitada sem mexer em dados existentes.

A aprovação raiz existe apenas nessa instalação física inicial e precisa de comando interativo exclusivo; jamais expor `Initialize` em HTTP. Na fase seguinte, o executável local verificará a posse da chave na inicialização com desafio de uso único.
