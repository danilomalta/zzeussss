package apicontract

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"os"
	"strings"
)

// CheckInventory compares documentation with routes actually mounted by Fiber.
// It deliberately says nothing about business authorization or payload schemas.
func CheckInventory(app *fiber.App, file, api, prefix string) error {
	raw, e := os.ReadFile(file)
	if e != nil {
		return e
	}
	var doc struct {
		Routes []struct{ API, Method, Path, Authentication string }
	}
	if e = json.Unmarshal(raw, &doc); e != nil {
		return e
	}
	expected := map[string]bool{}
	for _, r := range doc.Routes {
		if r.API != api {
			continue
		}
		key := r.Method + " " + r.Path
		if expected[key] || !strings.HasPrefix(r.Path, prefix) || (r.Authentication != "public" && r.Authentication != "session") {
			return fmt.Errorf("invalid inventory: %s", key)
		}
		expected[key] = true
	}
	actual := map[string]bool{}
	for _, r := range app.GetRoutes(true) {
		if r.Method == "HEAD" || !strings.HasPrefix(r.Path, prefix) {
			continue
		}
		actual[r.Method+" "+r.Path] = true
	}
	if len(expected) == 0 {
		return fmt.Errorf("empty inventory: %s", api)
	}
	for k := range expected {
		if !actual[k] {
			return fmt.Errorf("documented route absent: %s", k)
		}
	}
	for k := range actual {
		if !expected[k] {
			return fmt.Errorf("undocumented route: %s", k)
		}
	}
	return nil
}
