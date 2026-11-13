package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"darksamus.com/ReserveHub/provider-service/internal/domain"
	"darksamus.com/ReserveHub/provider-service/internal/outbox"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ Pool *pgxpool.Pool }

const columns = `p.id,p.tenant_id,p.name,p.city,p.status,p.created_at,p.updated_at,p.city_id,c.country_id`
const providerJoin = ` FROM provider.md_provider AS p LEFT JOIN provider.md_city AS c ON c.id=p.city_id`

func scan(row pgx.Row) (domain.Provider, error) {
	var p domain.Provider
	err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.City, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.CityID, &p.CountryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, domain.ErrNotFound
	}
	return p, err
}
func (s Postgres) Get(ctx context.Context, tenant, id uuid.UUID) (domain.Provider, error) {
	return scan(s.Pool.QueryRow(ctx, `SELECT `+columns+providerJoin+` WHERE p.tenant_id=$1 AND p.id=$2`, tenant, id))
}
func (s Postgres) List(ctx context.Context, tenant uuid.UUID, limit int, before time.Time, beforeID uuid.UUID) ([]domain.Provider, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+columns+providerJoin+` WHERE p.tenant_id=$1 AND ($2::timestamptz IS NULL OR (p.created_at,p.id)<($2,$3)) ORDER BY p.created_at DESC,p.id DESC LIMIT $4`, tenant, nullableTime(before), beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Provider, 0, limit)
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func (s Postgres) Create(ctx context.Context, operation, tenant, actor uuid.UUID, name, city string, cityID *uuid.UUID, digest [32]byte) (domain.Provider, bool, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Provider{}, false, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO provider.sys_create_provider_request(operation_id,tenant_id,request_hash) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, operation, tenant, digest[:])
	if err != nil {
		return domain.Provider{}, false, err
	}
	var existingTenant uuid.UUID
	var existingHash []byte
	var existingID *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT tenant_id,request_hash,provider_id FROM provider.sys_create_provider_request WHERE operation_id=$1 FOR UPDATE`, operation).Scan(&existingTenant, &existingHash, &existingID)
	if err != nil {
		return domain.Provider{}, false, err
	}
	if existingTenant != tenant || len(existingHash) != sha256.Size || !equalHash(existingHash, digest) {
		return domain.Provider{}, false, domain.ErrConflict
	}
	if existingID != nil {
		p, err := scan(tx.QueryRow(ctx, `SELECT `+columns+providerJoin+` WHERE p.tenant_id=$1 AND p.id=$2`, tenant, *existingID))
		if err != nil {
			return domain.Provider{}, false, err
		}
		if err = tx.Commit(ctx); err != nil {
			return domain.Provider{}, false, err
		}
		return p, true, nil
	}
	city, cityID, err = resolveCity(ctx, tx, city, cityID)
	if err != nil {
		return domain.Provider{}, false, err
	}
	id := uuid.New()
	if _, err = tx.Exec(ctx, `INSERT INTO provider.md_provider(id,tenant_id,name,city,city_id,status) VALUES($1,$2,$3,$4,$5,'ACTIVE')`, id, tenant, name, city, cityID); err != nil {
		return domain.Provider{}, false, err
	}
	p, err := scan(tx.QueryRow(ctx, `SELECT `+columns+providerJoin+` WHERE p.tenant_id=$1 AND p.id=$2`, tenant, id))
	if err != nil {
		return domain.Provider{}, false, err
	}
	_, err = tx.Exec(ctx, `UPDATE provider.sys_create_provider_request SET provider_id=$2 WHERE operation_id=$1`, operation, id)
	if err != nil {
		return domain.Provider{}, false, err
	}
	if err = insertAudit(ctx, tx, id, tenant, actor, "provider.created", p.Status); err != nil {
		return domain.Provider{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Provider{}, false, err
	}
	return p, false, nil
}
func equalHash(a []byte, b [32]byte) bool {
	var v byte
	for i := range b {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// resolveCity keeps legacy text writes working. Only a unique exact RU/EN
// catalog match is linked; an unknown or ambiguous name remains free text.
func resolveCity(ctx context.Context, tx pgx.Tx, city string, requestedID *uuid.UUID) (string, *uuid.UUID, error) {
	if requestedID != nil {
		var nameRU, nameEN string
		err := tx.QueryRow(ctx, `SELECT name_ru,name_en FROM provider.md_city WHERE id=$1`, *requestedID).Scan(&nameRU, &nameEN)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, domain.ErrInvalidCity
		}
		if err != nil {
			return "", nil, err
		}
		if city == "" {
			city = nameRU
		} else if !strings.EqualFold(city, nameRU) && !strings.EqualFold(city, nameEN) {
			return "", nil, domain.ErrInvalidCity
		}
		return city, requestedID, nil
	}
	if city == "" {
		return "", nil, domain.ErrInvalidCity
	}
	rows, err := tx.Query(ctx, `SELECT id FROM provider.md_city WHERE lower(name_ru)=lower($1) OR lower(name_en)=lower($1) LIMIT 2`, city)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var matches []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return "", nil, err
		}
		matches = append(matches, id)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(matches) == 1 {
		return city, &matches[0], nil
	}
	return city, nil, nil
}

func (s Postgres) ListCountries(ctx context.Context) ([]domain.Country, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,code,name_ru,name_en FROM provider.md_country ORDER BY code LIMIT 250`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Country, 0)
	for rows.Next() {
		var item domain.Country
		if err := rows.Scan(&item.ID, &item.Code, &item.NameRU, &item.NameEN); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Postgres) ListCities(ctx context.Context, countryID uuid.UUID, query string, limit int) ([]domain.City, error) {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provider.md_country WHERE id=$1)`, countryID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrCountryNotFound
	}
	rows, err := s.Pool.Query(ctx, `SELECT id,country_id,name_ru,name_en FROM provider.md_city WHERE country_id=$1 AND ($2='' OR starts_with(lower(name_ru),lower($2)) OR starts_with(lower(name_en),lower($2))) ORDER BY name_ru,id LIMIT $3`, countryID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.City, 0)
	for rows.Next() {
		var item domain.City
		if err := rows.Scan(&item.ID, &item.CountryID, &item.NameRU, &item.NameEN); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s Postgres) UpdateProfile(ctx context.Context, tenant, id, actor uuid.UUID, name, city *string, cityID *uuid.UUID) (domain.Provider, error) {
	return s.update(ctx, tenant, id, actor, name, city, cityID, "")
}
func (s Postgres) UpdateStatus(ctx context.Context, tenant, id, actor uuid.UUID, status string) (domain.Provider, error) {
	return s.update(ctx, tenant, id, actor, nil, nil, nil, status)
}
func (s Postgres) update(ctx context.Context, tenant, id, actor uuid.UUID, name, city *string, cityID *uuid.UUID, status string) (domain.Provider, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Provider{}, err
	}
	defer tx.Rollback(ctx)
	p, err := scan(tx.QueryRow(ctx, `SELECT `+columns+providerJoin+` WHERE p.tenant_id=$1 AND p.id=$2 FOR UPDATE OF p`, tenant, id))
	if err != nil {
		return domain.Provider{}, err
	}
	nextName, nextCity, nextStatus, nextCityID := p.Name, p.City, p.Status, p.CityID
	if name != nil {
		nextName = *name
	}
	if city != nil || cityID != nil {
		cityText := ""
		if city != nil {
			cityText = *city
		}
		nextCity, nextCityID, err = resolveCity(ctx, tx, cityText, cityID)
		if err != nil {
			return domain.Provider{}, err
		}
	}
	if status != "" {
		nextStatus = status
	}
	if nextName != p.Name || nextCity != p.City || nextStatus != p.Status || !sameID(nextCityID, p.CityID) {
		if _, err = tx.Exec(ctx, `UPDATE provider.md_provider SET name=$3,city=$4,city_id=$5,status=$6,updated_at=now() WHERE tenant_id=$1 AND id=$2`, tenant, id, nextName, nextCity, nextCityID, nextStatus); err != nil {
			return domain.Provider{}, err
		}
		p, err = scan(tx.QueryRow(ctx, `SELECT `+columns+providerJoin+` WHERE p.tenant_id=$1 AND p.id=$2`, tenant, id))
		if err != nil {
			return domain.Provider{}, err
		}
		eventType := "provider.profile_updated"
		if status != "" {
			eventType = "provider.status_changed"
		}
		if err = insertAudit(ctx, tx, id, tenant, actor, eventType, p.Status); err != nil {
			return domain.Provider{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Provider{}, err
	}
	return p, nil
}
func insertAudit(ctx context.Context, tx pgx.Tx, id, tenant, actor uuid.UUID, eventType, status string) error {
	payload, _ := json.Marshal(map[string]string{"provider_id": id.String(), "tenant_id": tenant.String(), "actor_id": actor.String(), "status": status})
	_, err := tx.Exec(ctx, `INSERT INTO provider.sys_outbox_event(aggregate_type,aggregate_id,event_type,schema_version,payload) VALUES('provider',$1,$2,1,$3)`, id, eventType, payload)
	return err
}

func (s Postgres) ClaimOutbox(ctx context.Context, limit int) ([]outbox.Event, error) {
	rows, err := s.Pool.Query(ctx, `WITH claimed AS (SELECT id FROM provider.sys_outbox_event WHERE published_at IS NULL AND next_attempt_at<=now() ORDER BY next_attempt_at,created_at,id FOR UPDATE SKIP LOCKED LIMIT $1) UPDATE provider.sys_outbox_event AS e SET attempts=e.attempts+1,next_attempt_at=now()+interval '2 minutes' FROM claimed WHERE e.id=claimed.id RETURNING e.id,e.aggregate_type,e.aggregate_id,e.event_type,e.schema_version,e.payload,e.created_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]outbox.Event, 0, limit)
	for rows.Next() {
		var e outbox.Event
		if err := rows.Scan(&e.ID, &e.AggregateType, &e.AggregateID, &e.EventType, &e.SchemaVersion, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s Postgres) MarkOutboxPublished(ctx context.Context, id uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `UPDATE provider.sys_outbox_event SET published_at=now() WHERE id=$1 AND published_at IS NULL`, id)
	return err
}
