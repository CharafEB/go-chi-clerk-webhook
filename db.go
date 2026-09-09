package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id                         TEXT PRIMARY KEY,
    external_id                UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    first_name                 TEXT NOT NULL,
    last_name                  TEXT,
    username                   TEXT UNIQUE,
    image_url                  TEXT NOT NULL,
    gender                     TEXT,
    birthday                   TEXT,
    password_enabled           BOOLEAN NOT NULL DEFAULT false,
    two_factor_enabled         BOOLEAN NOT NULL DEFAULT false,
    primary_email_address_id   TEXT,
    primary_phone_number_id    TEXT,
    created_at                 TIMESTAMPTZ NOT NULL,
    updated_at                 TIMESTAMPTZ NOT NULL,
    last_sign_in_at            TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS emailaddress (
    id            TEXT PRIMARY KEY,
    external_id   UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    email_address TEXT NOT NULL,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS phonenumbers (
    id           TEXT PRIMARY KEY,
    external_id  UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    phone_number TEXT NOT NULL,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS userprofile (
    user_id     TEXT PRIMARY KEY NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    pointes     INT NOT NULL DEFAULT 0 CHECK (pointes >= 0)
);`

func connectDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return db, nil
}

func ensureSchema(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, schema); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}
