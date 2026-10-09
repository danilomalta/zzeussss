package migrations

import _ "embed"

//go:embed 000008_online_catalog_management.sql
var CatalogManagementSQL string

// CatalogProductsSQL and CatalogDiscountsSQL expose the existing additive DDL
// for isolated integration fixtures. No application startup executes these.
//
//go:embed 000002_create_products.sql
var CatalogProductsSQL string

//go:embed 000003_create_discount_suggestions.sql
var CatalogDiscountsSQL string
