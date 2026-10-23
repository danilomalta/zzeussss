package localapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"io"
	"strconv"
	"strings"
	"time"
	"titansystem-backend/internal/localdb/localauth"
	"titansystem-backend/internal/localdb/management"
	"titansystem-backend/internal/localdb/production"
	"titansystem-backend/internal/localdb/stock"
	"titansystem-backend/internal/localdb/stocktaking"
)

func (s *Server) mountManagementReports(r fiber.Router) {
	s.mountDockAgenda(r)
	g := r.Group("/management", func(c *fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	g.Get("/sales/products", s.managementSales)
	g.Get("/sales/products.csv", s.managementSales)
	g.Get("/stock/positions", s.managementStock)
	g.Get("/stock/positions.csv", s.managementStock)
	g.Get("/stock/valuation", s.managementStock)
	g.Get("/stock/valuation.csv", s.managementStock)
	g.Get("/inventories/:id/variance", s.managementVariance)
	g.Post("/production/simultaneous-plan", s.managementPlan)
	g.Post("/replenishment/preview", s.managementRestock)
	g.Post("/production/recipe-versions/:id/cost-scenario", s.managementCostScenario)
}

func (s *Server) managementRestock(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 4096 {
		return c.SendStatus(413)
	}
	var in stock.RestockPreviewInput
	if err := decodeCash(c.Body(), &in, []string{"product_id", "from", "to", "lead_days", "coverage_days", "safety_milli"}); err != nil {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	out, err := stock.PreviewRestock(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if errors.Is(err, stock.ErrAvailabilityReference) {
		return c.SendStatus(404)
	}
	if errors.Is(err, stock.ErrAvailability) {
		return c.SendStatus(409)
	}
	if err != nil {
		return stockError(c, err)
	}
	return c.JSON(out)
}
func managementError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, management.ErrInvalid):
		return c.Status(400).JSON(fiber.Map{"code": "report_invalid"})
	case errors.Is(err, management.ErrLimit):
		return c.Status(422).JSON(fiber.Map{"code": "report_source_limit", "max_source_rows": management.MaxSourceRows})
	case errors.Is(err, management.ErrIntegrity):
		return c.Status(409).JSON(fiber.Map{"code": "report_source_inconsistent"})
	default:
		return catalogError(c, err)
	}
}
func salesReportInput(c *fiber.Ctx) (management.SalesInput, error) {
	var in management.SalesInput
	if !inventoryQuery(c, "from", "to", "compare_from", "compare_to", "limit", "offset") {
		return in, management.ErrInvalid
	}
	var err error
	in.Limit, in.Offset, err = inventoryPage(c)
	if err != nil {
		return in, management.ErrInvalid
	}
	in.Current.From, err = time.Parse(time.RFC3339Nano, c.Query("from"))
	if err != nil {
		return in, management.ErrInvalid
	}
	in.Current.To, err = time.Parse(time.RFC3339Nano, c.Query("to"))
	if err != nil {
		return in, management.ErrInvalid
	}
	if c.Context().QueryArgs().Has("compare_from") || c.Context().QueryArgs().Has("compare_to") {
		in.Previous.From, err = time.Parse(time.RFC3339Nano, c.Query("compare_from"))
		if err != nil {
			return in, management.ErrInvalid
		}
		in.Previous.To, err = time.Parse(time.RFC3339Nano, c.Query("compare_to"))
		if err != nil {
			return in, management.ErrInvalid
		}
	} else {
		in.Previous.To = in.Current.From
		in.Previous.From = in.Current.From.Add(-in.Current.To.Sub(in.Current.From))
	}
	return in, nil
}
func (s *Server) managementSales(c *fiber.Ctx) error {
	in, err := salesReportInput(c)
	if err != nil {
		return managementError(c, err)
	}
	session := c.Locals("session").(localauth.Session)
	result, err := management.Sales(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if err != nil {
		return managementError(c, err)
	}
	if strings.HasSuffix(c.Path(), ".csv") {
		var b bytes.Buffer
		if err = management.ExportSales(&b, result); err != nil {
			return c.SendStatus(500)
		}
		c.Set("Content-Type", "text/csv; charset=utf-8")
		c.Set("Content-Disposition", `attachment; filename="vendas-produtos-pagina.csv"`)
		return c.Send(b.Bytes())
	}
	return c.JSON(result)
}
func (s *Server) managementStock(c *fiber.Ctx) error {
	if !inventoryQuery(c, "location_id", "product_id", "limit", "offset") {
		return c.SendStatus(400)
	}
	limit, offset, err := inventoryPage(c)
	if err != nil {
		return c.SendStatus(400)
	}
	if c.Context().QueryArgs().Has("product_id") && c.Query("product_id") == "" {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	valued := strings.Contains(c.Path(), "/valuation")
	result, err := stock.Positions(c.UserContext(), s.DB, session.Actor, session.Device, stock.PositionInput{LocationID: c.Query("location_id"), ProductID: c.Query("product_id"), Limit: limit, Offset: offset}, valued)
	if errors.Is(err, stock.ErrAvailabilityReference) {
		return c.SendStatus(404)
	}
	if errors.Is(err, stock.ErrAvailability) {
		return c.SendStatus(409)
	}
	if err != nil {
		return stockError(c, err)
	}
	if !strings.HasSuffix(c.Path(), ".csv") {
		return c.JSON(result)
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	header := []string{"product_id", "sku", "name", "unit", "location_id", "physical_milli", "reserved_milli", "free_milli", "measured_at", "basis"}
	if valued {
		header = append(header, "current_catalog_cost_cents", "value_cents_numerator", "value_cents_denominator", "requires_cost_review")
	}
	if err = w.Write(header); err != nil {
		return c.SendStatus(500)
	}
	for _, v := range result.Items {
		row := []string{reportCell(v.ProductID), reportCell(v.SKU), reportCell(v.Name), v.Unit, reportCell(v.LocationID), strconv.FormatInt(v.PhysicalMilli, 10), strconv.FormatInt(v.ReservedMilli, 10), strconv.FormatInt(v.FreeMilli, 10), result.MeasuredAt, result.Basis}
		if valued {
			row = append(row, strconv.FormatInt(*v.CatalogCostCents, 10), *v.ValueNumerator, *v.ValueDenominator, strconv.FormatBool(*v.CostRequiresReview))
		}
		if err = w.Write(row); err != nil {
			return c.SendStatus(500)
		}
	}
	w.Flush()
	if w.Error() != nil {
		return c.SendStatus(500)
	}
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="estoque-pagina.csv"`)
	return c.Send(b.Bytes())
}
func reportCell(s string) string {
	t := strings.TrimLeft(s, " \t\r\n")
	if t != "" && strings.ContainsRune("=+-@", rune(t[0])) {
		return "'" + s
	}
	return s
}
func (s *Server) managementVariance(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	session := c.Locals("session").(localauth.Session)
	r, err := stocktaking.Variance(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"))
	if err != nil {
		return inventoryError(c, err)
	}
	return c.JSON(r)
}

// Strict optional fields: omitted means unknown; supplied null/duplicates and
// unknown fields are invalid. The second pass decodes int64, never float64.
func decodeReportObject(body []byte, target any, required, optional []string) error {
	allowed := map[string]bool{}
	seen := map[string]bool{}
	for _, s := range append(append([]string{}, required...), optional...) {
		allowed[s] = true
	}
	d := json.NewDecoder(bytes.NewReader(body))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return management.ErrInvalid
	}
	for d.More() {
		t, err = d.Token()
		key, ok := t.(string)
		if err != nil || !ok || !allowed[key] || seen[key] {
			return management.ErrInvalid
		}
		seen[key] = true
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return management.ErrInvalid
		}
	}
	t, err = d.Token()
	if err != nil || t != json.Delim('}') {
		return management.ErrInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return management.ErrInvalid
	}
	for _, key := range required {
		if !seen[key] {
			return management.ErrInvalid
		}
	}
	return json.Unmarshal(body, target)
}
func (s *Server) managementPlan(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 32768 {
		return c.SendStatus(413)
	}
	var in production.SimultaneousInput
	if err := decodeCash(c.Body(), &in, []string{"location_id", "recipes"}); err != nil {
		return c.SendStatus(400)
	}
	var raw struct {
		Recipes []json.RawMessage `json:"recipes"`
	}
	if err := json.Unmarshal(c.Body(), &raw); err != nil || len(raw.Recipes) < 1 || len(raw.Recipes) > 20 {
		return c.SendStatus(400)
	}
	for _, r := range raw.Recipes {
		var v production.PlanRecipe
		if err := decodeCash(r, &v, []string{"version_id", "batches"}); err != nil {
			return c.SendStatus(400)
		}
	}
	session := c.Locals("session").(localauth.Session)
	result, err := production.Simultaneous(c.UserContext(), s.DB, session.Actor, session.Device, in)
	if err != nil {
		return capacityError(c, err)
	}
	return c.JSON(result)
}
func (s *Server) managementCostScenario(c *fiber.Ctx) error {
	if !inventoryQuery(c) {
		return c.SendStatus(400)
	}
	if len(c.Body()) > 8192 {
		return c.SendStatus(413)
	}
	var in production.CostScenarioInput
	if err := decodeReportObject(c.Body(), &in, []string{"batches"}, []string{"labor", "overhead_cents", "sale_price_cents"}); err != nil {
		return c.SendStatus(400)
	}
	if in.Labor != nil {
		var raw struct {
			Labor json.RawMessage `json:"labor"`
		}
		if err := json.Unmarshal(c.Body(), &raw); err != nil {
			return c.SendStatus(400)
		}
		var l production.LaborScenario
		if err := decodeCash(raw.Labor, &l, []string{"minutes", "workers", "hourly_cost_cents"}); err != nil {
			return c.SendStatus(400)
		}
	}
	session := c.Locals("session").(localauth.Session)
	result, err := production.RecipeCostScenario(c.UserContext(), s.DB, session.Actor, session.Device, c.Params("id"), in)
	if err != nil {
		return productionError(c, err)
	}
	return c.JSON(result)
}
