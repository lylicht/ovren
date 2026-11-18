package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const limitsVersion = 1

type limitsFile struct {
	Version int                `json:"version"`
	Limits  map[string]float64 `json:"limits"`
}

func LimitsPath(dir string) string {
	return filepath.Join(limitsDir(dir), "limits.json")
}

func limitsDir(stateDir string) string { return limitsDirFor(runtime.GOOS, stateDir) }

func limitsDirFor(goos, stateDir string) string {
	if goos == "darwin" && stateDir == macStateDir {
		return macLimitsDir
	}
	return stateDir
}

func LoadLimits(dir string) (map[string]float64, error) {
	b, err := os.ReadFile(LimitsPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]float64{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f limitsFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", LimitsPath(dir), err)
	}
	limits := make(map[string]float64, len(f.Limits))
	for name, mbitPerSec := range f.Limits {
		if name != "" && mbitPerSec > 0 && mbitPerSec <= 1e6 {
			limits[name] = mbitPerSec
		}
	}
	return limits, nil
}

func SaveLimits(dir string, limits map[string]float64) error {
	if err := os.MkdirAll(limitsDir(dir), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(limitsFile{Version: limitsVersion, Limits: limits}, "", "  ")
	if err != nil {
		return err
	}
	tmp := LimitsPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return replaceFile(tmp, LimitsPath(dir))
}
