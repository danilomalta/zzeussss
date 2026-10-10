package delivery

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"titansystem-backend/internal/onlinecatalog"
)

func barcodeCSVError(c *fiber.Ctx, e error) error {
	var diagnostic *onlinecatalog.BarcodeCSVError
	if errors.As(e, &diagnostic) {
		return c.Status(400).JSON(fiber.Map{"erro": "CSV de códigos inválido; corrija a linha indicada e envie uma nova prévia", "csv_error": diagnostic})
	}
	return managementError(c, e)
}
func ImportarCodigosCSV(apply bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !noManagementQuery(c) {
			return managementError(c, onlinecatalog.ErrInput)
		}
		b, e := onlinecatalog.DecodeBarcodeCSV(c.Get("Content-Type"), c.Body(), apply)
		if e != nil {
			return barcodeCSVError(c, e)
		}
		ctx, cancel, s, a, e := managementContext(c)
		defer cancel()
		if e != nil {
			return managementError(c, e)
		}
		if apply {
			r, e := s.ApplyBarcodeBatch(ctx, a, b)
			if e != nil {
				return managementError(c, e)
			}
			return c.JSON(r)
		}
		p, e := s.PreviewBarcodeBatch(ctx, a, b)
		if e != nil {
			return managementError(c, e)
		}
		return c.JSON(p)
	}
}
func ModeloCodigosCSV(c *fiber.Ctx) error {
	if !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	model, e := s.BarcodeCSVTemplate(ctx, a)
	if e != nil {
		return managementError(c, e)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="titan-gtin-modelo.csv"`)
	c.Set("X-Content-Type-Options", "nosniff")
	return c.SendString(model)
}

func ExportarCodigosCSV(c *fiber.Ctx) error {
	product, e := onlinecatalog.ProductID(c.Params("id"))
	if e != nil || !noManagementQuery(c) || len(c.Body()) != 0 {
		return managementError(c, onlinecatalog.ErrInput)
	}
	ctx, cancel, s, a, e := managementContext(c)
	defer cancel()
	if e != nil {
		return managementError(c, e)
	}
	r, e := s.ExportBarcodeCSV(ctx, a, product)
	if e != nil {
		return managementError(c, e)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="titan-gtin-conferencia.csv"`)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Catalog-Product-ID", strconv.FormatInt(r.ProductID, 10))
	c.Set("X-Catalog-Product-Version", strconv.FormatInt(r.ProductVersion, 10))
	c.Set("X-Catalog-Row-Count", strconv.Itoa(r.Count))
	c.Set("X-Catalog-Source-Hash", r.SourceHash)
	return c.SendString(r.CSV)
}
