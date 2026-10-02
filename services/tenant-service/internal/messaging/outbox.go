package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"darksamus.com/ReserveHub/tenant-service/internal/repository"
	"github.com/nats-io/nats.go"
)

type Publisher struct {
	store     repository.Store
	conn      *nats.Conn
	jetstream nats.JetStreamContext
	interval  time.Duration
	logger    *slog.Logger
}

func NewPublisher(store repository.Store, url string, interval time.Duration, logger *slog.Logger) (*Publisher, error) {
	conn, err := nats.Connect(url, nats.Name("reservehub-tenant-service"), nats.Timeout(5*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("open JetStream context: %w", err)
	}
	if _, err := js.StreamInfo("TENANT_EVENTS"); err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			conn.Close()
			return nil, fmt.Errorf("inspect tenant event stream: %w", err)
		}
		if _, err := js.AddStream(&nats.StreamConfig{
			Name: "TENANT_EVENTS", Subjects: []string{"tenant.>"}, Storage: nats.FileStorage,
		}); err != nil {
			conn.Close()
			return nil, fmt.Errorf("create tenant event stream: %w", err)
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
			p.logger.Error("outbox publish failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context) error {
	events, err := p.store.FetchOutbox(ctx, 100)
	if err != nil {
		return err
	}
	for _, event := range events {
		message := nats.NewMsg(event.EventType)
		message.Data = event.Payload
		message.Header.Set(nats.MsgIdHdr, event.ID.String())
		message.Header.Set("content-type", "application/json")
		if _, err := p.jetstream.PublishMsg(message, nats.Context(ctx)); err != nil {
			return fmt.Errorf("publish event %s: %w", event.ID, err)
		}
		if err := p.store.MarkOutboxPublished(ctx, event.ID); err != nil {
			return fmt.Errorf("mark event %s published: %w", event.ID, err)
		}
		p.logger.Debug("outbox event published", "event_id", event.ID, "event_type", event.EventType)
	}
	return nil
}
