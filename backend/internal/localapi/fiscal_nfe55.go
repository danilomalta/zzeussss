package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
	"titansystem-backend/internal/sqlitewrite"
)

type nfeRecipientInput struct {
	CNPJ    string `json:"cnpj"`
	CPF     string `json:"cpf"`
	Name    string `json:"name"`
	IE      string `json:"ie"`
	Address struct {
		Street     string `json:"street"`
		Number     string `json:"number"`
		District   string `json:"district"`
		CityCode   string `json:"city_code"`
		City       string `json:"city"`
		UF         string `json:"uf"`
		PostalCode string `json:"postal_code"`
	} `json:"address"`
}

func nfe55Failure(c *fiber.Ctx, err error) error {
	code := ""
	switch {
	// Preparing a choice can race the fiscal gate's contract-clock write.
	// The authoritative selection already uses BEGIN IMMEDIATE and bounded
	// retries. Never turn a losing SQLite writer into an opaque server error.
	case sqlitewrite.Contention(err):
		return fiscalFailure(c, fiscal.ErrConflict)
	case errors.Is(err, fiscalproduction.ErrCheckoutProduction):
		code = "production_blocked"
	case errors.Is(err, fiscalproduction.ErrFocusA3):
		code = "a3_unsupported"
	case errors.Is(err, fiscalproduction.ErrFocusNFeIncomplete):
		code = "input_missing"
	case errors.Is(err, fiscalproduction.ErrFocusInvalidationUnavailable):
		code = "invalidation_unavailable"
	case errors.Is(err, fiscalproduction.ErrReferencedOperationUnavailable):
		code = "referenced_operation_unavailable"
	case errors.Is(err, fiscalproduction.ErrFreightChargeUnavailable):
		code = "freight_charge_unavailable"
	case errors.Is(err, fiscalproduction.ErrNFSeUnavailable):
		code = "nfse_unavailable"
	case errors.Is(err, fiscalcore.ErrUnsupported):
		code = "tax_mapping_unsupported"
	case errors.Is(err, fiscalproduction.ErrUnavailable):
		code = "provider_unavailable"
	}
	if code != "" {
		return c.Status(409).JSON(fiber.Map{"error": code})
	}
	return fiscalFailure(c, err)
}
func (s *Server) fiscalNFe55Preview(c *fiber.Ctx) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	var in struct {
		PaymentCode string `json:"payment_code"`
		Confirmed   bool   `json:"confirmed"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil ||
		(in.PaymentCode == "" && in.Confirmed) ||
		(in.PaymentCode != "" && (!in.Confirmed || (in.PaymentCode != "21" && in.PaymentCode != "91"))) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	c.Set("Cache-Control", "no-store")
	if in.PaymentCode != "" {
		if err := s.fiscalProviders.PrepareCustomerDeferredNFe55(c.UserContext(), s.productionContext(c), c.Params("id"), in.PaymentCode); err != nil {
			return nfe55Failure(c, err)
		}
	}
	out, err := s.fiscalProviders.PreviewCustomerNFe55(c.UserContext(), s.productionContext(c), c.Params("id"))
	if err != nil {
		return nfe55Failure(c, err)
	}
	return c.JSON(out)
}
func (s *Server) fiscalNFe55Emit(c *fiber.Ctx) error   { return s.fiscalNFe55Action(c, "emit") }
func (s *Server) fiscalNFe55Query(c *fiber.Ctx) error  { return s.fiscalNFe55Action(c, "query") }
func (s *Server) fiscalNFe55Cancel(c *fiber.Ctx) error { return s.fiscalNFe55Action(c, "cancel") }
func (s *Server) fiscalNFe55Action(c *fiber.Ctx, action string) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	p, _, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), a)
	if err != nil {
		return nfe55Failure(c, err)
	}
	req := fiscalproduction.DocumentRequest{SaleID: c.Params("id"), Model: "55", Environment: 2}
	if action == "emit" {
		var in struct {
			Nature      string                           `json:"nature"`
			Purpose     string                           `json:"purpose"`
			FreightMode string                           `json:"freight_mode"`
			Transport   *fiscalproduction.FocusTransport `json:"transport,omitempty"`
			Recipient   nfeRecipientInput                `json:"recipient"`
		}
		if fiscalcore.DecodeInput(c.Body(), &in) != nil {
			return fiscalFailure(c, fiscal.ErrInvalid)
		}
		if in.Purpose == "2" || in.Purpose == "3" || in.Purpose == "4" {
			return nfe55Failure(c, fiscalproduction.ErrReferencedOperationUnavailable)
		}
		preview, err := s.fiscalProviders.PreviewCustomerNFe55(c.UserContext(), a, req.SaleID)
		if err != nil {
			return nfe55Failure(c, err)
		}
		if !preview.MappingSupported {
			return nfe55Failure(c, fiscalcore.ErrUnsupported)
		}
		var bindings []fiscal.SaleTaxBinding
		charged := false
		matched := false
		for _, item := range preview.Items {
			if item.Tax == nil {
				if in.FreightMode == "0" || in.FreightMode == "1" || in.FreightMode == "2" {
					return nfe55Failure(c, fiscalproduction.ErrFreightChargeUnavailable)
				}
				continue
			}
			bindings = append(bindings, *item.Tax)
			if item.Tax.Freight != nil {
				charged = true
				if item.Tax.Freight.FreightMode == in.FreightMode {
					matched = true
				}
			}
		}
		if charged {
			if !matched || fiscal.ValidateSaleFreightOrigin(bindings, req.SaleID) != nil {
				return nfe55Failure(c, fiscalproduction.ErrFreightChargeUnavailable)
			}
		} else if in.FreightMode == "0" || in.FreightMode == "1" || in.FreightMode == "2" {
			if !matched || fiscal.ValidateSaleFreightOrigin(bindings, req.SaleID) != nil {
				return nfe55Failure(c, fiscalproduction.ErrFreightChargeUnavailable)
			}
		}
		r := in.Recipient
		d := r.Address
		// This matches only the existing, frozen internal cash consumer-sale contract.
		invoice := fiscalcore.Invoice{Environment: 2, Nature: in.Nature, Direction: "1", Destination: "1", FinalConsumer: "1", Presence: "1", Recipient: &fiscalcore.Party{CNPJ: r.CNPJ, CPF: r.CPF, Name: r.Name, IE: r.IE, IEDestination: "9", Address: fiscalcore.Address{Street: d.Street, Number: d.Number, District: d.District, CityCode: d.CityCode, City: d.City, UF: d.UF, PostalCode: d.PostalCode}}}
		req.Invoice, _ = json.Marshal(invoice)
		req.Purpose = in.Purpose
		req.FreightMode = in.FreightMode
		req.Transport = in.Transport
	} else if action == "cancel" {
		var in struct {
			OperationID string `json:"operation_id"`
			Reason      string `json:"reason"`
			Confirmed   bool   `json:"confirmed"`
		}
		if fiscalcore.DecodeInput(c.Body(), &in) != nil || !in.Confirmed {
			return fiscalFailure(c, fiscal.ErrInvalid)
		}
		req.OperationID = in.OperationID
		req.Reason = in.Reason
		req.ApprovalReference = "explicit-owner:" + in.OperationID
	} else if string(c.Body()) != "{}" {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	var out fiscalproduction.DocumentResult
	switch action {
	case "emit":
		out, err = p.Emit(c.UserContext(), a, req)
	case "cancel":
		out, err = p.Cancel(c.UserContext(), a, req)
	default:
		out, err = p.Reprocess(c.UserContext(), a, req)
	}
	if err != nil && out.State == "" {
		return nfe55Failure(c, err)
	}
	// No request, recipient, certificate, token, URL or tax input is echoed.
	return c.JSON(fiber.Map{"archive_state": out.ArchiveState, "state": out.State, "message": out.Message, "error_code": out.ErrorCode, "key": out.Key, "intent_id": out.IntentID})
}
func (s *Server) fiscalNFe55DANFE(c *fiber.Ctx) error {
	if !fiscalRead(c) || !noBagQuery(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	p, _, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), a)
	if err != nil {
		return nfe55Failure(c, err)
	}
	downloader, ok := p.(interface {
		DANFE(context.Context, fiscalproduction.RequestContext, fiscalproduction.DocumentRequest) ([]byte, error)
	})
	if !ok {
		return nfe55Failure(c, fiscalproduction.ErrUnavailable)
	}
	data, err := downloader.DANFE(c.UserContext(), a, fiscalproduction.DocumentRequest{Model: "55", Environment: 2, SaleID: c.Params("id")})
	if err != nil {
		return nfe55Failure(c, err)
	}
	c.Set("Content-Disposition", `attachment; filename="nfe55-homologacao.pdf"`)
	c.Type("pdf")
	return c.Send(data)
}

func (s *Server) fiscalNFe55Correction(c *fiber.Ctx) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	provider, _, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), a)
	if err != nil {
		return nfe55Failure(c, err)
	}
	var input fiscalproduction.CorrectionInput
	if fiscalcore.DecodeInput(c.Body(), &input) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	p, ok := provider.(interface {
		Correct(context.Context, fiscalproduction.RequestContext, fiscalproduction.DocumentRequest, fiscalproduction.CorrectionInput) (fiscalproduction.CorrectionResult, error)
	})
	if !ok {
		return nfe55Failure(c, fiscalproduction.ErrUnavailable)
	}
	out, err := p.Correct(c.UserContext(), a, fiscalproduction.DocumentRequest{SaleID: c.Params("id"), Model: "55", Environment: 2}, input)
	if err != nil {
		return nfe55Failure(c, err)
	}
	return c.JSON(out)
}

// Query has no original text, reference, sequence, URL or issuer supplied by a client.
func (s *Server) fiscalNFe55CorrectionQuery(c *fiber.Ctx) error {
	if !fiscalWrite(c) || string(c.Body()) != "{}" {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	provider, _, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), a)
	if err != nil {
		return nfe55Failure(c, err)
	}
	p, ok := provider.(interface {
		QueryCorrection(context.Context, fiscalproduction.RequestContext, fiscalproduction.DocumentRequest) (fiscalproduction.CorrectionResult, error)
	})
	if !ok {
		return nfe55Failure(c, fiscalproduction.ErrUnavailable)
	}
	out, err := p.QueryCorrection(c.UserContext(), a, fiscalproduction.DocumentRequest{SaleID: c.Params("id"), Model: "55", Environment: 2})
	if err != nil {
		return nfe55Failure(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}

func (s *Server) fiscalNFe55Invalidate(c *fiber.Ctx) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	var in struct {
		Year      int    `json:"year"`
		Series    int    `json:"series"`
		First     int    `json:"first"`
		Last      int    `json:"last"`
		Reason    string `json:"reason"`
		Confirmed bool   `json:"confirmed"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil || !in.Confirmed || in.Year < 2006 || in.Year > 2099 || in.Series < 0 || in.Series > 889 || in.First < 1 || in.Last < in.First || len(in.Reason) < 15 || len(in.Reason) > 255 {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if s.fiscalProviders == nil {
		return nfe55Failure(c, fiscalproduction.ErrUnavailable)
	}
	if _, _, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), s.productionContext(c)); err != nil {
		return nfe55Failure(c, err)
	}
	return nfe55Failure(c, fiscalproduction.ErrFocusInvalidationUnavailable)
}

func (s *Server) fiscalNFe55BindOrigin(c *fiber.Ctx) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	var in fiscal.OriginBindInput
	if fiscalcore.DecodeInput(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	_, b, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), s.productionContext(c))
	if err != nil {
		return nfe55Failure(c, err)
	}
	in.ProfileID, in.ProfileRevision = b.ProfileID, b.ProfileRevision
	a := s.productionContext(c)
	out, err := fiscal.BindNFeOrigin(c.UserContext(), s.DB, s.contracts, a.Actor, a.Device, in, a.Token)
	if err != nil {
		return nfe55Failure(c, err)
	}
	status := 201
	if out.Repeated {
		status = 200
	}
	return c.Status(status).JSON(fiber.Map{"origin_id": out.ID, "operation_id": out.OperationID, "kind": out.Snapshot.Origin.Kind, "repeated": out.Repeated})
}

func (s *Server) fiscalNFe55OriginEmit(c *fiber.Ctx) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	p, _, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), a)
	if err != nil {
		return nfe55Failure(c, err)
	}
	rec, err := fiscal.ReadNFeOrigin(c.UserContext(), s.DB, a.Actor, a.Device, c.Params("id"))
	if err != nil {
		return nfe55Failure(c, err)
	}
	var in struct {
		Nature      string                           `json:"nature"`
		Purpose     string                           `json:"purpose"`
		FreightMode string                           `json:"freight_mode"`
		Transport   *fiscalproduction.FocusTransport `json:"transport,omitempty"`
		Recipient   nfeRecipientInput                `json:"recipient"`
		Confirmed   bool                             `json:"confirmed"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil || !in.Confirmed {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	o := rec.Snapshot.Origin
	if o == nil || in.Recipient.Name != o.RecipientName || in.Recipient.Address.UF != o.RecipientUF || in.Recipient.CNPJ != o.RecipientCNPJ || in.Recipient.CPF != o.RecipientCPF {
		return nfe55Failure(c, fiscalproduction.ErrReferencedOperationUnavailable)
	}
	direction := "1"
	switch o.Kind {
	case "inbound_receipt":
		if in.Purpose != "1" {
			return nfe55Failure(c, fiscalproduction.ErrReferencedOperationUnavailable)
		}
		direction = "0"
	case "customer_return":
		if in.Purpose != "4" {
			return nfe55Failure(c, fiscalproduction.ErrReferencedOperationUnavailable)
		}
	default:
		return nfe55Failure(c, fiscalproduction.ErrReferencedOperationUnavailable)
	}
	if in.FreightMode != o.FreightMode {
		return nfe55Failure(c, fiscalproduction.ErrFreightChargeUnavailable)
	}
	r := in.Recipient
	d := r.Address
	invoice := fiscalcore.Invoice{Environment: 2, Nature: in.Nature, Direction: direction, Destination: "1", FinalConsumer: "1", Presence: "1", Recipient: &fiscalcore.Party{CNPJ: r.CNPJ, CPF: r.CPF, Name: r.Name, IE: r.IE, IEDestination: "9", Address: fiscalcore.Address{Street: d.Street, Number: d.Number, District: d.District, CityCode: d.CityCode, City: d.City, UF: d.UF, PostalCode: d.PostalCode}}}
	raw, err := json.Marshal(invoice)
	if err != nil {
		return nfe55Failure(c, err)
	}
	req := fiscalproduction.DocumentRequest{OriginID: rec.ID, Model: "55", Environment: 2, Purpose: in.Purpose, FreightMode: in.FreightMode, Transport: in.Transport, Invoice: raw}
	out, err := p.Emit(c.UserContext(), a, req)
	if err != nil && out.State == "" {
		return nfe55Failure(c, err)
	}
	return c.JSON(fiber.Map{"archive_state": out.ArchiveState, "state": out.State, "message": out.Message, "error_code": out.ErrorCode, "key": out.Key, "intent_id": out.IntentID})
}

func (s *Server) fiscalNFSe(c *fiber.Ctx) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	var in struct {
		MunicipalityIBGE string `json:"municipality_ibge"`
		ServiceCode      string `json:"service_code"`
		Confirmed        bool   `json:"confirmed"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil || !in.Confirmed || len(in.MunicipalityIBGE) != 7 || len(in.ServiceCode) < 1 || len(in.ServiceCode) > 16 {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	for _, r := range in.MunicipalityIBGE {
		if r < '0' || r > '9' {
			return fiscalFailure(c, fiscal.ErrInvalid)
		}
	}
	if s.fiscalProviders != nil {
		if _, _, err := s.fiscalProviders.CustomerNFe55Provider(c.UserContext(), s.productionContext(c)); err != nil {
			return nfe55Failure(c, err)
		}
	}
	return nfe55Failure(c, fiscalproduction.ErrNFSeUnavailable)
}
