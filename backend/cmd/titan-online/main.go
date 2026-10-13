package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"titansystem-backend/internal/core/database"
	"titansystem-backend/internal/onlinecatalog"
	"titansystem-backend/internal/onlinesessions"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func run(args []string) error {
	if len(args) != 1 || (args[0] != "migrate-sessions" && args[0] != "check-sessions" && args[0] != "migrate-catalog" && args[0] != "check-catalog" && args[0] != "migrate-discount-reviews" && args[0] != "check-discount-reviews" && args[0] != "migrate-costs" && args[0] != "check-costs" && args[0] != "migrate-stock-settings" && args[0] != "check-stock-settings" && args[0] != "migrate-replenishment-proposals" && args[0] != "check-replenishment-proposals" && args[0] != "migrate-suppliers" && args[0] != "check-suppliers" && args[0] != "migrate-supplier-offers" && args[0] != "check-supplier-offers" && args[0] != "migrate-purchase-drafts" && args[0] != "check-purchase-drafts" && args[0] != "migrate-purchase-documents" && args[0] != "check-purchase-documents") {
		return fmt.Errorf("uso: titan-online migrate-sessions | check-sessions | migrate-catalog | check-catalog | migrate-discount-reviews | check-discount-reviews | migrate-costs | check-costs | migrate-stock-settings | check-stock-settings | migrate-replenishment-proposals | check-replenishment-proposals | migrate-suppliers | check-suppliers | migrate-supplier-offers | check-supplier-offers | migrate-purchase-drafts | check-purchase-drafts | migrate-purchase-documents | check-purchase-documents")
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
	if args[0] == "migrate-purchase-documents" {
		return onlinecatalog.MigratePurchaseDocuments(ctx, db)
	}
	if args[0] == "check-purchase-documents" {
		return onlinecatalog.CheckPurchaseDocuments(ctx, db)
	}
	if args[0] == "migrate-purchase-drafts" {
		return onlinecatalog.MigratePurchaseDrafts(ctx, db)
	}
	if args[0] == "check-purchase-drafts" {
		return onlinecatalog.CheckPurchaseDrafts(ctx, db)
	}
	if args[0] == "migrate-supplier-offers" {
		return onlinecatalog.MigrateSupplierOffers(ctx, db)
	}
	if args[0] == "check-supplier-offers" {
		return onlinecatalog.CheckSupplierOffers(ctx, db)
	}
	if args[0] == "migrate-suppliers" {
		return onlinecatalog.MigrateSuppliers(ctx, db)
	}
	if args[0] == "check-suppliers" {
		return onlinecatalog.CheckSuppliers(ctx, db)
	}
	if args[0] == "migrate-replenishment-proposals" {
		return onlinecatalog.MigrateReplenishmentProposals(ctx, db)
	}
	if args[0] == "check-replenishment-proposals" {
		return onlinecatalog.CheckReplenishmentProposals(ctx, db)
	}
	if args[0] == "migrate-stock-settings" {
		return onlinecatalog.MigrateStockSettings(ctx, db)
	}
	if args[0] == "check-stock-settings" {
		return onlinecatalog.CheckStockSettings(ctx, db)
	}
	if args[0] == "migrate-costs" {
		return onlinecatalog.MigrateCosts(ctx, db)
	}
	if args[0] == "check-costs" {
		return onlinecatalog.CheckCosts(ctx, db)
	}
	if args[0] == "migrate-discount-reviews" {
		return onlinecatalog.MigrateDiscountReviews(ctx, db)
	}
	if args[0] == "check-discount-reviews" {
		return onlinecatalog.CheckDiscountReviews(ctx, db)
	}
	if args[0] == "migrate-catalog" {
		if err = onlinecatalog.Migrate(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.MigrateCreation(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.MigrateBatches(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.MigrateUndo(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.MigrateImports(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.MigrateAdjustments(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.MigrateBarcodes(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.MigrateBarcodeBatches(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckUndo(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckImports(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckAdjustments(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckBarcodes(ctx, db); err != nil {
			return err
		}
		return onlinecatalog.CheckBarcodeBatches(ctx, db)
	}
	if args[0] == "check-catalog" {
		if err = onlinecatalog.CheckSchema(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckCreation(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckBatches(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckUndo(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckImports(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckAdjustments(ctx, db); err != nil {
			return err
		}
		if err = onlinecatalog.CheckBarcodes(ctx, db); err != nil {
			return err
		}
		return onlinecatalog.CheckBarcodeBatches(ctx, db)
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
