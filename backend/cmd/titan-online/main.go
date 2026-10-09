package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/onlinecatalog"
	"titansystem-backend/internal/onlinesessions"
)

func run(args []string) error {
	if len(args) != 1 || (args[0] != "migrate-sessions" && args[0] != "check-sessions" && args[0] != "migrate-catalog" && args[0] != "check-catalog") {
		return fmt.Errorf("uso: titan-online migrate-sessions | check-sessions | migrate-catalog | check-catalog")
	}
	// Only an explicit invocation of this maintenance command applies schema.
	_ = godotenv.Load()
	pool, err := database.ConnectDB()
	if err != nil {
		return err
	}
	defer pool.Close()
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if args[0] == "migrate-catalog" {
		if err = onlinecatalog.Migrate(ctx, db); err != nil {
			return err
		}
		return onlinecatalog.CheckSchema(ctx, db)
	}
	if args[0] == "check-catalog" {
		return onlinecatalog.CheckSchema(ctx, db)
	}
	if args[0] == "migrate-sessions" {
		if err = onlinesessions.Migrate(ctx, db); err != nil {
			return err
		}
	}
	return onlinesessions.CheckSchema(ctx, db)
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Esquema solicitado verificado.")
}
