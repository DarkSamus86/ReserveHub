package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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

// ClaimOutbox leases pending rows so concurrent publishers do not send the same
// batch. A crash after publish can still result in delivery again after the lease.
func (s *Store) ClaimOutbox(ctx context.Context, limit int) ([]OutboxEvent, error) {
	rows, err := s.pool.Query(ctx, `
		WITH claimed AS (
			SELECT id FROM admin_service.sys_outbox_event
			WHERE published_at IS NULL AND next_attempt_at <= now()
			ORDER BY next_attempt_at, created_at, id
			FOR UPDATE SKIP LOCKED LIMIT $1
		)
		UPDATE admin_service.sys_outbox_event AS e
		SET attempts = e.attempts + 1,
		    next_attempt_at = now() + interval '2 minutes'
		FROM claimed WHERE e.id = claimed.id
		RETURNING e.id, e.aggregate_type, e.aggregate_id, e.event_type,
		          e.schema_version, e.payload, e.created_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]OutboxEvent, 0, limit)
	for rows.Next() {
		var event OutboxEvent
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID,
			&event.EventType, &event.SchemaVersion, &event.Payload, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) MarkOutboxPublished(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE admin_service.sys_outbox_event SET published_at = now()
		WHERE id = $1 AND published_at IS NULL`, id)
	return err
}
