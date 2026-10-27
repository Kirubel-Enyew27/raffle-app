package domain

import "testing"

func intPtr(v int) *int { return &v }

// Rock-Paper-Scissors is a UI redesign target, so pin the rules the new board
// presents: which move wins, that a draw scores for nobody, and that the
// best-of-three ends as soon as one side reaches two wins.
func TestRockPaperScissorsEngine_RoundOutcomes(t *testing.T) {
	cases := []struct {
		name   string
		p0     string
		p1     string
		winner *int
		isTie  bool
	}{
		{name: "rock beats scissors", p0: "rock", p1: "scissors", winner: intPtr(0)},
		{name: "scissors beats paper", p0: "scissors", p1: "paper", winner: intPtr(0)},
		{name: "paper beats rock", p0: "paper", p1: "rock", winner: intPtr(0)},
		{name: "rock loses to paper", p0: "rock", p1: "paper", winner: intPtr(1)},
		{name: "scissors loses to rock", p0: "scissors", p1: "rock", winner: intPtr(1)},
		{name: "paper loses to scissors", p0: "paper", p1: "scissors", winner: intPtr(1)},
		{name: "matching moves draw", p0: "paper", p1: "paper", isTie: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewRockPaperScissorsEngine()
			board := engine.InitBoard()

			board, roundOver, winner, isTie, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"choice": tc.p0})
			if err != nil {
				t.Fatalf("first pick rejected: %v", err)
			}
			if roundOver || winner != nil || isTie {
				t.Fatalf("round resolved before both picks: over=%v winner=%v tie=%v", roundOver, winner, isTie)
			}
			if choices, _ := board["choices"].(map[string]interface{}); len(choices) != 1 {
				t.Fatalf("expected one masked pick to be stored, got %d", len(choices))
			}

			board, roundOver, winner, isTie, err = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"choice": tc.p1})
			if err != nil {
				t.Fatalf("second pick rejected: %v", err)
			}
			if !roundOver {
				t.Fatal("expected the round to resolve once both players picked")
			}
			if isTie != tc.isTie {
				t.Fatalf("expected tie=%v, got %v", tc.isTie, isTie)
			}
			if tc.winner == nil {
				if winner != nil {
					t.Fatalf("draw must not award a round, got winner %d", *winner)
				}
			} else if winner == nil || *winner != *tc.winner {
				t.Fatalf("expected winner %d, got %v", *tc.winner, winner)
			}

			// The reveal is kept for the UI while the choices reset for the next round.
			if choices, _ := board["choices"].(map[string]interface{}); len(choices) != 0 {
				t.Fatalf("expected choices to be cleared, got %d entries", len(choices))
			}
			reveal, ok := board["last_round_result"].(map[string]interface{})
			if !ok {
				t.Fatal("expected last_round_result to be recorded")
			}
			if reveal["p0_choice"] != tc.p0 || reveal["p1_choice"] != tc.p1 {
				t.Fatalf("reveal stored %v/%v", reveal["p0_choice"], reveal["p1_choice"])
			}
		})
	}
}

func TestRockPaperScissorsEngine_RejectsSecondPickInSameRound(t *testing.T) {
	engine := NewRockPaperScissorsEngine()
	board := engine.InitBoard()

	board, _, _, _, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"choice": "rock"})
	if err != nil {
		t.Fatalf("first pick rejected: %v", err)
	}
	if _, _, _, _, err = engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"choice": "paper"}); err == nil {
		t.Fatal("expected a duplicate pick to be rejected")
	}
	if _, _, _, _, err = engine.ValidateAndApplyMove(board, 1, map[string]interface{}{"choice": "lizard"}); err == nil {
		t.Fatal("expected an unknown move to be rejected")
	}
}

func TestRockPaperScissorsEngine_BestOfThreeEndsAtTwoWins(t *testing.T) {
	engine := NewRockPaperScissorsEngine()

	cases := []struct {
		name         string
		scores       map[string]int
		currentRound int
		wantOver     bool
		wantWinner   *int
	}{
		{name: "first win is not enough", scores: map[string]int{"0": 1, "1": 0}, currentRound: 1, wantOver: false},
		{name: "two wins take the match early", scores: map[string]int{"0": 2, "1": 0}, currentRound: 2, wantOver: true, wantWinner: intPtr(0)},
		{name: "opponent can take it in three", scores: map[string]int{"0": 1, "1": 2}, currentRound: 3, wantOver: true, wantWinner: intPtr(1)},
		{name: "one win each after three is a tie", scores: map[string]int{"0": 1, "1": 1}, currentRound: 3, wantOver: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			over, winner := engine.CheckMatchOver(tc.scores, tc.currentRound, 3)
			if over != tc.wantOver {
				t.Fatalf("expected over=%v, got %v", tc.wantOver, over)
			}
			if tc.wantWinner == nil {
				if winner != nil {
					t.Fatalf("expected no winner, got %d", *winner)
				}
				return
			}
			if winner == nil || *winner != *tc.wantWinner {
				t.Fatalf("expected winner %d, got %v", *tc.wantWinner, winner)
			}
		})
	}
}
