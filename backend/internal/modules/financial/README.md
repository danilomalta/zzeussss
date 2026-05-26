# Módulo Financial (`internal/modules/financial/`)

Responsável pela gestão financeira, contas a pagar/receber, emissão e assinatura de Notas Fiscais Eletrônicas (NF-e) e conformidade contábil (geração de arquivos SPED).

## Arquitetura DDD

- **`domain/`**: Entidades de `SPEDJob` e faturas, regras tributárias para NF-e.
- **`usecase/`**: Geração assíncrona de SPED e assinatura/emissão digital de NF-e (`nfe_emitter.go`).
- **`repository/`**: Busca e persistência de jobs fiscais e faturas.
- **`delivery/`**: Endpoints contábeis e fiscais (`/accounting/sped/*`).
