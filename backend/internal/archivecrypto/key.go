// Package archivecrypto shares the existing private-file/AES-GCM policy of
// customer backups and local fiscal archives. It has no database dependencies.
package archivecrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"io"
	"os"
	"titansystem-backend/internal/privatefile"
)

var ErrKey = errors.New("private archive key unavailable")

func NewCipher(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, ErrKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrKey
	}
	return cipher.NewGCM(block)
}

// ReadKey accepts only a private regular 32-byte key, never a symlink.
func ReadKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !privatefile.Regular(path, info) || info.Size() != 32 {
		return nil, ErrKey
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrKey
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrKey
	}
	key, err := io.ReadAll(io.LimitReader(f, 33))
	if err != nil || len(key) != 32 {
		return nil, ErrKey
	}
	return key, nil
}
