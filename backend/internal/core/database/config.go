package database

import (
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConfiguration = errors.New("configuração PostgreSQL inválida; confira host, credenciais e TLS")
var ErrUnavailable = errors.New("conexão PostgreSQL indisponível")

// Configuration never returns parser errors: those can contain the DSN/password.
// DATABASE_URL accepts a single explicit endpoint; remote hosts require verify-full.
func Configuration(getenv func(string) string) (*pgxpool.Config, error) {
	raw := getenv("DATABASE_URL")
	if raw == "" {
		host, port := getenv("DB_HOST"), getenv("DB_PORT")
		if host == "" {
			host = "localhost"
		}
		if port == "" {
			port = "5432"
		}
		mode := getenv("DB_SSLMODE")
		if mode == "" {
			mode = "verify-full"
		}
		if getenv("DB_USER") == "" || getenv("DB_PASSWORD") == "" || getenv("DB_NAME") == "" {
			return nil, ErrConfiguration
		}
		u := url.URL{Scheme: "postgres", Host: net.JoinHostPort(host, port), Path: "/" + getenv("DB_NAME"), User: url.UserPassword(getenv("DB_USER"), getenv("DB_PASSWORD"))}
		q := url.Values{"sslmode": {mode}}
		if root := getenv("DB_SSLROOTCERT"); root != "" {
			q.Set("sslrootcert", root)
		}
		u.RawQuery = q.Encode()
		raw = u.String()
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil || u.Fragment != "" || u.Opaque != "" {
		return nil, ErrConfiguration
	}
	password, supplied := u.User.Password()
	if !supplied || password == "" || u.User.Username() == "" || u.Path == "" || u.Path == "/" {
		return nil, ErrConfiguration
	}
	host := u.Hostname()
	if host == "" || strings.ContainsAny(host, ",/ \\") {
		return nil, ErrConfiguration
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, ErrConfiguration
		}
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, ErrConfiguration
	}
	// Reject endpoint overrides, multi-host fallback, unsafe TLS modes and libpq
	// options capable of changing server-side search_path through configuration.
	for key, values := range q {
		if len(values) != 1 {
			return nil, ErrConfiguration
		}
		switch key {
		case "sslmode", "sslrootcert", "sslcert", "sslkey", "connect_timeout", "application_name":
		default:
			return nil, ErrConfiguration
		}
	}
	mode := q.Get("sslmode")
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil {
		loopback = ip.IsLoopback()
	}
	if mode != "verify-full" && !(mode == "disable" && loopback) {
		return nil, ErrConfiguration
	}
	config, err := pgxpool.ParseConfig(raw)
	if err != nil || len(config.ConnConfig.Fallbacks) != 0 {
		return nil, ErrConfiguration
	}
	// Do not permit PG* environment defaults to inject alternate endpoints/TLS.
	if config.ConnConfig.Host != host {
		return nil, ErrConfiguration
	}
	if mode == "verify-full" && (config.ConnConfig.TLSConfig == nil || config.ConnConfig.TLSConfig.InsecureSkipVerify || config.ConnConfig.TLSConfig.ServerName != host) {
		return nil, ErrConfiguration
	}
	if config.ConnConfig.RuntimeParams["options"] != "" {
		return nil, ErrConfiguration
	}
	if config.ConnConfig.TLSConfig != nil {
		config.ConnConfig.TLSConfig.MinVersion = tls.VersionTLS12
	}
	config.MaxConns = 50
	config.MinConns = 0
	config.MaxConnIdleTime = 15 * time.Minute
	config.MaxConnLifetime = time.Hour
	config.HealthCheckPeriod = 30 * time.Second
	config.ConnConfig.ConnectTimeout = 10 * time.Second
	return config, nil
}
