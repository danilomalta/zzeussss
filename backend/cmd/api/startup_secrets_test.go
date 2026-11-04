package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestStartupRefusesUnsafeSecretsBeforeDatabaseOrListener(t *testing.T) {
	base := map[string]string{"JWT_SECRET": jwtFixtureKey, "DB_USER": "operator", "DB_PASSWORD": "installation-specific-private-database-key", "DB_NAME": "company", "DB_HOST": "127.0.0.1", "DB_SSLMODE": "disable"}
	for _, tc := range []struct{ name, key, value string }{
		{"missing JWT", "JWT_SECRET", ""}, {"blank JWT", "JWT_SECRET", " \t"}, {"public JWT", "JWT_SECRET", "super_secret_key_change_in_prod"}, {"missing database password", "DB_PASSWORD", ""}, {"blank database password", "DB_PASSWORD", " \t"}, {"public database password", "DB_PASSWORD", "titanpass"},
		{"short JWT", "JWT_SECRET", "short"}, {"malformed JWT", "JWT_SECRET", strings.Repeat("x", 64)}, {"padded JWT", "JWT_SECRET", jwtFixtureKey + "="}, {"repeated JWT", "JWT_SECRET", strings.Repeat("A", 43)},
		{"public URL password", "DATABASE_URL", "postgres://operator:titanpass@127.0.0.1/company?sslmode=disable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := func(key string) string {
				if key == tc.key {
					return tc.value
				}
				return base[key]
			}
			err := validateStartupSecrets(env)
			if err == nil {
				t.Fatal("unsafe startup accepted")
			}
			if tc.value != "" && strings.Contains(err.Error(), tc.value) {
				t.Fatal("configuration value disclosed")
			}
		})
	}
	if err := validateStartupSecrets(func(key string) string { return base[key] }); err != nil {
		t.Fatal("private explicit configuration refused")
	}
}

// Exercise main itself from a directory without .env; no installed configuration
// is read and invalid credentials must fail before any database/listener access.
func TestMainStartupRefusesUnsafeCredentialsWithoutDisclosure(t *testing.T) {
	for _, tc := range []struct{ name, jwt, password string }{
		{"JWT", "super_secret_key_change_in_prod", "installation-private-database"},
		{"short JWT", "short", "installation-private-database"},
		{"malformed JWT", strings.Repeat("x", 64), "installation-private-database"},
		{"repeated JWT", strings.Repeat("A", 43), "installation-private-database"},
		{"database", jwtFixtureKey, "titanpass"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestStartupFailureChild$")
			command.Dir = t.TempDir()
			command.Env = []string{"TITAN_STARTUP_FAILURE_CHILD=1", "JWT_SECRET=" + tc.jwt, "DB_USER=operator", "DB_PASSWORD=" + tc.password, "DB_NAME=company", "DB_HOST=127.0.0.1", "DB_SSLMODE=disable"}
			out, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 {
				t.Fatal("unsafe main startup did not refuse")
			}
			expected := "JWT_SECRET"
			if tc.name == "database" {
				expected = "PostgreSQL"
			}
			if !bytes.Contains(out, []byte(expected)) {
				t.Fatal("startup did not identify invalid configuration")
			}
			for _, secret := range []string{tc.jwt, tc.password} {
				if bytes.Contains(out, []byte(secret)) {
					t.Fatal("secret disclosed by main")
				}
			}
		})
	}
}

func TestStartupFailureChild(t *testing.T) {
	if os.Getenv("TITAN_STARTUP_FAILURE_CHILD") == "1" {
		main()
		t.Fatal("unsafe main startup returned successfully")
	}
}
