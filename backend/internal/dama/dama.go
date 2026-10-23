// Package dama implements Ethiopian Dama (Ethiopian draughts) as a pure,
// deterministic game-rules engine.
//
// The package deliberately has no knowledge of users, HTTP, WebSockets,
// wallets, databases or clocks. It operates on a board plus a ruleset and
// rejects illegal input without mutating the supplied game.
//
// The default ruleset is documented in docs/ETHIOPIAN_DAMA.md. Ethiopian Dama
// has no single authoritative published rulebook, so every contested rule is a
// flag on Ruleset rather than a hard-coded assumption.
package dama

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// BoardSize is the number of rows and columns on a Dama board.
	BoardSize = 8
	// SquareCount is the number of squares on the board (including light squares).
	SquareCount = BoardSize * BoardSize

	// Variant is the stable identifier stored in board JSON and p2p_games.
	Variant = "ethiopian-dama"
)

// Player identifies one side. The zero value is reserved for empty squares,
// so Light and Dark start at 1; the p2p adapter converts them to and from its
// zero-based player index.
type Player int

const (
	// Light is the bottom side and moves first by default.
	Light Player = 1
	// Dark is the top side.
	Dark Player = 2
)

// Other returns the opposing player.
func (p Player) Other() Player {
	if p == Light {
		return Dark
	}
	return Light
}

// Piece is one man or king. The zero value is an empty square.
type Piece struct {
	Player Player
	King   bool
}

// IsEmpty reports whether the square holds no piece. Player 0 is never a valid
// side, which keeps the zero value empty.
func (p Piece) IsEmpty() bool {
	return p.Player == 0
}

// Board is an 8x8 board in row-major order, index = row*8 + column.
// Row 0 is the top of the board, row 7 the bottom.
type Board struct {
	Squares [SquareCount]Piece
}

// At returns the piece on the square. Out-of-range indices return the empty
// piece so callers never need a bounds check before reading.
func (b Board) At(index int) Piece {
	if index < 0 || index >= SquareCount {
		return Piece{}
	}
	return b.Squares[index]
}

// Set places a piece on the square. Out-of-range writes are ignored.
func (b *Board) Set(index int, piece Piece) {
	if index < 0 || index >= SquareCount {
		return
	}
	b.Squares[index] = piece
}

// IsPlayable reports whether a square is a dark square used by Dama.
func IsPlayable(index int) bool {
	if index < 0 || index >= SquareCount {
		return false
	}
	row, col := index/BoardSize, index%BoardSize
	return (row+col)%2 == 1
}

// InitialBoard returns the standard 12-pieces-per-side starting position.
// Light occupies rows 5-7 (bottom), Dark rows 0-2 (top).
func InitialBoard() Board {
	var b Board
	for row := 0; row < 3; row++ {
		for col := 0; col < BoardSize; col++ {
			index := row*BoardSize + col
			if IsPlayable(index) {
				b.Set(index, Piece{Player: Dark})
			}
		}
	}
	for row := 5; row < BoardSize; row++ {
		for col := 0; col < BoardSize; col++ {
			index := row*BoardSize + col
			if IsPlayable(index) {
				b.Set(index, Piece{Player: Light})
			}
		}
	}
	return b
}

// Ruleset describes the Dama variant. The zero value is intentionally not the
// default: use DefaultRuleset and override only the fields you need.
type Ruleset struct {
	// MenMoveBackwards allows uncrowned pieces to step backwards.
	MenMoveBackwards bool
	// MenCaptureBackwards allows uncrowned pieces to capture backwards.
	MenCaptureBackwards bool
	// FlyingKings allows kings to slide and capture over any distance.
	FlyingKings bool
	// MandatoryCapture rejects non-capturing moves while a capture exists.
	MandatoryCapture bool
	// MajorityCapture requires the capture sequence with the most pieces.
	MajorityCapture bool
	// PromotionDuringCapture promotes immediately when a man lands on the
	// back rank mid-chain and lets it continue as a king.
	PromotionDuringCapture bool
	// HalfmoveDrawPlies declares a draw after this many plies without a capture
	// or promotion. Zero disables the rule.
	HalfmoveDrawPlies int
	// RepetitionDraw declares a draw when the same position occurs this many
	// times. Values below two disable the rule.
	RepetitionDraw int
}

// DefaultRuleset is the ruleset enforced for online matches.
func DefaultRuleset() Ruleset {
	return Ruleset{
		MenMoveBackwards:       false,
		MenCaptureBackwards:    false,
		FlyingKings:            true,
		MandatoryCapture:       true,
		MajorityCapture:        false,
		PromotionDuringCapture: false,
		HalfmoveDrawPlies:      100,
		RepetitionDraw:         3,
	}
}

// Errors returned by the engine. The p2p adapter maps these onto domain errors.
var (
	ErrIllegalMove = errors.New("illegal dama move")
	ErrWrongTurn   = errors.New("not this player's turn")
	ErrStaleTurn   = errors.New("stale turn sequence")
	ErrGameOver    = errors.New("game is already over")
)

// Step is one submitted hop. A multi-capture turn is several steps.
type Step struct {
	From      int   `json:"from"`
	To        int   `json:"to"`
	Captures  []int `json:"captures,omitempty"`
	Promotion bool  `json:"promotion,omitempty"`
	// Continues is true when the same piece must keep capturing.
	Continues bool `json:"continues,omitempty"`
}

// HistoryEntry records one accepted hop for replay and notation.
type HistoryEntry struct {
	Seq       int   `json:"seq"`
	Player    int   `json:"player"`
	From      int   `json:"from"`
	To        int   `json:"to"`
	Captures  []int `json:"captures,omitempty"`
	Promotion bool  `json:"promotion,omitempty"`
}

// Result is the terminal outcome of a game.
type Result struct {
	Winner *Player `json:"winner,omitempty"`
	Draw   bool    `json:"draw"`
	Reason string  `json:"reason"`
}

// Result reason values.
const (
	ReasonNoLegalMoves = "no_legal_moves"
	ReasonHalfmoveDraw = "draw_halfmove"
	ReasonRepetition   = "draw_repetition"
)

// Game is a complete, self-contained Dama position.
type Game struct {
	Board Board
	Turn  Player
	Rules Ruleset

	TurnSeq       int
	MoveCount     int
	HalfmoveClock int
	// TurnHadCapture records whether the current turn already captured a piece.
	TurnHadCapture bool
	// ChainFrom is set while a multi-capture sequence is in progress.
	ChainFrom *int

	Repetitions map[string]int
	LastMove    *Step
	History     []HistoryEntry

	Result *Result
}

// NewGame starts a fresh game with the supplied ruleset.
func NewGame(rules Ruleset) *Game {
	g := &Game{
		Board:       InitialBoard(),
		Turn:        Light,
		Rules:       rules,
		Repetitions: make(map[string]int),
	}
	g.recordPosition()
	return g
}

// Clone returns a deep copy. Invalid input can therefore be tested against a
// clone without risking the live match state.
func (g *Game) Clone() *Game {
	if g == nil {
		return nil
	}
	cp := *g
	cp.Repetitions = make(map[string]int, len(g.Repetitions))
	for k, v := range g.Repetitions {
		cp.Repetitions[k] = v
	}
	if g.ChainFrom != nil {
		v := *g.ChainFrom
		cp.ChainFrom = &v
	}
	if g.LastMove != nil {
		step := *g.LastMove
		step.Captures = append([]int(nil), g.LastMove.Captures...)
		cp.LastMove = &step
	}
	if g.Result != nil {
		result := *g.Result
		if g.Result.Winner != nil {
			w := *g.Result.Winner
			result.Winner = &w
		}
		cp.Result = &result
	}
	cp.History = make([]HistoryEntry, len(g.History))
	for i, entry := range g.History {
		entry.Captures = append([]int(nil), entry.Captures...)
		cp.History[i] = entry
	}
	return &cp
}

// Cells encodes the board for the p2p JSON board state.
// 0 empty, 1 light man, 2 light king, 3 dark man, 4 dark king.
func (g *Game) Cells() [SquareCount]int {
	var cells [SquareCount]int
	for i, piece := range g.Board.Squares {
		if piece.IsEmpty() {
			continue
		}
		value := 1
		if piece.Player == Dark {
			value = 3
		}
		if piece.King {
			value++
		}
		cells[i] = value
	}
	return cells
}

// PieceFromCell decodes one board cell.
func PieceFromCell(value int) (Piece, bool) {
	switch value {
	case 1:
		return Piece{Player: Light}, true
	case 2:
		return Piece{Player: Light, King: true}, true
	case 3:
		return Piece{Player: Dark}, true
	case 4:
		return Piece{Player: Dark, King: true}, true
	case 0:
		return Piece{}, true
	default:
		return Piece{}, false
	}
}

// LegalSteps returns the legal next hops for the side to move. During a
// capture chain only continuations from ChainFrom are returned.
func (g *Game) LegalSteps() []Step {
	if g == nil || g.Result != nil {
		return nil
	}

	paths := g.legalPaths()
	steps := make([]Step, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		first := path.hops[0]
		key := strconv.Itoa(path.from) + ":" + strconv.Itoa(first.to)
		if seen[key] {
			continue
		}
		seen[key] = true

		step := Step{
			From:      path.from,
			To:        first.to,
			Continues: len(path.hops) > 1,
		}
		if first.hasCapture {
			step.Captures = []int{first.captured}
		}
		if !step.Continues {
			piece := g.Board.At(path.from)
			step.Promotion = !piece.IsEmpty() && !piece.King && promotes(piece.Player, first.to/BoardSize)
		}
		steps = append(steps, step)
	}
	return steps
}

// ApplyStep validates and applies one hop. It returns the accepted step and
// whether the full turn is complete. On error the game is untouched.
func (g *Game) ApplyStep(from, to int) (*Step, bool, error) {
	if g == nil {
		return nil, false, fmt.Errorf("%w: no game", ErrIllegalMove)
	}
	if g.Result != nil {
		return nil, false, ErrGameOver
	}
	if from < 0 || from >= SquareCount || to < 0 || to >= SquareCount {
		return nil, false, fmt.Errorf("%w: square out of range", ErrIllegalMove)
	}
	if from == to {
		return nil, false, fmt.Errorf("%w: source and destination are the same", ErrIllegalMove)
	}

	paths := g.legalPaths()
	var chosen *pathMove
	for i := range paths {
		if paths[i].from == from && paths[i].hops[0].to == to {
			chosen = &paths[i]
			break
		}
	}
	if chosen == nil {
		reason := "move is not legal"
		if g.ChainFrom != nil {
			reason = fmt.Sprintf("capture chain must continue from square %d", *g.ChainFrom)
		}
		return nil, false, fmt.Errorf("%w: %s", ErrIllegalMove, reason)
	}

	first := chosen.hops[0]
	piece := g.Board.At(from)
	if piece.IsEmpty() || piece.Player != g.Turn {
		return nil, false, fmt.Errorf("%w: no movable %s piece on square %d", ErrIllegalMove, playerName(g.Turn), from)
	}

	// Apply on the live game only after every validation above succeeded.
	g.Board.Set(from, Piece{})
	if first.hasCapture {
		g.Board.Set(first.captured, Piece{})
	}
	g.Board.Set(to, piece)
	if first.hasCapture {
		g.TurnHadCapture = true
	}

	g.TurnSeq++
	g.MoveCount++

	promoted := false
	lastHop := len(chosen.hops) == 1
	if !piece.King && promotes(g.Turn, to/BoardSize) && (lastHop || g.Rules.PromotionDuringCapture) {
		promotedPiece := g.Board.At(to)
		promotedPiece.King = true
		g.Board.Set(to, promotedPiece)
		promoted = true
	}

	step := &Step{
		From:      from,
		To:        to,
		Continues: !lastHop,
		Promotion: promoted,
	}
	if first.hasCapture {
		step.Captures = []int{first.captured}
	}
	g.LastMove = step
	g.History = append(g.History, HistoryEntry{
		Seq:       g.MoveCount,
		Player:    int(g.Turn),
		From:      from,
		To:        to,
		Captures:  append([]int(nil), step.Captures...),
		Promotion: promoted,
	})

	if !lastHop {
		next := to
		g.ChainFrom = &next
		return step, false, nil
	}

	g.ChainFrom = nil
	mover := g.Turn
	if g.TurnHadCapture || promoted {
		g.HalfmoveClock = 0
	} else {
		g.HalfmoveClock++
	}
	g.TurnHadCapture = false
	g.Turn = g.Turn.Other()
	g.recordPosition()
	g.evaluateEnd(mover)
	return step, true, nil
}

// IsTurnComplete reports whether the current turn has no outstanding capture
// chain. Used by the p2p service to decide whether to switch players.
func (g *Game) IsTurnComplete() bool {
	return g == nil || g.ChainFrom == nil
}

// CanMove reports whether the side to move has at least one legal step.
func (g *Game) CanMove() bool {
	return len(g.LegalSteps()) > 0
}

// ---------------------------------------------------------------------------
// Move generation
// ---------------------------------------------------------------------------

type direction struct {
	dr int
	dc int
}

var allDirections = [4]direction{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}

type hop struct {
	to         int
	captured   int
	hasCapture bool
}

type pathMove struct {
	from int
	hops []hop
}

func forwardDirections(player Player) []direction {
	if player == Light {
		return []direction{{-1, -1}, {-1, 1}}
	}
	return []direction{{1, -1}, {1, 1}}
}

func (g *Game) moveDirections(piece Piece) []direction {
	if piece.King {
		return allDirections[:]
	}
	dirs := forwardDirections(piece.Player)
	if g.Rules.MenMoveBackwards {
		dirs = append(dirs, forwardDirections(piece.Player.Other())...)
	}
	return dirs
}

func (g *Game) captureDirections(piece Piece) []direction {
	if piece.King {
		return allDirections[:]
	}
	dirs := forwardDirections(piece.Player)
	if g.Rules.MenCaptureBackwards {
		dirs = append(dirs, forwardDirections(piece.Player.Other())...)
	}
	return dirs
}

func promotes(player Player, row int) bool {
	if player == Light {
		return row == 0
	}
	return row == BoardSize-1
}

func playerName(player Player) string {
	if player == Light {
		return "light"
	}
	return "dark"
}

func inBounds(row, col int) bool {
	return row >= 0 && row < BoardSize && col >= 0 && col < BoardSize
}

func (g *Game) legalPaths() []pathMove {
	if g == nil || g.Result != nil {
		return nil
	}

	if g.ChainFrom != nil {
		// A capture chain is in progress: the player must keep jumping with
		// the same piece, and no other move is legal.
		return g.capturePathsFrom(*g.ChainFrom)
	}

	captures := g.allCapturePaths()
	if len(captures) > 0 && g.Rules.MandatoryCapture {
		if g.Rules.MajorityCapture {
			captures = keepMaximumCaptures(captures)
		}
		return captures
	}

	simples := g.allSimplePaths()
	if g.Rules.MandatoryCapture {
		return simples
	}
	return append(captures, simples...)
}

func (g *Game) allCapturePaths() []pathMove {
	var paths []pathMove
	for index := 0; index < SquareCount; index++ {
		piece := g.Board.At(index)
		if piece.IsEmpty() || piece.Player != g.Turn || !IsPlayable(index) {
			continue
		}
		paths = append(paths, g.capturePathsFrom(index)...)
	}
	return paths
}

func (g *Game) allSimplePaths() []pathMove {
	var paths []pathMove
	for index := 0; index < SquareCount; index++ {
		piece := g.Board.At(index)
		if piece.IsEmpty() || piece.Player != g.Turn || !IsPlayable(index) {
			continue
		}
		paths = append(paths, g.simplePathsFrom(index, piece)...)
	}
	return paths
}

func (g *Game) simplePathsFrom(from int, piece Piece) []pathMove {
	row, col := from/BoardSize, from%BoardSize

	if piece.King && g.Rules.FlyingKings {
		var paths []pathMove
		for _, d := range allDirections {
			r, c := row+d.dr, col+d.dc
			for inBounds(r, c) {
				index := r*BoardSize + c
				if !g.Board.At(index).IsEmpty() {
					break
				}
				paths = append(paths, pathMove{from: from, hops: []hop{{to: index}}})
				r += d.dr
				c += d.dc
			}
		}
		return paths
	}

	var paths []pathMove
	for _, d := range g.moveDirections(piece) {
		r, c := row+d.dr, col+d.dc
		if !inBounds(r, c) {
			continue
		}
		index := r*BoardSize + c
		if g.Board.At(index).IsEmpty() {
			paths = append(paths, pathMove{from: from, hops: []hop{{to: index}}})
		}
	}
	return paths
}

// capturePathsFrom enumerates every complete capture sequence starting from
// the given square. Captured pieces are treated as removed immediately; a
// jumped square cannot be captured twice in the same sequence.
func (g *Game) capturePathsFrom(from int) []pathMove {
	piece := g.Board.At(from)
	if piece.IsEmpty() || piece.Player != g.Turn || !IsPlayable(from) {
		return nil
	}

	var rec func(pos int, piece Piece, hops []hop, captured map[int]bool) []pathMove
	rec = func(pos int, piece Piece, hops []hop, captured map[int]bool) []pathMove {
		row, col := pos/BoardSize, pos%BoardSize
		var results []pathMove

		appendBranch := func(landing, capturedIndex int) {
			nextHops := make([]hop, len(hops)+1)
			copy(nextHops, hops)
			nextHops[len(hops)] = hop{to: landing, captured: capturedIndex, hasCapture: true}

			nextCaptured := make(map[int]bool, len(captured)+1)
			for k := range captured {
				nextCaptured[k] = true
			}
			nextCaptured[capturedIndex] = true

			nextPiece := piece
			if !nextPiece.King && g.Rules.PromotionDuringCapture && promotes(piece.Player, landing/BoardSize) {
				nextPiece.King = true
			}

			continuations := rec(landing, nextPiece, nextHops, nextCaptured)
			if len(continuations) == 0 {
				results = append(results, pathMove{from: from, hops: nextHops})
				return
			}
			results = append(results, continuations...)
		}

		for _, d := range g.captureDirections(piece) {
			if piece.King && g.Rules.FlyingKings {
				// Scan along the diagonal for the first piece. Empty and
				// already-captured squares are treated as empty.
				r, c := row+d.dr, col+d.dc
				for inBounds(r, c) {
					index := r*BoardSize + c
					if captured[index] || g.Board.At(index).IsEmpty() {
						r += d.dr
						c += d.dc
						continue
					}
					target := g.Board.At(index)
					if target.Player == piece.Player {
						break
					}
					// Any empty square beyond the enemy is a legal landing.
					lr, lc := r+d.dr, c+d.dc
					for inBounds(lr, lc) {
						landing := lr*BoardSize + lc
						if !g.Board.At(landing).IsEmpty() && !captured[landing] {
							break
						}
						appendBranch(landing, index)
						lr += d.dr
						lc += d.dc
					}
					break
				}
				continue
			}

			// Short-range capture (men and non-flying kings).
			mr, mc := row+d.dr, col+d.dc
			lr, lc := row+2*d.dr, col+2*d.dc
			if !inBounds(mr, mc) || !inBounds(lr, lc) {
				continue
			}
			mid := mr*BoardSize + mc
			landing := lr*BoardSize + lc
			if captured[mid] || g.Board.At(mid).IsEmpty() || g.Board.At(mid).Player == piece.Player {
				continue
			}
			if !g.Board.At(landing).IsEmpty() && !captured[landing] {
				continue
			}
			appendBranch(landing, mid)
		}

		return results
	}

	return rec(from, piece, nil, make(map[int]bool))
}

func captureCount(m pathMove) int {
	count := 0
	for _, h := range m.hops {
		if h.hasCapture {
			count++
		}
	}
	return count
}

func keepMaximumCaptures(paths []pathMove) []pathMove {
	maximum := 0
	for _, path := range paths {
		if count := captureCount(path); count > maximum {
			maximum = count
		}
	}
	filtered := make([]pathMove, 0, len(paths))
	for _, path := range paths {
		if captureCount(path) == maximum {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

// ---------------------------------------------------------------------------
// End-of-game and repetition rules
// ---------------------------------------------------------------------------

func (g *Game) recordPosition() {
	if g.Repetitions == nil {
		g.Repetitions = make(map[string]int)
	}
	g.Repetitions[g.positionKey()]++
}

func (g *Game) positionKey() string {
	var b strings.Builder
	b.Grow(SquareCount + 4)
	for _, piece := range g.Board.Squares {
		switch {
		case piece.IsEmpty():
			b.WriteByte('.')
		case piece.Player == Light && !piece.King:
			b.WriteByte('l')
		case piece.Player == Light && piece.King:
			b.WriteByte('L')
		case piece.Player == Dark && !piece.King:
			b.WriteByte('d')
		default:
			b.WriteByte('D')
		}
	}
	b.WriteByte('|')
	b.WriteByte(byte('0' + int(g.Turn)))
	if g.ChainFrom != nil {
		b.WriteByte('|')
		b.WriteString(strconv.Itoa(*g.ChainFrom))
	}
	return b.String()
}

func (g *Game) evaluateEnd(justMoved Player) {
	if len(g.legalPaths()) == 0 {
		winner := justMoved
		g.Result = &Result{Winner: &winner, Reason: ReasonNoLegalMoves}
		return
	}
	if g.Rules.HalfmoveDrawPlies > 0 && g.HalfmoveClock >= g.Rules.HalfmoveDrawPlies {
		g.Result = &Result{Draw: true, Reason: ReasonHalfmoveDraw}
		return
	}
	if g.Rules.RepetitionDraw > 1 && g.Repetitions[g.positionKey()] >= g.Rules.RepetitionDraw {
		g.Result = &Result{Draw: true, Reason: ReasonRepetition}
	}
}
