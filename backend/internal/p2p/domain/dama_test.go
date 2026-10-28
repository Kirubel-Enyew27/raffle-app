package domain

import (
	"errors"
	"testing"
)

func testDamaBoard(pieces map[int]int, turn int) map[string]interface{} {
	cells := make([]int, 64)
	for index, value := range pieces {
		cells[index] = value
	}
	return map[string]interface{}{
		"variant":            "ethiopian-dama",
		"cells":              cells,
		"turn":               turn,
		"turn_seq":           0,
		"move_count":         0,
		"halfmove_clock":     0,
		"repetitions":        map[string]int{},
		"must_continue_from": nil,
	}
}

func TestDamaEngineInitBoard(t *testing.T) {
	engine := NewDamaEngine()
	board := engine.InitBoard()

	if board["variant"] != "ethiopian-dama" {
		t.Fatalf("unexpected variant %v", board["variant"])
	}
	cells, ok := board["cells"].([]int)
	if !ok || len(cells) != 64 {
		t.Fatalf("expected 64 cells, got %T len=%d", board["cells"], len(cells))
	}
	light, dark := 0, 0
	for _, value := range cells {
		switch value {
		case 1:
			light++
		case 3:
			dark++
		}
	}
	if light != 12 || dark != 12 {
		t.Fatalf("expected 12 pieces each, got light=%d dark=%d", light, dark)
	}
	if board["turn"] != 0 {
		t.Fatalf("expected light (0) to move first, got %v", board["turn"])
	}
	if board["must_continue_from"] != nil {
		t.Fatalf("fresh board must not require a capture continuation")
	}
	moves, ok := board["legal_moves"].([]map[string]interface{})
	if !ok || len(moves) != 7 {
		t.Fatalf("expected 7 opening moves, got %T %d", board["legal_moves"], len(moves))
	}
}

func TestDamaEngineStaleAndWrongTurn(t *testing.T) {
	engine := NewDamaEngine()
	board := engine.InitBoard()

	first, err := func() (map[string]interface{}, error) {
		newBoard, over, _, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{
			"from": 40, "to": 33, "turn_seq": 0,
		})
		if over {
			t.Fatalf("an opening move must not end the game")
		}
		return newBoard, err
	}()
	if err != nil {
		t.Fatalf("opening move rejected: %v", err)
	}

	// The same player can no longer act after the turn passes.
	if _, _, _, _, err := engine.ValidateAndApplyMove(first, 0, map[string]interface{}{
		"from": 42, "to": 35, "turn_seq": 1,
	}); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("expected ErrNotYourTurn, got %v", err)
	}

	// A replayed request with the previous sequence number is rejected.
	if _, _, _, _, err := engine.ValidateAndApplyMove(first, 1, map[string]interface{}{
		"from": 17, "to": 26, "turn_seq": 0,
	}); !errors.Is(err, ErrStaleMove) {
		t.Fatalf("expected ErrStaleMove, got %v", err)
	}

	to := 0
	for _, raw := range first["legal_moves"].([]map[string]interface{}) {
		if raw["from"].(int) == 19 {
			to = raw["to"].(int)
			break
		}
	}
	if to == 0 {
		t.Fatalf("expected a dark opening move from 19")
	}
	if _, _, _, _, err := engine.ValidateAndApplyMove(first, 1, map[string]interface{}{
		"from": 19, "to": to, "turn_seq": 1,
	}); err != nil {
		t.Fatalf("legal dark move rejected: %v", err)
	}
}

func TestDamaEngineInvalidMoveDoesNotMutateBoard(t *testing.T) {
	engine := NewDamaEngine()
	board := engine.InitBoard()
	before, _ := board["cells"].([]int)

	_, _, _, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{
		"from": 40, "to": 49, "turn_seq": 0,
	})
	if !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("expected ErrInvalidMove, got %v", err)
	}

	after, _ := board["cells"].([]int)
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("board mutated at %d: %d -> %d", i, before[i], after[i])
		}
	}
	if board["turn_seq"] != 0 {
		t.Fatalf("turn sequence mutated after an invalid move")
	}
}

func TestDamaEngineBackwardManCapture(t *testing.T) {
	engine := NewDamaEngine()
	// Light man 35 (r4c3) jumps dark 42 (r5c2) behind it and lands on 49 (r6c1).
	// Dark keeps a mobile piece on 3 so the round continues.
	board := testDamaBoard(map[int]int{
		35: 1,
		42: 3,
		3:  3,
	}, 0)

	// The capture is mandatory, so the free forward step is rejected.
	if _, _, _, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{
		"from": 35, "to": 28, "turn_seq": 0,
	}); !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("expected the non-capturing move to be rejected, got %v", err)
	}
	// Leaving the board diagonally backwards is not an ordinary move.
	if _, _, _, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{
		"from": 35, "to": 44, "turn_seq": 0,
	}); !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("expected the backward step to be rejected, got %v", err)
	}

	after, over, winner, tie, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{
		"from": 35, "to": 49, "turn_seq": 0,
	})
	if err != nil {
		t.Fatalf("backward capture rejected: %v", err)
	}
	if over || winner != nil || tie {
		t.Fatalf("unexpected terminal state: over=%v winner=%v tie=%v", over, winner, tie)
	}

	cells := after["cells"].([]int)
	if cells[42] != 0 {
		t.Fatalf("captured piece on 42 was not removed, got %d", cells[42])
	}
	if cells[35] != 0 {
		t.Fatalf("source square 35 still holds a piece, got %d", cells[35])
	}
	if cells[49] != 1 {
		t.Fatalf("expected the light man on 49, got %d", cells[49])
	}
	if after["turn"] != 1 {
		t.Fatalf("turn should pass to dark after the capture, got %v", after["turn"])
	}
	if seq, ok := after["turn_seq"].(int); !ok || seq != 1 {
		t.Fatalf("expected turn_seq 1, got %v", after["turn_seq"])
	}

	history, ok := after["move_history"].([]map[string]interface{})
	if !ok || len(history) != 1 {
		t.Fatalf("expected one history entry, got %T %v", after["move_history"], after["move_history"])
	}
	captures, ok := history[0]["captures"].([]int)
	if !ok || len(captures) != 1 || captures[0] != 42 {
		t.Fatalf("expected the captured square in the history, got %v", history[0]["captures"])
	}
}

func TestDamaEngineMultiCaptureKeepsTurn(t *testing.T) {
	engine := NewDamaEngine()
	// Light man 35 captures dark 26 then dark 10, promoting on 3. Dark keeps a
	// piece on 62 so the capture does not immediately end the game.
	board := testDamaBoard(map[int]int{
		35: 1,
		26: 3,
		10: 3,
		44: 3,
	}, 0)

	after, over, winner, tie, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{
		"from": 35, "to": 17, "turn_seq": 0,
	})
	if err != nil || over || winner != nil || tie {
		t.Fatalf("first hop: err=%v over=%v winner=%v tie=%v", err, over, winner, tie)
	}
	if after["turn"] != 0 {
		t.Fatalf("turn must stay with light during the chain, got %v", after["turn"])
	}
	if after["must_continue_from"] != 17 {
		t.Fatalf("expected continuation from 17, got %v", after["must_continue_from"])
	}
	if engine.TurnComplete(after) {
		t.Fatalf("engine reports the turn complete during a chain")
	}

	after2, over, winner, tie, err := engine.ValidateAndApplyMove(after, 0, map[string]interface{}{
		"from": 17, "to": 3, "turn_seq": 1,
	})
	if err != nil || over || winner != nil || tie {
		t.Fatalf("final hop: err=%v over=%v winner=%v tie=%v", err, over, winner, tie)
	}
	if after2["turn"] != 1 {
		t.Fatalf("turn should pass to dark after the chain, got %v", after2["turn"])
	}
	if !engine.TurnComplete(after2) {
		t.Fatalf("turn should be complete after the final hop")
	}
	if after2["must_continue_from"] != nil {
		t.Fatalf("continuation marker should be cleared")
	}
	if _, ok := after2["last_move"]; !ok {
		t.Fatalf("last_move missing")
	}

	history, ok := after2["move_history"].([]map[string]interface{})
	if !ok || len(history) != 2 {
		t.Fatalf("expected two encoded history entries, got %T %d", after2["move_history"], len(history))
	}
	if history[1]["promotion"] != true {
		t.Fatalf("expected the final hop to be a promotion: %+v", history[1])
	}
}

func TestDamaEngineWinMapping(t *testing.T) {
	engine := NewDamaEngine()
	// Only one dark piece: capturing it leaves dark with no moves.
	board := testDamaBoard(map[int]int{
		35: 1,
		26: 3,
	}, 0)

	after, over, winner, tie, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{
		"from": 35, "to": 17, "turn_seq": 0,
	})
	if err != nil {
		t.Fatalf("capture rejected: %v", err)
	}
	if !over || tie {
		t.Fatalf("expected a decisive round over, got over=%v tie=%v", over, tie)
	}
	if winner == nil || *winner != 0 {
		t.Fatalf("expected light (0) to win, got %v", winner)
	}

	isOver, matchWinner := engine.CheckMatchOver(map[string]int{"0": 1, "1": 0}, 1, 1)
	if !isOver || matchWinner == nil || *matchWinner != 0 {
		t.Fatalf("match check should award light the match")
	}
	if after["result"] == nil {
		t.Fatalf("expected a result in the board state")
	}
}
