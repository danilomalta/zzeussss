package incoming

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"titansystem-backend/internal/localdb"
	"titansystem-backend/internal/localdb/outgoing"
)

func workerConfig(f *fixture, endpoint string) WorkerConfig {
	return WorkerConfig{DB: f.senderDB, Source: f.sender, Destination: f.receiver, SigningKey: f.senderKey, Endpoint: endpoint,
		IdleDelay: 10 * time.Millisecond, RetryBase: 50 * time.Millisecond, RetryMax: 200 * time.Millisecond}
}

func TestDeliveryWorkerRetriesAutomaticallyAndClearsPersistedDelay(t *testing.T) {
	f, _, handler := deliveryFixture(t)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 3 {
			w.WriteHeader(503)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config := workerConfig(f, server.URL+SealedReceiverPath)
	var deferred int
	var delivered bool
	var previousDue int64
	config.Observe = func(notice WorkerNotice) {
		if notice.Status == "deferred" {
			deferred++
			if notice.Failures != int64(deferred) || notice.NextAttemptUnixMilli <= previousDue {
				t.Error("espera não progrediu")
			}
			previousDue = notice.NextAttemptUnixMilli
			if notice.NextAttemptUnixMilli-time.Now().UnixMilli() > config.RetryMax.Milliseconds()+50 {
				t.Error("espera superou limite")
			}
		}
		if notice.Status == "delivered" {
			delivered = true
			cancel()
		}
	}
	if err := RunDeliveryWorker(ctx, config); err != nil {
		t.Fatal(err)
	}
	if !delivered || deferred != 3 || calls.Load() != 4 {
		t.Fatalf("entregue=%t falhas=%d chamadas=%d", delivered, deferred, calls.Load())
	}
	var failures, due int64
	if err := f.senderDB.QueryRow("SELECT failures,next_attempt_unix_milli FROM sync_worker_state").Scan(&failures, &due); err != nil || failures != 0 || due != 0 {
		t.Fatalf("espera residual: %d %d %v", failures, due, err)
	}
	receivedCount(t, f, 1)
}

func TestDeliveryWorkerRestartHonorsPersistedDelay(t *testing.T) {
	f, _, _ := deliveryFixture(t)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	config := workerConfig(f, server.URL+SealedReceiverPath)
	config.RetryBase = 30 * time.Second
	config.RetryMax = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config.Observe = func(notice WorkerNotice) {
		if notice.Status == "deferred" {
			cancel()
		}
	}
	if err := RunDeliveryWorker(ctx, config); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("tentativa inicial ausente")
	}
	var sequence int
	var name, path string
	if err := f.senderDB.QueryRow("PRAGMA database_list").Scan(&sequence, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := f.senderDB.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	f.senderDB, err = localdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	config.DB = f.senderDB
	ctx, restartCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer restartCancel()
	var waiting bool
	config.Observe = func(notice WorkerNotice) {
		if notice.Status == "waiting" {
			waiting = true
			restartCancel()
		}
	}
	if err := RunDeliveryWorker(ctx, config); err != nil {
		t.Fatal(err)
	}
	if !waiting || calls.Load() != 1 {
		t.Fatal("reinício ignorou pausa persistida")
	}
	pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 {
		t.Fatal("reinício perdeu evento")
	}
}

func TestDeliveryWorkerIdleCancellationDoesNotSend(t *testing.T) {
	f, _, handler := deliveryFixture(t)
	server := httptest.NewServer(handler)
	defer server.Close()
	if _, err := DeliverPendingOnce(context.Background(), f.senderDB, f.sender, f.senderKey, f.receiver, server.URL+SealedReceiverPath, nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config := workerConfig(f, server.URL+SealedReceiverPath)
	var idle bool
	config.Observe = func(notice WorkerNotice) {
		if notice.Status == "idle" {
			idle = true
			cancel()
		}
	}
	if err := RunDeliveryWorker(ctx, config); err != nil {
		t.Fatal(err)
	}
	if !idle {
		t.Fatal("fila vazia não entrou em espera")
	}
	receivedCount(t, f, 1)
}

func TestDeliveryWorkerRejectsRevokedTrustAndBadConfiguration(t *testing.T) {
	f, _, _ := deliveryFixture(t)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	base := workerConfig(f, server.URL+SealedReceiverPath)
	for _, change := range []func(*WorkerConfig){
		func(c *WorkerConfig) { c.RetryBase = -time.Second },
		func(c *WorkerConfig) { c.RetryMax = c.RetryBase / 2 },
		func(c *WorkerConfig) { c.IdleDelay = -time.Second },
		func(c *WorkerConfig) { c.Endpoint = "http://192.168.1.2/sync/v1/events" },
		func(c *WorkerConfig) { c.SigningKey = nil },
	} {
		config := base
		change(&config)
		if err := RunDeliveryWorker(context.Background(), config); err == nil {
			t.Fatal("configuração inválida aceita")
		}
	}
	if _, err := f.senderDB.Exec("UPDATE memberships SET status='revoked' WHERE identity_id='owner'"); err != nil {
		t.Fatal(err)
	}
	if err := RunDeliveryWorker(context.Background(), base); err == nil {
		t.Fatal("aprovação revogada aceita")
	}
	if calls.Load() != 0 {
		t.Fatal("configuração inválida enviou mensagem")
	}
}

func TestDeliveryWorkerStateFailureStopsWithoutRemovingEvent(t *testing.T) {
	f, _, _ := deliveryFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer server.Close()
	if _, err := f.senderDB.Exec(`CREATE TRIGGER fail_worker_delay BEFORE UPDATE ON sync_worker_state BEGIN SELECT RAISE(ABORT,'falha de teste'); END`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RunDeliveryWorker(ctx, workerConfig(f, server.URL+SealedReceiverPath)); err == nil {
		t.Fatal("falha ao persistir pausa ignorada")
	}
	pending, err := outgoing.Pending(context.Background(), f.senderDB, f.sender, 10)
	if err != nil || len(pending) != 1 || pending[0].Attempts != 1 {
		t.Fatal("falha do estado perdeu evento ou tentativa")
	}
}
