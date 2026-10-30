package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OutboxEvent struct {
	ID            uuid.UUID       `json:"id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	SchemaVersion int             `json:"schema_version"`
	Payload       json.RawMessage `json:"payload"`
	CreatedAt     time.Time       `json:"created_at"`
}

func (p *Postgres) ClaimAuditOutbox(ctx context.Context, limit int) ([]OutboxEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("invalid outbox batch size")
	}
	rows, err := p.pool.Query(ctx, `WITH claim AS (SELECT id FROM auth_service.sys_outbox_event WHERE published_at IS NULL AND next_attempt_at<=now() AND event_type<>$1 ORDER BY next_attempt_at,created_at,id FOR UPDATE SKIP LOCKED LIMIT $2)
		UPDATE auth_service.sys_outbox_event o SET attempts=attempts+1,next_attempt_at=now()+interval '2 minutes' FROM claim WHERE o.id=claim.id
		RETURNING o.id,o.aggregate_type,o.aggregate_id,o.event_type,o.schema_version,o.payload,o.created_at`, invitationDeliveryEvent, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err = rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.SchemaVersion, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (p *Postgres) MarkAuditOutboxPublished(ctx context.Context, id uuid.UUID) error {
	_, err := p.pool.Exec(ctx, `UPDATE auth_service.sys_outbox_event SET published_at=now() WHERE id=$1 AND published_at IS NULL AND event_type<>$2`, id, invitationDeliveryEvent)
	return err
}

func insertAuthEvent(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO auth_service.sys_outbox_event (aggregate_type,aggregate_id,event_type,payload) VALUES ($1,$2,$3,$4)`, aggregateType, aggregateID, eventType, encoded)
	if err != nil {
		return fmt.Errorf("record auth event: %w", err)
	}
	return nil
}
