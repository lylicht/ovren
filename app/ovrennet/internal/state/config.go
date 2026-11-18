package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	ConfigVersion = 1
	configMax     = 24 << 10
	maxAdapters   = 32
	maxNameRunes  = 48
	minWeight     = 0.05
	maxWeight     = 1000
	maxLimitMbps  = 100000
	minMTU        = 576
	maxMTU        = 9000
)

type Config struct {
	IncludedAdapters []string           `json:"includedAdapters"`
	AutoWeight       bool               `json:"autoWeight"`
	Weights          map[string]float64 `json:"weights"`
	LimitsMbps       map[string]float64 `json:"limitsMbps"`
	IPv6Mode         string             `json:"ipv6Mode"`
	AutoPause        bool               `json:"autoPause"`
	MTU              int                `json:"mtu"`
	StartWithOvren   bool               `json:"startWithOvren"`
}

type configRequest struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Config  Config `json:"config"`
}

func DefaultConfig() Config {
	return Config{
		IncludedAdapters: []string{},
		AutoWeight:       true,
		Weights:          map[string]float64{},
		LimitsMbps:       map[string]float64{},
		IPv6Mode:         "auto",
		MTU:              maxMTU,
	}
}

func (c Config) Validate() error {
	if c.IPv6Mode != "auto" && c.IPv6Mode != "on" && c.IPv6Mode != "off" {
		return errors.New("invalid IPv6 mode")
	}
	if c.MTU < minMTU || c.MTU > maxMTU {
		return errors.New("MTU must be between 576 and 9000")
	}
	if len(c.IncludedAdapters) > maxAdapters || len(c.Weights) > maxAdapters || len(c.LimitsMbps) > maxAdapters {
		return errors.New("too many adapter settings")
	}
	names := make(map[string]struct{}, len(c.IncludedAdapters))
	for _, name := range c.IncludedAdapters {
		if !validAdapterName(name) {
			return errors.New("invalid adapter name")
		}
		key := strings.ToLower(name)
		if _, ok := names[key]; ok {
			return errors.New("duplicate adapter name")
		}
		names[key] = struct{}{}
	}
	for name, weight := range c.Weights {
		if !validAdapterName(name) || math.IsNaN(weight) || math.IsInf(weight, 0) || weight < minWeight || weight > maxWeight {
			return errors.New("invalid adapter weight")
		}
	}
	for name, limit := range c.LimitsMbps {
		if !validAdapterName(name) || math.IsNaN(limit) || math.IsInf(limit, 0) || limit < 0 || limit > maxLimitMbps {
			return errors.New("invalid adapter speed limit")
		}
	}
	return nil
}

func DecodeConfig(b []byte) (Config, error) {
	if len(b) == 0 || len(b) > configMax {
		return Config{}, errors.New("configuration is empty or too large")
	}
	var config Config
	decoder := json.NewDecoder(strings.NewReader(string(b)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("invalid configuration: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Config{}, errors.New("invalid configuration payload")
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func WriteConfigRequest(dir string, config Config) (string, error) {
	if err := config.Validate(); err != nil {
		return "", err
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	id := hex.EncodeToString(idBytes)
	payload, err := json.Marshal(configRequest{Version: ConfigVersion, ID: id, Config: config})
	if err != nil {
		return "", err
	}
	if len(payload) > configMax {
		return "", errors.New("configuration is too large")
	}
	if err := Prepare(dir); err != nil {
		return "", err
	}
	ctl := filepath.Join(dir, "ctl")
	tmp, err := os.CreateTemp(ctl, "config-*.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(payload); err != nil {
		tmp.Close()
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err := replaceFile(tmpName, filepath.Join(ctl, "config")); err != nil {
		return "", err
	}
	return id, nil
}

func TakeConfigRequest(dir string) (Config, string, bool) {
	p := filepath.Join(dir, "ctl", "config")
	taken := p + ".taken"
	_ = os.Remove(taken)
	if err := os.Rename(p, taken); err != nil {
		return Config{}, "", false
	}
	defer os.Remove(taken)
	f, err := openLimitFile(taken)
	if err != nil {
		return Config{}, "", false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > configMax {
		return Config{}, "", false
	}
	payload, err := io.ReadAll(io.LimitReader(f, configMax+1))
	if err != nil || len(payload) > configMax {
		return Config{}, "", false
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	var request configRequest
	if decoder.Decode(&request) != nil || request.Version != ConfigVersion || len(request.ID) != 32 || !isHex(request.ID) {
		return Config{}, "", false
	}
	if decoder.Decode(new(any)) != io.EOF || request.Config.Validate() != nil {
		return Config{}, "", false
	}
	return request.Config, request.ID, true
}

func validAdapterName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxNameRunes || strings.TrimSpace(name) != name {
		return false
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func isHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}
