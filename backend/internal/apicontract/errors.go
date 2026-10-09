// Package apicontract contains compatibility rules shared by both HTTP APIs.
package apicontract

import (
	"errors"
	"github.com/gofiber/fiber/v2"
)

const ErrorFormatHeader = "X-Titan-Error-Format"

// FrameworkError is an explicit opt-in for server-level failures that occur
// before route middleware (oversized body/header, malformed HTTP, timeout).
// It never returns parser diagnostics or an arbitrary error's text.
func FrameworkError(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	var fe *fiber.Error
	if errors.As(err, &fe) && fe.Code >= 400 && fe.Code <= 599 {
		status = fe.Code
	}
	code, message := errorText(status)
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Vary(ErrorFormatHeader)
	if c.Get(ErrorFormatHeader) == "v1" {
		var body ErrorBody
		body.Error.Code, body.Error.Message = code, message
		return c.Status(status).JSON(body)
	}
	return c.Status(status).JSON(fiber.Map{"error": message})
}

type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func errorText(status int) (string, string) {
	switch status {
	case 400:
		return "invalid_request", "Confira os campos e parâmetros da solicitação."
	case 401:
		return "unauthorized", "Autenticação necessária ou inválida."
	case 403:
		return "forbidden", "Operação não autorizada."
	case 404:
		return "not_found", "Recurso não encontrado."
	case 405:
		return "method_not_allowed", "Método não permitido."
	case 409:
		return "conflict", "Operação em conflito; consulte seu estado antes de tentar novamente."
	case 413:
		return "payload_too_large", "Solicitação acima do limite permitido."
	case 415:
		return "unsupported_media_type", "Formato de conteúdo não aceito."
	case 422:
		return "unprocessable_request", "Solicitação não pode ser processada."
	case 428:
		return "precondition_required", "Identidade de operação obrigatória; salve a chave antes do envio."
	case 429:
		return "rate_limited", "Limite de solicitações atingido; aguarde antes de tentar novamente."
	case 501:
		return "not_implemented", "Funcionalidade ainda indisponível."
	case 503:
		return "unavailable", "Serviço indisponível; consulte o estado da operação antes de tentar novamente."
	default:
		if status >= 500 {
			return "internal_error", "Falha interna; consulte o estado da operação antes de tentar novamente."
		}
		return "request_rejected", "Solicitação recusada."
	}
}

// Errors is opt-in, retains status/cookies/retry headers, and never serializes
// handler errors, request fields or database diagnostics. It does not retry.
func Errors() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Locals("titan.error-format.installed") != nil {
			return c.Next()
		}
		c.Locals("titan.error-format.installed", true)
		c.Vary(ErrorFormatHeader)
		if c.Get(ErrorFormatHeader) != "v1" {
			return c.Next()
		}
		err := c.Next()
		if err != nil {
			status := fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) && fe.Code >= 400 && fe.Code <= 599 {
				status = fe.Code
			}
			c.Status(status)
		}
		status := c.Response().StatusCode()
		if status < 400 {
			return err
		}
		var body ErrorBody
		body.Error.Code, body.Error.Message = errorText(status)
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Status(status).JSON(body)
	}
}
