package state

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

type LimitRequest struct {
	Adapter string  `json:"adapter"`
	Mbps    float64 `json:"mbps"`
}

const limitRequestMax = 4 << 10

func limitRequestPath(dir string) string { return filepath.Join(dir, "ctl", "limit") }

func SendLimit(dir string, req LimitRequest) error {
	if err := os.MkdirAll(filepath.Join(dir, "ctl"), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	tmp := limitRequestPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o666); err != nil {
		return err
	}
	return os.Rename(tmp, limitRequestPath(dir))
}

func TakeLimit(dir string) (LimitRequest, bool) {
	p := limitRequestPath(dir)
	taken := p + ".taken"
	os.Remove(taken)
	if err := os.Rename(p, taken); err != nil {
		return LimitRequest{}, false
	}
	b, ok := readLimitRequest(taken)
	os.Remove(taken)
	if !ok {
		return LimitRequest{}, false
	}
	var req LimitRequest
	if json.Unmarshal(b, &req) != nil || req.Adapter == "" {
		return LimitRequest{}, false
	}
	return req, true
}

func readLimitRequest(path string) ([]byte, bool) {
	f, err := openLimitFile(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, false
	}
	b, err := io.ReadAll(io.LimitReader(f, limitRequestMax))
	if err != nil {
		return nil, false
	}
	return b, true
}
