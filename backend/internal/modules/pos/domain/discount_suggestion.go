package domain

import (
	"gorm.io/gorm"
)

// Discount suggestion status constants
const (
	DiscountStatusPending  = "PENDING"
	DiscountStatusApproved = "APPROVED"
	DiscountStatusRejected = "REJECTED"
)

// DiscountSuggestion representa uma recomendação do motor de descontos
type DiscountSuggestion struct {
	gorm.Model
	ProductID         uint    `json:"product_id" gorm:"index"`
	SuggestedDiscount float64 `json:"suggested_discount"` // e.g., 10 for 10%
	SuggestedRange    string  `json:"suggested_range"`    // e.g., "5% - 15%"
	Reason            string  `json:"reason"`
	Criteria          string  `json:"criteria"` // Which criteria triggered this (e.g., "LOW_TURNOVER")
	Status            string  `json:"status"`   // PENDING, APPROVED, REJECTED
	ReviewedBy        string  `json:"reviewed_by"` // User ID that reviewed it
}
