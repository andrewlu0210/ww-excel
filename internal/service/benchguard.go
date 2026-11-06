package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"path/filepath"
	"strings"
)

const (
	maxBenchEvents        = 1000
	maxBenchBlocks        = 16
	maxBenchBlockCopies   = 1000
	maxBenchEventBytes    = 2 << 20
	maxBenchBurstBytes    = 32 << 20
	maxBenchChain         = 100_000
	maxBenchReportBytes   = 1 << 20
	maxBenchSnapshotBytes = 8 << 20
	maxBenchImagePixels   = 16_000_000
	maxBenchChartImages   = 128
	maxBenchChartURLs     = 24 << 20
)

var errBenchDisabled = errors.New("test operation is unavailable in normal mode")
var errSelfTestOnly = errors.New("operation is only available in self-test mode")
var errBenchBusy = errors.New("another test operation is running")

// Mode flags are captured at startup; frontend calls cannot enable the harness.
func (b *BenchService) enabled(selfTestOnly bool) error {
	if selfTestOnly && !b.selfTest {
		return errSelfTestOnly
	}
	if !b.bench && !b.selfTest {
		return errBenchDisabled
	}
	return nil
}

// Only one expensive probe or output operation may run across all windows.
func (b *BenchService) begin(ctx context.Context, selfTestOnly bool) (func(), error) {
	if err := b.enabled(selfTestOnly); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !b.busy.CompareAndSwap(false, true) {
		return nil, errBenchBusy
	}
	return func() { b.busy.Store(false) }, nil
}
func (b *BenchService) outputFile(name, suffix string) (string, error) {
	if name == "" || name == "." || name == ".." || len(name) > 120 || strings.ContainsAny(name, "/\\\x00") {
		return "", fmt.Errorf("invalid test output filename")
	}
	if suffix != "" && !strings.HasSuffix(strings.ToLower(name), suffix) {
		name += suffix
	}
	return filepath.Abs(filepath.Join(filepath.Dir(b.output), name))
}
func benchPNG(dataURL string) ([]byte, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return nil, fmt.Errorf("test image must be a PNG data URL")
	}
	payload := dataURL[len(prefix):]
	if len(payload) > base64.StdEncoding.EncodedLen(maxBenchSnapshotBytes) {
		return nil, fmt.Errorf("test image exceeds %d bytes", maxBenchSnapshotBytes)
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	if len(data) > maxBenchSnapshotBytes {
		return nil, fmt.Errorf("test image exceeds %d bytes", maxBenchSnapshotBytes)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 || cfg.Width > maxBenchImagePixels/cfg.Height {
		return nil, fmt.Errorf("test image dimensions exceed safety limits")
	}
	return data, nil
}

// Estimate a conservative encoded size before json.Marshal allocates its output.
func benchBlockBytes(blocks []BlockDTO) (int, error) {
	left := maxBenchEventBytes
	take := func(n int) bool {
		if n > left {
			return false
		}
		left -= n
		return true
	}
	text := func(s string) bool {
		if len(s) > left/6 {
			return false
		}
		return take(len(s) * 6)
	}
	for _, block := range blocks {
		if !take(128) {
			return 0, fmt.Errorf("test event payload is too large")
		}
		for _, c := range block.Cells {
			if !take(256) || !text(c.V) || !text(c.Fc) || !text(c.Bc) || !text(c.BarBorderColor) {
				return 0, fmt.Errorf("test event payload is too large")
			}
			if c.Icon != nil && !text(c.Icon.Set) {
				return 0, fmt.Errorf("test event payload is too large")
			}
			for _, run := range c.Runs {
				if !take(256) || !text(run.Text) || !text(run.Font.Name) || !text(run.Font.Color) {
					return 0, fmt.Errorf("test event payload is too large")
				}
			}
		}
	}
	encoded, err := json.Marshal(blocks)
	if err != nil {
		return 0, err
	}
	size := len(encoded) + 256 // Envelope, sequence numbers and timestamp.
	if size > maxBenchEventBytes {
		return 0, fmt.Errorf("test event payload is too large")
	}
	return size, nil
}
