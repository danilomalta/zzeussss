package models

import "gorm.io/gorm"

// AuditLog registra eventos relevantes de auditoria e segurança.
// Ele deve capturar ações, tentativas de acesso, alterações críticas e seus metadados.
type AuditLog struct {
	gorm.Model

	// Quem realizou (quando houver contexto autenticado).
	UsuarioID *uint   `json:"usuario_id" gorm:"index"`
	Perfil    string `json:"perfil" gorm:"index"` // Dono, Vendedor, Cliente, Contador

	// O que aconteceu.
	Acao    string `json:"acao" gorm:"index"`    // Ex.: "login_falho", "criar_produto"
	Recurso string `json:"recurso" gorm:"index"` // Ex.: "auth", "produtos", "pedidos"

	// Contexto técnico.
	EnderecoIP string `json:"endereco_ip" gorm:"index"`
	Agente     string `json:"agente"` // Cabeçalho User-Agent

	// Resultado e detalhes.
	Sucesso  bool   `json:"sucesso" gorm:"index"`
	Detalhes string `json:"detalhes" gorm:"type:text"`

	// Correlação entre eventos (útil para investigações).
	IDCorrelacao string `json:"id_correlacao" gorm:"index"`
}

