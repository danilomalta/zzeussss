package localapi

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/core/entitlements"
	"titansystem-backend/internal/core/modules"
	"titansystem-backend/internal/localdb/catalog"
	"titansystem-backend/internal/localdb/entitlementstore"
	"titansystem-backend/internal/localdb/identity"
	"titansystem-backend/internal/localdb/localauth"
)

func (s *Server) mountCatalog(router fiber.Router) {
	router.Get("/products", s.listProducts)
 router.Get("/catalog/search",s.searchCatalog)
	router.Post("/products", s.createProduct)
	router.Get("/locations", s.listLocations)
	router.Post("/locations", s.createLocation)
}

func (s *Server) listProducts(c *fiber.Ctx) error {
	limit, err := strconv.Atoi(c.Query("limit", "50"))
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	items, err := catalog.ListProducts(c.UserContext(), s.DB, session.Actor, session.Device, limit, offset)
	if err != nil {
		return catalogError(c, err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) createProduct(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	var input catalog.ProductInput
	if err := c.BodyParser(&input); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	id, err := catalog.CreateProductWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, input)
	if err != nil {
		return catalogError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (s *Server) listLocations(c *fiber.Ctx) error {
	session := c.Locals("session").(localauth.Session)
	items, err := catalog.ListLocations(c.UserContext(), s.DB, session.Actor, session.Device)
	if err != nil {
		return catalogError(c, err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (s *Server) createLocation(c *fiber.Ctx) error {
	if s.contracts == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	var input catalog.LocationInput
	if err := c.BodyParser(&input); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	session := c.Locals("session").(localauth.Session)
	id, err := catalog.CreateLocationWithContract(c.UserContext(), s.DB, s.contracts, session.Actor, session.Device, input)
	if err != nil {
		return catalogError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func catalogError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, identity.ErrDenied), errors.Is(err, entitlementstore.ErrNotInstalled),
		errors.Is(err, modules.ErrUnavailable), errors.Is(err, entitlements.ErrTrust),
		errors.Is(err, entitlements.ErrSignature), errors.Is(err, entitlements.ErrClaims),
		errors.Is(err, entitlements.ErrTenant), errors.Is(err, entitlements.ErrValidity):
		return c.SendStatus(fiber.StatusForbidden)
	case errors.Is(err, entitlementstore.ErrClockRollback):
		return c.SendStatus(fiber.StatusConflict)
	case errors.Is(err, catalog.ErrInvalidCatalog):
		return c.SendStatus(fiber.StatusBadRequest)
	default:
		return c.SendStatus(fiber.StatusInternalServerError)
	}
}
