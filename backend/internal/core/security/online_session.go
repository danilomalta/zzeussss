package security

import (
	"context"
	"time"

	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/onlinesessions"
)

func ActiveOnlineSession(user, tenant, role, id string) (bool, error) {
	if !onlinesessions.ValidID(id) {
		return false, nil
	}
	if database.DB == nil {
		return false, onlinesessions.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var active bool
	err := database.DB.WithContext(ctx).Raw(`SELECT EXISTS (
	 SELECT 1 FROM online_sessions s JOIN users u ON u.id=s.user_id AND u.tenant_id=s.tenant_id JOIN tenants t ON t.id=s.tenant_id
	 WHERE u.id=? AND u.tenant_id=? AND u.role=? AND s.id=? AND s.role=u.role
	 AND t.status='active' AND s.revoked_at IS NULL AND s.expires_at > clock_timestamp()
	)`, user, tenant, role, id).Scan(&active).Error
	if err != nil {
		return false, onlinesessions.ErrUnavailable
	}
	return active, nil
}
