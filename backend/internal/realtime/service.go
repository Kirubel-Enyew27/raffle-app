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
	Payload   interface{} `json:"payload"`
	CreatedAt time.Time   `json:"created_at"`
}

type Client struct {
	UserID string
	Role   string
	Ch     chan Event
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
	ev := Event{
		Type:      eventType,
		UserID:    userID,
		Role:      role,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	return s.rdb.Publish(ctx, s.channel, data).Err()
}

// Subscribe registers a local client for receiving filtered events.
func (s *Service) Subscribe(userID, role string) *Client {
	c := &Client{
		UserID: userID,
		Role:   role,
		Ch:     make(chan Event, 64),
	}

	s.mu.Lock()
	s.clients[c] = true
	s.mu.Unlock()

	s.log.Debug("Client subscribed to real-time events", zap.String("user_id", userID), zap.String("role", role))
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
		if ev.UserID != "" {
			if client.UserID == ev.UserID {
				match = true
			}
		} else if ev.Role != "" {
			if client.Role == ev.Role {
				match = true
			}
		} else {
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
