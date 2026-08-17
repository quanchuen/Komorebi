"""
load_db.py — upsert shadow grid results into environment.shadow_grid.

Strategy: DELETE existing rows for (month) then INSERT new rows in batches.
This makes re-runs idempotent without requiring a unique index on cell geometry.
"""

import psycopg
from shapely.geometry import Polygon


def _polygon_to_wkt(poly: Polygon) -> str:
    coords = list(poly.exterior.coords)
    pts = ", ".join(f"{x} {y}" for x, y in coords)
    return f"POLYGON(({pts}))"


def load_to_db(
    grid: list[dict],
    month: int,
    db_url: str,
    bbox: list[float],
    batch_size: int = 500,
) -> None:
    """
    Upsert grid rows for the given month and ward bbox.

    Deletes existing rows for this month *within the ward's bbox* first
    (idempotent re-runs), then inserts in batches. The delete must be
    bbox-scoped: a global per-month delete would wipe every previously
    loaded ward each time the next ward loads.
    """
    if not grid:
        return

    with psycopg.connect(db_url) as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                DELETE FROM environment.shadow_grid
                WHERE month = %s
                  AND cell_geometry && ST_MakeEnvelope(%s, %s, %s, %s, 4326)
                """,
                (month, bbox[0], bbox[1], bbox[2], bbox[3]),
            )
            print(f"    Deleted existing rows for month={month} in ward bbox")

            batch = []
            total = 0
            for row in grid:
                wkt = _polygon_to_wkt(row["cell_polygon"])
                batch.append((
                    wkt,
                    row["hour_slot"],
                    month,
                    row["shade_coverage"],
                ))

                if len(batch) >= batch_size:
                    _insert_batch(cur, batch)
                    total += len(batch)
                    batch = []

            if batch:
                _insert_batch(cur, batch)
                total += len(batch)

        conn.commit()
    print(f"    Inserted {total} rows for month={month}")


def _insert_batch(cur, batch: list[tuple]) -> None:
    """Execute a multi-row INSERT for a batch of shadow grid rows."""
    values_parts = []
    params = []
    for wkt, hour_slot, month, shade_coverage in batch:
        # Use ST_GeomFromText as a literal SQL expression for the geometry.
        values_parts.append(f"(ST_GeomFromText(%s, 4326), %s, %s, %s)")
        params.extend([wkt, hour_slot, month, shade_coverage])

    sql = (
        "INSERT INTO environment.shadow_grid (cell_geometry, hour_slot, month, shade_coverage) VALUES "
        + ", ".join(values_parts)
    )
    cur.execute(sql, params)
