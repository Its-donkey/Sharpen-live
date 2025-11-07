package monitor

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry represents a persisted monitor log line for a streaming platform.
type Entry struct {
	ID        int64
	Platform  string
	Timestamp time.Time
	Message   string
}

// Store exposes read operations for monitor log entries backed by PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store that queries monitor logs using the supplied pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// RecentEntries fetches the latest monitor log entries across supported platforms.
// The limit argument is capped at 500 to avoid unbounded result sets.
func (s *Store) RecentEntries(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	const query = `
WITH youtube AS (
	SELECT
		id,
		'youtube'::text AS platform,
		(log_date::timestamp + log_time) AS occurred_at,
		message
	FROM youtube_logs
)
SELECT id, platform, occurred_at, message
FROM youtube
ORDER BY occurred_at DESC, id DESC
LIMIT $1;
`

	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]Entry, 0, limit)
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.Platform, &entry.Timestamp, &entry.Message); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}
