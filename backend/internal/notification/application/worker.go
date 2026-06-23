package application

import (
	"context"
	"time"

	"go.uber.org/zap"
	"github.com/raffle-app/backend/internal/notification/domain"
)

// Worker drains the queue and dispatches notifications through the appropriate sender.
type Worker struct {
	repo        domain.NotificationRepository
	queue       domain.Queue
	emailSender domain.EmailSender
	log         *zap.Logger
}

func NewWorker(repo domain.NotificationRepository, queue domain.Queue, email domain.EmailSender, log *zap.Logger) *Worker {
	return &Worker{repo: repo, queue: queue, emailSender: email, log: log}
}

// Run processes notifications until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	for {
		n, err := w.queue.Dequeue(ctx)
		if err != nil {
			// ctx cancelled — shut down cleanly
			return
		}
		w.dispatch(ctx, n)
	}
}

func (w *Worker) dispatch(ctx context.Context, n *domain.Notification) {
	var err error
	switch n.Channel {
	case domain.ChannelEmail:
		err = w.emailSender.Send(ctx, n.UserID, n.Subject, n.Body)
	case domain.ChannelInApp:
		// In-app notifications are stored at creation; nothing further to dispatch.
		err = nil
	}

	if err == nil {
		_ = w.repo.UpdateStatus(ctx, n.ID, domain.StatusSent, "")
		return
	}

	w.log.Warn("notification dispatch failed", zap.String("id", n.ID), zap.Error(err))

	if n.Retries >= maxRetries {
		_ = w.repo.UpdateStatus(ctx, n.ID, domain.StatusFailed, err.Error())
		return
	}

	// Exponential back-off before re-enqueue: 2^retries seconds.
	delay := time.Duration(1<<uint(n.Retries)) * time.Second
	time.Sleep(delay)

	_ = w.repo.IncrRetries(ctx, n.ID)
	n.Retries++
	if enqErr := w.queue.Enqueue(ctx, n); enqErr != nil {
		_ = w.repo.UpdateStatus(ctx, n.ID, domain.StatusFailed, enqErr.Error())
	}
}
