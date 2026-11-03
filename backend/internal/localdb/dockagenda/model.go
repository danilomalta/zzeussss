// Package dockagenda manages local receiving appointments. A local reservation
// is neither a supplier acknowledgement, a stock receipt, nor a payment.
package dockagenda

import (
	"errors"
	"time"
)

const MaxExact int64 = 9007199254740991

var ErrInvalid = errors.New("invalid dock agenda input")
var ErrConflict = errors.New("dock agenda revision/state/operation conflict")
var ErrOverlap = errors.New("dock agenda slot occupied")
var ErrNotFound = errors.New("dock agenda reference not found")
var ErrOrder = errors.New("order cancelled or supplier inactive")

type DockInput struct {
	OperationID      string `json:"operation_id"`
	DockID           string `json:"dock_id"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	ExpectedRevision int64  `json:"expected_revision"`
}
type Dock struct {
	ID        string `json:"dock_id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Revision  int64  `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
type BookInput struct {
	OperationID string `json:"operation_id"`
	ID          string `json:"appointment_id"`
	DockID      string `json:"dock_id"`
	OrderID     string `json:"order_id"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	Reason      string `json:"reason"`
}
type ActionInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	DockID           string `json:"dock_id,omitempty"`
	StartsAt         string `json:"starts_at,omitempty"`
	EndsAt           string `json:"ends_at,omitempty"`
}
type Appointment struct {
	ID                string  `json:"appointment_id"`
	DockID            string  `json:"dock_id"`
	OrderID           string  `json:"order_id"`
	SupplierID        string  `json:"supplier_id"`
	StartsAt          string  `json:"starts_at"`
	EndsAt            string  `json:"ends_at"`
	Status            string  `json:"status"`
	Revision          int64   `json:"revision"`
	CreatedBy         string  `json:"created_by"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
	ArrivedAt         *string `json:"arrived_at"`
	UnloadingAt       *string `json:"unloading_at"`
	CompletedAt       *string `json:"completed_at"`
	SupplierConfirmed bool    `json:"supplier_confirmed"`
	StockReceived     bool    `json:"stock_received"`
}
type Result struct {
	OperationID string       `json:"operation_id"`
	EntityKind  string       `json:"entity_kind"`
	EntityID    string       `json:"entity_id"`
	Action      string       `json:"action"`
	Revision    int64        `json:"revision"`
	OccurredAt  string       `json:"occurred_at"`
	Dock        *Dock        `json:"dock,omitempty"`
	Appointment *Appointment `json:"appointment,omitempty"`
	Repeated    bool         `json:"repeated"`
}
type Page[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}
type Event struct {
	Result   Result `json:"result"`
	ActorID  string `json:"actor_id"`
	DeviceID string `json:"device_id"`
	Request  string `json:"request_json"`
}
type Window struct {
	From   string
	To     string
	DockID string
	Status string
	Limit  int
	Offset int
}

func stamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }
