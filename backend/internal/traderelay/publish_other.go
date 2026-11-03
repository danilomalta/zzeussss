//go:build !linux

package traderelay

func publishQueueDirectory(string, string) error { return ErrInvalid }
