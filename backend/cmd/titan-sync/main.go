package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"titansystem-backend/internal/localdb/incoming"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runSync(ctx, os.Args[1:], os.Stdout); err != nil {
		// No underlying error/URL/file contents are printed here.
		fmt.Fprintln(os.Stderr, "Titan sync: não foi possível iniciar ou manter o envio; confira configuração, aprovação dos aparelhos e acesso aos arquivos.")
		os.Exit(1)
	}
}

func runSync(ctx context.Context, args []string, output io.Writer) error {
	if ctx == nil || output == nil || len(args) == 0 || args[0] != "send" {
		return errConfiguration
	}
	options := flag.NewFlagSet("send", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	dbPath := options.String("db", "", "SQLite existente")
	stationPath := options.String("station", "", "arquivo privado de aparelho existente")
	configPath := options.String("config", "", "configuração privada de destino")
	once := options.Bool("once", false, "uma tentativa, sem worker contínuo")
	if err := options.Parse(args[1:]); err != nil {
		return errConfiguration
	}
	if *dbPath == "" || *stationPath == "" || *configPath == "" || options.NArg() != 0 {
		return errConfiguration
	}
	config, err := loadSyncConfig(*configPath)
	if err != nil {
		return err
	}
	station, key, err := loadSyncStation(*stationPath)
	if err != nil {
		return err
	}
	if station.DeviceID == config.DestinationDeviceID {
		return errConfiguration
	}
	db, device, err := openSyncVerified(ctx, *dbPath, station, key)
	if err != nil {
		return err
	}
	defer db.Close()
	destination := device
	destination.DeviceID = config.DestinationDeviceID
	if *once {
		result, err := incoming.DeliverPendingOnce(ctx, db, device, key, destination, config.Endpoint, nil)
		if err != nil {
			return err
		}
		if result.Empty {
			_, err = fmt.Fprintln(output, "Fila sem eventos pendentes.")
		} else {
			_, err = fmt.Fprintln(output, "Recepção durável confirmada; evento confirmado na outbox.")
		}
		return err
	}
	// This message reports invocation, not connection/health or delivery.
	if _, err := fmt.Fprintln(output, "Iniciando worker de envio; Ctrl+C encerra sem apagar a fila."); err != nil {
		return err
	}
	return incoming.RunDeliveryWorker(ctx, incoming.WorkerConfig{
		DB: db, Source: device, Destination: destination,
		SigningKey: append(ed25519.PrivateKey(nil), key...), Endpoint: config.Endpoint,
	})
}
