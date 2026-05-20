# TitanSystem: Frontend Web (Caixa e Retaguarda)

**O que você vai codar aqui**: 
As Views e fluxos da web acessadas via Nuvem pelo dono ou navegador de um PDV mais simples que não queira baixar o desktop app.

**Linguagem**: React e TypeScript, empacotados com Vite, estilizados com Tailwind.

**Processo e Regras**:
1. O design "Titan Prime" dita cores escuras e "Neon". Tudo focado numa experiência premium.
2. Todo Fetch e POST deve usar serviços Axios encapsulados com Headers contendo o Bearer JWT do usuário e o Tenant_ID correto. O Tenant_ID jamais fica exposto cru no .env, pois cada cliente tem o seu.

**Barreira de Segurança Contras Vazamentos**:
* XSS: Jamais use `dangerouslySetInnerHTML`.
* Redirecionamentos: Bloquear parâmetros de URL que redirecionam clientes para sites golpistas caso a loja sofra phishing (`url?redirect=site_fraudulento`).
* `.env`: Frontend Web SÓ DEVE expor vars cujo nome comece com `VITE_` se for EXTREMAMENTE NECESSÁRIO (ex: URL Pública do Backend Go). NUNCA coloque chaves secretas de Strip/Pagar.me no `.env` do front.
