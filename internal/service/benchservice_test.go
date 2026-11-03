package service

import (
	"context"
	"testing"
)

func TestChartRewriteOptInIsUnavailableOutsideSelfTest(t *testing.T) {
	t.Setenv("WW_SELFTEST", "0")
	if _, err := (&BenchService{}).SaveCopy(context.Background(), "chart.xlsx", true); err == nil {
		t.Fatal("fixture writer must be unavailable in normal mode")
	}
}
