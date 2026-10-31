package realtime

import (
	"context"
	"encoding/json"
	"sort"
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
	Type                    string      `json:"type"`
	UserID                  string      `json:"user_id,omitempty"`
	Role                    string      `json:"role,omitempty"`
	Channel                 string      `json:"channel,omitempty"`
	Payload                 interface{} `json:"payload"`
	CreatedAt               time.Time   `json:"created_at"`
	ServerDeliveryLatencyMS float64     `json:"server_delivery_latency_ms,omitempty"`
}

// Client represents a single SSE connection.
type Client struct {
	UserID  string
	Role    string
	Channel string
	Ch      chan Event

	// seq orders connections by acceptance time so the per-user cap can evict
	// the oldest connection deterministically.
	seq int64

	// LastPing is updated each time a heartbeat is written to the client.
	// It is read and written by multiple goroutines so it uses atomic
	// operations (stored as int64 UnixNano).
	lastPing atomic.Int64
}

type overflowedClient struct {
	client    *Client
	eventType string
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
	byUser  map[string]map[*Client]bool
	byRole  map[string]map[*Client]bool
	byChan  map[string]map[*Client]bool

	// userConns tracks per-user connection counts for the per-user cap.
	userConnsMu sync.Mutex
	userConns   map[string]int

	// Atomic counters — hot-path read/write without the mutex.
	dropCount        atomic.Int64
	slowClientCount  atomic.Int64
	eventCount       atomic.Int64
	publishedCount   atomic.Int64
	publishFailures  atomic.Int64
	publishTimeNanos atomic.Int64
	publishBytes     atomic.Int64
	connectSeq       atomic.Int64
}

// QueueStats summarizes current local SSE buffering without exposing clients.
type QueueStats struct {
	Clients    int `json:"clients"`
	TotalDepth int `json:"total_depth"`
	MaxDepth   int `json:"max_depth"`
	Capacity   int `json:"capacity"`
}

// PublishStats summarizes Redis publication work since process start.
type PublishStats struct {
	Published           int64   `json:"published"`
	Failures            int64   `json:"failures"`
	AverageLatencyMS    float64 `json:"average_latency_ms"`
	AverageMessageBytes float64 `json:"average_message_bytes"`
}

// NewService creates a new realtime service backed by the given Redis client.
func NewService(rdb *redis.Client, log *zap.Logger) *Service {
	return &Service{
		rdb:       rdb,
		log:       log,
		channel:   "raffle_realtime_events",
		clients:   make(map[*Client]bool),
		byUser:    make(map[string]map[*Client]bool),
		byRole:    make(map[string]map[*Client]bool),
		byChan:    make(map[string]map[*Client]bool),
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
	s.byUser = make(map[string]map[*Client]bool)
	s.byRole = make(map[string]map[*Client]bool)
	s.byChan = make(map[string]map[*Client]bool)
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
	started := time.Now()
	data, err := json.Marshal(ev)
	if err != nil {
		s.publishFailures.Add(1)
		s.publishTimeNanos.Add(time.Since(started).Nanoseconds())
		return err
	}
	err = s.rdb.Publish(ctx, s.channel, data).Err()
	s.publishTimeNanos.Add(time.Since(started).Nanoseconds())
	if err != nil {
		s.publishFailures.Add(1)
		return err
	}
	s.publishedCount.Add(1)
	s.publishBytes.Add(int64(len(data)))
	return nil
}

// Subscribe registers a local client and returns it.
func (s *Service) Subscribe(userID, role string) *Client {
	return s.SubscribeChannel(userID, role, "")
}

// Connect registers a client for an HTTP stream without ever rejecting it.
//
// Two connections are never allowed to represent the same logical session: a
// newer stream for the same (user, channel) replaces the older one, which is
// what a page reload, React StrictMode remount or a second tab really is. The
// per-user cap is enforced by evicting the oldest connections instead of
// returning an error, because refusing the newest connection left players whose
// browser had reconnected before the server noticed the old socket permanently
// stuck on "Connecting" (the SSE request was answered with 429, retried, and
// answered with 429 again).
//
// Evicted clients are notified through a closed Ch, so their goroutines exit
// and their per-user slot is released.
func (s *Service) Connect(userID, role, channel string) *Client {
	c := &Client{
		UserID:  userID,
		Role:    role,
		Channel: channel,
		Ch:      make(chan Event, ClientChannelBuffer),
	}
	c.SetLastPing(time.Now())

	c.seq = s.connectSeq.Add(1)

	var evicted []*Client

	s.mu.Lock()
	if userID != "" {
		sameUser := make([]*Client, 0, len(s.clients))
		for existing := range s.clients {
			if existing.UserID == userID {
				sameUser = append(sameUser, existing)
			}
		}
		// Oldest first, so eviction always drops the least recent connection.
		sort.Slice(sameUser, func(i, j int) bool { return sameUser[i].seq < sameUser[j].seq })

		// 1. A newer stream for the same channel represents the same logical
		//    session (reload, remount, duplicate tab): it replaces the old one.
		kept := make([]*Client, 0, len(sameUser))
		for _, existing := range sameUser {
			if existing.Channel == channel {
				if s.removeClientLocked(existing) {
					evicted = append(evicted, existing)
				}
				continue
			}
			kept = append(kept, existing)
		}

		// 2. Keep at most maxConnectionsPerUser-1 existing connections so the
		//    new one still fits under the cap; evict the oldest ones.
		evictCount := len(kept) - (maxConnectionsPerUser - 1)
		for i := 0; i < evictCount && i < len(kept); i++ {
			if s.removeClientLocked(kept[i]) {
				evicted = append(evicted, kept[i])
			}
		}
	}
	s.addClientLocked(c)
	s.mu.Unlock()

	for _, old := range evicted {
		close(old.Ch)
		s.log.Info("SSE connection replaced by a newer one",
			zap.String("user_id", old.UserID),
			zap.String("channel", old.Channel),
		)
	}

	s.log.Debug("Client subscribed to real-time events",
		zap.String("user_id", userID),
		zap.String("role", role),
		zap.String("channel", channel),
	)
	return c
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
	s.addClientLocked(c)
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
	existed := s.removeClientLocked(c)
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

// SlowClientDisconnectCount returns the number of slow SSE clients disconnected
// after their outbound queue overflowed. They reconnect and reload authoritative
// state instead of remaining attached after losing an event.
func (s *Service) SlowClientDisconnectCount() int {
	return int(s.slowClientCount.Load())
}

// EventCount returns the total number of events processed from Redis since the
// service started.
func (s *Service) EventCount() int {
	return int(s.eventCount.Load())
}

// QueueStats reports the current SSE queue depth across local clients.
func (s *Service) QueueStats() QueueStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := QueueStats{Clients: len(s.clients), Capacity: ClientChannelBuffer}
	for client := range s.clients {
		depth := len(client.Ch)
		stats.TotalDepth += depth
		if depth > stats.MaxDepth {
			stats.MaxDepth = depth
		}
	}
	return stats
}

// PublishStats reports Redis publication counts and the average serialization
// plus Redis publish call duration since process start.
func (s *Service) PublishStats() PublishStats {
	published := s.publishedCount.Load()
	failures := s.publishFailures.Load()
	count := published + failures
	var average float64
	var averageBytes float64
	if count > 0 {
		average = float64(s.publishTimeNanos.Load()) / float64(count) / float64(time.Millisecond)
	}
	if published > 0 {
		averageBytes = float64(s.publishBytes.Load()) / float64(published)
	}
	return PublishStats{
		Published:           published,
		Failures:            failures,
		AverageLatencyMS:    average,
		AverageMessageBytes: averageBytes,
	}
}

// broadcastLocal fans out ev to every matching local client.
// It is called from the Redis subscriber goroutine and MUST NOT block.
func (s *Service) broadcastLocal(ev Event) {
	s.mu.RLock()

	var recipients map[*Client]bool
	switch {
	case ev.UserID != "":
		recipients = s.byUser[ev.UserID]
	case ev.Role != "":
		recipients = s.byRole[ev.Role]
	case ev.Channel != "":
		recipients = s.byChan[ev.Channel]
	default:
		recipients = s.clients
	}

	var overflowed []overflowedClient
	for client := range recipients {

		select {
		case client.Ch <- ev:
		default:
			// Never block other players on a slow consumer. Record the affected
			// connection and disconnect it after releasing the read lock; its
			// reconnect path fetches a fresh authoritative snapshot.
			s.dropCount.Add(1)
			overflowed = append(overflowed, overflowedClient{client: client, eventType: ev.Type})
		}
	}
	s.mu.RUnlock()

	if len(overflowed) > 0 {
		s.disconnectSlowClients(overflowed)
	}
}

func (s *Service) disconnectSlowClients(clients []overflowedClient) {
	var removed []overflowedClient
	s.mu.Lock()
	for _, overflow := range clients {
		if s.removeClientLocked(overflow.client) {
			removed = append(removed, overflow)
		}
	}
	s.mu.Unlock()

	for _, overflow := range removed {
		close(overflow.client.Ch)
		s.ReleaseConnection(overflow.client.UserID)
		s.slowClientCount.Add(1)
		s.log.Warn("SSE client disconnected after outbound queue overflow",
			zap.String("user_id", overflow.client.UserID),
			zap.String("channel", overflow.client.Channel),
			zap.String("event_type", overflow.eventType),
			zap.Int64("total_drops", s.dropCount.Load()),
		)
	}
}

// addClientLocked and removeClientLocked maintain recipient indexes alongside
// the canonical connection set. Callers must hold s.mu for writing.
func (s *Service) addClientLocked(c *Client) {
	if s.clients == nil {
		s.clients = make(map[*Client]bool)
	}
	s.clients[c] = true
	addClientIndex(s.byUser, c.UserID, c)
	addClientIndex(s.byRole, c.Role, c)
	addClientIndex(s.byChan, c.Channel, c)
}

func (s *Service) removeClientLocked(c *Client) bool {
	if !s.clients[c] {
		return false
	}
	delete(s.clients, c)
	removeClientIndex(s.byUser, c.UserID, c)
	removeClientIndex(s.byRole, c.Role, c)
	removeClientIndex(s.byChan, c.Channel, c)
	return true
}

func addClientIndex(index map[string]map[*Client]bool, key string, c *Client) {
	if key == "" {
		return
	}
	clients := index[key]
	if clients == nil {
		clients = make(map[*Client]bool)
		index[key] = clients
	}
	clients[c] = true
}

func removeClientIndex(index map[string]map[*Client]bool, key string, c *Client) {
	if key == "" {
		return
	}
	clients := index[key]
	delete(clients, c)
	if len(clients) == 0 {
		delete(index, key)
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

	// Select and remove stale clients under one exclusive lock. A separate
	// read pass followed by a write pass let Connect evict and close one of the
	// same channels in between, creating a possible double close.
	s.mu.Lock()
	for c := range s.clients {
		lp := c.LastPing()
		if !lp.IsZero() && now.Sub(lp) > staleTimeout {
			if s.removeClientLocked(c) {
				stale = append(stale, c)
			}
		}
	}
	s.mu.Unlock()

	if len(stale) == 0 {
		return
	}

	for _, c := range stale {
		close(c.Ch)
		s.ReleaseConnection(c.UserID)
		s.log.Info("Stale SSE client removed",
			zap.String("user_id", c.UserID),
			zap.String("channel", c.Channel),
		)
	}
}
