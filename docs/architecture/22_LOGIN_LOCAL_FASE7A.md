# Fase 7A — Identidade humana offline

`localauth` guarda apenas hashes bcrypt das senhas e SHA-256 dos tokens de sessão. O login associa a pessoa à empresa, loja e aparelho já aprovado. A cada chamada, `Resolve` verifica se o vínculo e o aparelho continuam ativos. Trocar senha revoga sessões anteriores. A senha inicial é criada somente durante o provisionamento local explícito da próxima fase.

Ainda não há rota HTTP nem tela de login local. A prova de posse da chave privada do aparelho na inicialização e limites de tentativas no endpoint também são requisitos para ligar a API. Não usar credenciais JWT do PostgreSQL como autorização local.
