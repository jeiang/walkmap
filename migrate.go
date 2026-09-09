package main

import (
	"context"
	"fmt"
)

// schemaSQL is idempotent: extensions and index/table creation all use the
// "if not exists" forms so `walkmap migrate` is safe to rerun.
const schemaSQL = `
create extension if not exists postgis;
create extension if not exists pg_trgm;

create table if not exists places (
  id          text primary key,
  source      text not null,
  name        text,
  category    text not null,
  raw_tags    jsonb not null,
  confidence  real,
  hidden      boolean not null default false,
  geom        geometry(point, 4326) not null
);

create index if not exists places_geom_idx on places using gist (geom);
create index if not exists places_name_trgm_idx on places using gin (name gin_trgm_ops);
`

func cmdMigrate(ctx context.Context) error {
	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	fmt.Println("migrate: ok")
	return nil
}
