// Package stocktaking implements a reviewed local inventory. Counting never
// changes physical stock; only an independent manager's approval can do so.
package stocktaking

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid inventory request")
	ErrConflict = errors.New("inventory revision, state or operation conflict")
	ErrNotFound = errors.New("inventory not found")
	ErrStale    = errors.New("stock moved since the physical count; recount required")
	ErrReserved = errors.New("physical count cannot cover active reservations")
)

const MaxLines = 200
const MaxQuantity int64 = 9007199254740991

type Item struct {
	ProductID string `json:"product_id"`
	Unit      string `json:"unit"`
}
type OpenInput struct {
	OperationID string `json:"operation_id"`
	ID          string `json:"inventory_id"`
	LocationID  string `json:"location_id"`
	Title       string `json:"title"`
	Items       []Item `json:"items"`
}
type CountItem struct {
	ProductID    string `json:"product_id"`
	CountedMilli int64  `json:"counted_milli"`
}
type CountInput struct {
	OperationID      string      `json:"operation_id"`
	ExpectedRevision int64       `json:"expected_revision"`
	Items            []CountItem `json:"items"`
}
type StateInput struct {
	OperationID      string `json:"operation_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Action           string `json:"action"`
	Reason           string `json:"reason"`
}
type Result struct {
	OperationID  string `json:"operation_id"`
	InventoryID  string `json:"inventory_id"`
	Action       string `json:"action"`
	Revision     int64  `json:"revision"`
	Status       string `json:"status"`
	ChangedLines int    `json:"changed_lines"`
	OccurredAt   string `json:"occurred_at"`
	Repeated     bool   `json:"repeated"`
}
type Session struct {
	ID         string `json:"inventory_id"`
	LocationID string `json:"location_id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Revision   int64  `json:"revision"`
	CreatedBy  string `json:"created_by"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
type Line struct {
	ProductID       string  `json:"product_id"`
	Unit            string  `json:"unit"`
	CountedMilli    *int64  `json:"counted_milli"`
	PreviousMilli   *int64  `json:"previous_milli"`
	DifferenceMilli *int64  `json:"difference_milli"`
	CountedBy       *string `json:"counted_by"`
	CountedAt       *string `json:"counted_at"`
	MovementID      *string `json:"movement_id"`
}
type Detail struct {
	Session Session `json:"inventory"`
	Lines   []Line  `json:"lines"`
}
type Event struct {
	Revision    int64           `json:"revision"`
	OperationID string          `json:"operation_id"`
	ActorID     string          `json:"actor_id"`
	DeviceID    string          `json:"device_id"`
	Action      string          `json:"action"`
	Reason      string          `json:"reason"`
	OccurredAt  string          `json:"occurred_at"`
	Request     json.RawMessage `json:"request"`
	Result      json.RawMessage `json:"result"`
}
type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func timestamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }
