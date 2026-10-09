package migrations

import _ "embed"

//go:embed 000008_online_catalog_management.sql
var CatalogManagementSQL string

//go:embed 000009_online_catalog_creation.sql
var CatalogCreationSQL string

// CatalogProductsSQL and CatalogDiscountsSQL expose the existing additive DDL
// for isolated integration fixtures. No application startup executes these.
//
//go:embed 000002_create_products.sql
var CatalogProductsSQL string

//go:embed 000003_create_discount_suggestions.sql
var CatalogDiscountsSQL string

//go:embed 000010_online_catalog_batches.sql
var CatalogBatchesSQL string

//go:embed 000011_online_catalog_undo.sql
var CatalogUndoSQL string

//go:embed 000012_online_catalog_imports.sql
var CatalogImportsSQL string

//go:embed 000013_online_catalog_adjustments.sql
var CatalogAdjustmentsSQL string
