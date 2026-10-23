package domain

import (
	"fmt"

	"github.com/raffle-app/backend/internal/dama"
)

// DamaEngine adapts the pure Ethiopian Dama rules package to the p2p
// RuleEngine interface. It is the only bridge between match orchestration and
// the rules; the rules package itself has no dependency on p2p or the database.
type DamaEngine struct {
	rules dama.Ruleset
}

// NewDamaEngine returns an engine enforcing the documented default ruleset.
func NewDamaEngine() *DamaEngine {
	return &DamaEngine{rules: dama.DefaultRuleset()}
}

// NewDamaEngineWithRules allows alternate regional rulesets in tests or future
// game variants without touching match orchestration.
func NewDamaEngineWithRules(rules dama.Ruleset) *DamaEngine {
	return &DamaEngine{rules: rules}
}

// InitBoard returns the starting position plus the first player's legal steps.
func (e *DamaEngine) InitBoard() map[string]interface{} {
	return encodeDamaGame(dama.NewGame(e.rules))
}

// ValidateAndApplyMove validates a single hop against the server board and
// returns the resulting board. Invalid moves return an error and leave the
// stored board untouched.
func (e *DamaEngine) ValidateAndApplyMove(boardState map[string]interface{}, playerIndex int, action map[string]interface{}) (map[string]interface{}, bool, *int, bool, error) {
	if playerIndex != 0 && playerIndex != 1 {
		return nil, false, nil, false, fmt.Errorf("%w: invalid player index", ErrInvalidMove)
	}

	game, err := e.decode(boardState)
	if err != nil {
		return nil, false, nil, false, fmt.Errorf("%w: %v", ErrInvalidMove, err)
	}

	if game.Result != nil {
		return nil, false, nil, false, fmt.Errorf("%w: the game is already over", ErrInvalidMove)
	}

	// A client-supplied turn sequence makes retries and stale requests
	// detectable even if the same player still owns the turn (for example in a
	// multi-capture chain). Older clients may omit it; the row lock and turn
	// ownership checks still serialise those submissions.
	if raw, ok := action["turn_seq"]; ok && raw != nil {
		expected, err := parseInt(raw)
		if err != nil {
			return nil, false, nil, false, fmt.Errorf("%w: turn_seq must be an integer", ErrInvalidMove)
		}
		if expected != game.TurnSeq {
			return nil, false, nil, false, fmt.Errorf("%w: turn_seq %d is stale (current %d)", ErrStaleMove, expected, game.TurnSeq)
		}
	}

	if dama.Player(playerIndex+1) != game.Turn {
		return nil, false, nil, false, fmt.Errorf("%w: it is %s's turn", ErrNotYourTurn, damaPlayerName(game.Turn))
	}

	fromVal, ok := action["from"]
	if !ok {
		return nil, false, nil, false, fmt.Errorf("%w: missing 'from' square", ErrInvalidMove)
	}
	toVal, ok := action["to"]
	if !ok {
		return nil, false, nil, false, fmt.Errorf("%w: missing 'to' square", ErrInvalidMove)
	}
	from, err := parseInt(fromVal)
	if err != nil {
		return nil, false, nil, false, fmt.Errorf("%w: 'from' must be an integer square", ErrInvalidMove)
	}
	to, err := parseInt(toVal)
	if err != nil {
		return nil, false, nil, false, fmt.Errorf("%w: 'to' must be an integer square", ErrInvalidMove)
	}

	if _, _, err := game.ApplyStep(from, to); err != nil {
		switch {
		case err == dama.ErrWrongTurn:
			return nil, false, nil, false, fmt.Errorf("%w: %v", ErrNotYourTurn, err)
		case err == dama.ErrGameOver:
			return nil, false, nil, false, fmt.Errorf("%w: %v", ErrInvalidMove, err)
		default:
			return nil, false, nil, false, fmt.Errorf("%w: %v", ErrInvalidMove, err)
		}
	}

	newBoard := encodeDamaGame(game)
	if game.Result == nil {
		return newBoard, false, nil, false, nil
	}

	if game.Result.Draw {
		return newBoard, true, nil, true, nil
	}
	if game.Result.Winner == nil {
		// Defensive: a result without a winner is treated as a draw rather than
		// silently awarding the match to one side.
		return newBoard, true, nil, true, nil
	}

	winner := int(*game.Result.Winner) - 1
	return newBoard, true, &winner, false, nil
}

// CheckMatchOver mirrors the single-round structure of Dama: the winner of the
// round wins the match, while a drawn round ends the match tied.
func (e *DamaEngine) CheckMatchOver(scores map[string]int, currentRound int, maxRounds int) (bool, *int) {
	if scores["0"] > 0 {
		winner := 0
		return true, &winner
	}
	if scores["1"] > 0 {
		winner := 1
		return true, &winner
	}
	if currentRound >= maxRounds {
		return true, nil
	}
	return false, nil
}

// TurnComplete reports whether the turn can be handed to the opponent. During
// a multi-capture chain the same player must keep moving.
func (e *DamaEngine) TurnComplete(boardState map[string]interface{}) bool {
	if boardState == nil {
		return true
	}
	value, ok := boardState["must_continue_from"]
	return !ok || value == nil
}

func (e *DamaEngine) decode(boardState map[string]interface{}) (*dama.Game, error) {
	if boardState == nil {
		return nil, fmt.Errorf("missing board state")
	}
	if variant, ok := boardState["variant"].(string); ok && variant != "" && variant != dama.Variant {
		return nil, fmt.Errorf("unexpected variant %q", variant)
	}

	cellsRaw, ok := boardState["cells"]
	if !ok {
		return nil, fmt.Errorf("missing cells")
	}
	cells, err := extractIntSlice(cellsRaw)
	if err != nil || len(cells) != dama.SquareCount {
		return nil, fmt.Errorf("cells must contain %d squares", dama.SquareCount)
	}

	var board dama.Board
	for index, value := range cells {
		piece, ok := dama.PieceFromCell(value)
		if !ok {
			return nil, fmt.Errorf("invalid piece value %d on square %d", value, index)
		}
		board.Set(index, piece)
	}

	turnRaw, ok := boardState["turn"]
	if !ok {
		return nil, fmt.Errorf("missing turn")
	}
	turnValue, err := parseInt(turnRaw)
	if err != nil || (turnValue != 0 && turnValue != 1) {
		return nil, fmt.Errorf("turn must be 0 or 1")
	}

	game := &dama.Game{
		Board:       board,
		Turn:        dama.Player(turnValue + 1),
		Rules:       e.rules,
		Repetitions: make(map[string]int),
	}

	if raw, ok := boardState["turn_seq"]; ok {
		value, err := parseInt(raw)
		if err != nil || value < 0 {
			return nil, fmt.Errorf("turn_seq must be a non-negative integer")
		}
		game.TurnSeq = value
	}
	if raw, ok := boardState["move_count"]; ok {
		value, err := parseInt(raw)
		if err != nil || value < 0 {
			return nil, fmt.Errorf("move_count must be a non-negative integer")
		}
		game.MoveCount = value
	}
	if raw, ok := boardState["halfmove_clock"]; ok {
		value, err := parseInt(raw)
		if err != nil || value < 0 {
			return nil, fmt.Errorf("halfmove_clock must be a non-negative integer")
		}
		game.HalfmoveClock = value
	}
	if raw, ok := boardState["turn_had_capture"]; ok {
		if value, ok := raw.(bool); ok {
			game.TurnHadCapture = value
		}
	}

	if raw, ok := boardState["must_continue_from"]; ok && raw != nil {
		value, err := parseInt(raw)
		if err != nil || value < 0 || value >= dama.SquareCount {
			return nil, fmt.Errorf("invalid must_continue_from")
		}
		piece := board.At(value)
		if piece.IsEmpty() || piece.Player != game.Turn {
			return nil, fmt.Errorf("must_continue_from does not hold a %s piece", damaPlayerName(game.Turn))
		}
		game.ChainFrom = &value
	}

	if raw, ok := boardState["repetitions"]; ok && raw != nil {
		switch reps := raw.(type) {
		case map[string]int:
			for k, v := range reps {
				game.Repetitions[k] = v
			}
		case map[string]interface{}:
			for k, v := range reps {
				parsed, err := parseInt(v)
				if err != nil {
					continue
				}
				game.Repetitions[k] = parsed
			}
		}
	}
	if len(game.Repetitions) == 0 {
		// Older/corrupt snapshots still need deterministic draw accounting.
		game.Repetitions["initial"] = 1
	}

	if raw, ok := boardState["last_move"]; ok && raw != nil {
		if step, err := decodeDamaStep(raw); err == nil {
			game.LastMove = step
		}
	}
	if raw, ok := boardState["move_history"]; ok && raw != nil {
		entries, err := decodeDamaHistory(raw)
		if err != nil {
			return nil, err
		}
		game.History = entries
	}
	if raw, ok := boardState["result"]; ok && raw != nil {
		if result, err := decodeDamaResult(raw); err == nil {
			game.Result = result
		}
	}

	return game, nil
}

func encodeDamaGame(game *dama.Game) map[string]interface{} {
	if game == nil {
		return map[string]interface{}{}
	}

	cells := game.Cells()
	cellValues := make([]int, dama.SquareCount)
	for i, value := range cells {
		cellValues[i] = value
	}

	steps := game.LegalSteps()
	legalMoves := make([]map[string]interface{}, 0, len(steps))
	for _, step := range steps {
		encoded := map[string]interface{}{
			"from":      step.From,
			"to":        step.To,
			"continues": step.Continues,
		}
		if len(step.Captures) > 0 {
			encoded["captures"] = step.Captures
		}
		if step.Promotion {
			encoded["promotion"] = true
		}
		legalMoves = append(legalMoves, encoded)
	}

	historyEntries := game.History
	if len(historyEntries) > 40 {
		historyEntries = historyEntries[len(historyEntries)-40:]
	}
	history := make([]map[string]interface{}, 0, len(historyEntries))
	for _, entry := range historyEntries {
		encoded := map[string]interface{}{
			"seq":    entry.Seq,
			"player": entry.Player - 1,
			"from":   entry.From,
			"to":     entry.To,
		}
		if len(entry.Captures) > 0 {
			encoded["captures"] = entry.Captures
		}
		if entry.Promotion {
			encoded["promotion"] = true
		}
		history = append(history, encoded)
	}

	board := map[string]interface{}{
		"variant":          dama.Variant,
		"board_size":       dama.BoardSize,
		"cells":            cellValues,
		"turn":             int(game.Turn) - 1,
		"turn_seq":         game.TurnSeq,
		"move_count":       game.MoveCount,
		"halfmove_clock":   game.HalfmoveClock,
		"turn_had_capture": game.TurnHadCapture,
		"legal_moves":      legalMoves,
		"move_history":     history,
		"repetitions":      game.Repetitions,
		"rules": map[string]interface{}{
			"men_move_backwards":       game.Rules.MenMoveBackwards,
			"men_capture_backwards":    game.Rules.MenCaptureBackwards,
			"flying_kings":             game.Rules.FlyingKings,
			"mandatory_capture":        game.Rules.MandatoryCapture,
			"majority_capture":         game.Rules.MajorityCapture,
			"promotion_during_capture": game.Rules.PromotionDuringCapture,
			"halfmove_draw_plies":      game.Rules.HalfmoveDrawPlies,
			"repetition_draw":          game.Rules.RepetitionDraw,
		},
	}
	if game.ChainFrom != nil {
		board["must_continue_from"] = *game.ChainFrom
	} else {
		board["must_continue_from"] = nil
	}
	if game.LastMove != nil {
		board["last_move"] = game.LastMove
	} else {
		board["last_move"] = nil
	}
	if game.Result != nil {
		result := map[string]interface{}{
			"draw":   game.Result.Draw,
			"reason": game.Result.Reason,
		}
		if game.Result.Winner != nil {
			result["winner"] = int(*game.Result.Winner) - 1
		} else {
			result["winner"] = nil
		}
		board["result"] = result
	} else {
		board["result"] = nil
	}
	return board
}

func decodeDamaStep(raw interface{}) (*dama.Step, error) {
	switch value := raw.(type) {
	case *dama.Step:
		if value == nil {
			return nil, fmt.Errorf("nil step")
		}
		cp := *value
		cp.Captures = append([]int(nil), value.Captures...)
		return &cp, nil
	case dama.Step:
		cp := value
		cp.Captures = append([]int(nil), value.Captures...)
		return &cp, nil
	case map[string]interface{}:
		step := &dama.Step{}
		if v, ok := value["from"]; ok {
			parsed, err := parseInt(v)
			if err != nil {
				return nil, err
			}
			step.From = parsed
		}
		if v, ok := value["to"]; ok {
			parsed, err := parseInt(v)
			if err != nil {
				return nil, err
			}
			step.To = parsed
		}
		if v, ok := value["captures"]; ok && v != nil {
			captures, err := extractIntSlice(v)
			if err != nil {
				return nil, err
			}
			step.Captures = captures
		}
		if v, ok := value["promotion"].(bool); ok {
			step.Promotion = v
		}
		if v, ok := value["continues"].(bool); ok {
			step.Continues = v
		}
		return step, nil
	default:
		return nil, fmt.Errorf("unsupported last_move value")
	}
}

func decodeDamaHistory(raw interface{}) ([]dama.HistoryEntry, error) {
	var items []interface{}
	switch value := raw.(type) {
	case []dama.HistoryEntry:
		return append([]dama.HistoryEntry(nil), value...), nil
	case []map[string]interface{}:
		items = make([]interface{}, len(value))
		for i, item := range value {
			items[i] = item
		}
	case []interface{}:
		items = value
	default:
		return nil, fmt.Errorf("unsupported move_history value")
	}
	{
		entries := make([]dama.HistoryEntry, 0, len(items))
		for _, item := range items {
			encoded, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			entry := dama.HistoryEntry{}
			if v, ok := encoded["seq"]; ok {
				if parsed, err := parseInt(v); err == nil {
					entry.Seq = parsed
				}
			}
			if v, ok := encoded["player"]; ok {
				if parsed, err := parseInt(v); err == nil && (parsed == 0 || parsed == 1) {
					entry.Player = parsed + 1
				}
			}
			if v, ok := encoded["from"]; ok {
				if parsed, err := parseInt(v); err == nil {
					entry.From = parsed
				}
			}
			if v, ok := encoded["to"]; ok {
				if parsed, err := parseInt(v); err == nil {
					entry.To = parsed
				}
			}
			if v, ok := encoded["captures"]; ok && v != nil {
				if captures, err := extractIntSlice(v); err == nil {
					entry.Captures = captures
				}
			}
			if v, ok := encoded["promotion"].(bool); ok {
				entry.Promotion = v
			}
			entries = append(entries, entry)
		}
		return entries, nil
	}
}

func decodeDamaResult(raw interface{}) (*dama.Result, error) {
	encoded, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unsupported result value")
	}
	result := &dama.Result{}
	if v, ok := encoded["draw"].(bool); ok {
		result.Draw = v
	}
	if v, ok := encoded["reason"].(string); ok {
		result.Reason = v
	}
	if v, ok := encoded["winner"]; ok && v != nil {
		parsed, err := parseInt(v)
		if err != nil || (parsed != 0 && parsed != 1) {
			return nil, fmt.Errorf("invalid result winner")
		}
		winner := dama.Player(parsed + 1)
		result.Winner = &winner
	}
	return result, nil
}

func damaPlayerName(player dama.Player) string {
	if player == dama.Light {
		return "light"
	}
	return "dark"
}
