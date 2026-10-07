# Entrega 12 — Contratos HTTP, erros compatíveis e inventário

Base: `fde43f8`. Escopo: backend e documentação; sem migrações, frontend, alterações em dados ou integração da branch de produção.

A biblioteca `internal/apicontract` fornece negociação opcional de erro JSON. APIs local e online a instalam antes dos guardas. O processo online também a instala antes do limitador de login externo ao roteador; a instalação duplicada é reconhecida por estado interno da requisição. CORS online permite o header de negociação. Nenhum identificador de requisição é recebido como dado confiável ou retornado sem validação.

O inventário e mapa OpenAPI registram 69 combinações de método/caminho da base. Os testes usam as rotas realmente montadas pelo Fiber e falham quando a documentação fica diferente. HEAD automático e middleware de grupo não são operações independentes nesse inventário. Isso verifica presença de rota, não autorização por papel: os testes de isolamento anteriores continuam obrigatórios.

Paginação existente foi documentada e seus limites/auth-first foram testados. Não houve uniformização forçada dos payloads de listas. Documentos de payload completos para os demais módulos, padronização dos parsers e paginação de todas as coleções continuam pendentes e devem ser entregues por domínio.

Verificações exigidas: testes completos Go, vet, race dos contratos/rotas, build sem CGO do servidor online e local, JSON OpenAPI e diff. PostgreSQL real não precisa ser reprovisionado nesta entrega: não há SQL novo nem mudança de persistência; a entrega 11 já foi validada no PC. Testes Go sem a URL de banco não comprovam execução de PostgreSQL real.

Aceite manual: chamada anônima ao catálogo com header v1 retorna 401 JSON; a mesma chamada sem header mantém o erro legado; health mantém seu payload. Usar processos já configurados, sem criar outra instância em porta ocupada. Não expor Swagger público nem introduzir leitura de dados na documentação.
