package database

import (
	"context"
	"fmt"
	"log"
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

	once sync.Once
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
func InitDB() {
	once.Do(func() {
		pool, err := ConnectDB()
		if err != nil {
			log.Fatalf("Erro crítico: falha ao inicializar conexão PostgreSQL: %v", err)
		}
		Pool = pool

		// Conecta o GORM usando o driver postgres sob o pool de conexões pgxpool existente
		dbSQL := stdlib.OpenDBFromPool(pool)
		gormDB, err := gorm.Open(postgres.New(postgres.Config{
			Conn: dbSQL,
		}), &gorm.Config{})
		if err != nil {
			log.Fatalf("Erro crítico: falha ao conectar GORM com o pool pgxpool: %v", err)
		}
		DB = gormDB
		log.Println("PostgreSQL inicializado com sucesso via pgxpool e GORM.")
	})
}

// ConnectDB estabelece e configura o pool de conexões pgxpool
func ConnectDB() (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Fallback para construir a partir de variáveis de ambiente individuais se DATABASE_URL não estiver definida
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("DB_USER")
		if user == "" {
			user = "titan"
		}
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			password = "titanpass"
		}
		dbname := os.Getenv("DB_NAME")
		if dbname == "" {
			dbname = "titansystem"
		}
		sslmode := os.Getenv("DB_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}

		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			user, password, host, port, dbname, sslmode)
	}

	// Configurando o pool pgxpool
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Erro crítico: falha ao analisar string de conexão (DSN): %v", err)
		return nil, err
	}

	// Configurações do Pool de Alta Concorrência para PDV
	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnIdleTime = 15 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour
	config.HealthCheckPeriod = 30 * time.Second

	// Estabelece a conexão
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("Erro crítico: falha ao criar pool de conexões pgxpool: %v", err)
		return nil, err
	}

	// Verifica a conectividade pingando o banco
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		log.Fatalf("Erro crítico: falha ao pingar banco de dados PostgreSQL: %v", err)
		return nil, err
	}

	return pool, nil
}
