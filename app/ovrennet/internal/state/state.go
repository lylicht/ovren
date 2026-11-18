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
	PID     int
	Mode    string
	V6      string
	Since   time.Time
	Updated time.Time
	NICs    []engine.SlotStats
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

func Prepare(dir string) error {
	ctl := filepath.Join(dir, "ctl")
	if err := os.MkdirAll(ctl, 0o755); err != nil {
		return err
	}
	return os.Chmod(ctl, os.ModeSticky|0o777)
}

func Write(dir string, st Status) {
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	tmp := filepath.Join(dir, "status.json.tmp")
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, filepath.Join(dir, "status.json"))
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
	if err := os.MkdirAll(filepath.Join(dir, "ctl"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ctl", name), nil, 0o666)
}

func TakeMarker(dir, name string) bool { return os.Remove(filepath.Join(dir, "ctl", name)) == nil }
