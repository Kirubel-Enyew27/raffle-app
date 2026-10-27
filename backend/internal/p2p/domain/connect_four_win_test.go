package domain

import "testing"

func connectFourBoard(cells map[int]int) []int {
	board := make([]int, C4Cols*C4Rows)
	for index, marker := range cells {
		board[index] = marker
	}
	return board
}

func sameIndexes(got []int, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[int]bool, len(got))
	for _, index := range got {
		seen[index] = true
	}
	for _, index := range want {
		if !seen[index] {
			return false
		}
	}
	return true
}

// The board renders each token itself, so the winning four must be detected in
// every direction regardless of how the tokens look.
func TestCheckConnectFourWin_FindsEveryDirection(t *testing.T) {
	cases := []struct {
		name   string
		cells  map[int]int
		row    int
		col    int
		marker int
		want   []int
	}{
		{
			name:   "horizontal row",
			cells:  map[int]int{5*C4Cols + 0: 1, 5*C4Cols + 1: 1, 5*C4Cols + 2: 1, 5*C4Cols + 3: 1},
			row:    5,
			col:    1,
			marker: 1,
			want:   []int{35, 36, 37, 38},
		},
		{
			name:   "vertical column",
			cells:  map[int]int{2*C4Cols + 4: 2, 3*C4Cols + 4: 2, 4*C4Cols + 4: 2, 5*C4Cols + 4: 2},
			row:    4,
			col:    4,
			marker: 2,
			want:   []int{18, 25, 32, 39},
		},
		{
			name:   "diagonal down-right",
			cells:  map[int]int{2*C4Cols + 3: 1, 3*C4Cols + 2: 1, 4*C4Cols + 1: 1, 5*C4Cols + 0: 1},
			row:    4,
			col:    1,
			marker: 1,
			want:   []int{17, 23, 29, 35},
		},
		{
			name:   "diagonal down-left",
			cells:  map[int]int{2*C4Cols + 0: 2, 3*C4Cols + 1: 2, 4*C4Cols + 2: 2, 5*C4Cols + 3: 2},
			row:    4,
			col:    2,
			marker: 2,
			want:   []int{14, 22, 30, 38},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkConnectFourWin(connectFourBoard(tc.cells), tc.row, tc.col, tc.marker)
			if !sameIndexes(got, tc.want) {
				t.Fatalf("expected winning line %v, got %v", tc.want, got)
			}
		})
	}
}

func TestCheckConnectFourWin_IgnoresShortOrMixedLines(t *testing.T) {
	// Three in a row is not a win.
	three := connectFourBoard(map[int]int{5*C4Cols + 0: 1, 5*C4Cols + 1: 1, 5*C4Cols + 2: 1})
	if line := checkConnectFourWin(three, 5, 1, 1); line != nil {
		t.Fatalf("three tokens must not win, got %v", line)
	}

	// A four-cell span split between the two players is not a win either.
	mixed := connectFourBoard(map[int]int{5*C4Cols + 0: 2, 5*C4Cols + 1: 1, 5*C4Cols + 2: 2, 5*C4Cols + 3: 1})
	if line := checkConnectFourWin(mixed, 5, 1, 1); line != nil {
		t.Fatalf("a mixed row must not win, got %v", line)
	}
}

// A full board with no four-in-a-row must end as a draw, and the drawn match
// must not name a winner.
func TestConnectFourEngine_FullBoardWithoutLineIsADraw(t *testing.T) {
	engine := NewConnectFourEngine()

	// Blocks of two rows per column, alternating between the two markers, keep
	// every horizontal, vertical and diagonal run at two tokens or fewer.
	cells := make([]int, C4Cols*C4Rows)
	for row := 0; row < C4Rows; row++ {
		for col := 0; col < C4Cols; col++ {
			cells[row*C4Cols+col] = 1 + ((row/2 + col) % 2)
		}
	}

	// Leave the bottom of column 0 empty so the final drop fills the board at
	// (row 5, col 0), where the pattern expects marker 1 (player 0).
	lastIndex := 5*C4Cols + 0
	cells[lastIndex] = 0

	board := map[string]interface{}{
		"cells":      cells,
		"columns":    C4Cols,
		"rows":       C4Rows,
		"last_move":  nil,
		"move_count": C4Cols*C4Rows - 1,
	}

	filled, isOver, winner, isTie, err := engine.ValidateAndApplyMove(board, 0, map[string]interface{}{"column": 0})
	if err != nil {
		t.Fatalf("filling drop rejected: %v", err)
	}
	if !isOver || !isTie || winner != nil {
		t.Fatalf("expected a draw, got over=%v tie=%v winner=%v", isOver, isTie, winner)
	}
	if final, _ := filled["cells"].([]int); final[lastIndex] != 1 {
		t.Fatalf("expected player 0's marker in the last slot, got %d", final[lastIndex])
	}
}
