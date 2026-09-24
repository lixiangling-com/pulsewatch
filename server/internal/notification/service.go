package notification

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
)

type MailSender interface {
	Send(context.Context, string, string, string, string) error
}

type SMTPSender struct{ Addr, From string }

func (s SMTPSender) Send(ctx context.Context, to, subject, body, _ string) error {
	for _, header := range []string{s.From, to, subject} {
		if strings.ContainsAny(header, "\r\n") {
			return errors.New("invalid SMTP header")
		}
	}
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	r := bufio.NewReader(conn)
	read := func() error {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return err
			}
			if len(line) < 4 {
				return errors.New("invalid SMTP response")
			}
			if line[0] == '4' || line[0] == '5' {
				return errors.New("SMTP server rejected request")
			}
			if line[3] == ' ' {
				return nil
			}
		}
	}
	write := func(command string) error {
		if _, err := conn.Write([]byte(command + "\r\n")); err != nil {
			return err
		}
		return read()
	}
	if err := read(); err != nil {
		return err
	}
	if err := write("HELO localhost"); err != nil {
		return err
	}
	if err := write("MAIL FROM:<" + s.From + ">"); err != nil {
		return err
	}
	if err := write("RCPT TO:<" + to + ">"); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("DATA\r\n")); err != nil {
		return err
	}
	if err := read(); err != nil {
		return err
	}
	body = strings.ReplaceAll(body, "\r\n.", "\r\n..")
	if strings.HasPrefix(body, ".") {
		body = "." + body
	}
	msg := "From: " + s.From + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body + "\r\n.\r\n"
	if _, err := conn.Write([]byte(msg)); err != nil {
		return err
	}
	if err := read(); err != nil {
		return err
	}
	if _, err := conn.Write([]byte("QUIT\r\n")); err != nil {
		return err
	}
	return nil
}

type Consumer struct {
	repo   mailRepository
	sender MailSender
	logger *slog.Logger
}

type mailRepository interface {
	Get(context.Context, uuid.UUID) (sqlc.GetNotificationForEmailRow, error)
	MarkSent(context.Context, pgtype.UUID) error
	MarkFailed(context.Context, pgtype.UUID) error
}

type sqlMailRepository struct{ queries *sqlc.Queries }

func (r sqlMailRepository) Get(ctx context.Context, id uuid.UUID) (sqlc.GetNotificationForEmailRow, error) {
	return r.queries.GetNotificationForEmail(ctx, pgtype.UUID{Bytes: id, Valid: true})
}
func (r sqlMailRepository) MarkSent(ctx context.Context, id pgtype.UUID) error {
	_, err := r.queries.MarkNotificationSent(ctx, id)
	return err
}
func (r sqlMailRepository) MarkFailed(ctx context.Context, id pgtype.UUID) error {
	_, err := r.queries.MarkNotificationFailed(ctx, sqlc.MarkNotificationFailedParams{ID: id, ErrorCode: pgtype.Text{String: "smtp_error", Valid: true}, ErrorSummary: pgtype.Text{String: "邮件服务器暂时不可用", Valid: true}})
	return err
}

func NewConsumer(pool *pgxpool.Pool, sender MailSender, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{repo: sqlMailRepository{queries: sqlc.New(pool)}, sender: sender, logger: logger}
}

func NewConsumerWithDependencies(repo mailRepository, sender MailSender, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{repo: repo, sender: sender, logger: logger}
}
func (c *Consumer) HandleMail(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseMailPayload(task.Payload())
	if err != nil {
		return err
	}
	n, err := c.repo.Get(ctx, payload.NotificationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if n.Status == "sent" {
		return nil
	}
	if err := c.sender.Send(ctx, n.Email, n.Title, n.Body, n.MonitorName); err != nil {
		_ = c.repo.MarkFailed(context.Background(), n.ID)
		return err
	}
	return c.repo.MarkSent(ctx, n.ID)
}
