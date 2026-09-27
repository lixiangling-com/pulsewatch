package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
)

type fakeMailRepository struct {
	row           sqlc.GetNotificationForEmailRow
	err           error
	failedIDs     []pgtype.UUID
	sentIDs       []pgtype.UUID
	markFailedErr error
}

func (r *fakeMailRepository) Get(context.Context, uuid.UUID) (sqlc.GetNotificationForEmailRow, error) {
	return r.row, r.err
}
func (r *fakeMailRepository) MarkSent(_ context.Context, id pgtype.UUID) error {
	r.sentIDs = append(r.sentIDs, id)
	return nil
}
func (r *fakeMailRepository) MarkFailed(_ context.Context, id pgtype.UUID) error {
	r.failedIDs = append(r.failedIDs, id)
	return r.markFailedErr
}

type fakeMailSender struct {
	err   error
	calls int
}

func (s *fakeMailSender) Send(context.Context, string, string, string, string) error {
	s.calls++
	return s.err
}

func TestMailConsumerMarksFailureAndReturnsErrorForRetry(t *testing.T) {
	id := uuid.New()
	repo := &fakeMailRepository{row: sqlc.GetNotificationForEmailRow{
		ID: pgtype.UUID{Bytes: id, Valid: true}, Status: "queued", Email: "user@example.com",
	}}
	sendErr := errors.New("mail server unavailable")
	sender := &fakeMailSender{err: sendErr}
	consumer := NewConsumerWithDependencies(repo, sender, nil)
	task, err := NewMailTask(id)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.HandleMail(context.Background(), task)
	if !errors.Is(err, sendErr) {
		t.Fatalf("HandleMail error = %v, want %v", err, sendErr)
	}
	if sender.calls != 1 {
		t.Fatalf("sender called %d times, want 1", sender.calls)
	}
	if len(repo.failedIDs) != 1 || repo.failedIDs[0].Bytes != id {
		t.Fatalf("failed IDs = %#v, want notification %s", repo.failedIDs, id)
	}
	if len(repo.sentIDs) != 0 {
		t.Fatalf("sent IDs = %#v, want none", repo.sentIDs)
	}
}

func TestMailConsumerSkipsAlreadySentNotification(t *testing.T) {
	id := uuid.New()
	repo := &fakeMailRepository{row: sqlc.GetNotificationForEmailRow{
		ID: pgtype.UUID{Bytes: id, Valid: true}, Status: "sent",
	}}
	sender := &fakeMailSender{}
	consumer := NewConsumerWithDependencies(repo, sender, nil)
	task, err := NewMailTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.HandleMail(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 0 || len(repo.sentIDs) != 0 || len(repo.failedIDs) != 0 {
		t.Fatalf("sent notification should be a no-op: sender=%d sent=%v failed=%v", sender.calls, repo.sentIDs, repo.failedIDs)
	}
}
