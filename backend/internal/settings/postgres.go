package settings

import (
	"context"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists settings in a PostgreSQL database.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore constructs a Postgres-backed Store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// EnsureSchema creates the settings table when it does not already exist.
func (s *PostgresStore) EnsureSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS site_settings (
	settings TEXT PRIMARY KEY,
	value    TEXT NOT NULL DEFAULT ''
);`
	_, err := s.pool.Exec(ctx, schema)
	return err
}

// Load fetches the persisted settings.
func (s *PostgresStore) Load(ctx context.Context) (Settings, error) {
	const query = `SELECT settings, value FROM site_settings`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return Settings{}, err
	}
	defer rows.Close()

	values := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return Settings{}, err
		}
		values[key] = value
	}
	if rows.Err() != nil {
		return Settings{}, rows.Err()
	}
	if len(values) == 0 {
		return Settings{}, ErrNotFound
	}
	return mapToSettings(values), nil
}

// Save upserts the provided settings into PostgreSQL.
func (s *PostgresStore) Save(ctx context.Context, settings Settings) error {
	pairs := settingsToMap(settings)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	const query = `
INSERT INTO site_settings (settings, value)
VALUES ($1, $2)
ON CONFLICT (settings) DO UPDATE
SET value = EXCLUDED.value`

	for key, value := range pairs {
		if _, err = tx.Exec(ctx, query, key, value); err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	return err
}

func settingsToMap(value Settings) map[string]string {
	result := make(map[string]string)
	v := reflect.ValueOf(value)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" {
			continue
		}
		key := strings.Split(tag, ",")[0]
		if key == "" {
			continue
		}
		if field.Type.Kind() == reflect.String {
			result[key] = v.Field(i).String()
		}
	}

	return result
}

func mapToSettings(values map[string]string) Settings {
	var result Settings
	v := reflect.ValueOf(&result).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" {
			continue
		}
		key := strings.Split(tag, ",")[0]
		if key == "" {
			continue
		}
		if value, ok := values[key]; ok && field.Type.Kind() == reflect.String {
			v.Field(i).SetString(value)
		}
	}

	return result
}
