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


def load_to_db(grid: list[dict], month: int, db_url: str, batch_size: int = 500) -> None:
    """
    Upsert grid rows for the given month.

    Deletes all existing rows for this month first (idempotent re-runs),
    then inserts in batches.
    """
    if not grid:
        return

    with psycopg.connect(db_url) as conn:
        with conn.cursor() as cur:
            # Wipe existing data for this month to allow clean re-runs.
            cur.execute(
                "DELETE FROM environment.shadow_grid WHERE month = %s",
                (month,),
            )
            print(f"    Deleted existing rows for month={month}")

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
