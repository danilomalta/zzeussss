package database

/*
POSTGRESQL CONNECTION (Produção)
================================
Ponto de conexão mestre do sistema para dados persistentes.
Substitui a ideia de SQLite (focado em Edge/Mobile/Dev) por um banco relacional massivo.

Regras de Segurança e Infraestrutura (DevSecOps):
1. [INJEÇÃO DE SQL]: Proibido o uso de concatenação de strings para queries (Ex: `WHERE id = ` + user_id).
   Sempre utilize "Prepared Statements" (bind variables como $1, $2) ou um ORM seguro como GORM/SQLX.
2. [CONNECTION POOLING]: Deve configurar limites estritos de `MaxOpenConns` e `MaxIdleConns`
   para prevenir vazamento de recursos e ataques de exaustão de conexões no banco.
3. [SSL/TLS]: Forçar conexão criptografada (sslmode=verify-full em produção).
*/

// DBConfig armazena as chaves extraídas do cofre de senhas (.env seguro)
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DatabaseManager define a ponte para o SQL de forma genérica para permitir testes mockados
type DatabaseManager interface {
	Connect(config DBConfig) error
	Close() error
	GetPoolStats() map[string]interface{}
}
