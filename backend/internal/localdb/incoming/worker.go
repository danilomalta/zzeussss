package incoming

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/outgoing"
)

type WorkerConfig struct {
	DB          *sql.DB
	Source      identity.DeviceContext
	Destination identity.DeviceContext
	SigningKey  ed25519.PrivateKey
	Endpoint    string
	Client      *http.Client
	IdleDelay   time.Duration
	RetryBase   time.Duration
	RetryMax    time.Duration
	// Observe runs synchronously in the worker. It must return promptly and
	// must not expose private keys, tokens, payloads or raw internal errors.
	Observe func(WorkerNotice)
}

type WorkerNotice struct {
	Status               string
	EventID              string
	Failures             int64
	NextAttemptUnixMilli int64
}

// RunDeliveryWorker blocks until cancellation or a local configuration/trust
// error. It must run outside the PDV UI goroutine. It does not start itself,
// discover peers, provision devices, open listeners or mutate business tables.
func RunDeliveryWorker(ctx context.Context, config WorkerConfig) error {
	if ctx == nil || config.DB == nil || !validKey(config.SigningKey) {
		return ErrInvalid
	}
	if config.IdleDelay == 0 {
		config.IdleDelay = 2 * time.Second
	}
	if config.RetryBase == 0 {
		config.RetryBase = time.Second
	}
	if config.RetryMax == 0 {
		config.RetryMax = time.Minute
	}
	if config.IdleDelay < time.Millisecond || config.IdleDelay > time.Hour || config.RetryBase < time.Millisecond || config.RetryMax < config.RetryBase || config.RetryMax > time.Hour {
		return ErrInvalid
	}
	if !validWorkerEndpoint(config.Endpoint) {
		return ErrInvalid
	}
	config.SigningKey = append(ed25519.PrivateKey(nil), config.SigningKey...)
	if err := prepareWorker(ctx, config); err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		events, err := outgoing.Pending(ctx, config.DB, config.Source, 1)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if len(events) == 0 {
			if err := clearWorkerState(ctx, config, ""); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			notifyWorker(config, WorkerNotice{Status: "idle"})
			if !waitWorker(ctx, config.IdleDelay) {
				return nil
			}
			continue
		}
		eventID := events[0].EventID
		var storedEvent string
		var due, failures int64
		err = config.DB.QueryRowContext(ctx, `SELECT event_id,failures,next_attempt_unix_milli FROM sync_worker_state WHERE tenant_id=? AND store_id=? AND source_device_id=? AND destination_device_id=?`,
			config.Source.TenantID, config.Source.StoreID, config.Source.DeviceID, config.Destination.DeviceID).Scan(&storedEvent, &failures, &due)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if storedEvent == eventID && due > time.Now().UnixMilli() {
			delay := time.Until(time.UnixMilli(due))
			// Bound waiting if the wall clock moved backwards; recheck after it.
			if delay > config.RetryMax {
				delay = config.RetryMax
			}
			notifyWorker(config, WorkerNotice{Status: "waiting", EventID: eventID, Failures: failures, NextAttemptUnixMilli: due})
			if !waitWorker(ctx, delay) {
				return nil
			}
			continue
		}
		result, err := DeliverPendingOnce(ctx, config.DB, config.Source, config.SigningKey, config.Destination, config.Endpoint, config.Client)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			if err := clearWorkerState(ctx, config, eventID); err != nil {
				return err
			}
			if !result.Empty {
				notifyWorker(config, WorkerNotice{Status: "delivered", EventID: result.EventID})
			}
			continue
		}
		if errors.Is(err, ErrDenied) || errors.Is(err, identity.ErrDenied) || errors.Is(err, outgoing.ErrDenied) || errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) {
			return err
		}
		notice, err := recordWorkerFailure(ctx, config, eventID)
		if err != nil {
			return err
		}
		notifyWorker(config, notice)
	}
}

func validWorkerEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Path != SealedReceiverPath || u.RawPath != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && ip != nil && ip.IsLoopback()
}

func prepareWorker(ctx context.Context, c WorkerConfig) error {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := deliveryTrustTx(ctx, tx, c.Source, c.SigningKey, c.Destination); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_worker_state (tenant_id,store_id,source_device_id,destination_device_id,event_id,failures,next_attempt_unix_milli) VALUES (?,?,?,?, '',0,0) ON CONFLICT(tenant_id,store_id,source_device_id) DO NOTHING`,
		c.Source.TenantID, c.Source.StoreID, c.Source.DeviceID, c.Destination.DeviceID)
	if err != nil {
		return err
	}
	var destination string
	if err := tx.QueryRowContext(ctx, `SELECT destination_device_id FROM sync_worker_state WHERE tenant_id=? AND store_id=? AND source_device_id=?`, c.Source.TenantID, c.Source.StoreID, c.Source.DeviceID).Scan(&destination); err != nil {
		return err
	}
	if destination != c.Destination.DeviceID {
		return ErrConflict
	}
	return tx.Commit()
}

func recordWorkerFailure(ctx context.Context, c WorkerConfig, eventID string) (WorkerNotice, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return WorkerNotice{}, err
	}
	defer tx.Rollback()
	var previous string
	var failures int64
	err = tx.QueryRowContext(ctx, `SELECT event_id,failures FROM sync_worker_state WHERE tenant_id=? AND store_id=? AND source_device_id=? AND destination_device_id=?`,
		c.Source.TenantID, c.Source.StoreID, c.Source.DeviceID, c.Destination.DeviceID).Scan(&previous, &failures)
	if err != nil {
		return WorkerNotice{}, err
	}
	if previous != eventID {
		failures = 0
	}
	if failures < 63 {
		failures++
	}
	delay := c.RetryBase
	for n := int64(1); n < failures && delay < c.RetryMax; n++ {
		if delay > c.RetryMax/2 {
			delay = c.RetryMax
			break
		}
		delay *= 2
	}
	if delay > c.RetryMax {
		delay = c.RetryMax
	}
	due := time.Now().Add(delay).UnixMilli()
	_, err = tx.ExecContext(ctx, `UPDATE sync_worker_state SET event_id=?,failures=?,next_attempt_unix_milli=? WHERE tenant_id=? AND store_id=? AND source_device_id=? AND destination_device_id=?`,
		eventID, failures, due, c.Source.TenantID, c.Source.StoreID, c.Source.DeviceID, c.Destination.DeviceID)
	if err != nil {
		return WorkerNotice{}, err
	}
	if err := tx.Commit(); err != nil {
		return WorkerNotice{}, err
	}
	return WorkerNotice{Status: "deferred", EventID: eventID, Failures: failures, NextAttemptUnixMilli: due}, nil
}

func clearWorkerState(ctx context.Context, c WorkerConfig, eventID string) error {
	query := `UPDATE sync_worker_state SET event_id='',failures=0,next_attempt_unix_milli=0 WHERE tenant_id=? AND store_id=? AND source_device_id=? AND destination_device_id=?`
	args := []any{c.Source.TenantID, c.Source.StoreID, c.Source.DeviceID, c.Destination.DeviceID}
	if eventID != "" {
		query += " AND event_id=?"
		args = append(args, eventID)
	}
	_, err := c.DB.ExecContext(ctx, query, args...)
	return err
}

func notifyWorker(c WorkerConfig, notice WorkerNotice) {
	if c.Observe != nil {
		c.Observe(notice)
	}
}

func waitWorker(ctx context.Context, delay time.Duration) bool {
	if delay < 0 {
		delay = 0
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
