package database

import (
	"log"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"titansystem-backend/internal/models"
	"titansystem-backend/internal/rewards"
)

var DB *gorm.DB

func InitDB() {
	var err error

	dbPath := os.Getenv("TITAN_DB_PATH")
	if dbPath == "" {
		// Padrão simples para dev: cria/usa o arquivo no diretório de execução.
		dbPath = "titan_pos.db"
	}

	// Conecta no SQLite via GORM.
	//
	// Evolução prevista (logística com coordenadas):
	// - Ativação do SpatiaLite para suporte a colunas e funções geoespaciais (GPS).
	// - Estratégia típica: carregar extensão do SQLite e/ou inicializar pragmas/rotinas
	//   específicas antes de operar com dados geográficos.
	//
	// Evolução prevista (Modo Apagão):
	// - Operação em modo desconectado com fila de gravações locais e sincronização
	//   posterior com o servidor central.
	// - O banco local (SQLite) vira o “diário de bordo” das transações pendentes,
	//   com marcação de estado e resolução de conflitos na sincronização.
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatalf("Erro crítico: falha ao conectar no banco SQLite (%s): %v", dbPath, err)
	}

	// Ajustes básicos de pool para SQLite.
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Erro crítico: falha ao obter handle sql.DB: %v", err)
	}
	// SQLite não lida bem com alta concorrência; manter 1 conexão aberta é o padrão seguro.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	log.Printf("Conexão com SQLite estabelecida com sucesso. db_path=%s", dbPath)

	log.Println("Executando AutoMigrate (schema mínimo do PDV)...")
	if err := DB.AutoMigrate(
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
		&models.Sale{},
		&models.SaleItem{},
		&models.MensagemChat{},
		&models.StatusFunilLogistico{},
		&rewards.Indicacao{},
		&rewards.RecompensaIndicacao{},
		&models.SpedJob{},
		&models.DiscountSuggestion{},
		&models.AuditLog{},
	); err != nil {
		log.Fatalf("Erro crítico: falha ao executar auto-migrations: %v", err)
	}
}

func Migrate(models ...interface{}) {
	err := DB.AutoMigrate(models...)
	if err != nil {
		log.Fatalf("Erro crítico: falha ao executar auto-migrations: %v", err)
	}
	log.Println("Estruturas do banco migradas com sucesso.")
}
