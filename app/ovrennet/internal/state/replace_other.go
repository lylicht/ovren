//go:build !windows

package state

import "os"

func replaceFile(tmp, dst string) error { return os.Rename(tmp, dst) }
