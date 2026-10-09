package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

type onlineObservedListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

func (ln *onlineObservedListener) Close() error {
	err := ln.Listener.Close()
	ln.once.Do(func() { close(ln.closed) })
	return err
}

func waitOnlineEvent(t *testing.T, event <-chan struct{}) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(2 * time.Second):
		t.Fatal("lifecycle event timed out")
	}
}

func TestOnlineDrainCompletesActiveResponseBeforeResourcesClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := fiber.New(onlineServerConfig())
	started, release := make(chan struct{}), make(chan struct{})
	var completed, resourcesClosed atomic.Bool
	app.Get("/work", func(c *fiber.Ctx) error {
		close(started)
		<-release
		completed.Store(true)
		return c.SendString("confirmed")
	})
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln := &onlineObservedListener{Listener: raw, closed: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- serveOnline(ctx, app, ln, time.Second, func() error {
			if !completed.Load() {
				return errors.New("resource closed under active handler")
			}
			resourcesClosed.Store(true)
			return nil
		})
	}()
	response := make(chan string, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
		resp, e := client.Get("http://" + ln.Addr().String() + "/work")
		if e != nil {
			response <- "failed"
			return
		}
		defer resp.Body.Close()
		body, e := io.ReadAll(resp.Body)
		if e != nil || resp.StatusCode != 200 {
			response <- "failed"
			return
		}
		response <- string(body)
	}()
	waitOnlineEvent(t, started)
	cancel()
	waitOnlineEvent(t, ln.closed)
	if resourcesClosed.Load() {
		t.Fatal("resources closed before active work")
	}
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("new connection accepted during drain")
	}
	close(release)
	select {
	case result := <-response:
		if result != "confirmed" {
			t.Fatal("active response was interrupted")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("response timed out")
	}
	select {
	case err := <-done:
		if err != nil || !resourcesClosed.Load() {
			t.Fatal("drain or resource close failed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drain timed out")
	}
}

func TestOnlineDrainTimeoutDoesNotCloseDatabaseOrReportSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := fiber.New(onlineServerConfig())
	started, release := make(chan struct{}), make(chan struct{})
	var closed atomic.Bool
	app.Get("/work", func(c *fiber.Ctx) error { close(started); <-release; return c.SendStatus(200) })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- serveOnline(ctx, app, ln, 50*time.Millisecond, func() error { closed.Store(true); return nil })
	}()
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
		resp, e := client.Get("http://" + ln.Addr().String() + "/work")
		if e == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()
	waitOnlineEvent(t, started)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, errOnlineDrain) || closed.Load() {
			t.Fatal("timeout claimed successful drain")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("drain did not respect timeout")
	}
	close(release)
	waitOnlineEvent(t, requestDone)
}

func TestOnlineAlreadyCancelledDoesNotStartListenerAndClosesResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	if serveOnline(ctx, fiber.New(onlineServerConfig()), ln, time.Second, func() error { closed = true; return nil }) != nil || !closed {
		t.Fatal("cancelled startup leaked resources")
	}
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("cancelled startup still accepts connections")
	}
}
