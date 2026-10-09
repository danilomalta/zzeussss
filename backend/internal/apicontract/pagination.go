package apicontract

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

var ErrPagination = errors.New("parametros de paginacao invalidos")

// PageQuery validates a bounded query without silently falling back on malformed
// values. Extra keys are permitted only when explicitly named by the handler.
func PageQuery(c *fiber.Ctx, extra ...string) (int, int, error) {
	allowed := map[string]bool{"limit": true, "offset": true}
	for _, key := range extra {
		allowed[key] = true
	}
	values := map[string]string{}
	invalid := false
	c.Context().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if _, seen := values[key]; seen || !allowed[key] {
			invalid = true
		}
		values[key] = string(v)
	})
	if invalid {
		return 0, 0, ErrPagination
	}
	parse := func(key string, fallback, min, max int) (int, error) {
		raw, present := values[key]
		if !present {
			return fallback, nil
		}
		if len(raw) == 0 || len(raw) > 10 {
			return 0, ErrPagination
		}
		for _, r := range raw {
			if r < '0' || r > '9' {
				return 0, ErrPagination
			}
		}
		n, e := strconv.ParseUint(raw, 10, 32)
		if e != nil || n < uint64(min) || n > uint64(max) {
			return 0, ErrPagination
		}
		return int(n), nil
	}
	limit, e := parse("limit", 50, 1, 100)
	if e != nil {
		return 0, 0, e
	}
	offset, e := parse("offset", 0, 0, 1000000000)
	if e != nil {
		return 0, 0, e
	}
	return limit, offset, nil
}
