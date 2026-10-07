package database

import (
	"strings"
	"testing"
)

func TestConfigurationRequiresExplicitCredentialsAndVerifiedRemoteTLS(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"local explicit", "postgres://user:secret@127.0.0.1:5432/demo?sslmode=disable", true},
		{"remote verified", "postgres://user:secret@db.example:5432/demo?sslmode=verify-full", true},
		{"ipv6 local", "postgres://user:secret@[::1]:5432/demo?sslmode=disable", true},
		{"missing mode", "postgres://user:secret@localhost/demo", false},
		{"missing password", "postgres://user@localhost/demo?sslmode=disable", false},
		{"LAN plaintext", "postgres://user:secret@192.168.1.5/demo?sslmode=disable", false},
		{"unverified TLS", "postgres://user:secret@db.example/demo?sslmode=require", false},
		{"fallback", "postgres://user:secret@localhost/demo?sslmode=prefer", false},
		{"override", "postgres://user:secret@localhost/demo?sslmode=disable&host=evil.example", false},
		{"duplicate", "postgres://user:secret@localhost/demo?sslmode=disable&sslmode=verify-full", false},
		{"invalid secret url", "postgres://user:private%ZZ@localhost/demo", false},
		{"keyword DSN", "host=localhost password=private sslmode=disable", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := Configuration(func(key string) string {
				if key == "DATABASE_URL" {
					return tc.raw
				}
				return ""
			})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, configuração aceita=%v", tc.valid, err == nil)
			}
			if err != nil && err != ErrConfiguration {
				t.Fatal("erro expôs detalhes da configuração")
			}
			if config != nil && (len(config.ConnConfig.Fallbacks) != 0 || config.MinConns != 0) {
				t.Fatal("fallback ou abertura excessiva")
			}
		})
	}
}

func TestConfigurationEscapesCredentialsWithoutDefaults(t *testing.T) {
	env := map[string]string{"DB_USER": "user@x", "DB_PASSWORD": "private:@/?#%", "DB_NAME": "demo", "DB_SSLMODE": "disable"}
	get := func(key string) string { return env[key] }
	config, err := Configuration(get)
	if err != nil {
		t.Fatal("credenciais especiais recusadas")
	}
	if config.ConnConfig.Password != env["DB_PASSWORD"] || config.ConnConfig.User != env["DB_USER"] {
		t.Fatal("credenciais modificadas")
	}
	delete(env, "DB_PASSWORD")
	if _, err := Configuration(get); err != ErrConfiguration {
		t.Fatal("senha padrão aceita")
	}
	if strings.Contains(ErrConfiguration.Error(), "private") {
		t.Fatal("segredo exposto")
	}
}
