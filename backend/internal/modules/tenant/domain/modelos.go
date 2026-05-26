package domain

import (
	"time"

	"gorm.io/gorm"
)

type StatusRecompensa string

const (
	StatusRecompensaPendente StatusRecompensa = "pendente"
	StatusRecompensaAprovada StatusRecompensa = "aprovada"
	StatusRecompensaEstornada StatusRecompensa = "estornada"
)

// Indicacao representa um vínculo “quem indicou quem”.
type Indicacao struct {
	gorm.Model
	IDExterno string `json:"id_externo" gorm:"uniqueIndex"`

	IndicadorID *uint  `json:"indicador_id" gorm:"index"`
	IndicadorPerfil string `json:"indicador_perfil" gorm:"index"` // Dono, Vendedor, Cliente, Contador

	EmailIndicado string `json:"email_indicado" gorm:"index"`
	TelefoneIndicado string `json:"telefone_indicado" gorm:"index"`

	Status string `json:"status" gorm:"index"` // Ex.: "registrada", "validada", "recusada"
}

// RecompensaIndicacao representa um crédito financeiro por indicação.
type RecompensaIndicacao struct {
	gorm.Model
	IDExterno string `json:"id_externo" gorm:"uniqueIndex"`

	IndicacaoID uint `json:"indicacao_id" gorm:"index"`

	ValorCentavos int64 `json:"valor_centavos" gorm:"index"`
	Moeda string `json:"moeda"`

	Status StatusRecompensa `json:"status" gorm:"index"`

	Motivo string `json:"motivo"` // Ex.: "conta_aprovada", "primeira_compra"
	ConcedidaEm *time.Time `json:"concedida_em" gorm:"index"`
}

