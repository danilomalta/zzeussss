package production

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeStagePlanPreservesSequenceAndCaller(t *testing.T) {
	in := StagePlanInput{OperationID: "op", OrderID: "order", Reason: " Plano ", Stages: []StageDefinition{{StageID: "z", Name: " Mistura ", ResponsibleID: "staff"}, {StageID: "a", Name: "Forno", ResponsibleID: "staff"}}}
	out, err := normalizeStagePlan(in)
	if err != nil || out.Reason != "Plano" || out.Stages[0].Name != "Mistura" || out.Stages[0].StageID != "z" || in.Stages[0].Name != " Mistura " {
		t.Fatal(out, in, err)
	}
	in.Stages[1].StageID = "z"
	if _, err = normalizeStagePlan(in); !errors.Is(err, ErrInvalid) {
		t.Fatal("duplicate stage", err)
	}
	in.Stages[1].StageID = "a"
	in.Stages[0].Name = strings.Repeat("á", 61)
	if _, err = normalizeStagePlan(in); !errors.Is(err, ErrInvalid) {
		t.Fatal("UTF-8 byte bound", err)
	}
}
