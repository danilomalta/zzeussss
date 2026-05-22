# TitanSystem (Monorepo)

O **TitanSystem** é um ecossistema de **PDV (POS) multiplataforma** organizado como monorepo.

## Pilares

- **Backend (Go)**: API HTTP com **Fiber** + persistência com **GORM** em **PostgreSQL** (Banco de dados oficial de alta concorrência).
- **Frontend Web (React)**: dashboard/gestão e UI principal (base para o Desktop).
- **Desktop (Electron)**: empacota o frontend web para Windows/Linux/macOS.
- **Mobile (React Native + Expo)**: operação em campo (garçons/atendentes) e dispositivos móveis.

## Onde está o código

O código do monorepo fica dentro de `TitanSystem/`:

- `TitanSystem/backend/`
- `TitanSystem/frontend/`
- `TitanSystem/desktop/`
- `TitanSystem/mobile/`

## Documentação por pasta

Cada pasta do monorepo possui um `README.md` explicando:

- o propósito da pasta
- como ela se conecta ao restante do sistema
- o que deve ser implementado ali

