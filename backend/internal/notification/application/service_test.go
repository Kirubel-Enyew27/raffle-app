package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/raffle-app/backend/internal/notification/domain"
)

// --- fakes ---

type fakeRepo struct {
	records map[string]*domain.Notification
	seq     int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{records: map[string]*domain.Notification{}} }

func (f *fakeRepo) Create(_ context.Context, n *domain.Notification) error {
	f.seq++
	n.ID = "n-" + string(rune('0'+f.seq))
	clone := *n
	f.records[n.ID] = &clone
	return nil
}
func (f *fakeRepo) UpdateStatus(_ context.Context, id string, s domain.Status, e string) error {
	if n, ok := f.records[id]; ok {
		n.Status = s
		n.Error = e
	}
	return nil
}
func (f *fakeRepo) IncrRetries(_ context.Context, id string) error {
	if n, ok := f.records[id]; ok {
		n.Retries++
	}
	return nil
}
func (f *fakeRepo) FindByUserID(_ context.Context, userID string, limit, offset int) ([]domain.Notification, int, error) {
	var ns []domain.Notification
	for _, n := range f.records {
		if n.UserID == userID && n.Channel == domain.ChannelInApp {
			ns = append(ns, *n)
		}
	}
	return ns, len(ns), nil
}
func (f *fakeRepo) FindByID(_ context.Context, id string) (*domain.Notification, error) {
	n, ok := f.records[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return n, nil
}
func (f *fakeRepo) MarkRead(_ context.Context, id, userID string) error {
	n, ok := f.records[id]
	if !ok || n.UserID != userID {
		return errors.New("not found")
	}
	now := time.Now()
	n.ReadAt = &now
	return nil
}
func (f *fakeRepo) CountUnread(_ context.Context, userID string) (int, error) {
	count := 0
	for _, n := range f.records {
		if n.UserID == userID && n.Channel == domain.ChannelInApp && n.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

type fakeQueue struct{ items []*domain.Notification }

func (q *fakeQueue) Enqueue(_ context.Context, n *domain.Notification) error {
	q.items = append(q.items, n)
	return nil
}
func (q *fakeQueue) Dequeue(_ context.Context) (*domain.Notification, error) {
	if len(q.items) == 0 {
		return nil, errors.New("empty")
	}
	n := q.items[0]
	q.items = q.items[1:]
	return n, nil
}

// --- tests ---

func TestSend_EnqueuesForEachChannel(t *testing.T) {
	repo := newFakeRepo()
	q := &fakeQueue{}
	svc := NewNotificationService(repo, q)

	err := svc.Send(context.Background(), "user-1",
		[]domain.Channel{domain.ChannelEmail, domain.ChannelInApp},
		domain.EventDeposit,
		domain.Payload{UserName: "Alice", Amount: 50},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.records) != 2 {
		t.Errorf("expected 2 records, got %d", len(repo.records))
	}
	if len(q.items) != 2 {
		t.Errorf("expected 2 queued items, got %d", len(q.items))
	}
}

func TestSend_CorrectSubjectRendered(t *testing.T) {
	repo := newFakeRepo()
	q := &fakeQueue{}
	svc := NewNotificationService(repo, q)

	svc.Send(context.Background(), "user-1", []domain.Channel{domain.ChannelInApp},
		domain.EventRegistration, domain.Payload{UserName: "Bob"})

	for _, n := range repo.records {
		if n.Subject == "" {
			t.Error("expected non-empty subject")
		}
	}
}

func TestListForUser(t *testing.T) {
	repo := newFakeRepo()
	q := &fakeQueue{}
	svc := NewNotificationService(repo, q)

	svc.Send(context.Background(), "user-1", []domain.Channel{domain.ChannelInApp}, domain.EventDeposit, domain.Payload{})
	svc.Send(context.Background(), "user-1", []domain.Channel{domain.ChannelInApp}, domain.EventWithdrawal, domain.Payload{})
	svc.Send(context.Background(), "user-2", []domain.Channel{domain.ChannelInApp}, domain.EventDeposit, domain.Payload{})

	ns, total, err := svc.ListForUser(context.Background(), "user-1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("expected 2 notifications for user-1, got %d", total)
	}
	_ = ns
}

func TestMarkRead(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNotificationService(repo, &fakeQueue{})

	svc.Send(context.Background(), "user-1", []domain.Channel{domain.ChannelInApp}, domain.EventDeposit, domain.Payload{})

	var id string
	for k := range repo.records {
		id = k
	}

	if err := svc.MarkRead(context.Background(), id, "user-1"); err != nil {
		t.Fatal(err)
	}
	if repo.records[id].ReadAt == nil {
		t.Error("expected ReadAt to be set")
	}
}

func TestCountUnread(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNotificationService(repo, &fakeQueue{})

	svc.Send(context.Background(), "user-1", []domain.Channel{domain.ChannelInApp}, domain.EventDeposit, domain.Payload{})
	svc.Send(context.Background(), "user-1", []domain.Channel{domain.ChannelInApp}, domain.EventWithdrawal, domain.Payload{})

	count, err := svc.CountUnread(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 unread, got %d", count)
	}
}

func TestSend_QueueFailureSetsStatusFailed(t *testing.T) {
	repo := newFakeRepo()
	q := &failQueue{}
	svc := NewNotificationService(repo, q)

	err := svc.Send(context.Background(), "user-1", []domain.Channel{domain.ChannelEmail},
		domain.EventDeposit, domain.Payload{})
	if err == nil {
		t.Fatal("expected error when queue fails")
	}
	for _, n := range repo.records {
		if n.Status != domain.StatusFailed {
			t.Errorf("expected status failed, got %s", n.Status)
		}
	}
}

type failQueue struct{}

func (q *failQueue) Enqueue(_ context.Context, _ *domain.Notification) error {
	return errors.New("queue unavailable")
}
func (q *failQueue) Dequeue(_ context.Context) (*domain.Notification, error) {
	return nil, errors.New("queue unavailable")
}
