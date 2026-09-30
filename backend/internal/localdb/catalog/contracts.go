package catalog

import (
	"context"
	"database/sql"

	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
)

// CreateProductWithContract requires an active inventory contract and stock
// permission inside the same transaction that inserts the product.
// The caller must resolve the human session and prove the device first.
func CreateProductWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in ProductInput) (string, error) {
	return createProduct(ctx, db, actor, device, in, inventoryAuthorization(contracts))
}

// CreateLocationWithContract applies the same policy to stock locations.
// Neither the required module nor permission can be supplied by an HTTP client.
func CreateLocationWithContract(ctx context.Context, db *sql.DB, contracts *entitlementstore.Store, actor identity.Scope, device identity.DeviceContext, in LocationInput) (string, error) {
	return createLocation(ctx, db, actor, device, in, inventoryAuthorization(contracts))
}

func inventoryAuthorization(contracts *entitlementstore.Store) catalogAuthorization {
	return func(ctx context.Context, tx *sql.Tx, actor identity.Scope, device identity.DeviceContext) error {
		return contracts.RequireTx(ctx, tx, actor, device, identity.ManageStock, modules.Inventory)
	}
}
