package state

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func replaceFile(tmp, dst string) error {
	deadline := time.Now().Add(time.Second)
	for {
		err := os.Rename(tmp, dst)
		if err == nil || time.Now().After(deadline) ||
			!errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}
