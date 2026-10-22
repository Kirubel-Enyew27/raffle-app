package domain

import (
	"testing"
)

func TestCalculateStreaks_Empty(t *testing.T) {
	res := CalculateStreaks([]bool{})
	if res.CurrentStreak != 0 || res.LongestStreak != 0 {
		t.Fatalf("expected 0/0, got %d/%d", res.CurrentStreak, res.LongestStreak)
	}
}

func TestCalculateStreaks_AllWins(t *testing.T) {
	res := CalculateStreaks([]bool{true, true, true, true})
	if res.CurrentStreak != 4 || res.LongestStreak != 4 {
		t.Fatalf("expected 4/4, got %d/%d", res.CurrentStreak, res.LongestStreak)
	}
}

func TestCalculateStreaks_AllLossesOrDraws(t *testing.T) {
	res := CalculateStreaks([]bool{false, false, false})
	if res.CurrentStreak != 0 || res.LongestStreak != 0 {
		t.Fatalf("expected 0/0, got %d/%d", res.CurrentStreak, res.LongestStreak)
	}
}

func TestCalculateStreaks_ResetOnLoss(t *testing.T) {
	// 3 wins, 1 loss, 2 wins
	res := CalculateStreaks([]bool{true, true, true, false, true, true})
	if res.CurrentStreak != 2 || res.LongestStreak != 3 {
		t.Fatalf("expected current 2, longest 3, got %d/%d", res.CurrentStreak, res.LongestStreak)
	}
}

func TestCalculateStreaks_ResetOnDraw(t *testing.T) {
	// 4 wins, 1 draw (false), 1 win
	res := CalculateStreaks([]bool{true, true, true, true, false, true})
	if res.CurrentStreak != 1 || res.LongestStreak != 4 {
		t.Fatalf("expected current 1, longest 4, got %d/%d", res.CurrentStreak, res.LongestStreak)
	}
}

func TestCalculateStreaks_LossAtEnd(t *testing.T) {
	// 5 wins, ends in loss
	res := CalculateStreaks([]bool{true, true, true, true, true, false})
	if res.CurrentStreak != 0 || res.LongestStreak != 5 {
		t.Fatalf("expected current 0, longest 5, got %d/%d", res.CurrentStreak, res.LongestStreak)
	}
}
