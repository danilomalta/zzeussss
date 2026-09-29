# TitanSystem (Monorepo)

O **TitanSystem** é um ecossistema de **PDV (POS) multiplataforma** organizado como monorepo.

## Pilares

- **Backend (Go)**: API Fiber com PostgreSQL via pgx e GORM.
- **Frontend Web (React)**: dashboard/gestão e UI principal (base para o Desktop).
- **Desktop (Electron)**: estrutura existente; funcionamento completo ainda não verificado.
- **Mobile (React Native + Expo)**: estrutura existente; vendas offline ainda não implementadas.

## Onde está o código

O código ativo fica na raiz do repositório:

- `backend/`
- `frontend-web/`
- `desktop/`
- `mobile/`

## Documentação por pasta

Cada pasta do monorepo possui um `README.md` explicando:

- o propósito da pasta
- como ela se conecta ao restante do sistema
- o que deve ser implementado ali


A árvore `TitanSystem/` está preservada para inventário e comparação; Compose e CI usam as pastas da raiz. O modo offline e a venda pelo smartphone ainda são planejados.
