package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"titansystem-backend/internal/onlinesessions"
)

// Deliberately does not use BodyParser: duplicate/unknown keys, null, nested
// values and trailing objects must never produce ambiguous credential changes.
func passwordInput(c *fiber.Ctx) (string, string, int) {
	body := c.Body()
	if len(body) > 2048 {
		return "", "", 413
	}
	media, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return "", "", 415
	}
	if !utf8.Valid(body) {
		return "", "", 400
	}
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return "", "", 400
	}
	values := map[string]string{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return "", "", 400
		}
		name, ok := key.(string)
		if !ok || (name != "current_password" && name != "new_password") {
			return "", "", 400
		}
		if _, exists := values[name]; exists {
			return "", "", 400
		}
		token, err = d.Token()
		value, ok := token.(string)
		if err != nil || !ok {
			return "", "", 400
		}
		values[name] = value
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') || len(values) != 2 {
		return "", "", 400
	}
	if _, err = d.Token(); err != io.EOF {
		return "", "", 400
	}
	return values["current_password"], values["new_password"], 0
}

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	current, next, status := passwordInput(c)
	if status != 0 {
		return c.SendStatus(status)
	}
	if err := onlinesessions.ValidatePasswordChange(current, next); err != nil {
		return passwordFailure(c, err)
	}
	store, err := sessionStore()
	if err != nil {
		return sessionFailure(c, err)
	}
	ctx, cancel := sessionContext(c)
	defer cancel()
	if err = store.ChangePassword(ctx, sessionScope(c), current, next); err != nil {
		return passwordFailure(c, err)
	}
	cookie := cookieRenovacao("", time.Unix(1, 0))
	cookie.MaxAge = -1
	c.Cookie(cookie)
	return c.SendStatus(204)
}

func passwordFailure(c *fiber.Ctx, err error) error {
	if errors.Is(err, onlinesessions.ErrPasswordPolicy) {
		return c.Status(400).JSON(fiber.Map{"error": onlinesessions.ErrPasswordPolicy.Error()})
	}
	return sessionFailure(c, err)
}
