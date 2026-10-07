package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"titansystem-backend/internal/deployment"
	"titansystem-backend/internal/localapi"
	"titansystem-backend/internal/localdb/stationfile"
)

func describeMode(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("mode", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("deployment", "", "perfil privado de operação")
	if fs.Parse(args) != nil || *path == "" || fs.NArg() != 0 || out == nil {
		return deployment.ErrProfile
	}
	profile, err := deployment.Load(*path)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(profile.Describe())
}

func serveStation(args []string) error {
	return serveStationContext(context.Background(), args, io.Discard)
}

func serveStationContext(ctx context.Context, args []string, out io.Writer) error {
	if ctx == nil || out == nil {
		return deployment.ErrProfile
	}
	options := flag.NewFlagSet("serve", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	dbPath := options.String("db", "", "SQLite existente")
	stationPath := options.String("station", "", "arquivo privado do aparelho")
	issuerKeys := options.String("issuer-keys", "", "chaves públicas emissoras confiáveis")
	port := options.Int("port", 8181, "porta loopback sem perfil de operação")
	profilePath := options.String("deployment", "", "perfil privado de operação local ou servidor")
	if options.Parse(args) != nil || options.NArg() != 0 || *dbPath == "" || *stationPath == "" {
		return errors.New("informe banco e aparelho")
	}
	if *port < 1 || *port > 65535 {
		return errors.New("porta local deve estar entre 1 e 65535")
	}
	profile := deployment.Local(*port)
	if *profilePath != "" {
		conflict := false
		options.Visit(func(f *flag.Flag) {
			if f.Name == "port" {
				conflict = true
			}
		})
		if conflict {
			return errors.New("use listen do perfil; não combine deployment e port")
		}
		var err error
		profile, err = deployment.Load(*profilePath)
		if err != nil {
			return err
		}
	}
	tlsConfig, err := profile.TLS(time.Now())
	if err != nil {
		return err
	}
	verifier, err := readIssuerVerifier(*issuerKeys)
	if err != nil {
		return err
	}
	// Unsupported profiles, bad certificate, network and flags are rejected
	// before opening/migrating the existing database.
	if err = ctx.Err(); err != nil {
		return err
	}
	db, device, key, err := stationfile.OpenVerified(ctx, *dbPath, *stationPath)
	if err != nil {
		return errors.New("banco ou aparelho não aprovado; serviço não iniciado")
	}
	defer db.Close()
	defer clear(key)
	app, err := localapi.NewWithVerifierAndGate(db, device, verifier, networkGate(profile))
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", profile.Listen)
	if err != nil {
		return errors.New("endereço indisponível; nenhum processo foi encerrado")
	}
	defer listener.Close()
	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}
	scheme := "http"
	if tlsConfig != nil {
		scheme = "https"
	}
	if _, err = fmt.Fprintf(out, "API %s em %s://%s/local/v1/health\n", profile.Mode, scheme, listener.Addr()); err != nil {
		return err
	}
	if verifier == nil {
		if _, err = fmt.Fprintln(out, "Cadastros indisponiveis: configure --issuer-keys e instale um contrato valido. Login e consultas continuam disponiveis."); err != nil {
			return err
		}
	}
	return runLocalListener(ctx, app, listener)
}

func networkGate(profile deployment.Profile) fiber.Handler {
	limit := limiter.New(limiter.Config{Max: 120, Expiration: time.Second})
	active := make(chan struct{}, 32)
	return func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		// Fiber's default IP() is the connection peer: no ProxyHeader is configured.
		// X-Forwarded-For/Forwarded headers cannot grant network access.
		if !profile.Allows(c.IP()) {
			return c.SendStatus(fiber.StatusForbidden)
		}
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			return c.SendStatus(fiber.StatusTooManyRequests)
		}
		return limit(c)
	}
}

// fasthttp stops draining if a second listener.Close returns net.ErrClosed.
// Keep close idempotent so cancellation can close admission before Shutdown
// without skipping the wait for already active requests.
type closeOnceListener struct {
	net.Listener
	once sync.Once
	err  error
}

func (l *closeOnceListener) Close() error {
	l.once.Do(func() {
		l.err = l.Listener.Close()
		if errors.Is(l.err, net.ErrClosed) {
			l.err = nil
		}
	})
	return l.err
}

func runLocalListener(ctx context.Context, app *fiber.App, listener net.Listener) error {
	managed := &closeOnceListener{Listener: listener}
	done := make(chan error, 1)
	go func() { done <- app.Listener(managed) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Stop accepting before draining, including cancellation just before Fiber
		// starts serving. Do not close the DB until the listener goroutine finishes.
		_ = managed.Close()
		err := app.ShutdownWithTimeout(5 * time.Second)
		serveErr := <-done
		if err != nil && !errors.Is(err, net.ErrClosed) {
			return err
		}
		if serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
			return errors.New("serviço interrompido")
		}
		return nil
	}
}
