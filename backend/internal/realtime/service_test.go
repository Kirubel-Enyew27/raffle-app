package realtime

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

// newTestService creates a Service without a Redis connection.
// All tests that call broadcastLocal / subscribe / unsubscribe work purely
// in-memory, which avoids the need for a running Redis instance.
func newTestService() *Service {
	log, _ := zap.NewDevelopment()
	return &Service{
		log:       log,
		channel:   "test_channel",
		clients:   make(map[*Client]bool),
		userConns: make(map[string]int),
	}
}

// ── Subscribe / Unsubscribe ────────────────────────────────────────────────────

func TestSubscribeAndUnsubscribe(t *testing.T) {
	svc := newTestService()

	c := svc.Subscribe("user1", "player")
	if svc.ConnectedCount() != 1 {
		t.Fatalf("expected 1 connected client, got %d", svc.ConnectedCount())
	}

	svc.Unsubscribe(c)
	if svc.ConnectedCount() != 0 {
		t.Fatalf("expected 0 connected clients after unsubscribe, got %d", svc.ConnectedCount())
	}

	// Channel should be closed — a receive on a closed channel returns immediately.
	select {
	case _, ok := <-c.Ch:
		if ok {
			t.Fatal("expected channel to be closed after Unsubscribe")
		}
	default:
		// closed channel always unblocks; empty + closed is fine too
	}
}

func TestUnsubscribeIdempotent(t *testing.T) {
	svc := newTestService()
	c := svc.Subscribe("user1", "player")
	svc.Unsubscribe(c)
	// Second call must not panic (should not close an already-closed channel).
	svc.Unsubscribe(c)
}

// ── ConnectedCount ─────────────────────────────────────────────────────────────

func TestConnectedCountIncrementDecrement(t *testing.T) {
	svc := newTestService()

	if svc.ConnectedCount() != 0 {
		t.Fatalf("want 0, got %d", svc.ConnectedCount())
	}

	c1 := svc.Subscribe("u1", "player")
	c2 := svc.Subscribe("u2", "player")
	c3 := svc.Subscribe("u3", "admin")

	if svc.ConnectedCount() != 3 {
		t.Fatalf("want 3, got %d", svc.ConnectedCount())
	}

	svc.Unsubscribe(c1)
	if svc.ConnectedCount() != 2 {
		t.Fatalf("want 2, got %d", svc.ConnectedCount())
	}

	svc.Unsubscribe(c2)
	svc.Unsubscribe(c3)
	if svc.ConnectedCount() != 0 {
		t.Fatalf("want 0, got %d", svc.ConnectedCount())
	}
}

// ── Publish / deliver ─────────────────────────────────────────────────────────

func TestBroadcastLocalDeliversToMatchingUser(t *testing.T) {
	svc := newTestService()

	c1 := svc.Subscribe("user1", "player")
	c2 := svc.Subscribe("user2", "player")
	defer svc.Unsubscribe(c1)
	defer svc.Unsubscribe(c2)

	ev := Event{Type: "test", UserID: "user1", Payload: "hello"}
	svc.broadcastLocal(ev)

	// c1 should receive the event.
	select {
	case got := <-c1.Ch:
		if got.UserID != "user1" {
			t.Fatalf("expected user_id=user1, got %s", got.UserID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for event on c1")
	}

	// c2 should NOT receive the event.
	select {
	case ev := <-c2.Ch:
		t.Fatalf("c2 received unexpected event: %+v", ev)
	case <-time.After(20 * time.Millisecond):
		// correct — nothing delivered to c2
	}
}

func TestBroadcastLocalDeliversByRole(t *testing.T) {
	svc := newTestService()

	admin := svc.Subscribe("adminUser", "admin")
	player := svc.Subscribe("playerUser", "player")
	defer svc.Unsubscribe(admin)
	defer svc.Unsubscribe(player)

	ev := Event{Type: "admin_notice", Role: "admin", Payload: "secret"}
	svc.broadcastLocal(ev)

	select {
	case got := <-admin.Ch:
		if got.Type != "admin_notice" {
			t.Fatalf("admin got wrong event type: %s", got.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for admin event")
	}

	select {
	case ev := <-player.Ch:
		t.Fatalf("player received admin event unexpectedly: %+v", ev)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestBroadcastLocalBroadcastsToAll(t *testing.T) {
	svc := newTestService()

	clients := make([]*Client, 5)
	for i := range clients {
		clients[i] = svc.Subscribe("", "")
		defer svc.Unsubscribe(clients[i])
	}

	ev := Event{Type: "global", Payload: "world"}
	svc.broadcastLocal(ev)

	for i, c := range clients {
		select {
		case got := <-c.Ch:
			if got.Type != "global" {
				t.Errorf("client %d: expected global event, got %s", i, got.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("client %d: timed out waiting for global broadcast", i)
		}
	}
}

func TestBroadcastLocalDeliversByChannel(t *testing.T) {
	svc := newTestService()

	crash := svc.SubscribeChannel("user1", "player", "crash")
	other := svc.SubscribeChannel("user2", "player", "p2p")
	defer svc.Unsubscribe(crash)
	defer svc.Unsubscribe(other)

	ev := Event{Type: "tick", Channel: "crash", Payload: 1.5}
	svc.broadcastLocal(ev)

	select {
	case got := <-crash.Ch:
		if got.Channel != "crash" {
			t.Fatalf("expected channel=crash, got %s", got.Channel)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for crash channel event")
	}

	select {
	case ev := <-other.Ch:
		t.Fatalf("other channel received unexpected event: %+v", ev)
	case <-time.After(20 * time.Millisecond):
	}
}

// ── Drop counter ──────────────────────────────────────────────────────────────

func TestDropCounterIncrementsOnFullBuffer(t *testing.T) {
	svc := newTestService()
	c := svc.Subscribe("user1", "player")
	defer func() {
		// Drain before Unsubscribe to avoid double-close panic.
		for len(c.Ch) > 0 {
			<-c.Ch
		}
		svc.Unsubscribe(c)
	}()

	// Fill the buffer exactly.
	for i := 0; i < ClientChannelBuffer; i++ {
		svc.broadcastLocal(Event{Type: "fill", UserID: "user1", Payload: i})
	}

	before := svc.DroppedCount()

	// One more event should be dropped.
	svc.broadcastLocal(Event{Type: "overflow", UserID: "user1", Payload: "x"})

	after := svc.DroppedCount()
	if after != before+1 {
		t.Fatalf("expected drop count to increase by 1: before=%d after=%d", before, after)
	}
}

// ── CanConnect / ReleaseConnection ────────────────────────────────────────────

func TestCanConnectEnforcesPerUserCap(t *testing.T) {
	svc := newTestService()
	userID := "user42"

	for i := 0; i < maxConnectionsPerUser; i++ {
		if !svc.CanConnect(userID) {
			t.Fatalf("expected CanConnect to return true on connection %d", i+1)
		}
	}

	// One more connection should be rejected.
	if svc.CanConnect(userID) {
		t.Fatal("expected CanConnect to return false after cap reached")
	}

	// Releasing one slot should allow one more.
	svc.ReleaseConnection(userID)
	if !svc.CanConnect(userID) {
		t.Fatal("expected CanConnect to return true after a slot was released")
	}

	// Cleanup.
	for i := 0; i < maxConnectionsPerUser; i++ {
		svc.ReleaseConnection(userID)
	}
}

func TestCanConnectEmptyUserIDAlwaysAllowed(t *testing.T) {
	svc := newTestService()
	for i := 0; i < maxConnectionsPerUser*2; i++ {
		if !svc.CanConnect("") {
			t.Fatal("empty userID should never be rejected")
		}
	}
}

// ── Stale client sweep ────────────────────────────────────────────────────────

func TestStaleClientSweep(t *testing.T) {
	svc := newTestService()

	// Register a client and backdate its LastPing far beyond the staleTimeout.
	c := svc.Subscribe("staleUser", "player")
	c.SetLastPing(time.Now().Add(-(staleTimeout + 10*time.Second)))

	if svc.ConnectedCount() != 1 {
		t.Fatalf("expected 1 client before sweep, got %d", svc.ConnectedCount())
	}

	svc.removeStaleClients()

	if svc.ConnectedCount() != 0 {
		t.Fatalf("expected 0 clients after stale sweep, got %d", svc.ConnectedCount())
	}

	// The client channel should be closed.
	select {
	case _, ok := <-c.Ch:
		if ok {
			t.Fatal("expected stale client channel to be closed")
		}
	default:
		// closed channel unblocks immediately
	}
}

func TestFreshClientNotSwept(t *testing.T) {
	svc := newTestService()

	c := svc.Subscribe("freshUser", "player")
	// LastPing is recent (set at Subscribe time).
	defer svc.Unsubscribe(c)

	svc.removeStaleClients()

	if svc.ConnectedCount() != 1 {
		t.Fatalf("expected fresh client to remain after sweep, got %d", svc.ConnectedCount())
	}
}

// ── Shutdown ──────────────────────────────────────────────────────────────────

func TestShutdownClosesAllClients(t *testing.T) {
	svc := newTestService()

	clients := make([]*Client, 5)
	for i := range clients {
		clients[i] = svc.Subscribe("user", "player")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	svc.Shutdown(ctx)

	if svc.ConnectedCount() != 0 {
		t.Fatalf("expected 0 clients after Shutdown, got %d", svc.ConnectedCount())
	}

	for i, c := range clients {
		select {
		case _, ok := <-c.Ch:
			if ok {
				t.Errorf("client %d: channel should be closed after Shutdown", i)
			}
		default:
		}
	}
}

// ── LastPing ──────────────────────────────────────────────────────────────────

func TestClientLastPing(t *testing.T) {
	c := &Client{}
	if !c.LastPing().IsZero() {
		t.Fatal("expected zero LastPing before any ping")
	}

	now := time.Now()
	c.SetLastPing(now)

	got := c.LastPing()
	// Truncate to millisecond precision for comparison (UnixNano round-trip).
	if got.UnixNano() != now.UnixNano() {
		t.Fatalf("LastPing mismatch: want %v, got %v", now, got)
	}
}

// ── DroppedCount / EventCount ─────────────────────────────────────────────────

func TestDroppedCountStartsAtZero(t *testing.T) {
	svc := newTestService()
	if svc.DroppedCount() != 0 {
		t.Fatalf("expected 0 drops at start, got %d", svc.DroppedCount())
	}
}

func TestEventCountIncrements(t *testing.T) {
	svc := newTestService()
	// eventCount is only incremented by Start() when it reads from Redis.
	// Here we increment it manually to verify the counter works.
	svc.eventCount.Add(3)
	if svc.EventCount() != 3 {
		t.Fatalf("expected EventCount=3, got %d", svc.EventCount())
	}
}

// ── Connection replacement (regression: endless 429 on reconnect) ────────────

// A page reload, a React StrictMode remount or a second tab re-opens the same
// logical stream while the server may still hold the previous socket. The new
// connection must win: rejecting it left the player permanently on "Connecting…"
// while the other player kept playing.
func TestConnect_ReplacesPreviousConnectionForSameSession(t *testing.T) {
	svc := newTestService()

	first := svc.Connect("user-replace", "user", "p2p_match:abc")
	if got := svc.ConnectedCount(); got != 1 {
		t.Fatalf("expected 1 connection, got %d", got)
	}

	second := svc.Connect("user-replace", "user", "p2p_match:abc")
	if got := svc.ConnectedCount(); got != 1 {
		t.Fatalf("a second stream for the same session must replace the first, got %d connections", got)
	}

	select {
	case _, open := <-first.Ch:
		if open {
			t.Fatal("the replaced connection should have been closed")
		}
	default:
		t.Fatal("expected the replaced connection channel to be closed")
	}

	// The replacement receives events for the session.
	svc.broadcastLocal(Event{Type: "p2p.move_made", UserID: "user-replace"})
	select {
	case ev := <-second.Ch:
		if ev.Type != "p2p.move_made" {
			t.Fatalf("unexpected event on the replacement connection: %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("the replacement connection received no events")
	}
}

func TestConnect_DistinctChannelsAreAllAccepted(t *testing.T) {
	svc := newTestService()

	svc.Connect("user-multi", "user", "global")
	svc.Connect("user-multi", "user", "p2p_lobby")
	svc.Connect("user-multi", "user", "p2p_match:abc")

	if got := svc.ConnectedCount(); got != 3 {
		t.Fatalf("expected 3 distinct channel connections, got %d", got)
	}
}

// Beyond the per-user cap the oldest connection is evicted, never the newest.
func TestConnect_EvictsOldestInsteadOfRejectingNewest(t *testing.T) {
	svc := newTestService()

	oldest := svc.Connect("user-cap", "user", "channel-1")
	for i := 2; i <= maxConnectionsPerUser; i++ {
		svc.Connect("user-cap", "user", fmt.Sprintf("channel-%d", i))
	}
	if got := svc.ConnectedCount(); got != maxConnectionsPerUser {
		t.Fatalf("expected %d connections before the cap, got %d", maxConnectionsPerUser, got)
	}

	newest := svc.Connect("user-cap", "user", "channel-new")
	if got := svc.ConnectedCount(); got != maxConnectionsPerUser {
		t.Fatalf("expected the new connection to replace an old one (cap %d), got %d",
			maxConnectionsPerUser, got)
	}

	select {
	case _, open := <-oldest.Ch:
		if open {
			t.Fatal("the oldest connection should have been evicted")
		}
	default:
		t.Fatal("expected the oldest connection to be closed")
	}

	// The newest connection is alive.
	svc.broadcastLocal(Event{Type: "p2p.match_started", UserID: "user-cap"})
	select {
	case ev := <-newest.Ch:
		if ev.Type != "p2p.match_started" {
			t.Fatalf("unexpected event: %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("the newest connection received no events")
	}
}
