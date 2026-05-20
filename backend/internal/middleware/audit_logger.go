package middleware

import "time"

/*
AUDIT LOGGER (Trilha de Auditoria Governamental)
================================================
Módulo crítico de responsabilização (Accountability).

Regras de Negócio e Segurança:
1. Registra **TODA E QUALQUER** modificação no estado do sistema (Mutações - POST/PUT/DELETE).
2. O Log não pode ser alterado retroativamente (Append-Only log).
3. Deve capturar: Quem fez (UserID), Quando (Timestamp), O Que (Endpoint/Method) e de Onde (IP/UserAgent).
4. Essencial para investigações de fraudes internas no ERP (Ex: Lojista apagando notas fiscais).
*/

// AuditRecord é a matriz que será guardada (banco SQL secundário, ElasticSearch ou CloudWatch)
type AuditRecord struct {
	RecordID     string
	Timestamp    time.Time
	UserID       string // Preenchido automaticamente pelo AuthGuard
	Action       string // Ex: "DELETE /api/orders/123"
	IPAddress    string
	UserAgent    string
	StatusResult int // Ex: 200, 403, 500
}

// AuditLogger assina os métodos para espionar a rede local e gravar na trilha de fogo
type AuditLogger interface {
	WatchMutations() func(ctx interface{}) error
	PersistLog(record AuditRecord) error
}
