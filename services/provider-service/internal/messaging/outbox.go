package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"darksamus.com/ReserveHub/provider-service/internal/outbox"
	"github.com/nats-io/nats.go"
)

type Publisher struct {
	store    outbox.Store
	conn     *nats.Conn
	js       nats.JetStreamContext
	interval time.Duration
	logger   *slog.Logger
}

func NewPublisher(store outbox.Store, url string, interval time.Duration, logger *slog.Logger) (*Publisher, error) {
	conn, err := nats.Connect(url, nats.Name("reservehub-provider-service"), nats.Timeout(5*time.Second))
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if _, err = js.StreamInfo("PROVIDER_AUDIT"); err != nil {
		if !errors.Is(err, nats.ErrStreamNotFound) {
			conn.Close()
			return nil, err
		}
		if _, err = js.AddStream(&nats.StreamConfig{Name: "PROVIDER_AUDIT", Subjects: []string{"provider.audit.v1"}, Storage: nats.FileStorage}); err != nil {
			if _, checkErr := js.StreamInfo("PROVIDER_AUDIT"); checkErr != nil {
				conn.Close()
				return nil, err
			}
		}
	}
	return &Publisher{store: store, conn: conn, js: js, interval: interval, logger: logger}, nil
}

func (p *Publisher) Close() {
	if err := p.conn.Drain(); err != nil {
		p.logger.Warn("NATS drain failed", "error", err)
	}
	p.conn.Close()
}
func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		if err := p.publishBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			p.logger.Error("provider audit publish failed", "error", err)
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
			return err
		}
		msg := nats.NewMsg("provider.audit.v1")
		msg.Data = encoded
		msg.Header.Set(nats.MsgIdHdr, event.ID.String())
		msg.Header.Set("content-type", "application/json")
		publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err = p.js.PublishMsg(msg, nats.Context(publishCtx))
		cancel()
		if err != nil {
			return fmt.Errorf("publish event %s: %w", event.ID, err)
		}
		if err = p.store.MarkOutboxPublished(ctx, event.ID); err != nil {
			return err
		}
	}
	return nil
}
