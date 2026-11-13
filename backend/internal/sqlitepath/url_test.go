package sqlitepath

import (
	"net/url"
	"path/filepath"
	"testing"
)

func TestLocalFileURLIdentityAndEscaping(t *testing.T) {
	cases := []struct {
		path    string
		windows bool
		want    string
	}{
		{`D:\customer\vault.sqlite`, true, "file:///D:/customer/vault.sqlite"},
		{`C:/synthetic/a #?%.sqlite`, true, "file:///C:/synthetic/a%20%23%3F%25.sqlite"},
		{`\\?\D:\customer\vault.sqlite`, true, "file:///D:/customer/vault.sqlite"},
		{"/synthetic/a #?%.sqlite", false, "file:///synthetic/a%20%23%3F%25.sqlite"},
	}
	for _, c := range cases {
		u, e := fileURL(c.path, c.windows)
		if e != nil || u.String() != c.want || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
			t.Fatal("file URI identity or escaping changed")
		}
		parsed, e := url.Parse(u.String())
		if e != nil || parsed.Host != "" || parsed.Path != u.Path {
			t.Fatal("URI changed identity on parse")
		}
	}
	for _, p := range []string{`C:relative.sqlite`, `\\server\share\db.sqlite`, `file://host/db`, `relative.sqlite`, "D:/bad\x00.sqlite"} {
		if _, e := fileURL(p, true); e == nil {
			t.Fatal("nonlocal or ambiguous Windows SQLite path accepted")
		}
	}
	path := filepath.Join(t.TempDir(), "native.sqlite")
	u, e := URL(path)
	if e != nil || u.Host != "" {
		t.Fatal("native absolute path refused")
	}
}
