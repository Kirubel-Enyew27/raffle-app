package application

import (
	"context"
	"time"

	"github.com/raffle-app/backend/internal/notification/domain"
)

const maxRetries = 3

type NotificationService struct {
	repo  domain.NotificationRepository
	queue domain.Queue
}

func NewNotificationService(repo domain.NotificationRepository, queue domain.Queue) *NotificationService {
	return &NotificationService{repo: repo, queue: queue}
}

// Send creates a notification record and enqueues it for async delivery on each channel.
func (s *NotificationService) Send(ctx context.Context, userID string, channels []domain.Channel, event domain.EventType, p domain.Payload) error {
	subject, body := domain.Render(event, p)
	for _, ch := range channels {
		n := &domain.Notification{
			UserID:    userID,
			Channel:   ch,
			Event:     event,
			Subject:   subject,
			Body:      body,
			Status:    domain.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.repo.Create(ctx, n); err != nil {
			return err
		}
		if err := s.queue.Enqueue(ctx, n); err != nil {
			_ = s.repo.UpdateStatus(ctx, n.ID, domain.StatusFailed, err.Error())
			return err
		}
	}
	return nil
}

// ListForUser returns paginated in-app notifications for a user.
func (s *NotificationService) ListForUser(ctx context.Context, userID string, limit, offset int) ([]domain.Notification, int, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repo.FindByUserID(ctx, userID, limit, offset)
}

// MarkRead marks a single notification as read (in-app only).
func (s *NotificationService) MarkRead(ctx context.Context, id, userID string) error {
	return s.repo.MarkRead(ctx, id, userID)
}

// CountUnread returns the unread in-app notification count for a user.
func (s *NotificationService) CountUnread(ctx context.Context, userID string) (int, error) {
	return s.repo.CountUnread(ctx, userID)
}
