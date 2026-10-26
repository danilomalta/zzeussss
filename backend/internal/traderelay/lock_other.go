//go:build !linux

package traderelay

// Other operating systems require a tested durable lock implementation.
func lockState(string) (func(), error) { return nil, ErrInvalid }
