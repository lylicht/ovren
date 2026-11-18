//go:build !windows

package platform

import "golang.org/x/sys/unix"

func OwnerProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := unix.Kill(pid, 0)
	return err == nil || err == unix.EPERM
}
