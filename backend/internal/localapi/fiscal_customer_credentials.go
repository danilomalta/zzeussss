package localapi

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/fiscalproduction"
	"titansystem-backend/internal/localdb/fiscal"
)

func (s *Server) fiscalCustomerCredentials(c *fiber.Ctx) error {
	if c.Method() == "GET" {
		if !fiscalRead(c) || !noBagQuery(c) {
			return fiscalFailure(c, fiscal.ErrInvalid)
		}
		out, err := s.fiscalProviders.ReadCustomerCredentials(c.UserContext(), s.productionContext(c), c.Params("id"))
		if err != nil {
			return fiscalFailure(c, err)
		}
		return c.JSON(out)
	}
	var in struct {
		OperationID      string `json:"operation_id"`
		ExpectedRevision int    `json:"expected_revision"`
		ProfileRevision  int    `json:"profile_revision"`
		Token            string `json:"token"`
		CSC              string `json:"csc"`
	}
	if !fiscalWrite(c) {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if _, err := exactSaleObject(c.Body(), []string{"operation_id", "expected_revision", "profile_revision", "token", "csc"}); err != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	if json.Unmarshal(c.Body(), &in) != nil {
		return fiscalFailure(c, fiscal.ErrInvalid)
	}
	token, csc := []byte(in.Token), []byte(in.CSC)
	defer clear(token)
	defer clear(csc)
	out, err := s.fiscalProviders.SaveCustomerCredentials(c.UserContext(), s.productionContext(c), c.Params("id"), fiscalproduction.CustomerCredentialInput{OperationID: in.OperationID, ExpectedRevision: in.ExpectedRevision, ProfileRevision: in.ProfileRevision, Token: token, CSC: csc})
	if err != nil {
		return fiscalFailure(c, err)
	}
	return c.JSON(out)
}
