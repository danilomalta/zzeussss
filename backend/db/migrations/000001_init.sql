-- Migração Inicial: Criação de Tabelas Críticas do TitanSystem
-- Banco de Dados Homologado: PostgreSQL (13+)
-- -------------------------------------------------------------

-- Habilita extensão necessária para geração opcional de UUIDs legados se necessário,
-- embora gen_random_uuid() seja built-in a partir da versão 13 do PostgreSQL.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. TABELA DE CLIENTES (Tenants / Empresas do ecossistema)
CREATE TABLE IF NOT EXISTS clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cnpj VARCHAR(18) NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Índices de otimização de busca rápida de clientes por CNPJ e Status
CREATE INDEX IF NOT EXISTS idx_clients_cnpj ON clients(cnpj);
CREATE INDEX IF NOT EXISTS idx_clients_status ON clients(status);

-- 2. TABELA DE USUÁRIOS (Operadores, administradores e gerentes)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Chave Estrangeira com deleção em cascata (caso a empresa dona seja ejetada)
    CONSTRAINT fk_users_client FOREIGN KEY (client_id) 
        REFERENCES clients(id) ON DELETE CASCADE
);

-- Índices para busca rápida de e-mail e login do operador
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_client_id ON users(client_id);

-- 3. TABELA DE PRODUTOS (Catálogo / Estoque dos clientes)
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL CHECK (price >= 0),
    stock INTEGER NOT NULL DEFAULT 0 CHECK (stock >= 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Chave Estrangeira com deleção em cascata
    CONSTRAINT fk_products_client FOREIGN KEY (client_id) 
        REFERENCES clients(id) ON DELETE CASCADE
);

-- Índices para velocidade em busca de catálogo de produtos por inquilino
CREATE INDEX IF NOT EXISTS idx_products_client_id ON products(client_id);
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
