package domain

import (
	"fmt"
	"math"
	"strconv"
)

type RuleEngine interface {
	InitBoard() map[string]interface{}
	ValidateAndApplyMove(boardState map[string]interface{}, playerIndex int, action map[string]interface{}) (newBoard map[string]interface{}, isRoundOver bool, roundWinnerIndex *int, isTie bool, err error)
	CheckMatchOver(scores map[string]int, currentRound int, maxRounds int) (isMatchOver bool, matchWinnerIndex *int)
}

// ---------------------------------------------------------------------------
// Tic-Tac-Toe Rule Engine (Blitz Best-of-3)
// ---------------------------------------------------------------------------

type TicTacToeEngine struct{}

func NewTicTacToeEngine() *TicTacToeEngine {
	return &TicTacToeEngine{}
}

func (e *TicTacToeEngine) InitBoard() map[string]interface{} {
	// 9 cells, initially 0 (empty). 1 = Player 0 (X), 2 = Player 1 (O).
	return map[string]interface{}{
		"cells":      []int{0, 0, 0, 0, 0, 0, 0, 0, 0},
		"last_move":  nil,
		"move_count": 0,
	}
}

func (e *TicTacToeEngine) ValidateAndApplyMove(boardState map[string]interface{}, playerIndex int, action map[string]interface{}) (map[string]interface{}, bool, *int, bool, error) {
	cellVal, ok := action["cell"]
	if !ok {
		return nil, false, nil, false, fmt.Errorf("%w: missing 'cell' parameter", ErrInvalidMove)
	}

	cellIdx, err := parseInt(cellVal)
	if err != nil || cellIdx < 0 || cellIdx > 8 {
		return nil, false, nil, false, fmt.Errorf("%w: cell must be an integer between 0 and 8", ErrInvalidMove)
	}

	cells, err := extractIntSlice(boardState["cells"])
	if err != nil || len(cells) != 9 {
		return nil, false, nil, false, fmt.Errorf("%w: corrupted board state", ErrInvalidMove)
	}

	if cells[cellIdx] != 0 {
		return nil, false, nil, false, fmt.Errorf("%w: cell %d is already occupied", ErrInvalidMove, cellIdx)
	}

	marker := playerIndex + 1 // 1 or 2
	cells[cellIdx] = marker

	moveCount := 0
	if mc, ok := boardState["move_count"].(float64); ok {
		moveCount = int(mc) + 1
	} else if mc, ok := boardState["move_count"].(int); ok {
		moveCount = mc + 1
	} else {
		moveCount = 1
	}

	newBoard := map[string]interface{}{
		"cells":      cells,
		"last_move":  cellIdx,
		"move_count": moveCount,
	}

	// Check win lines
	winLines := [8][3]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // cols
		{0, 4, 8}, {2, 4, 6}, // diagonals
	}

	for _, line := range winLines {
		if cells[line[0]] == marker && cells[line[1]] == marker && cells[line[2]] == marker {
			newBoard["winning_line"] = []int{line[0], line[1], line[2]}
			winner := playerIndex
			return newBoard, true, &winner, false, nil
		}
	}

	// Check tie (board full)
	isFull := true
	for _, c := range cells {
		if c == 0 {
			isFull = false
			break
		}
	}

	if isFull {
		return newBoard, true, nil, true, nil
	}

	return newBoard, false, nil, false, nil
}

func (e *TicTacToeEngine) CheckMatchOver(scores map[string]int, currentRound int, maxRounds int) (bool, *int) {
	p0Wins := scores["0"]
	p1Wins := scores["1"]
	targetWins := 2

	if p0Wins >= targetWins {
		w := 0
		return true, &w
	}
	if p1Wins >= targetWins {
		w := 1
		return true, &w
	}

	// If max rounds reached
	if currentRound >= maxRounds {
		if p0Wins > p1Wins {
			w := 0
			return true, &w
		} else if p1Wins > p0Wins {
			w := 1
			return true, &w
		}
		// True tie at end of max rounds
		return true, nil
	}

	return false, nil
}

// ---------------------------------------------------------------------------
// Connect Four Rule Engine (7 columns x 6 rows)
// ---------------------------------------------------------------------------

type ConnectFourEngine struct{}

func NewConnectFourEngine() *ConnectFourEngine {
	return &ConnectFourEngine{}
}

const (
	C4Cols = 7
	C4Rows = 6
)

func (e *ConnectFourEngine) InitBoard() map[string]interface{} {
	cells := make([]int, C4Cols*C4Rows)
	return map[string]interface{}{
		"cells":      cells,
		"columns":    C4Cols,
		"rows":       C4Rows,
		"last_move":  nil,
		"move_count": 0,
	}
}

func (e *ConnectFourEngine) ValidateAndApplyMove(boardState map[string]interface{}, playerIndex int, action map[string]interface{}) (map[string]interface{}, bool, *int, bool, error) {
	colVal, ok := action["column"]
	if !ok {
		return nil, false, nil, false, fmt.Errorf("%w: missing 'column' parameter", ErrInvalidMove)
	}

	col, err := parseInt(colVal)
	if err != nil || col < 0 || col >= C4Cols {
		return nil, false, nil, false, fmt.Errorf("%w: column must be an integer between 0 and %d", ErrInvalidMove, C4Cols-1)
	}

	cells, err := extractIntSlice(boardState["cells"])
	if err != nil || len(cells) != C4Cols*C4Rows {
		return nil, false, nil, false, fmt.Errorf("%w: corrupted board state", ErrInvalidMove)
	}

	// Gravity drop: search from bottom row (5) up to row (0)
	targetRow := -1
	for r := C4Rows - 1; r >= 0; r-- {
		idx := r*C4Cols + col
		if cells[idx] == 0 {
			targetRow = r
			break
		}
	}

	if targetRow == -1 {
		return nil, false, nil, false, fmt.Errorf("%w: column %d is full", ErrInvalidMove, col)
	}

	marker := playerIndex + 1
	placedIdx := targetRow*C4Cols + col
	cells[placedIdx] = marker

	moveCount := 0
	if mc, ok := boardState["move_count"].(float64); ok {
		moveCount = int(mc) + 1
	} else if mc, ok := boardState["move_count"].(int); ok {
		moveCount = mc + 1
	} else {
		moveCount = 1
	}

	newBoard := map[string]interface{}{
		"cells":      cells,
		"columns":    C4Cols,
		"rows":       C4Rows,
		"last_move":  map[string]int{"row": targetRow, "col": col, "index": placedIdx},
		"move_count": moveCount,
	}

	// Check 4-in-a-row around placed chip
	winningCells := checkConnectFourWin(cells, targetRow, col, marker)
	if len(winningCells) == 4 {
		newBoard["winning_line"] = winningCells
		winner := playerIndex
		return newBoard, true, &winner, false, nil
	}

	// Check full board tie
	isFull := true
	for c := 0; c < C4Cols; c++ {
		if cells[c] == 0 { // top row check
			isFull = false
			break
		}
	}

	if isFull {
		return newBoard, true, nil, true, nil
	}

	return newBoard, false, nil, false, nil
}

func (e *ConnectFourEngine) CheckMatchOver(scores map[string]int, currentRound int, maxRounds int) (bool, *int) {
	p0 := scores["0"]
	p1 := scores["1"]
	if p0 > 0 {
		w := 0
		return true, &w
	}
	if p1 > 0 {
		w := 1
		return true, &w
	}
	if currentRound >= maxRounds {
		return true, nil
	}
	return false, nil
}

func checkConnectFourWin(cells []int, r, c, marker int) []int {
	directions := [4][2]int{
		{0, 1},  // Horizontal
		{1, 0},  // Vertical
		{1, 1},  // Diagonal Down-Right
		{1, -1}, // Diagonal Down-Left
	}

	for _, d := range directions {
		dr, dc := d[0], d[1]
		line := []int{r*C4Cols + c}

		// Forward along direction
		for step := 1; step < 4; step++ {
			nr, nc := r+dr*step, c+dc*step
			if nr >= 0 && nr < C4Rows && nc >= 0 && nc < C4Cols && cells[nr*C4Cols+nc] == marker {
				line = append(line, nr*C4Cols+nc)
			} else {
				break
			}
		}

		// Backward along direction
		for step := 1; step < 4; step++ {
			nr, nc := r-dr*step, c-dc*step
			if nr >= 0 && nr < C4Rows && nc >= 0 && nc < C4Cols && cells[nr*C4Cols+nc] == marker {
				line = append(line, nr*C4Cols+nc)
			} else {
				break
			}
		}

		if len(line) >= 4 {
			return line[:4]
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Rock-Paper-Scissors Rule Engine (Best-of-3)
// ---------------------------------------------------------------------------

type RockPaperScissorsEngine struct{}

func NewRockPaperScissorsEngine() *RockPaperScissorsEngine {
	return &RockPaperScissorsEngine{}
}

func (e *RockPaperScissorsEngine) InitBoard() map[string]interface{} {
	return map[string]interface{}{
		"choices":           map[string]interface{}{},
		"last_round_result": nil,
		"move_count":        0,
	}
}

func (e *RockPaperScissorsEngine) ValidateAndApplyMove(boardState map[string]interface{}, playerIndex int, action map[string]interface{}) (map[string]interface{}, bool, *int, bool, error) {
	choiceVal, ok := action["choice"]
	if !ok {
		return nil, false, nil, false, fmt.Errorf("%w: missing 'choice' parameter", ErrInvalidMove)
	}

	choice, ok := choiceVal.(string)
	if !ok {
		return nil, false, nil, false, fmt.Errorf("%w: choice must be a string", ErrInvalidMove)
	}

	if choice != "rock" && choice != "paper" && choice != "scissors" {
		return nil, false, nil, false, fmt.Errorf("%w: choice must be 'rock', 'paper', or 'scissors'", ErrInvalidMove)
	}

	choices := make(map[string]interface{})
	if raw, ok := boardState["choices"].(map[string]interface{}); ok {
		for k, v := range raw {
			choices[k] = v
		}
	}

	playerKey := strconv.Itoa(playerIndex)
	if _, alreadyChosen := choices[playerKey]; alreadyChosen {
		return nil, false, nil, false, fmt.Errorf("%w: player %d already made choice for this round", ErrInvalidMove, playerIndex)
	}

	choices[playerKey] = choice

	moveCount := 0
	if mc, ok := boardState["move_count"].(float64); ok {
		moveCount = int(mc) + 1
	} else if mc, ok := boardState["move_count"].(int); ok {
		moveCount = mc + 1
	} else {
		moveCount = 1
	}

	newBoard := map[string]interface{}{
		"choices":           choices,
		"last_round_result": boardState["last_round_result"],
		"move_count":        moveCount,
	}

	// If only 1 player has chosen so far:
	if len(choices) < 2 {
		return newBoard, false, nil, false, nil
	}

	// Both players have chosen! Determine round winner
	c0, ok0 := choices["0"].(string)
	c1, ok1 := choices["1"].(string)
	if !ok0 || !ok1 {
		return nil, false, nil, false, fmt.Errorf("%w: corrupted choices in board state", ErrInvalidMove)
	}

	roundResult := map[string]interface{}{
		"p0_choice": c0,
		"p1_choice": c1,
	}

	if c0 == c1 {
		// Tie round
		roundResult["is_tie"] = true
		roundResult["winner"] = nil
		newBoard["last_round_result"] = roundResult
		newBoard["choices"] = map[string]interface{}{}
		return newBoard, true, nil, true, nil
	}

	// Rock beats scissors, scissors beats paper, paper beats rock
	p0Wins := (c0 == "rock" && c1 == "scissors") ||
		(c0 == "scissors" && c1 == "paper") ||
		(c0 == "paper" && c1 == "rock")

	var winner int
	if p0Wins {
		winner = 0
	} else {
		winner = 1
	}

	roundResult["is_tie"] = false
	roundResult["winner"] = winner
	newBoard["last_round_result"] = roundResult
	newBoard["choices"] = map[string]interface{}{}

	return newBoard, true, &winner, false, nil
}

func (e *RockPaperScissorsEngine) CheckMatchOver(scores map[string]int, currentRound int, maxRounds int) (bool, *int) {
	p0Wins := scores["0"]
	p1Wins := scores["1"]
	targetWins := 2 // Best-of-3

	if p0Wins >= targetWins {
		w := 0
		return true, &w
	}
	if p1Wins >= targetWins {
		w := 1
		return true, &w
	}

	if currentRound >= maxRounds {
		if p0Wins > p1Wins {
			w := 0
			return true, &w
		} else if p1Wins > p0Wins {
			w := 1
			return true, &w
		}
		return true, nil
	}

	return false, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		// Never truncate: 1.9 must not be accepted as cell 1.
		if math.IsNaN(val) || math.IsInf(val, 0) || math.Trunc(val) != val {
			return 0, fmt.Errorf("expected an integer, got %v", val)
		}
		if val > math.MaxInt32 || val < math.MinInt32 {
			return 0, fmt.Errorf("integer out of range")
		}
		return int(val), nil
	case string:
		return strconv.Atoi(val)
	default:
		return 0, fmt.Errorf("cannot parse integer")
	}
}

func extractIntSlice(v interface{}) ([]int, error) {
	switch val := v.(type) {
	case []int:
		cp := make([]int, len(val))
		copy(cp, val)
		return cp, nil
	case []interface{}:
		res := make([]int, len(val))
		for i, item := range val {
			n, err := parseInt(item)
			if err != nil {
				return nil, err
			}
			res[i] = n
		}
		return res, nil
	default:
		return nil, fmt.Errorf("not a slice")
	}
}
