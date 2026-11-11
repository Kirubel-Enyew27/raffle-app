package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/raffle-app/backend/internal/p2p/domain"
	"github.com/redis/go-redis/v9"
)

const (
	seatKeyPrefix = "p2p:match_seats:"
	seatTTL       = time.Hour
	// Redis is an optimization here; never let a slow or unavailable instance
	// add meaningful latency to the authoritative PostgreSQL path.
	seatOperationTimeout = 10 * time.Millisecond
)

// SeatCache stores the immutable seat mapping of an in-progress match.
//
// Entries are keyed by match ID and never shared between matches. Only the
// seat rows are cached; the board, turn, version and outcome stay in
// PostgreSQL and are read with FOR UPDATE on every move.
type SeatCache struct {
	rdb *redis.Client
}

func NewSeatCache(rdb *redis.Client) *SeatCache {
	return &SeatCache{rdb: rdb}
}

func seatKey(matchID string) string {
	return seatKeyPrefix + matchID
}

// Get returns a decoded copy of the seat rows. Any Redis or decoding failure is
// reported as a miss so callers fall back to PostgreSQL.
func (c *SeatCache) Get(ctx context.Context, matchID string) ([]*domain.MatchPlayer, bool) {
	if c == nil || c.rdb == nil || matchID == "" {
		return nil, false
	}
	opCtx, cancel := context.WithTimeout(ctx, seatOperationTimeout)
	defer cancel()

	raw, err := c.rdb.Get(opCtx, seatKey(matchID)).Bytes()
	if err != nil {
		return nil, false
	}
	var players []*domain.MatchPlayer
	if err := json.Unmarshal(raw, &players); err != nil || len(players) == 0 {
		return nil, false
	}
	return players, true
}

// Set caches the seat rows after a successful repository read. Failures are
// intentionally silent: the caller already has authoritative rows.
func (c *SeatCache) Set(ctx context.Context, matchID string, players []*domain.MatchPlayer) {
	if c == nil || c.rdb == nil || matchID == "" || len(players) == 0 {
		return
	}
	raw, err := json.Marshal(players)
	if err != nil {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, seatOperationTimeout)
	defer cancel()
	_ = c.rdb.Set(opCtx, seatKey(matchID), raw, seatTTL).Err()
}

// Delete removes the hint when a match reaches a terminal state.
func (c *SeatCache) Delete(ctx context.Context, matchID string) {
	if c == nil || c.rdb == nil || matchID == "" {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, seatOperationTimeout)
	defer cancel()
	_ = c.rdb.Del(opCtx, seatKey(matchID)).Err()
}
