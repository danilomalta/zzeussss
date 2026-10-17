package production

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeQualityExactRevisionAndText(t *testing.T) {
	in := QualityInput{OperationID: "op", LotID: "lot", ExpectedRevision: 0, Status: "passed", Criterion: " Criterio declarado ", Reason: " Observacao "}
	out, err := normalizeQuality(in)
	if err != nil || out.Criterion != "Criterio declarado" || out.Reason != "Observacao" || in.Reason != " Observacao " {
		t.Fatal(out, in, err)
	}
	for _, rev := range []int64{-1, MaxRevision, MaxRevision + 1} {
		v := in
		v.ExpectedRevision = rev
		if _, err = normalizeQuality(v); !errors.Is(err, ErrInvalid) {
			t.Fatal(rev, err)
		}
	}
	in.ExpectedRevision = MaxRevision - 1
	if _, err = normalizeQuality(in); err != nil {
		t.Fatal(err)
	}
	in.Criterion = strings.Repeat("á", 128)
	if _, err = normalizeQuality(in); !errors.Is(err, ErrInvalid) {
		t.Fatal("UTF-8 bound", err)
	}
}
