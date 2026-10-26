//go:build linux

package traderelay

import "golang.org/x/sys/unix"

func publishQueueDirectory(from, to string) error {
	return unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
}
