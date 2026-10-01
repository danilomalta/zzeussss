package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"titansystem-backend/internal/localdb/incoming"
	"titansystem-backend/internal/localdb/stationfile"
)

var errReceiverConfiguration = errors.New("configuração do receptor inválida")

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runReceive(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Titan receive: receptor indisponível; confira arquivos, aparelho aprovado, TLS e endereço de escuta.")
		os.Exit(1)
	}
}

func runReceive(ctx context.Context, args []string, output io.Writer) error {
	if ctx == nil || output == nil || len(args) == 0 || args[0] != "serve" {
		return errReceiverConfiguration
	}
	options := flag.NewFlagSet("serve", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	dbPath := options.String("db", "", "SQLite existente")
	stationPath := options.String("station", "", "aparelho privado existente")
	encryptionPath := options.String("encryption", "", "chave X25519 privada existente")
	listen := options.String("listen", "127.0.0.1:8282", "endereço explícito")
	certPath := options.String("tls-cert", "", "certificado PEM")
	tlsKeyPath := options.String("tls-key", "", "chave privada TLS PEM")
	if err := options.Parse(args[1:]); err != nil || options.NArg() != 0 || *dbPath == "" || *stationPath == "" || *encryptionPath == "" {
		return errReceiverConfiguration
	}
	tlsConfig, err := receiverTLS(*listen, *certPath, *tlsKeyPath)
	if err != nil {
		return err
	}
	db, device, signing, err := stationfile.OpenVerified(ctx, *dbPath, *stationPath)
	if err != nil {
		return err
	}
	defer db.Close()
	encryption, err := incoming.LoadEncryptionKey(*encryptionPath, device)
	if err != nil {
		return err
	}
	handler, err := incoming.NewSealedReceiverHTTP(ctx, db, device, signing, encryption)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	scheme := "http"
	if tlsConfig != nil {
		scheme = "https"
	}
	if _, err := fmt.Fprintf(output, "Receptor iniciado em %s://%s%s\n", scheme, listener.Addr(), incoming.SealedReceiverPath); err != nil {
		return err
	}
	return serveReceiver(ctx, listener, boundReceiver(handler), tlsConfig)
}

func receiverTLS(address, certPath, keyPath string) (*tls.Config, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errReceiverConfiguration
	}
	ip := net.ParseIP(host)
	number, err := strconv.Atoi(port)
	if ip == nil || ip.IsUnspecified() || err != nil || number < 0 || number > 65535 {
		return nil, errReceiverConfiguration
	}
	if certPath == "" && keyPath == "" {
		if !ip.IsLoopback() {
			return nil, errReceiverConfiguration
		}
		return nil, nil
	}
	if certPath == "" || keyPath == "" {
		return nil, errReceiverConfiguration
	}
	info, err := os.Lstat(keyPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errReceiverConfiguration
	}
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, errReceiverConfiguration
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}, nil
}

func serveReceiver(ctx context.Context, listener net.Listener, handler http.Handler, tlsConfig *tls.Config) error {
	server := &http.Server{Handler: handler, TLSConfig: tlsConfig, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	defer server.Close()
	// Default net/http logging can contain remote addresses and TLS errors.
	// Structured telemetry is a later integration; never print payload/keys.
	finished := make(chan error, 1)
	go func() {
		if tlsConfig == nil {
			finished <- server.Serve(listener)
		} else {
			finished <- server.ServeTLS(listener, "", "")
		}
	}()
	select {
	case err := <-finished:
		if ctx.Err() != nil && errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		err := <-finished
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Global bounded admission: at most 8 active requests and 20 starts/second.
// This is a basic limit, not a replacement for network perimeter controls.
func boundReceiver(next http.Handler) http.Handler {
	active := make(chan struct{}, 8)
	var mutex sync.Mutex
	var window time.Time
	var starts int
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		mutex.Lock()
		if time.Since(window) >= time.Second {
			window = time.Now()
			starts = 0
		}
		allowed := starts < 20
		if allowed {
			starts++
		}
		mutex.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "temporarily unavailable", 429)
			return
		}
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			http.Error(w, "temporarily unavailable", 503)
			return
		}
		next.ServeHTTP(w, r)
	})
}
