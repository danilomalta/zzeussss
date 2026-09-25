package domain

import (
	"time"

	"gorm.io/gorm"
)

// Product representa um item vendável no sistema de PDV.
type Product struct {
	gorm.Model

	TenantID  string  `json:"-" gorm:"column:tenant_id;type:uuid;not null;uniqueIndex:idx_products_tenant_sku"`
	Nome      string  `json:"nome"`
	Descricao string  `json:"descricao"`
	Preco     float64 `json:"preco"`
	SKU       string  `json:"sku" gorm:"uniqueIndex:idx_products_tenant_sku"`
	Estoque   int     `json:"estoque"`
	Ativo     bool    `json:"ativo" gorm:"default:true"`

	// Campos para controle inteligente de estoque.
	EstoqueMinimo           int        `json:"estoque_minimo"`
	PontoReposicao          int        `json:"ponto_reposicao"`
	DemandaMediaDiaria      float64    `json:"demanda_media_diaria"`
	VariabilidadeDemanda    float64    `json:"variabilidade_demanda"`
	TempoReposicaoDias      int        `json:"tempo_reposicao_dias"`
	UltimaPrevisaoDemandaEm *time.Time `json:"ultima_previsao_demanda_em"`
	AlertaReposicaoAtivo    bool       `json:"alerta_reposicao_ativo" gorm:"default:false"`
}
