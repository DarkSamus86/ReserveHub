package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/repository"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type OutboxRepository interface {
	ClaimOutbox(context.Context, int) ([]repository.OutboxEvent, error)
	MarkOutboxPublished(context.Context, uuid.UUID) error
}

type Publisher struct {
	store     OutboxRepository
	conn      *nats.Conn
	jetstream nats.JetStreamContext
	interval  time.Duration
	logger    *slog.Logger
}

func NewPublisher(store OutboxRepository, url string, interval time.Duration, logger *slog.Logger) (*Publisher, error) {
	conn, err := nats.Connect(url, nats.Name("reservehub-admin-service"), nats.Timeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open JetStream context: %w", err)
	}
	if _, err := js.StreamInfo("ADMIN_AUDIT"); err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			conn.Close()
			return nil, fmt.Errorf("inspect admin audit stream: %w", err)
		}
		if _, err := js.AddStream(&nats.StreamConfig{
			Name: "ADMIN_AUDIT", Subjects: []string{"admin.audit.v1"}, Storage: nats.FileStorage,
		}); err != nil {
			// Another replica may have created the stream after StreamInfo.
			if _, inspectErr := js.StreamInfo("ADMIN_AUDIT"); inspectErr != nil {
				conn.Close()
				return nil, fmt.Errorf("create admin audit stream: %w", err)
			}
		}
	}
	return &Publisher{store: store, conn: conn, jetstream: js, interval: interval, logger: logger}, nil
}

func (p *Publisher) Close() {
	if err := p.conn.Drain(); err != nil {
		p.logger.Warn("failed to drain NATS connection", "error", err)
	}
	p.conn.Close()
}

func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		if err := p.publishBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			p.logger.Error("admin audit publish failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context) error {
	events, err := p.store.ClaimOutbox(ctx, 10)
	if err != nil {
		return err
	}
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("encode event %s: %w", event.ID, err)
		}
		message := nats.NewMsg("admin.audit.v1")
		message.Data = encoded
		message.Header.Set(nats.MsgIdHdr, event.ID.String())
		message.Header.Set("content-type", "application/json")
		publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err = p.jetstream.PublishMsg(message, nats.Context(publishCtx))
		cancel()
		if err != nil {
			return fmt.Errorf("publish event %s: %w", event.ID, err)
		}
		if err := p.store.MarkOutboxPublished(ctx, event.ID); err != nil {
			return fmt.Errorf("mark event %s published: %w", event.ID, err)
		}
		p.logger.Debug("admin audit published", "event_id", event.ID, "event_type", event.EventType)
	}
	return nil
}
