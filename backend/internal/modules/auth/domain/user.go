package domain

import (
	"time"
)

// User representa a entidade corporativa do operador do PDV ou administrador do sistema.
// Ele é mapeado de forma estrita para a tabela relacional 'users' do PostgreSQL.
type User struct {
	ID           string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ClientID     string    `gorm:"type:uuid;not null" json:"client_id"`
	Name         string    `gorm:"size:255;not null" json:"name"`
	Email        string    `gorm:"size:255;not null;uniqueIndex" json:"email"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:50;not null" json:"role"`
	CreatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

// TableName especifica o nome correto da tabela relacional mapeada no PostgreSQL
func (User) TableName() string {
	return "users"
}
