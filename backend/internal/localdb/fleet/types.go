// Package fleet implements tenant/store scoped operational fleet and maintenance.
// Financial records are managerial annotations, never payments or stock postings.
package fleet

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid fleet input")
	ErrConflict = errors.New("fleet state, revision or evidence conflict")
	ErrNotFound = errors.New("fleet record not found")
	ErrLimit    = errors.New("fleet source limit exceeded")
)

const MaxValue int64 = 9007199254740991
const MaxSource = 5000

type Record struct {
	Kind       string          `json:"kind"`
	ID         string          `json:"id"`
	AssetID    string          `json:"asset_id"`
	State      string          `json:"state"`
	AssignedTo string          `json:"assigned_to"`
	Revision   int64           `json:"revision"`
	Data       json.RawMessage `json:"data"`
	CreatedBy  string          `json:"created_by"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
}

// Input has no tenant/user/device fields: all context comes from the session.
type Input struct {
	OperationID      string          `json:"operation_id"`
	ID               string          `json:"id"`
	ExpectedRevision int64           `json:"expected_revision"`
	Action           string          `json:"action"`
	Reason           string          `json:"reason"`
	Data             json.RawMessage `json:"data"`
}
type Result struct {
	OperationID string     `json:"operation_id"`
	Record      *Record    `json:"record,omitempty"`
	Repeated    bool       `json:"repeated"`
	Ack         *NoticeAck `json:"ack,omitempty"`
}
type Asset struct {
	Reference  string `json:"reference"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Plate      string `json:"plate"`
	Location   string `json:"location"`
	MeterBasis string `json:"meter_basis"`
	MeterMilli int64  `json:"meter_milli"`
	Note       string `json:"note"`
}
type OpenRequest struct {
	AssetID   string `json:"asset_id"`
	Issue     string `json:"issue"`
	Priority  string `json:"priority"`
	Location  string `json:"location"`
	Blocks    bool   `json:"blocks"`
	Impact    string `json:"impact"`
	Reference string `json:"reference"`
}
type Request struct {
	OpenRequest
	RequesterID string `json:"requester_id"`
	QuoteID     string `json:"quote_id"`
}
type RequestAction struct {
	AssignedTo string `json:"assigned_to"`
	QuoteID    string `json:"quote_id"`
}
type Provider struct {
	Name     string `json:"name"`
	Contact  string `json:"contact"`
	Services string `json:"services"`
	Note     string `json:"note"`
}
type Plan struct {
	AssetID       string `json:"asset_id"`
	Label         string `json:"label"`
	DueAt         string `json:"due_at"`
	DueMeterMilli *int64 `json:"due_meter_milli"`
	Note          string `json:"note"`
}
type TripOpen struct {
	AssetID     string `json:"asset_id"`
	DriverID    string `json:"driver_id"`
	Reference   string `json:"reference"`
	Description string `json:"description"`
}
type Trip struct {
	TripOpen
	StartMeterMilli int64  `json:"start_meter_milli"`
	EndMeterMilli   *int64 `json:"end_meter_milli"`
	DepartedAt      string `json:"departed_at"`
	ReturnedAt      string `json:"returned_at"`
}
type TripAction struct {
	Loaded        bool   `json:"loaded"`
	EndMeterMilli *int64 `json:"end_meter_milli"`
}
type Quote struct {
	AssetID    string `json:"asset_id"`
	RequestID  string `json:"request_id"`
	ProviderID string `json:"provider_id"`
	PartsCents int64  `json:"parts_cents"`
	LaborCents int64  `json:"labor_cents"`
	OtherCents int64  `json:"other_cents"`
	Note       string `json:"note"`
}
type Amount struct {
	AssetID     string `json:"asset_id"`
	Category    string `json:"category"`
	AmountCents int64  `json:"amount_cents"`
	IncurredAt  string `json:"incurred_at"`
	ProviderID  string `json:"provider_id"`
	RequestID   string `json:"request_id"`
	Reference   string `json:"reference"`
}
type Profile struct {
	AssetID          string `json:"asset_id"`
	AcquisitionCents int64  `json:"acquisition_cents"`
	ResidualCents    int64  `json:"residual_cents"`
	UsefulMonths     int64  `json:"useful_months"`
	InServiceOn      string `json:"in_service_on"`
	Note             string `json:"note"`
}
type Page struct {
	Items      []Record `json:"items"`
	Total      int64    `json:"total"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
	MeasuredAt string   `json:"measured_at"`
}
type Query struct {
	AssetID, State, AssignedTo string
	Limit, Offset              int
}
type HistoryItem struct {
	Action   string `json:"action"`
	ActorID  string `json:"actor_id"`
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason"`
	Result   Result `json:"result"`
}
type History struct {
	Items  []HistoryItem `json:"items"`
	Total  int64         `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

func ID(v string) bool {
	if v == "" || len(v) > 128 || !utf8.ValidString(v) || strings.TrimSpace(v) != v || strings.Contains(v, "/") {
		return false
	}
	for _, r := range v {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func text(v string, max int, empty bool) bool {
	if !utf8.ValidString(v) || len(v) > max || strings.TrimSpace(v) != v || (!empty && v == "") {
		return false
	}
	for _, r := range v {
		if r < 32 && r != '\n' && r != '\t' || r == 127 {
			return false
		}
	}
	return true
}
func number(v int64) bool { return v >= 0 && v <= MaxValue }
func oneOf(v string, choices ...string) bool {
	for _, s := range choices {
		if v == s {
			return true
		}
	}
	return false
}
func financial(kind string) bool {
	return oneOf(kind, "quote", "expense", "income", "impact", "profile")
}
func known(kind string) bool {
	return oneOf(kind, "asset", "request", "trip", "plan", "provider", "quote", "expense", "income", "impact", "profile")
}
func stamp(s string, empty bool) (string, error) {
	if s == "" && empty {
		return "", nil
	}
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil || t.Year() < 2000 || t.Year() > 2200 || t.Nanosecond() != 0 {
		return "", ErrInvalid
	}
	return t.UTC().Format(time.RFC3339), nil
}
func decode(raw []byte, out any, required ...string) error {
	if len(raw) == 0 || len(raw) > 16384 {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, e := d.Token()
	if e != nil || token != json.Delim('{') {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for d.More() {
		t, e := d.Token()
		if e != nil {
			return ErrInvalid
		}
		key, ok := t.(string)
		if !ok || seen[key] {
			return ErrInvalid
		}
		seen[key] = true
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) && key != "due_meter_milli" && key != "end_meter_milli" {
			return ErrInvalid
		}
	}
	if _, e = d.Token(); e != nil {
		return ErrInvalid
	}
	if _, e = d.Token(); e != io.EOF {
		return ErrInvalid
	}
	if len(seen) != len(required) {
		return ErrInvalid
	}
	for _, k := range required {
		if !seen[k] {
			return ErrInvalid
		}
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return ErrInvalid
	}
	return nil
}
func encode(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func validate(kind string, raw []byte) bool {
	switch kind {
	case "asset":
		var a Asset
		if decode(raw, &a, "reference", "name", "category", "plate", "location", "meter_basis", "meter_milli", "note") != nil {
			return false
		}
		return text(a.Reference, 120, false) && text(a.Name, 160, false) && oneOf(a.Category, "truck", "car", "motorcycle", "van", "machine", "forklift", "other") && text(a.Plate, 32, true) && text(a.Location, 240, true) && text(a.Note, 1000, true) && oneOf(a.MeterBasis, "km", "hour", "none") && number(a.MeterMilli) && (a.MeterBasis != "none" || a.MeterMilli == 0)
	case "provider":
		var v Provider
		return decode(raw, &v, "name", "contact", "services", "note") == nil && text(v.Name, 160, false) && text(v.Contact, 240, false) && text(v.Services, 500, false) && text(v.Note, 1000, true)
	}
	return false
}
