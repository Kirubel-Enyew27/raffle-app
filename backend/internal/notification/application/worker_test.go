package application

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"github.com/raffle-app/backend/internal/notification/domain"
)

type fakeEmailSender struct{ fail bool }

func (f *fakeEmailSender) Send(_ context.Context, to, subject, body string) error {
	if f.fail {
		return errors.New("smtp error")
	}
	return nil
}

func newWorker(repo *fakeRepo, q *fakeQueue, fail bool) *Worker {
	return NewWorker(repo, q, &fakeEmailSender{fail: fail}, zap.NewNop())
}

func singleNotification(ch domain.Channel) *domain.Notification {
	return &domain.Notification{
		ID: "n-1", UserID: "user-1", Channel: ch,
		Subject: "Test", Body: "body", Status: domain.StatusPending,
	}
}

func TestWorker_EmailSuccess(t *testing.T) {
	repo := newFakeRepo()
	repo.records["n-1"] = singleNotification(domain.ChannelEmail)
	q := &fakeQueue{items: []*domain.Notification{repo.records["n-1"]}}
	w := newWorker(repo, q, false)

	w.dispatch(context.Background(), repo.records["n-1"])

	if repo.records["n-1"].Status != domain.StatusSent {
		t.Errorf("expected sent, got %s", repo.records["n-1"].Status)
	}
}

func TestWorker_EmailFailureSchedulesRetry(t *testing.T) {
	repo := newFakeRepo()
	n := singleNotification(domain.ChannelEmail)
	repo.records["n-1"] = n
	q := &fakeQueue{}
	w := NewWorker(repo, q, &fakeEmailSender{fail: true}, zap.NewNop())

	w.dispatch(context.Background(), n)

	// Should be re-enqueued (not failed yet — retries=0 < maxRetries)
	if len(q.items) != 1 {
		t.Errorf("expected 1 re-enqueued item, got %d", len(q.items))
	}
	if repo.records["n-1"].Retries != 1 {
		t.Errorf("expected retries=1, got %d", repo.records["n-1"].Retries)
	}
}

func TestWorker_ExhaustedRetriesMarksFailed(t *testing.T) {
	repo := newFakeRepo()
	n := singleNotification(domain.ChannelEmail)
	n.Retries = maxRetries
	repo.records["n-1"] = n
	q := &fakeQueue{}
	w := NewWorker(repo, q, &fakeEmailSender{fail: true}, zap.NewNop())

	w.dispatch(context.Background(), n)

	if repo.records["n-1"].Status != domain.StatusFailed {
		t.Errorf("expected failed, got %s", repo.records["n-1"].Status)
	}
	if len(q.items) != 0 {
		t.Error("should not re-enqueue after max retries")
	}
}

func TestWorker_InAppRequiresNoDispatch(t *testing.T) {
	repo := newFakeRepo()
	n := singleNotification(domain.ChannelInApp)
	repo.records["n-1"] = n
	q := &fakeQueue{}
	w := newWorker(repo, q, true) // email sender fails — should not be called

	w.dispatch(context.Background(), n)

	if repo.records["n-1"].Status != domain.StatusSent {
		t.Errorf("in-app should be marked sent, got %s", repo.records["n-1"].Status)
	}
}
