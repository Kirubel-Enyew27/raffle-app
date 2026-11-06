package application

import (
	"testing"
	"time"
)

func TestPercentChange(t *testing.T) {
	if got := percentChange(120, 100); got == nil || *got != 20 {
		t.Fatalf("expected +20%%, got %v", got)
	}
	if got := percentChange(80, 100); got == nil || *got != -20 {
		t.Fatalf("expected -20%%, got %v", got)
	}
	if got := percentChange(5, 0); got != nil {
		t.Fatalf("expected nil when the previous window is empty, got %v", *got)
	}
}

func TestNormalizeRangeIsEndExclusive(t *testing.T) {
	from := time.Date(2026, 11, 1, 14, 30, 0, 0, time.UTC)
	to := time.Date(2026, 11, 6, 9, 15, 0, 0, time.UTC)
	gotFrom, gotTo := normalizeRange(from, to)
	if gotFrom.Hour() != 0 || gotFrom.Minute() != 0 {
		t.Fatalf("expected midnight start, got %s", gotFrom)
	}
	if want := time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC); !gotTo.Equal(want) {
		t.Fatalf("expected end-exclusive %s, got %s", want, gotTo)
	}
}
