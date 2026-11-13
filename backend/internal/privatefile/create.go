package privatefile

import (
	"errors"
	"os"
)

var ErrPrivate = errors.New("private file permissions unavailable")

// OpenNew never replaces a file. Validate the Windows inheritance boundary
// before creation, and the new object before writing any private content.
func OpenNew(path string, flags int) (*os.File, error) {
	if !Parent(path) {
		return nil, ErrPrivate
	}
	f, err := os.OpenFile(path, flags|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !Regular(path, info) {
		f.Close()
		return nil, ErrPrivate
	}
	return f, nil
}
