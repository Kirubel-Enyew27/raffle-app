package dama

import (
	"errors"
	"reflect"
	"testing"
)

func emptyGame(rules Ruleset) *Game {
	g := NewGame(rules)
	g.Board = Board{}
	g.Repetitions = map[string]int{}
	return g
}

func place(g *Game, index int, player Player, king bool) {
	g.Board.Set(index, Piece{Player: player, King: king})
}

func stepTo(steps []Step, from, to int) *Step {
	for i := range steps {
		if steps[i].From == from && steps[i].To == to {
			return &steps[i]
		}
	}
	return nil
}

func TestInitialSetup(t *testing.T) {
	g := NewGame(DefaultRuleset())

	if g.Turn != Light {
		t.Fatalf("expected light to move first, got %v", g.Turn)
	}

	light, dark, kings := 0, 0, 0
	for index, piece := range g.Board.Squares {
		if piece.IsEmpty() {
			continue
		}
		if !IsPlayable(index) {
			t.Fatalf("piece placed on a light square at %d", index)
		}
		if piece.King {
			kings++
		}
		if piece.Player == Light {
			light++
		} else {
			dark++
		}
	}
	if light != 12 || dark != 12 {
		t.Fatalf("expected 12 pieces each, got light=%d dark=%d", light, dark)
	}
	if kings != 0 {
		t.Fatalf("expected no kings at the start, got %d", kings)
	}

	// Four men on the front row: the edge man has one destination, the other
	// three have two each, for 7 opening moves.
	steps := g.LegalSteps()
	if len(steps) != 7 {
		t.Fatalf("expected 7 opening moves, got %d: %+v", len(steps), steps)
	}
	for _, step := range steps {
		if step.Continues || step.Promotion || len(step.Captures) != 0 {
			t.Fatalf("unexpected opening step metadata: %+v", step)
		}
	}
}

func TestInvalidMoveLeavesGameUnchanged(t *testing.T) {
	g := NewGame(DefaultRuleset())
	before := g.Cells()
	beforeSeq := g.TurnSeq
	beforeHistory := len(g.History)

	// Empty square.
	if _, _, err := g.ApplyStep(20, 27); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected ErrIllegalMove for empty source, got %v", err)
	}
	// Opponent's piece.
	if _, _, err := g.ApplyStep(1, 8); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected ErrIllegalMove for opponent piece, got %v", err)
	}
	// Men cannot move backwards.
	if _, _, err := g.ApplyStep(40, 49); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected ErrIllegalMove for blocked diagonal, got %v", err)
	}
	// Same square.
	if _, _, err := g.ApplyStep(40, 40); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected ErrIllegalMove for identical squares, got %v", err)
	}
	// Out of range.
	if _, _, err := g.ApplyStep(-1, 99); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected ErrIllegalMove for out-of-range squares, got %v", err)
	}

	if g.Cells() != before {
		t.Fatalf("board changed after invalid moves")
	}
	if g.TurnSeq != beforeSeq || len(g.History) != beforeHistory {
		t.Fatalf("sequence/history changed after invalid moves")
	}
}

func TestMandatoryCapture(t *testing.T) {
	g := emptyGame(DefaultRuleset())
	g.Turn = Light
	place(g, 35, Light, false) // r4 c3
	place(g, 26, Dark, false)  // r3 c2
	place(g, 62, Dark, false)  // keeps dark alive after the capture

	steps := g.LegalSteps()
	if len(steps) != 1 {
		t.Fatalf("expected only the mandatory capture, got %+v", steps)
	}
	capture := stepTo(steps, 35, 17)
	if capture == nil {
		t.Fatalf("expected capture 35->17, got %+v", steps)
	}
	if len(capture.Captures) != 1 || capture.Captures[0] != 26 {
		t.Fatalf("expected captured square 26, got %+v", capture.Captures)
	}

	// A non-capturing move is rejected and must not mutate the game.
	before := g.Cells()
	if _, _, err := g.ApplyStep(35, 28); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected non-capturing move to be rejected, got %v", err)
	}
	if g.Cells() != before {
		t.Fatalf("board mutated by rejected non-capture")
	}

	if _, complete, err := g.ApplyStep(35, 17); err != nil || !complete {
		t.Fatalf("capture failed: complete=%v err=%v", complete, err)
	}
	if !g.Board.At(26).IsEmpty() {
		t.Fatalf("captured piece was not removed")
	}
	if g.Board.At(17) != (Piece{Player: Light}) {
		t.Fatalf("capturing piece did not land on 17")
	}
	if g.Turn != Dark {
		t.Fatalf("turn should pass to dark after a completed capture")
	}
}

func TestMultiCaptureChainKeepsTurn(t *testing.T) {
	g := emptyGame(DefaultRuleset())
	g.Turn = Light
	place(g, 35, Light, false) // r4c3
	place(g, 26, Dark, false)  // r3c2 - first victim
	place(g, 10, Dark, false)  // r1c2 - second victim
	place(g, 49, Light, false) // r6c1, another piece that must stay locked
	place(g, 62, Dark, false)  // keeps dark alive

	steps := g.LegalSteps()
	first := stepTo(steps, 35, 17)
	if first == nil || !first.Continues {
		t.Fatalf("expected first hop 35->17 to continue, got %+v", steps)
	}

	step, complete, err := g.ApplyStep(35, 17)
	if err != nil || complete {
		t.Fatalf("first hop should not complete the turn: complete=%v err=%v", complete, err)
	}
	if step == nil || !step.Continues {
		t.Fatalf("expected continues=true, got %+v", step)
	}
	if g.Turn != Light {
		t.Fatalf("turn must stay with light during the chain")
	}
	if g.ChainFrom == nil || *g.ChainFrom != 17 {
		t.Fatalf("expected chain to continue from 17, got %v", g.ChainFrom)
	}
	if g.TurnSeq != 1 {
		t.Fatalf("expected turn sequence 1, got %d", g.TurnSeq)
	}

	// Only the chain square may move now.
	if _, _, err := g.ApplyStep(49, 40); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("expected other pieces to be locked during a chain, got %v", err)
	}
	continuations := g.LegalSteps()
	if len(continuations) != 1 || continuations[0].From != 17 || continuations[0].To != 3 {
		t.Fatalf("expected continuation 17->3, got %+v", continuations)
	}

	step, complete, err = g.ApplyStep(17, 3)
	if err != nil || !complete {
		t.Fatalf("final hop failed: complete=%v err=%v", complete, err)
	}
	if step == nil || !step.Promotion || len(step.Captures) != 1 || step.Captures[0] != 10 {
		t.Fatalf("expected promotion capturing 10, got %+v", step)
	}
	if !g.Board.At(3).King {
		t.Fatalf("piece on the back rank should be promoted")
	}
	if g.Turn != Dark {
		t.Fatalf("turn should pass after the chain completes")
	}
	if len(g.History) != 2 {
		t.Fatalf("expected two history entries, got %d", len(g.History))
	}
}

func TestFlyingKingCapturesAtDistance(t *testing.T) {
	g := emptyGame(DefaultRuleset())
	g.Turn = Light
	place(g, 26, Light, true) // r3c2
	place(g, 44, Dark, false) // r5c4
	place(g, 62, Dark, false) // keeps dark alive

	steps := g.LegalSteps()
	capture := stepTo(steps, 26, 53)
	if capture == nil {
		t.Fatalf("expected flying king capture 26->53, got %+v", steps)
	}
	if len(capture.Captures) != 1 || capture.Captures[0] != 44 {
		t.Fatalf("expected captured square 44, got %+v", capture.Captures)
	}
	if _, _, err := g.ApplyStep(26, 53); err != nil {
		t.Fatalf("flying capture rejected: %v", err)
	}
}

func TestPromotionOnSimpleMove(t *testing.T) {
	g := emptyGame(DefaultRuleset())
	g.Turn = Light
	place(g, 8, Light, false) // r1c0
	place(g, 62, Dark, false) // keeps dark alive

	step, complete, err := g.ApplyStep(8, 1)
	if err != nil || !complete {
		t.Fatalf("promotion move failed: complete=%v err=%v", complete, err)
	}
	if step == nil || !step.Promotion {
		t.Fatalf("expected promotion flag, got %+v", step)
	}
	if !g.Board.At(1).King {
		t.Fatalf("piece should be a king after promotion")
	}
}

func TestBackwardCaptureIsConfigurable(t *testing.T) {
	position := func(rules Ruleset) *Game {
		g := emptyGame(rules)
		g.Turn = Light
		place(g, 35, Light, false) // r4c3
		place(g, 42, Dark, false)  // r5c2, behind light
		place(g, 62, Dark, false)  // keeps dark alive
		return g
	}

	defaultGame := position(DefaultRuleset())
	if stepTo(defaultGame.LegalSteps(), 35, 49) != nil {
		t.Fatalf("default rules must not allow a backward capture")
	}

	rules := DefaultRuleset()
	rules.MenCaptureBackwards = true
	backwardsGame := position(rules)
	if stepTo(backwardsGame.LegalSteps(), 35, 49) == nil {
		t.Fatalf("MenCaptureBackwards should allow 35->49")
	}
	if _, _, err := backwardsGame.ApplyStep(35, 49); err != nil {
		t.Fatalf("backward capture rejected: %v", err)
	}
}

func TestBlockedPlayerLoses(t *testing.T) {
	g := emptyGame(DefaultRuleset())
	g.Turn = Light
	place(g, 28, Light, false) // mover
	place(g, 8, Light, false)  // blocks dark's left destination
	place(g, 10, Light, false) // blocks dark's right destination
	place(g, 1, Dark, false)   // trapped in the corner

	if _, _, err := g.ApplyStep(28, 19); err != nil {
		t.Fatalf("freeing move failed: %v", err)
	}
	if g.Result == nil || g.Result.Winner == nil || *g.Result.Winner != Light {
		t.Fatalf("expected light to win when dark is blocked, got %+v", g.Result)
	}
	if g.Result.Reason != ReasonNoLegalMoves {
		t.Fatalf("expected no_legal_moves, got %q", g.Result.Reason)
	}
}

func TestHalfmoveDraw(t *testing.T) {
	rules := DefaultRuleset()
	rules.HalfmoveDrawPlies = 2
	rules.RepetitionDraw = 0

	g := emptyGame(rules)
	g.Turn = Light
	place(g, 1, Light, true)
	place(g, 62, Dark, true)

	if _, _, err := g.ApplyStep(1, 10); err != nil {
		t.Fatalf("light shuttle failed: %v", err)
	}
	if g.Result != nil {
		t.Fatalf("game ended too early: %+v", g.Result)
	}
	if _, _, err := g.ApplyStep(62, 53); err != nil {
		t.Fatalf("dark shuttle failed: %v", err)
	}
	if g.Result == nil || !g.Result.Draw || g.Result.Reason != ReasonHalfmoveDraw {
		t.Fatalf("expected halfmove draw, got %+v", g.Result)
	}
}

func TestThreefoldRepetitionDraw(t *testing.T) {
	rules := DefaultRuleset()
	rules.HalfmoveDrawPlies = 0
	rules.RepetitionDraw = 3

	g := emptyGame(rules)
	g.Turn = Light
	place(g, 1, Light, true)
	place(g, 62, Dark, true)
	g.recordPosition()

	shuttle := [][2]int{{1, 10}, {62, 53}, {10, 1}, {53, 62}}
	for round := 0; round < 2 && g.Result == nil; round++ {
		for _, move := range shuttle {
			if g.Result != nil {
				break
			}
			if _, _, err := g.ApplyStep(move[0], move[1]); err != nil {
				t.Fatalf("shuttle %v failed: %v", move, err)
			}
		}
	}
	if g.Result == nil || !g.Result.Draw || g.Result.Reason != ReasonRepetition {
		t.Fatalf("expected threefold repetition draw, got %+v", g.Result)
	}
}

func TestMajorityCaptureRule(t *testing.T) {
	rules := DefaultRuleset()
	rules.MajorityCapture = true

	g := emptyGame(rules)
	g.Turn = Light
	// A two-capture chain: 40 -> 26 (captures 33) -> 12 (captures 19).
	place(g, 40, Light, false) // r5c0
	place(g, 33, Dark, false)  // r4c1
	place(g, 19, Dark, false)  // r2c3
	// A competing single capture: 35 -> 21 (captures 28).
	place(g, 35, Light, false) // r4c3
	place(g, 28, Dark, false)  // r3c4
	place(g, 62, Dark, false)  // keeps dark alive

	steps := g.LegalSteps()
	if stepTo(steps, 40, 26) == nil {
		t.Fatalf("expected the two-capture chain first hop 40->26, got %+v", steps)
	}
	if stepTo(steps, 35, 21) != nil {
		t.Fatalf("majority rule should exclude the single capture, got %+v", steps)
	}
}

func TestDeterministicReplay(t *testing.T) {
	play := func() *Game {
		g := emptyGame(DefaultRuleset())
		g.Turn = Light
		place(g, 35, Light, false)
		place(g, 26, Dark, false)
		place(g, 10, Dark, false)
		place(g, 62, Dark, false)
		for _, move := range [][2]int{{35, 17}, {17, 3}} {
			if _, _, err := g.ApplyStep(move[0], move[1]); err != nil {
				t.Fatalf("replay move %v failed: %v", move, err)
			}
		}
		return g
	}

	first := play()
	second := play()
	if first.Cells() != second.Cells() {
		t.Fatalf("replay produced a different board")
	}
	if first.Turn != second.Turn || first.TurnSeq != second.TurnSeq || first.MoveCount != second.MoveCount {
		t.Fatalf("replay produced different turn metadata")
	}
	if !reflect.DeepEqual(first.History, second.History) {
		t.Fatalf("replay produced different history")
	}
}
