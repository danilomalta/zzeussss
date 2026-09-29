package security

import (
	"errors"
	"strings"

	"titansystem-backend/internal/core/database"
)

// ActiveSession verifica o vínculo legado de uma única empresa em PostgreSQL.
// Consulta a base da API; não substitui a política local SQLite.
func ActiveSession(userID, tenantID, role string) (bool, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(role) == "" {
		return false, nil
	}
	if database.DB == nil {
		return false, errors.New("base PostgreSQL indisponível")
	}
	var active bool
	err := database.DB.Raw(`SELECT EXISTS (
		SELECT 1 FROM users u JOIN tenants t ON t.id = u.tenant_id
		WHERE u.id = ? AND u.tenant_id = ? AND u.role = ? AND t.status = 'active'
	)`, userID, tenantID, role).Scan(&active).Error
	if err != nil {
		return false, err
	}
	return active, nil
}
