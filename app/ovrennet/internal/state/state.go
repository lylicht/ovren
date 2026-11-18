package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/lylicht/ovren/app/ovrennet/internal/engine"
)

const macStateDir = "/var/run/OvrenNET"

const macLimitsDir = "/Library/Application Support/OvrenNET"

type Status struct {
	PID             int
	Mode            string
	Enabled         bool
	V6              string
	PausedReason    string
	Config          Config
	AppliedConfigID string
	ConfigError     string
	Since           time.Time
	Updated         time.Time
	NICs            []engine.SlotStats
}

func DefaultDir() string {

	if runtime.GOOS == "darwin" {
		return macStateDir
	}
	if runtime.GOOS == "linux" {
		return "/run/OvrenNET"
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "OvrenNET")
}

func DefaultUserDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "OvrenNET")
}

func Prepare(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	ctl := filepath.Join(dir, "ctl")
	if err := os.MkdirAll(ctl, 0o700); err != nil {
		return err
	}
	return os.Chmod(ctl, 0o700)
}

func Write(dir string, st Status) {
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, "status-*.tmp")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(b); err != nil {
		tmp.Close()
		return
	}
	if err = tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return
	}
	if err = tmp.Close(); err == nil {
		os.Rename(tmpName, filepath.Join(dir, "status.json"))
	}
}

func Clear(dir string) { os.Remove(filepath.Join(dir, "status.json")) }

func Read(dir string) (*Status, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "status.json"))
	if err != nil {
		return nil, false
	}
	var st Status
	if json.Unmarshal(b, &st) != nil || time.Since(st.Updated) > 3*time.Second {
		return nil, false
	}
	return &st, true
}

func SendMarker(dir, name string) error {
	if err := Prepare(dir); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ctl", name), nil, 0o600)
}

func TakeMarker(dir, name string) bool { return os.Remove(filepath.Join(dir, "ctl", name)) == nil }
