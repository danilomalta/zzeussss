// Package sqlitepath constructs local SQLite file URIs without treating a
// Windows drive letter as a URI authority. It does not change query policies.
package sqlitepath

import (
	"errors"
	"net/url"
	"runtime"
	"strings"
)

var ErrPath = errors.New("local SQLite file path required")

func URL(path string) (url.URL, error) { return fileURL(path, runtime.GOOS == "windows") }

func fileURL(path string, windows bool) (url.URL, error) {
	if strings.ContainsRune(path, 0) {
		return url.URL{}, ErrPath
	}
	if windows {
		path = strings.TrimPrefix(path, `\\?\`)
		path = strings.ReplaceAll(path, `\`, "/")
		// No relative drive paths or network SQLite file. Client backup archives
		// remain a separate concept; this helper is for opened SQLite databases.
		if len(path) < 3 || !((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) || path[1] != ':' || path[2] != '/' {
			return url.URL{}, ErrPath
		}
		path = "/" + path
	} else if !strings.HasPrefix(path, "/") {
		return url.URL{}, ErrPath
	}
	return url.URL{Scheme: "file", Path: path}, nil
}
