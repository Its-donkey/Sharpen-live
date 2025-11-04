package logstore

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists log entries to PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store backed by the supplied connection pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// EnsureSchema creates the youtube_logs table if needed.
func (s *Store) EnsureSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS youtube_logs (
	id        bigserial PRIMARY KEY,
	log_date  date NOT NULL,
	log_time  time NOT NULL,
	message   text NOT NULL
);`
	_, err := s.pool.Exec(ctx, schema)
	return err
}

// Write stores the supplied log message with the current timestamp.
func (s *Store) Write(message string) error {
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	const query = `
INSERT INTO youtube_logs (log_date, log_time, message)
VALUES ($1, $2, $3)`

	_, err := s.pool.Exec(ctx, query, now.Format("2006-01-02"), now.Format("15:04:05"), message)
	return err
}

// Writer implements io.Writer so the Store can be used in a log.Logger.
type Writer struct {
	store *Store
}

// NewWriter wraps a Store with a log-compatible writer.
func NewWriter(store *Store) *Writer {
	return &Writer{store: store}
}

// Write normalizes the payload and persists it to the database.
func (w *Writer) Write(p []byte) (int, error) {
	if w.store == nil {
		return len(p), nil
	}
	message := strings.TrimSpace(string(p))
	if message == "" {
		return len(p), nil
	}
	_ = w.store.Write(message)
	return len(p), nil
}
