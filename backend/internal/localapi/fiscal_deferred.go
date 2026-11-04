package localapi

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"strings"
	"titansystem-backend/internal/fiscalcore"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
	"unicode"
	"unicode/utf8"
)

func (s *Server) mountDeferredFiscal(r fiber.Router) {
	r.Post("/fiscal/deferred/sales/:id/emit", s.emitDeferredFiscal)
	r.Post("/fiscal/deferred/sales/:id/query", s.queryDeferredFiscal)
}
func (s *Server) emitDeferredFiscal(c *fiber.Ctx) error {
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	var in struct {
		OperationID string `json:"operation_id"`
		Code        string `json:"payment_code"`
		Nature      string `json:"nature"`
		Presence    string `json:"presence"`
		Confirmed   bool   `json:"confirmed"`
	}
	if fiscalcore.DecodeInput(c.Body(), &in) != nil || !in.Confirmed || (in.Code != "21" && in.Code != "91") || (in.Presence != "1" && in.Presence != "4") || strings.TrimSpace(in.Nature) != in.Nature || !utf8.ValidString(in.Nature) || utf8.RuneCountInString(in.Nature) < 1 || utf8.RuneCountInString(in.Nature) > 60 {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	for _, letter := range in.Nature {
		if unicode.IsControl(letter) {
			return fiscalFailure(c, fiscal.ErrInvalid)
		}
	}
	a := s.productionContext(c)
	p, _, err := s.fiscalProviders.CustomerDeferredNFCeProvider(c.UserContext(), a)
	if err != nil {
		return nfe55Failure(c, err)
	}
	if err = p.ValidateCheckout(c.UserContext(), a, 2); err != nil {
		return nfe55Failure(c, err)
	}
	if err = fiscal.SelectDeferredPayment(c.UserContext(), s.DB, s.contracts, a.Actor, a.Device, a.Token, c.Params("id"), in.OperationID, in.Code); err != nil {
		return fiscalFailure(c, err)
	}
	raw, _ := json.Marshal(fiscalcore.Invoice{Environment: 2, Nature: in.Nature, Presence: in.Presence})
	out, err := p.Emit(c.UserContext(), a, fiscalproduction.DocumentRequest{Model: "65", Environment: 2, SaleID: c.Params("id"), Invoice: raw})
	if err != nil && out.State == "" {
		return nfe55Failure(c, err)
	}
	return deferredFiscalResult(c, out)
}
func (s *Server) queryDeferredFiscal(c *fiber.Ctx) error {
	if !fiscalWrite(c) || string(c.Body()) != "{}" {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	a := s.productionContext(c)
	p, _, err := s.fiscalProviders.CustomerDeferredNFCeProvider(c.UserContext(), a)
	if err != nil {
		return nfe55Failure(c, err)
	}
	if _, err = fiscal.DeferredPaymentForSale(c.UserContext(), s.DB, s.contracts, a.Actor, a.Device, a.Token, c.Params("id")); err != nil {
		return fiscalFailure(c, err)
	}
	// Existing Focus recovery only GETs the original reference; never emits.
	out, err := p.Reprocess(c.UserContext(), a, fiscalproduction.DocumentRequest{Model: "65", Environment: 2, SaleID: c.Params("id")})
	if err != nil && out.State == "" {
		return nfe55Failure(c, err)
	}
	return deferredFiscalResult(c, out)
}
func deferredFiscalResult(c *fiber.Ctx, out fiscalproduction.DocumentResult) error {
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"sale_id": c.Params("id"), "state": out.State, "operation_id": out.OperationID, "key": out.Key, "error_code": out.ErrorCode, "message": out.Message, "archive_state": out.ArchiveState})
}
