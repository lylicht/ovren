//go:build !unix

package state

import "os"

func openLimitFile(path string) (*os.File, error) { return os.Open(path) }
