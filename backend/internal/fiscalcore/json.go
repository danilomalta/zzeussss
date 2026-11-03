package fiscalcore

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

func uniqueJSON(raw []byte) error {
	if len(raw) == 0 || len(raw) > 4*MaxXML {
		return ErrInput
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 32 {
			return ErrInput
		}
		token, err := dec.Token()
		if err != nil {
			return ErrInput
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return ErrInput
				}
				name, ok := key.(string)
				if !ok {
					return ErrInput
				}
				name = strings.ToLower(name)
				if seen[name] {
					return ErrInput
				}
				seen[name] = true
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim('}') {
				return ErrInput
			}
		case '[':
			for dec.More() {
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			end, err := dec.Token()
			if err != nil || end != json.Delim(']') {
				return ErrInput
			}
		default:
			return ErrInput
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return ErrInput
	}
	return nil
}

// DecodeInput rejects duplicate/case-colliding fields, trailing values and
// unknown fields. Decimal values remain strings in the destination structures.
func DecodeInput(raw []byte, target any) error {
	if err := uniqueJSON(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return ErrInput
	}
	return nil
}
