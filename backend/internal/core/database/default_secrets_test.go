package database

import (
	"net/url"
	"strings"
	"testing"
)

func TestConfigurationRejectsPublicAndBlankPasswordsInBothInputs(t *testing.T) {
	for i, value := range []string{"", " \t", "titanpass", "super_secret_key_change_in_prod", " TITANPASS "} {
		for _, viaURL := range []bool{false, true} {
			env := map[string]string{"DB_USER": "operator", "DB_PASSWORD": value, "DB_NAME": "company", "DB_SSLMODE": "disable"}
			if viaURL {
				u := url.URL{Scheme: "postgres", Host: "127.0.0.1", Path: "/company", User: url.UserPassword("operator", value), RawQuery: "sslmode=disable"}
				env["DATABASE_URL"] = u.String()
			}
			cfg, err := Configuration(func(k string) string { return env[k] })
			if err != ErrConfiguration || cfg != nil {
				t.Fatalf("credential case %d accepted", i)
			}
			if value != "" && strings.Contains(err.Error(), value) {
				t.Fatal("password disclosed")
			}
		}
	}
}
