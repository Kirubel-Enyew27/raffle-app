package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Event struct {
	Type      string      `json:"type"`
	UserID    string      `json:"user_id,omitempty"`
	Role      string      `json:"role,omitempty"`
	Channel   string      `json:"channel,omitempty"`
	Payload   interface{} `json:"payload"`
	CreatedAt time.Time   `json:"created_at"`
}

type Client struct {
	UserID string
	Role   string
	// Channel is an optional extra subscription (for example the shared crash
	// game room). A client with no channel behaves exactly as before.
	Channel string
	Ch      chan Event
}

type Service struct {
	rdb     *redis.Client
	log     *zap.Logger
	channel string

	mu      sync.RWMutex
	clients map[*Client]bool
}

func NewService(rdb *redis.Client, log *zap.Logger) *Service {
	return &Service{
		rdb:     rdb,
		log:     log,
		channel: "raffle_realtime_events",
		clients: make(map[*Client]bool),
	}
}

// Start listens for events on Redis Pub/Sub and broadcasts them to local clients.
func (s *Service) Start(ctx context.Context) {
	pubsub := s.rdb.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	s.log.Info("Real-time Redis Pub/Sub listener started")

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
			s.broadcastLocal(ev)
		}
	}
}

// Publish sends an event to Redis Pub/Sub for distribution.
func (s *Service) Publish(ctx context.Context, eventType string, userID string, role string, payload interface{}) error {
	return s.publish(ctx, Event{
		Type:      eventType,
		UserID:    userID,
		Role:      role,
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// PublishToChannel publishes an event that is only delivered to clients
// subscribed to the given channel (for example every player watching the
// shared crash game room).
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

// Subscribe registers a local client for receiving filtered events.
func (s *Service) Subscribe(userID, role string) *Client {
	return s.SubscribeChannel(userID, role, "")
}

// SubscribeChannel registers a local client that additionally receives events
// published to the given channel. Passing an empty channel is identical to
// Subscribe, so existing connections are unaffected.
func (s *Service) SubscribeChannel(userID, role, channel string) *Client {
	c := &Client{
		UserID:  userID,
		Role:    role,
		Channel: channel,
		Ch:      make(chan Event, 64),
	}

	s.mu.Lock()
	s.clients[c] = true
	s.mu.Unlock()

	s.log.Debug("Client subscribed to real-time events",
		zap.String("user_id", userID), zap.String("role", role), zap.String("channel", channel))
	return c
}

// Unsubscribe unregisters a local client.
func (s *Service) Unsubscribe(c *Client) {
	s.mu.Lock()
	delete(s.clients, c)
	s.mu.Unlock()
	close(c.Ch)
	s.log.Debug("Client unsubscribed from real-time events", zap.String("user_id", c.UserID), zap.String("role", c.Role))
}

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
			match = true // Broadcast to all
		}

		if match {
			select {
			case client.Ch <- ev:
			default:
				// Channel is full, skip slow client
			}
		}
	}
}
