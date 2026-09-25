package domain

import "gorm.io/gorm"

const (
	DiscountStatusPending  = "PENDING"
	DiscountStatusApproved = "APPROVED"
	DiscountStatusRejected = "REJECTED"
)

type DiscountSuggestion struct {
	gorm.Model
	TenantID          string  `json:"-" gorm:"type:uuid;not null;index"`
	ProductID         uint    `json:"product_id" gorm:"not null;index"`
	SuggestedDiscount float64 `json:"suggested_discount"`
	SuggestedRange    string  `json:"suggested_range"`
	Reason            string  `json:"reason"`
	Criteria          string  `json:"criteria"`
	Status            string  `json:"status"`
	ReviewedBy        string  `json:"reviewed_by"`
}
