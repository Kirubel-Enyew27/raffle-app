package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// ClientChannelBuffer is the size of the per-client event channel.
	// Non-blocking sends drop the event when the buffer is full.
	ClientChannelBuffer = 64

	// staleTimeout is how long a client can go without a ping before
	// being swept as stale.
	staleTimeout = 90 * time.Second

	// sweepInterval is how often the stale-client sweep goroutine runs.
	sweepInterval = 30 * time.Second

	// maxConnectionsPerUser is the maximum number of concurrent SSE
	// connections allowed for a single user_id.
	maxConnectionsPerUser = 3
)

// Event is the payload that flows from Redis Pub/Sub to every connected client.
type Event struct {
	Type      string      `json:"type"`
	UserID    string      `json:"user_id,omitempty"`
	Role      string      `json:"role,omitempty"`
	Channel   string      `json:"channel,omitempty"`
	Payload   interface{} `json:"payload"`
	CreatedAt time.Time   `json:"created_at"`
}

// Client represents a single SSE connection.
type Client struct {
	UserID  string
	Role    string
	Channel string
	Ch      chan Event

	// LastPing is updated each time a heartbeat is written to the client.
	// It is read and written by multiple goroutines so it uses atomic
	// operations (stored as int64 UnixNano).
	lastPing atomic.Int64
}

// SetLastPing records the current time as the last successful ping.
func (c *Client) SetLastPing(t time.Time) {
	c.lastPing.Store(t.UnixNano())
}

// LastPing returns the time of the most recent successful ping.
func (c *Client) LastPing() time.Time {
	ns := c.lastPing.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// Service manages all local SSE clients and the Redis Pub/Sub fan-out.
type Service struct {
	rdb     *redis.Client
	log     *zap.Logger
	channel string

	mu      sync.RWMutex
	clients map[*Client]bool

	// userConns tracks per-user connection counts for the per-user cap.
	userConnsMu sync.Mutex
	userConns   map[string]int

	// Atomic counters — hot-path read/write without the mutex.
	dropCount  atomic.Int64
	eventCount atomic.Int64
}

// NewService creates a new realtime service backed by the given Redis client.
func NewService(rdb *redis.Client, log *zap.Logger) *Service {
	return &Service{
		rdb:       rdb,
		log:       log,
		channel:   "raffle_realtime_events",
		clients:   make(map[*Client]bool),
		userConns: make(map[string]int),
	}
}

// Start begins listening for events on Redis Pub/Sub and broadcasting them to
// local clients. It also launches the stale-client sweep goroutine.
// Call this in a separate goroutine; it returns when ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	pubsub := s.rdb.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	s.log.Info("Real-time Redis Pub/Sub listener started")

	go s.sweepStaleClients(ctx)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Real-time Redis Pub/Sub listener stopping")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev Event
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				s.log.Error("Failed to unmarshal real-time event", zap.Error(err))
				continue
			}
			s.eventCount.Add(1)
			s.broadcastLocal(ev)
		}
	}
}

// Shutdown drains and closes all client channels, then waits until done or the
// context deadline is reached. It is safe to call from a SIGTERM handler.
func (s *Service) Shutdown(ctx context.Context) {
	s.log.Info("Realtime service shutting down — draining clients")

	s.mu.Lock()
	clients := make([]*Client, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	// Clear the map so no new events are routed while we drain.
	s.clients = make(map[*Client]bool)
	s.mu.Unlock()

	// Close every remaining client channel so their goroutines unblock.
	for _, c := range clients {
		close(c.Ch)
	}

	// Release per-user connection counters.
	s.userConnsMu.Lock()
	s.userConns = make(map[string]int)
	s.userConnsMu.Unlock()

	s.log.Info("Realtime service shutdown complete", zap.Int("clients_drained", len(clients)))
}

// Publish sends a user/role-targeted event to Redis Pub/Sub.
func (s *Service) Publish(ctx context.Context, eventType string, userID string, role string, payload interface{}) error {
	return s.publish(ctx, Event{
		Type:      eventType,
		UserID:    userID,
		Role:      role,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// PublishToChannel publishes a channel-scoped event (e.g. a shared game room).
func (s *Service) PublishToChannel(ctx context.Context, channel, eventType string, payload interface{}) error {
	if channel == "" {
		return s.Publish(ctx, eventType, "", "", payload)
	}
	return s.publish(ctx, Event{
		Type:      eventType,
		Channel:   channel,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

func (s *Service) publish(ctx context.Context, ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return s.rdb.Publish(ctx, s.channel, data).Err()
}

// Subscribe registers a local client and returns it.
func (s *Service) Subscribe(userID, role string) *Client {
	return s.SubscribeChannel(userID, role, "")
}

// SubscribeChannel registers a local client that additionally receives events
// published to the given channel. Passing an empty channel is identical to
// Subscribe, so existing callers are unaffected.
func (s *Service) SubscribeChannel(userID, role, channel string) *Client {
	c := &Client{
		UserID:  userID,
		Role:    role,
		Channel: channel,
		Ch:      make(chan Event, ClientChannelBuffer),
	}
	c.SetLastPing(time.Now())

	s.mu.Lock()
	s.clients[c] = true
	s.mu.Unlock()

	s.log.Debug("Client subscribed to real-time events",
		zap.String("user_id", userID),
		zap.String("role", role),
		zap.String("channel", channel),
	)
	return c
}

// Unsubscribe unregisters a local client and closes its channel.
func (s *Service) Unsubscribe(c *Client) {
	s.mu.Lock()
	_, existed := s.clients[c]
	delete(s.clients, c)
	s.mu.Unlock()

	if existed {
		close(c.Ch)
	}

	s.log.Debug("Client unsubscribed from real-time events",
		zap.String("user_id", c.UserID),
		zap.String("role", c.Role),
	)
}

// CanConnect returns true if the user has fewer than maxConnectionsPerUser
// concurrent SSE connections, and atomically increments the counter if so.
func (s *Service) CanConnect(userID string) bool {
	if userID == "" {
		return true
	}
	s.userConnsMu.Lock()
	defer s.userConnsMu.Unlock()
	if s.userConns[userID] >= maxConnectionsPerUser {
		return false
	}
	s.userConns[userID]++
	return true
}

// ReleaseConnection decrements the per-user connection counter. Must be called
// when an SSE connection is closed (typically via defer).
func (s *Service) ReleaseConnection(userID string) {
	if userID == "" {
		return
	}
	s.userConnsMu.Lock()
	defer s.userConnsMu.Unlock()
	if s.userConns[userID] > 0 {
		s.userConns[userID]--
	}
	if s.userConns[userID] == 0 {
		delete(s.userConns, userID)
	}
}

// ConnectedCount returns the current number of locally subscribed clients.
func (s *Service) ConnectedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// DroppedCount returns the total number of events dropped due to full client
// buffers since the service started.
func (s *Service) DroppedCount() int {
	return int(s.dropCount.Load())
}

// EventCount returns the total number of events processed from Redis since the
// service started.
func (s *Service) EventCount() int {
	return int(s.eventCount.Load())
}

// broadcastLocal fans out ev to every matching local client.
// It is called from the Redis subscriber goroutine and MUST NOT block.
func (s *Service) broadcastLocal(ev Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.clients {
		match := false
		switch {
		case ev.UserID != "":
			match = client.UserID == ev.UserID
		case ev.Role != "":
			match = client.Role == ev.Role
		case ev.Channel != "":
			match = client.Channel == ev.Channel
		default:
			match = true // Broadcast to all connected clients
		}

		if !match {
			continue
		}

		select {
		case client.Ch <- ev:
		default:
			// Buffer full — count and log but never block.
			s.dropCount.Add(1)
			s.log.Warn("Event dropped: client channel full",
				zap.String("user_id", client.UserID),
				zap.String("channel", client.Channel),
				zap.String("event_type", ev.Type),
				zap.Int64("total_drops", s.dropCount.Load()),
			)
		}
	}
}

// sweepStaleClients periodically removes clients whose last ping is older than
// staleTimeout. It runs until ctx is cancelled.
func (s *Service) sweepStaleClients(ctx context.Context) {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.removeStaleClients()
		}
	}
}

func (s *Service) removeStaleClients() {
	now := time.Now()
	var stale []*Client

	s.mu.RLock()
	for c := range s.clients {
		lp := c.LastPing()
		if !lp.IsZero() && now.Sub(lp) > staleTimeout {
			stale = append(stale, c)
		}
	}
	s.mu.RUnlock()

	if len(stale) == 0 {
		return
	}

	s.mu.Lock()
	for _, c := range stale {
		delete(s.clients, c)
	}
	s.mu.Unlock()

	for _, c := range stale {
		close(c.Ch)
		s.ReleaseConnection(c.UserID)
		s.log.Info("Stale SSE client removed",
			zap.String("user_id", c.UserID),
			zap.String("channel", c.Channel),
		)
	}
}
