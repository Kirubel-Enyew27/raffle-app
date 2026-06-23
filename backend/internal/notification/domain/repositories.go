package domain

import "context"

type NotificationRepository interface {
	Create(ctx context.Context, n *Notification) error
	UpdateStatus(ctx context.Context, id string, status Status, errMsg string) error
	IncrRetries(ctx context.Context, id string) error
	FindByUserID(ctx context.Context, userID string, limit, offset int) ([]Notification, int, error)
	FindByID(ctx context.Context, id string) (*Notification, error)
	MarkRead(ctx context.Context, id, userID string) error
	CountUnread(ctx context.Context, userID string) (int, error)
}

// Queue is the async delivery queue.
type Queue interface {
	Enqueue(ctx context.Context, n *Notification) error
	// Dequeue blocks until a notification is available or ctx is cancelled.
	Dequeue(ctx context.Context) (*Notification, error)
}

// EmailSender sends an email.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}
