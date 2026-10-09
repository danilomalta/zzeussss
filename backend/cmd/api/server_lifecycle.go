package main

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

const onlineDrainTimeout = 45 * time.Second

var errOnlineDrain = errors.New("encerramento HTTP não confirmado; operações em curso podem ter resultado incerto")
var errOnlineServe = errors.New("servidor HTTP online interrompido")

type onlineReadyListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

func (ln *onlineReadyListener) Accept() (net.Conn, error) {
	// fasthttp registers the listener before calling Accept. This fence prevents
	// cancellation from shutting down a server which has not started serving yet.
	ln.once.Do(func() { close(ln.ready) })
	return ln.Listener.Accept()
}

// serveOnline owns the provided listener. Resources close only after HTTP
// drain succeeds; a timeout must not report successful drain or close the
// database under an active handler. The executable exits nonzero on failure.
func serveOnline(ctx context.Context, app *fiber.App, listener net.Listener, grace time.Duration, closeResources func() error) error {
	if ctx == nil || app == nil || listener == nil || grace <= 0 || closeResources == nil {
		return errOnlineServe
	}
	defer listener.Close()
	if ctx.Err() != nil {
		if closeResources() != nil {
			return errOnlineServe
		}
		return nil
	}
	ln := &onlineReadyListener{Listener: listener, ready: make(chan struct{})}
	finished := make(chan error, 1)
	go func() { finished <- app.Listener(ln) }()
	var serveErr error
	var ended bool
	select {
	case <-ln.ready:
	case serveErr = <-finished:
		ended = true
	}
	if !ended {
		select {
		case <-ctx.Done():
		case serveErr = <-finished:
			ended = true
		}
	}
	drain, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if app.ShutdownWithContext(drain) != nil {
		return errOnlineDrain
	}
	if !ended {
		select {
		case serveErr = <-finished:
		case <-drain.Done():
			return errOnlineDrain
		}
	}
	if closeResources() != nil {
		return errOnlineServe
	}
	if serveErr != nil {
		return errOnlineServe
	}
	return nil
}
