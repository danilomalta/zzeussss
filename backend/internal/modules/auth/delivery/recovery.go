package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"io"
	"mime"
	"time"
	"titansystem-backend/internal/onlinesessions"
	"unicode/utf8"
)

func recoveryInput(c *fiber.Ctx, names ...string) (map[string]string, int) {
	c.Set("Cache-Control", "no-store")
	if len(c.Body()) > 2048 {
		return nil, 413
	}
	media, _, e := mime.ParseMediaType(c.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return nil, 415
	}
	if !utf8.Valid(c.Body()) || len(c.Request().URI().QueryString()) != 0 {
		return nil, 400
	}
	allowed := map[string]bool{}
	for _, n := range names {
		allowed[n] = true
	}
	d := json.NewDecoder(bytes.NewReader(c.Body()))
	tok, e := d.Token()
	if e != nil || tok != json.Delim('{') {
		return nil, 400
	}
	values := map[string]string{}
	for d.More() {
		tok, e = d.Token()
		n, ok := tok.(string)
		if e != nil || !ok || !allowed[n] {
			return nil, 400
		}
		if _, ok = values[n]; ok {
			return nil, 400
		}
		tok, e = d.Token()
		v, ok := tok.(string)
		if e != nil || !ok {
			return nil, 400
		}
		values[n] = v
	}
	if tok, e = d.Token(); e != nil || tok != json.Delim('}') || len(values) != len(names) {
		return nil, 400
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, 400
	}
	return values, 0
}
func (h *AuthHandler) IssueRecovery(c *fiber.Ctx) error {
	v, status := recoveryInput(c, "current_password")
	if status != 0 {
		return c.SendStatus(status)
	}
	s, e := sessionStore()
	if e != nil {
		return sessionFailure(c, e)
	}
	ctx, cancel := sessionContext(c)
	defer cancel()
	key, e := s.IssueRecovery(ctx, sessionScope(c), v["current_password"])
	if errors.Is(e, onlinesessions.ErrRecoveryRate) {
		return c.SendStatus(429)
	}
	if e != nil {
		return sessionFailure(c, e)
	}
	return c.Status(201).JSON(key)
}
func (h *AuthHandler) Recover(c *fiber.Ctx) error {
	v, status := recoveryInput(c, "recovery_key", "new_password")
	if status != 0 {
		return c.SendStatus(status)
	}
	s, e := sessionStore()
	if e != nil {
		return sessionFailure(c, e)
	}
	ctx, cancel := sessionContext(c)
	defer cancel()
	if e = s.Recover(ctx, v["recovery_key"], v["new_password"]); e != nil {
		return passwordFailure(c, e)
	}
	cookie := cookieRenovacao("", time.Unix(1, 0))
	cookie.MaxAge = -1
	c.Cookie(cookie)
	return c.SendStatus(204)
}
