# Fase 6B — Política local de reposição

Gerente ou dono configura um limite mínimo e um alvo de estoque por loja e produto. `replenishment.SetPolicy` registra versão, identidade do responsável, histórico imutável e evento outbox na mesma transação. Estoquista pode acompanhar pedidos, mas não alterar o limite.

O cálculo de sugestão e a aprovação humana entram nas próximas fases. Alterar uma política não dispara compra e não dá acesso ao fornecedor. IDs de sessão humana/aparelho precisam ser autenticados pela camada local antes de chamar o serviço.
