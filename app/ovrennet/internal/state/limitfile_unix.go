//go:build unix

package state

import (
	"os"
	"syscall"
)

func openLimitFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
