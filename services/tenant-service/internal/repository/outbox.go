package repository

import (
	"context"

	"darksamus.com/ReserveHub/tenant-service/internal/domain"
	"github.com/google/uuid"
)

func (s *PostgresStore) FetchOutbox(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		FROM sys_outbox_event
		WHERE published_at IS NULL
		ORDER BY created_at, id
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.OutboxEvent, 0, limit)
	for rows.Next() {
		var event domain.OutboxEvent
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload, &event.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func (s *PostgresStore) MarkOutboxPublished(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE sys_outbox_event SET published_at = now() WHERE id = $1 AND published_at IS NULL`, id)
	return err
}
