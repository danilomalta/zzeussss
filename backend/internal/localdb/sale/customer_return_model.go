package sale

import (
	"errors"
	"math/big"
	"strings"
	"unicode/utf8"
)

var ErrReturnRange = errors.New("historico de devolucoes excede limite")

// A return records physical receipt and a cash refund, never a card reversal.
type ReturnItemInput struct {
	SaleItemID    string `json:"sale_item_id"`
	QuantityMilli int64  `json:"quantity_milli"`
	Disposition   string `json:"disposition"`
}
type CustomerReturnInput struct {
	RefundCashSessionID string            `json:"refund_cash_session_id,omitempty"`
	OperationID         string            `json:"operation_id"`
	SaleID              string            `json:"sale_id"`
	Reason              string            `json:"reason"`
	ExpectedRefundCents int64             `json:"expected_refund_cents"`
	Items               []ReturnItemInput `json:"items"`
}
type CustomerReturnLine struct {
	ReturnItemInput
	ProductID          string  `json:"product_id"`
	LocationID         string  `json:"location_id"`
	OriginalMovementID string  `json:"original_movement_id"`
	ReturnMovementID   *string `json:"return_movement_id"`
	RefundCents        int64   `json:"refund_cents"`
}
type CustomerReturn struct {
	OriginalCashSessionID string               `json:"original_cash_session_id,omitempty"`
	ID                    string               `json:"id"`
	OperationID           string               `json:"operation_id"`
	SaleID                string               `json:"sale_id"`
	CashSessionID         string               `json:"cash_session_id"`
	ActorID               string               `json:"actor_id"`
	Reason                string               `json:"reason"`
	RefundCents           int64                `json:"refund_cents"`
	CashMovementID        string               `json:"cash_movement_id"`
	CreatedAt             string               `json:"created_at"`
	Repeated              bool                 `json:"repeated"`
	FiscalAuthorized      bool                 `json:"fiscal_authorized"`
	Items                 []CustomerReturnLine `json:"items"`
}
type ReturnBalanceLine struct {
	SaleItemID       string `json:"sale_item_id"`
	ProductID        string `json:"product_id"`
	LocationID       string `json:"location_id"`
	SoldMilli        int64  `json:"sold_milli"`
	ReturnedMilli    int64  `json:"returned_milli"`
	CancelledMilli   int64  `json:"cancelled_milli"`
	RemainingMilli   int64  `json:"remaining_milli"`
	OriginalCents    int64  `json:"original_cents"`
	RefundedCents    int64  `json:"refunded_cents"`
	CancelledCents   int64  `json:"cancelled_cents"`
	RemainingCents   int64  `json:"remaining_cents"`
	originalMovement string
}
type CustomerReturnHistory struct {
	SaleID             string              `json:"sale_id"`
	SaleStatus         string              `json:"sale_status"`
	CashSessionID      string              `json:"cash_session_id"`
	OriginalTotalCents int64               `json:"original_total_cents"`
	RefundedCents      int64               `json:"refunded_cents"`
	CancelledCents     int64               `json:"cancelled_cents"`
	RemainingCents     int64               `json:"remaining_cents"`
	Complete           bool                `json:"complete"`
	Items              []ReturnBalanceLine `json:"items"`
	Returns            []CustomerReturn    `json:"returns"`
}
type CustomerReturnQuote struct {
	OriginalCashSessionID string               `json:"original_cash_session_id,omitempty"`
	SaleID                string               `json:"sale_id"`
	CashSessionID         string               `json:"cash_session_id"`
	RefundCents           int64                `json:"refund_cents"`
	Items                 []CustomerReturnLine `json:"items"`
}

func validReturnItems(items []ReturnItemInput) bool {
	if len(items) == 0 || len(items) > 500 {
		return false
	}
	seen := map[string]bool{}
	for _, i := range items {
		if !proofKey(i.SaleItemID) || seen[i.SaleItemID] || i.QuantityMilli <= 0 || i.QuantityMilli > checkoutMaxSafe || (i.Disposition != "restock" && i.Disposition != "no_restock") {
			return false
		}
		seen[i.SaleItemID] = true
	}
	return true
}
func validCustomerReturn(in CustomerReturnInput) bool {
	return (in.RefundCashSessionID == "" || planID(in.RefundCashSessionID)) && proofKey(in.OperationID) && planID(in.SaleID) && utf8.ValidString(in.Reason) && strings.TrimSpace(in.Reason) != "" && len(in.Reason) <= 2000 && utf8.RuneCountInString(in.Reason) <= 500 && !strings.ContainsRune(in.Reason, 0) && in.ExpectedRefundCents > 0 && in.ExpectedRefundCents <= checkoutMaxSafe && validReturnItems(in.Items)
}

// Cumulative floor allocation makes repeated fractional returns sum to the
// exact original line total. It never uses today's price or binary floats.
func returnProportion(total, quantity, sold int64) int64 {
	n := new(big.Int).Mul(big.NewInt(total), big.NewInt(quantity))
	n.Quo(n, big.NewInt(sold))
	return n.Int64()
}

func quoteReturn(history CustomerReturnHistory, items []ReturnItemInput) (CustomerReturnQuote, error) {
	q := CustomerReturnQuote{SaleID: history.SaleID, CashSessionID: history.CashSessionID, Items: make([]CustomerReturnLine, 0, len(items))}
	if history.SaleStatus != "committed" {
		return q, ErrConflict
	}
	for _, in := range items {
		var found *ReturnBalanceLine
		for i := range history.Items {
			if history.Items[i].SaleItemID == in.SaleItemID {
				found = &history.Items[i]
				break
			}
		}
		if found == nil {
			return q, ErrNotFound
		}
		if in.QuantityMilli > found.RemainingMilli {
			return q, ErrConflict
		}
		amount := returnProportion(found.OriginalCents, found.ReturnedMilli+in.QuantityMilli, found.SoldMilli) - found.RefundedCents
		if amount < 0 || amount > checkoutMaxSafe-q.RefundCents {
			return q, ErrConflict
		}
		q.RefundCents += amount
		q.Items = append(q.Items, CustomerReturnLine{ReturnItemInput: in, ProductID: found.ProductID, LocationID: found.LocationID, OriginalMovementID: found.originalMovement, RefundCents: amount})
	}
	if q.RefundCents <= 0 {
		return q, ErrConflict
	}
	return q, nil
}
