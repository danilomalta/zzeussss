package database

import (
	"gorm.io/gorm"
)

// DB é a instância global do banco de dados (GORM) compartilhada por todo o sistema.
var DB *gorm.DB

/*
POSTGRESQL CONNECTION (Produção)
================================
Ponto de conexão mestre do sistema para dados persistentes.
Substitui a ideia de SQLite (focado em Edge/Mobile/Dev) por um banco relacional massivo.

Regras de Segurança e Infraestrutura (DevSecOps):
1. [INJEÇÃO DE SQL]: Proibido o uso de concatenação de strings para queries (Ex: `WHERE id = ` + user_id).
   Sempre utilize "Prepared Statements" (bind variables como $1, $2) ou um ORM seguro como GORM/SQLX.
2. [CONNECTION POOLING & HIGH CONCURRENCY]:
   - O sistema de PDV (Ponto de Venda) opera com alta concorrência e transações simultâneas de checkout.
   - Para suportar este volume sem exaustão de sockets, o uso de "Connection Pooling" nativo via 'pgxpool' (do driver pgx/v5) é OBRIGATÓRIO.
   - Recomenda-se configurar os limites de pool da seguinte forma:
     * `MaxConns` (limite máximo de conexões simultâneas): dimensionado com base no limite do PostgreSQL e na capacidade de CPU do container. Evita a falha de "too many clients already".
     * `MinConns` (mínimo de conexões ativas no pool): mantém conexões prontas para mitigar a latência do handshake inicial TCP/TLS (quentes no pool).
     * `MaxConnIdleTime` / `MaxConnLifetime`: evita conexões zumbis que drenam a RAM do servidor de banco de dados.
   - Monitoramento: Deve expor métricas do pool (ex: GetPoolStats) para Prometheus/Grafana a fim de prever gargalos de infraestrutura.
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
