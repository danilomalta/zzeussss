package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) productionContext(c *fiber.Ctx) fiscalproduction.RequestContext {
	ss := c.Locals("session").(localauth.Session)
	return fiscalproduction.RequestContext{DB: s.DB, License: s.contracts, Actor: ss.Actor, Device: ss.Device, IntentID: c.Params("id"), Token: ss.Token}
}
func productionWrite(c *fiber.Ctx) bool {
	c.Set(fiber.HeaderCacheControl, "no-store")
	return noBagQuery(c) && len(c.Body()) <= fiscalcore.MaxXML
}
func productionFailure(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, fiscalproduction.ErrUnavailable), errors.Is(err, fiscalcore.ErrCredential):
		return c.Status(503).JSON(fiber.Map{"error": "fiscal_production_unavailable"})
	case errors.Is(err, fiscalcore.ErrRecorded):
		return c.Status(409).JSON(fiber.Map{"error": "fiscal_original_operation_requires_recovery"})
	case errors.Is(err, fiscal.ErrRestored):
		return c.Status(409).JSON(fiber.Map{"error": "fiscal_restore_requires_reconciliation"})
	case errors.Is(err, fiscalcore.ErrInput), errors.Is(err, fiscalcore.ErrUnsupported):
		return c.Status(400).JSON(fiber.Map{"error": "invalid_fiscal_production_input"})
	case errors.Is(err, fiscalcore.ErrSchema), errors.Is(err, fiscalcore.ErrXML), errors.Is(err, fiscalcore.ErrRule):
		return c.Status(422).JSON(fiber.Map{"error": "fiscal_document_validation_failed"})
	case errors.Is(err, fiscalcore.ErrUncertain):
		return c.Status(202).JSON(fiber.Map{"state": "uncertain", "requires_query": true})
	}
	return fiscalFailure(c, err)
}

func (s *Server) prepareProduction(c *fiber.Ctx) error {
	if !productionWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if s.production == nil {
		return productionFailure(c, fiscalproduction.ErrUnavailable)
	}
	var in struct {
		OperationID      string             `json:"operation_id"`
		ExpectedRevision int                `json:"expected_revision"`
		Invoice          fiscalcore.Invoice `json:"invoice"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	out, err := s.production.Prepare(c.UserContext(), s.productionContext(c), in.OperationID, in.ExpectedRevision, in.Invoice)
	if err != nil {
		return productionFailure(c, err)
	}
	return c.Status(201).JSON(out)
}
func (s *Server) approveProduction(c *fiber.Ctx) error {
	if !productionWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if s.production == nil {
		return productionFailure(c, fiscalproduction.ErrUnavailable)
	}
	var in struct {
		OperationID  string `json:"operation_id"`
		Revision     int    `json:"revision"`
		ReviewSHA256 string `json:"review_sha256"`
		TaxReference string `json:"tax_reference"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if err := s.production.Approve(c.UserContext(), s.productionContext(c), in.OperationID, in.Revision, in.ReviewSHA256, in.TaxReference); err != nil {
		return productionFailure(c, err)
	}
	return c.SendStatus(204)
}
func (s *Server) emitProduction(c *fiber.Ctx) error {
	if !productionWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if s.production == nil {
		return productionFailure(c, fiscalproduction.ErrUnavailable)
	}
	var in struct {
		OperationID  string `json:"operation_id"`
		Revision     int    `json:"revision"`
		ReviewSHA256 string `json:"review_sha256"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	out, err := s.production.Emit(c.UserContext(), s.productionContext(c), in.OperationID, in.Revision, in.ReviewSHA256)
	if errors.Is(err, fiscalcore.ErrUncertain) {
		return c.Status(202).JSON(out)
	}
	if err != nil {
		return productionFailure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) queryProduction(c *fiber.Ctx) error {
	if !productionWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if s.production == nil {
		return productionFailure(c, fiscalproduction.ErrUnavailable)
	}
	var in struct {
		OperationID string `json:"operation_id"`
		Revision    int    `json:"revision"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	out, err := s.production.Query(c.UserContext(), s.productionContext(c), in.OperationID, in.Revision)
	if err != nil {
		return productionFailure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) readProduction(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	out, err := fiscal.ReadProduction(c.UserContext(), s.DB, a.Actor, a.Device, a.IntentID, a.Token)
	if err != nil {
		return productionFailure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) readProductionXML(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	out, err := fiscal.ReadProduction(c.UserContext(), s.DB, a.Actor, a.Device, a.IntentID, a.Token)
	if err != nil {
		return productionFailure(c, err)
	}
	if out.Evidence.State != "authorized" || len(out.Evidence.ProcessedXML) == 0 {
		return c.Status(409).JSON(fiber.Map{"error": "fiscal_authorized_xml_unavailable"})
	}
	c.Set(fiber.HeaderContentType, "application/xml; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="NFe-`+out.Evidence.Key+`.xml"`)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Titan-Production-Revision", strconv.Itoa(out.EvidenceRevision))
	return c.Send(out.Evidence.ProcessedXML)
}

func (s *Server) cancelProduction(c *fiber.Ctx) error {
	if !productionWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if s.production == nil {
		return productionFailure(c, fiscalproduction.ErrUnavailable)
	}
	var in struct {
		OperationID       string `json:"operation_id"`
		QueryOperationID  string `json:"query_operation_id"`
		Reason            string `json:"reason"`
		ApprovalReference string `json:"approval_reference"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	out, err := s.production.Cancel(c.UserContext(), s.productionContext(c), in.OperationID, in.QueryOperationID, in.Reason, in.ApprovalReference)
	if errors.Is(err, fiscalcore.ErrUncertain) {
		return c.Status(202).JSON(out)
	}
	if err != nil {
		return productionFailure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) readProductionCancellation(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	out, err := fiscal.ReadProductionCancellation(c.UserContext(), s.DB, a.Actor, a.Device, a.IntentID, a.Token)
	if err != nil {
		return productionFailure(c, err)
	}
	return c.JSON(out)
}

func (s *Server) readProductionCancellationXML(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	out, err := fiscal.ReadProductionCancellation(c.UserContext(), s.DB, a.Actor, a.Device, a.IntentID, a.Token)
	if err != nil {
		return productionFailure(c, err)
	}
	if out.State != "cancellation_indicated_production" || len(out.ProcessedXML) == 0 {
		return c.Status(409).JSON(fiber.Map{"error": "fiscal_registered_cancellation_xml_unavailable"})
	}
	c.Set(fiber.HeaderContentType, "application/xml; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="Cancelamento-`+out.Key+`.xml"`)
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(out.ProcessedXML)
}
