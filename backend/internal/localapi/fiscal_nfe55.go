package localapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
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
	case errors.Is(err, fiscalproduction.ErrCheckoutProduction):
		code = "production_blocked"
	case errors.Is(err, fiscalproduction.ErrFocusA3):
		code = "a3_unsupported"
	case errors.Is(err, fiscalproduction.ErrFocusNFeIncomplete):
		code = "input_missing"
	case errors.Is(err, fiscalproduction.ErrFocusInvalidationUnavailable):
		code = "invalidation_unavailable"
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
	if !fiscalWrite(c) || string(c.Body()) != "{}" {
		return fiscalFailure(c, fiscal.ErrInvalid)
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
			Nature      string            `json:"nature"`
			Purpose     string            `json:"purpose"`
			FreightMode string            `json:"freight_mode"`
			Recipient   nfeRecipientInput `json:"recipient"`
		}
		if fiscalcore.DecodeInput(c.Body(), &in) != nil {
			return fiscalFailure(c, fiscal.ErrInvalid)
		}
		preview, err := s.fiscalProviders.PreviewCustomerNFe55(c.UserContext(), a, req.SaleID)
		if err != nil {
			return nfe55Failure(c, err)
		}
		if !preview.MappingSupported {
			return nfe55Failure(c, fiscalcore.ErrUnsupported)
		}
		r := in.Recipient
		d := r.Address
		// This matches only the existing, frozen internal cash consumer-sale contract.
		invoice := fiscalcore.Invoice{Environment: 2, Nature: in.Nature, Direction: "1", Destination: "1", FinalConsumer: "1", Presence: "1", Recipient: &fiscalcore.Party{CNPJ: r.CNPJ, CPF: r.CPF, Name: r.Name, IE: r.IE, IEDestination: "9", Address: fiscalcore.Address{Street: d.Street, Number: d.Number, District: d.District, CityCode: d.CityCode, City: d.City, UF: d.UF, PostalCode: d.PostalCode}}}
		req.Invoice, _ = json.Marshal(invoice)
		req.Purpose = in.Purpose
		req.FreightMode = in.FreightMode
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
	return c.JSON(fiber.Map{"state": out.State, "message": out.Message, "error_code": out.ErrorCode, "key": out.Key, "intent_id": out.IntentID})
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
