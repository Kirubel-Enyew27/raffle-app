package domain

import (
	"testing"
)

func TestTicTacToeEngine_WinningHorizontal(t *testing.T) {
	engine := NewTicTacToeEngine()
	board := engine.InitBoard()

	// Moves:
	// P0: 0, P1: 3
	// P0: 1, P1: 4
	// P0: 2 (wins!)
	var isOver bool
	var winner *int
	var isTie bool
	var err error

	board, isOver, winner, isTie, err = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"cell": 0})
	if err != nil || isOver {
		t.Fatalf("unexpected result move 1: err=%v, isOver=%v", err, isOver)
	}

	board, isOver, winner, isTie, err = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"cell": 3})
	if err != nil || isOver {
		t.Fatalf("unexpected result move 2: err=%v, isOver=%v", err, isOver)
	}

	board, isOver, winner, isTie, err = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"cell": 1})
	if err != nil || isOver {
		t.Fatalf("unexpected result move 3: err=%v, isOver=%v", err, isOver)
	}

	board, isOver, winner, isTie, err = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"cell": 4})
	if err != nil || isOver {
		t.Fatalf("unexpected result move 4: err=%v, isOver=%v", err, isOver)
	}

	board, isOver, winner, isTie, err = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"cell": 2})
	if err != nil {
		t.Fatalf("unexpected error move 5: %v", err)
	}
	if !isOver || winner == nil || *winner != 0 || isTie {
		t.Fatalf("expected P0 win, got isOver=%v, winner=%v, isTie=%v", isOver, winner, isTie)
	}
}

func TestTicTacToeEngine_WinningDiagonal(t *testing.T) {
	engine := NewTicTacToeEngine()
	board := engine.InitBoard()

	// P0: 0, P1: 1
	// P0: 4, P1: 2
	// P0: 8 (wins on diagonal 0-4-8)
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"cell": 0})
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"cell": 1})
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"cell": 4})
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"cell": 2})
	board, isOver, winner, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"cell": 8})

	if err != nil || !isOver || winner == nil || *winner != 0 {
		t.Fatalf("expected P0 diagonal win, got err=%v, isOver=%v, winner=%v", err, isOver, winner)
	}
}

func TestTicTacToeEngine_OccupiedCellRejected(t *testing.T) {
	engine := NewTicTacToeEngine()
	board := engine.InitBoard()

	board, _, _, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"cell": 4})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	_, _, _, _, err = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"cell": 4})
	if err == nil {
		t.Fatalf("expected error playing on occupied cell, got nil")
	}
}

func TestTicTacToeEngine_Tie(t *testing.T) {
	engine := NewTicTacToeEngine()
	board := engine.InitBoard()

	// 0 1 0
	// 0 1 1
	// 1 0 0
	// Board pattern:
	// X(0) O(1) X(2)
	// X(3) O(4) O(5)
	// O(6) X(7) X(8)
	moves := []struct {
		p    int
		cell int
	}{
		{0, 0}, {1, 1}, {0, 2},
		{0, 3}, {1, 4}, {1, 5},
		{1, 6}, {0, 7}, {0, 8},
	}

	var isOver bool
	var winner *int
	var isTie bool
	var err error

	for _, m := range moves {
		board, isOver, winner, isTie, err = engine.ValidateAndApplyMove(board, m.p, map[string]interface{}{"cell": m.cell})
		if err != nil {
			t.Fatalf("move failed: %v", err)
		}
	}

	if !isOver || !isTie || winner != nil {
		t.Fatalf("expected tie, got isOver=%v, isTie=%v, winner=%v", isOver, isTie, winner)
	}
}

func TestConnectFourEngine_VerticalWin(t *testing.T) {
	engine := NewConnectFourEngine()
	board := engine.InitBoard()

	// Column 0:
	// P0 drops 4 chips in col 0
	// P1 drops 3 chips in col 1
	var isOver bool
	var winner *int
	var err error

	for i := 0; i < 3; i++ {
		board, isOver, winner, _, err = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 0})
		if err != nil || isOver {
			t.Fatalf("unexpected step %d: err=%v, isOver=%v", i, err, isOver)
		}

		board, isOver, winner, _, err = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"column": 1})
		if err != nil || isOver {
			t.Fatalf("unexpected step %d p1: err=%v, isOver=%v", i, err, isOver)
		}
	}

	// 4th chip in col 0 for P0
	board, isOver, winner, _, err = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 0})
	if err != nil || !isOver || winner == nil || *winner != 0 {
		t.Fatalf("expected P0 vertical win, got err=%v, isOver=%v, winner=%v", err, isOver, winner)
	}
}

func TestConnectFourEngine_HorizontalWin(t *testing.T) {
	engine := NewConnectFourEngine()
	board := engine.InitBoard()

	// P0 plays cols 0, 1, 2, 3 (all bottom row)
	// P1 plays cols 0, 1, 2 (second row)
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 0}) // row 5, col 0
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"column": 0}) // row 4, col 0
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 1}) // row 5, col 1
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"column": 1}) // row 4, col 1
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 2}) // row 5, col 2
	board, _, _, _, _ = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"column": 2}) // row 4, col 2

	board, isOver, winner, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 3}) // row 5, col 3 -> 4 in a row!
	if err != nil || !isOver || winner == nil || *winner != 0 {
		t.Fatalf("expected P0 horizontal win, got err=%v, isOver=%v, winner=%v", err, isOver, winner)
	}
}

func TestConnectFourEngine_ColumnFull(t *testing.T) {
	engine := NewConnectFourEngine()
	board := engine.InitBoard()

	// Fill col 0 with 6 chips
	for i := 0; i < 6; i++ {
		p := i % 2
		var err error
		board, _, _, _, err = engine.ValidateAndApplyMove(board, p, map[string]interface{}{"column": 0})
		if err != nil {
			t.Fatalf("unexpected err on chip %d: %v", i, err)
		}
	}

	// 7th chip must fail
	_, _, _, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 0})
	if err == nil {
		t.Fatalf("expected error playing on full column, got nil")
	}
}

func TestCalculateSettlement(t *testing.T) {
	// 100 ETB + 100 ETB = 200 ETB (20000 cents). 5% fee (500 bp) = 10 ETB (1000 cents).
	res := CalculateSettlement(20000, 500, false)
	if res.FeeCents != 1000 {
		t.Errorf("expected FeeCents 1000, got %d", res.FeeCents)
	}
	if res.NetPayoutCents != 19000 {
		t.Errorf("expected NetPayoutCents 19000, got %d", res.NetPayoutCents)
	}
	if res.IsTie {
		t.Errorf("expected IsTie false")
	}

	// Tie test: 100% refund, 0 fee
	tieRes := CalculateSettlement(20000, 500, true)
	if tieRes.FeeCents != 0 || tieRes.NetPayoutCents != 0 || !tieRes.IsTie {
		t.Errorf("expected tie settlement with 0 fee, got %+v", tieRes)
	}
}
