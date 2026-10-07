package delivery

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRefreshOriginRejectsBrowserCSRFBeforeSessionLookup(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS","https://app.example,http://localhost:3000")
	for _,tc:=range []struct{ origin,site string; status int }{
		{"https://app.example","same-site",204},{"http://localhost:3000","same-origin",204},
		{"","",204},{"https://evil.example","",403},{"null","",403},{"https://app.example","cross-site",403},
	} {
		app:=fiber.New();app.Post("/refresh",RefreshOrigin,func(c *fiber.Ctx)error{return c.SendStatus(204)})
		req:=httptest.NewRequest("POST","/refresh",nil);req.Header.Set("Origin",tc.origin);req.Header.Set("Sec-Fetch-Site",tc.site)
		resp,err:=app.Test(req);if err!=nil {t.Fatal(err)};resp.Body.Close();if resp.StatusCode!=tc.status {t.Fatal("barreira de origem incorreta")}
	}
}
