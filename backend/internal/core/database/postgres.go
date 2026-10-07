package database

import (
	"context"
	"gorm.io/gorm/logger"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	// DB é a instância global do banco de dados (GORM) compartilhada por todo o sistema.
	DB *gorm.DB

	// Pool é a conexão pool aberta via pgxpool para manipulação direta de alta concorrência.
	Pool *pgxpool.Pool

	once    sync.Once
	initErr error
)

/*
POSTGRESQL CONNECTION (Produção)
================================
Ponto de conexão mestre do sistema para dados persistentes.

Regras de Segurança e Infraestrutura (DevSecOps):
1. [INJEÇÃO DE SQL]: Proibido o uso de concatenação de strings para queries.
   Sempre utilize "Prepared Statements" (bind variables) ou um ORM seguro como GORM/SQLX.
2. [CONNECTION POOLING & HIGH CONCURRENCY]:
   - O sistema de PDV (Ponto de Venda) opera com alta concorrência e transações simultâneas de checkout.
   - Para suportar este volume sem exaustão de sockets, o uso de "Connection Pooling" nativo via 'pgxpool' é OBRIGATÓRIO.
3. [SSL/TLS]: Forçar conexão criptografada (sslmode=verify-full em produção).
*/

// InitDB inicializa a conexão com o PostgreSQL para manter a compatibilidade com main.go
func InitDB() error {
	once.Do(func() {
		pool, err := ConnectDB()
		if err != nil {
			initErr = err
			return
		}

		// Conecta o GORM usando o driver postgres sob o pool de conexões pgxpool existente
		dbSQL := stdlib.OpenDBFromPool(pool)
		gormDB, err := gorm.Open(postgres.New(postgres.Config{
			Conn: dbSQL,
		}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			dbSQL.Close()
			pool.Close()
			initErr = ErrUnavailable
			return
		}
		Pool = pool
		DB = gormDB
	})
	return initErr
}

// ConnectDB estabelece e configura o pool de conexões pgxpool
func ConnectDB() (*pgxpool.Pool, error) {
	config, err := Configuration(os.Getenv)
	if err != nil {
		return nil, err
	}

	// Estabelece a conexão
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, ErrUnavailable
	}

	// Verifica a conectividade pingando o banco
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, ErrUnavailable
	}

	return pool, nil
}
