package platform

import "fmt"

func RelaunchTray(args []string, hidden bool) error {
	if err := RelaunchElevated(args, hidden); err != nil {
		return fmt.Errorf("could not start elevated (UAC declined?): %w", err)
	}
	return nil
}
