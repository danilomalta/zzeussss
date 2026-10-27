package fleet

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
	"unicode/utf8"
)

const MaxWorkshopSource = 1000

var ErrWorkshopLimit = errors.New("workshop source limit exceeded")

// Workshop operations have their own explicitly named recovery namespace.
type WorkshopResult struct {
	OperationID string          `json:"operation_id"`
	Kind        string          `json:"kind"`
	ID          string          `json:"id"`
	RequestID   string          `json:"request_id"`
	Revision    int64           `json:"revision"`
	State       string          `json:"state"`
	Data        json.RawMessage `json:"data"`
	ActorID     string          `json:"actor_id"`
	DeviceID    string          `json:"device_id"`
	RecordedAt  string          `json:"recorded_at"`
	Repeated    bool            `json:"repeated"`
}
type CheckItem struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}
type CheckTemplate struct {
	Name  string      `json:"name"`
	Items []CheckItem `json:"items"`
	Note  string      `json:"note"`
}
type BindingInput struct {
	RequestID               string `json:"request_id"`
	ExpectedRequestRevision int64  `json:"expected_request_revision"`
	TemplateID              string `json:"template_id"`
}
type CheckBinding struct {
	BindingInput
	Template CheckTemplate `json:"template"`
}
type CheckAnswer struct {
	ItemID  string `json:"item_id"`
	Outcome string `json:"outcome"`
	Note    string `json:"note"`
}
type InspectionInput struct {
	RequestID               string        `json:"request_id"`
	ExpectedRequestRevision int64         `json:"expected_request_revision"`
	ExpectedBindingRevision int64         `json:"expected_binding_revision"`
	Answers                 []CheckAnswer `json:"answers"`
	Note                    string        `json:"note"`
}
type Inspection struct {
	InspectionInput
	TemplateID    string `json:"template_id"`
	EpochRevision int64  `json:"epoch_revision"`
	Passed        bool   `json:"passed"`
}
type LaborInput struct {
	RequestID               string `json:"request_id"`
	ExpectedRequestRevision int64  `json:"expected_request_revision"`
	StartedAt               string `json:"started_at"`
	EndedAt                 string `json:"ended_at"`
	Description             string `json:"description"`
}
type Labor struct {
	LaborInput
	WorkerID string `json:"worker_id"`
	Seconds  int64  `json:"seconds"`
}
type WorkshopPage struct {
	Items  []WorkshopResult `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}
type WorkshopHistoryItem struct {
	Result WorkshopResult `json:"result"`
	Action string         `json:"action"`
	Reason string         `json:"reason"`
}
type WorkshopHistory struct {
	Items  []WorkshopHistoryItem `json:"items"`
	Total  int64                 `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}
type WorkshopSummary struct {
	RequestID             string `json:"request_id"`
	RequestRevision       int64  `json:"request_revision"`
	ChecklistRequired     bool   `json:"checklist_required"`
	BindingRevision       int64  `json:"binding_revision"`
	LatestInspectionID    string `json:"latest_inspection_id"`
	InspectionSatisfied   bool   `json:"inspection_satisfied"`
	DeclaredSecondsExact  string `json:"declared_seconds_exact"`
	ConfirmedSecondsExact string `json:"confirmed_seconds_exact"`
	RejectedSecondsExact  string `json:"rejected_seconds_exact"`
	VoidedSecondsExact    string `json:"voided_seconds_exact"`
	SourceRecords         int    `json:"source_records"`
	PayrollPosted         bool   `json:"payroll_posted"`
	MeasuredAt            string `json:"measured_at"`
}

func workshopKind(kind string) bool { return oneOf(kind, "template", "binding", "inspection", "labor") }
func strictTree(d *json.Decoder, depth int) error {
	if depth > 8 {
		return ErrInvalid
	}
	t, e := d.Token()
	if e != nil {
		return ErrInvalid
	}
	if delim, ok := t.(json.Delim); ok {
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return ErrInvalid
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return ErrInvalid
				}
				seen[s] = true
				if e = strictTree(d, depth+1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return ErrInvalid
			}
		case '[':
			for d.More() {
				if e = strictTree(d, depth+1); e != nil {
					return e
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
	}
	return nil
}
func workshopCanonical(raw []byte) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 16384 || !utf8.Valid(raw) {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if strictTree(d, 0) != nil {
		return nil, ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return nil, ErrInvalid
	}
	var v map[string]any
	d = json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&v) != nil || v == nil {
		return nil, ErrInvalid
	}
	return encode(v), nil
}
func templateDecode(raw []byte) (CheckTemplate, error) {
	var v CheckTemplate
	var parts struct {
		Name  string            `json:"name"`
		Items []json.RawMessage `json:"items"`
		Note  string            `json:"note"`
	}
	if decode(raw, &parts, "name", "items", "note") != nil || !text(parts.Name, 160, false) || !text(parts.Note, 1000, true) || len(parts.Items) < 1 || len(parts.Items) > 50 {
		return v, ErrInvalid
	}
	v.Name = parts.Name
	v.Note = parts.Note
	seen := map[string]bool{}
	for _, raw := range parts.Items {
		var i CheckItem
		if decode(raw, &i, "id", "label", "required") != nil || !ID(i.ID) || seen[i.ID] || !text(i.Label, 240, false) {
			return v, ErrInvalid
		}
		seen[i.ID] = true
		v.Items = append(v.Items, i)
	}
	return v, nil
}
func inspectionDecode(raw []byte) (InspectionInput, error) {
	var v InspectionInput
	var parts struct {
		RequestID               string            `json:"request_id"`
		ExpectedRequestRevision int64             `json:"expected_request_revision"`
		ExpectedBindingRevision int64             `json:"expected_binding_revision"`
		Answers                 []json.RawMessage `json:"answers"`
		Note                    string            `json:"note"`
	}
	if decode(raw, &parts, "request_id", "expected_request_revision", "expected_binding_revision", "answers", "note") != nil || !ID(parts.RequestID) || parts.ExpectedRequestRevision < 1 || parts.ExpectedRequestRevision >= MaxValue || parts.ExpectedBindingRevision < 1 || parts.ExpectedBindingRevision >= MaxValue || len(parts.Answers) < 1 || len(parts.Answers) > 50 || !text(parts.Note, 1000, false) {
		return v, ErrInvalid
	}
	v.RequestID = parts.RequestID
	v.ExpectedRequestRevision = parts.ExpectedRequestRevision
	v.ExpectedBindingRevision = parts.ExpectedBindingRevision
	v.Note = parts.Note
	seen := map[string]bool{}
	for _, raw := range parts.Answers {
		var answer CheckAnswer
		if decode(raw, &answer, "item_id", "outcome", "note") != nil || !ID(answer.ItemID) || seen[answer.ItemID] || !oneOf(answer.Outcome, "pass", "fail", "not_applicable") || !text(answer.Note, 1000, answer.Outcome == "pass") {
			return v, ErrInvalid
		}
		seen[answer.ItemID] = true
		v.Answers = append(v.Answers, answer)
	}
	return v, nil
}
func validRequestRevision(id string, revision int64) bool {
	return ID(id) && revision > 0 && revision < MaxValue
}
func laborDecode(raw []byte) (LaborInput, int64, error) {
	var v LaborInput
	if decode(raw, &v, "request_id", "expected_request_revision", "started_at", "ended_at", "description") != nil || !validRequestRevision(v.RequestID, v.ExpectedRequestRevision) || !text(v.Description, 1000, false) {
		return v, 0, ErrInvalid
	}
	start, e := stamp(v.StartedAt, false)
	if e != nil {
		return v, 0, e
	}
	end, e := stamp(v.EndedAt, false)
	if e != nil {
		return v, 0, e
	}
	s, _ := time.Parse(time.RFC3339, start)
	t, _ := time.Parse(time.RFC3339, end)
	seconds := int64(t.Sub(s) / time.Second)
	if seconds < 1 || seconds > 16*3600 || t.After(time.Now().UTC()) {
		return v, 0, ErrInvalid
	}
	v.StartedAt = start
	v.EndedAt = end
	return v, seconds, nil
}
