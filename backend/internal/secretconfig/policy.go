// Package secretconfig rejects publicly documented example credentials. It
// never supplies fallback values, generates shared keys, or renders a secret.
package secretconfig

import "strings"

func Unsafe(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	// Deny-list only: these public examples must never become runtime defaults.
	switch normalized {
	case "", "super_secret_key_change_in_prod", "titanpass":
		return true
	}
	return false
}
