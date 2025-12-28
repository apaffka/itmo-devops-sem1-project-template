package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DBUser = "validator"
	DBPass = "val1dat0r"
	DBName = "project-sem-1"
	DBPort = 5432
	TablePrices = "prices"
)

func Open(host string) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		DBUser, DBPass, host, DBPort, DBName)

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping failed: %w", err)
	}

	return pool, nil
}

func Migrate(pool *pgxpool.Pool) error {
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS prices (
  id BIGINT NOT NULL DEFAULT 0,
  name TEXT NOT NULL,
  category TEXT NOT NULL,
  price NUMERIC(12,2) NOT NULL CHECK (price > 0),
  create_date DATE NOT NULL
);`); err != nil {
		return err
	}

	stmts := []string{
		`ALTER TABLE prices ADD COLUMN IF NOT EXISTS id BIGINT NOT NULL DEFAULT 0;`,
		`ALTER TABLE prices ADD COLUMN IF NOT EXISTS name TEXT;`,
		`ALTER TABLE prices ADD COLUMN IF NOT EXISTS category TEXT;`,
		`ALTER TABLE prices ADD COLUMN IF NOT EXISTS price NUMERIC(12,2);`,
		`ALTER TABLE prices ADD COLUMN IF NOT EXISTS create_date DATE;`,
		// If price column exists but not numeric(12,2), attempt to cast:
		`ALTER TABLE prices ALTER COLUMN price TYPE NUMERIC(12,2) USING price::numeric;`,
	}

	for _, q := range stmts {
		_, _ = pool.Exec(ctx, q)
	}

	if _, err := pool.Exec(ctx, `
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ux_prices_dedupe' AND conrelid = 'prices'::regclass
  ) THEN
    ALTER TABLE prices
      ADD CONSTRAINT ux_prices_dedupe UNIQUE (name, category, price, create_date);
  END IF;
END$$;`); err != nil {
		return err
	}

	idx := []string{
		`CREATE INDEX IF NOT EXISTS ix_prices_date ON prices(create_date);`,
		`CREATE INDEX IF NOT EXISTS ix_prices_price ON prices(price);`,
		`CREATE INDEX IF NOT EXISTS ix_prices_category ON prices(category);`,
	}
	for _, q := range idx {
		if _, err := pool.Exec(ctx, q); err != nil {
			return err
		}
	}

	return nil
}