package secretconfig

import "testing"

func TestUnsafeRejectsBlankAndPublicExamples(t *testing.T) {
	for i, value := range []string{"", " \t\n", "super_secret_key_change_in_prod", "titanpass", " TITANPASS ", " SUPER_SECRET_KEY_CHANGE_IN_PROD "} {
		if !Unsafe(value) {
			t.Fatalf("unsafe credential case %d accepted", i)
		}
	}
	if Unsafe("installation-specific-random-material") {
		t.Fatal("explicit private credential refused")
	}
}
