package main

import (
	"context"
	"fmt"
)

// dedupeSQL implements the rule in docs/WALKMAP.md: an overture row is
// hidden when an osm row within 150m has the same normalized name, or one
// name is a substring of the other with the shorter normalized name's
// length > 5. Names are normalized the same way as
// internal/taxonomy.NormalizeName (lowercase, [a-z0-9] only), reimplemented
// here in SQL since the comparison has to happen in the database.
//
// Resets all overture hidden flags first so the command is safe to rerun
// (e.g. after re-importing osm) without accumulating stale hides.
const dedupeSQL = `
update places set hidden = false where source = 'overture';

with osm_norm as (
  select id, geom, regexp_replace(lower(coalesce(name, '')), '[^a-z0-9]', '', 'g') as norm
  from places where source = 'osm'
),
ovt_norm as (
  select id, geom, regexp_replace(lower(coalesce(name, '')), '[^a-z0-9]', '', 'g') as norm
  from places where source = 'overture'
)
update places p
set hidden = true
from ovt_norm v
join osm_norm o
  on st_dwithin(o.geom::geography, v.geom::geography, 150)
 and v.norm <> ''
 and (
   v.norm = o.norm
   or (length(v.norm) > 5 and position(v.norm in o.norm) > 0)
   or (length(o.norm) > 5 and position(o.norm in v.norm) > 0)
 )
where p.id = v.id;
`

func cmdDedupe(ctx context.Context) error {
	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, dedupeSQL); err != nil {
		return fmt.Errorf("dedupe: %w", err)
	}

	var total, hidden int
	if err := conn.QueryRow(ctx, `select count(*) from places where source = 'overture'`).Scan(&total); err != nil {
		return err
	}
	if err := conn.QueryRow(ctx, `select count(*) from places where source = 'overture' and hidden`).Scan(&hidden); err != nil {
		return err
	}
	fmt.Printf("dedupe: overture total=%d hidden=%d visible=%d\n", total, hidden, total-hidden)
	return nil
}
