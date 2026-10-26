package traderelay

import (
	"os"
	"syscall"
)

func lockState(name string) (func(), error) {
	fd, e := syscall.Open(name, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if e != nil {
		return nil, ErrInvalid
	}
	f := os.NewFile(uintptr(fd), name)
	i, e := f.Stat()
	if e != nil || !i.Mode().IsRegular() || i.Mode().Perm() != 0600 {
		f.Close()
		return nil, ErrInvalid
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, ErrInvalid
	}
	return func() { _ = syscall.Flock(fd, syscall.LOCK_UN); _ = f.Close() }, nil
}
