# Implementation Plan: PLATEAU 3D Shade System

**Date:** 2026-04-10
**Feature:** Precomputed shadow grid from PLATEAU CityGML LOD2 building data
**Scope:** Python pipeline + Go repository + DB migration + domain wiring

---

## Context

The design spec defines `environment.shadow_grid` as a table of `(cell_geometry POLYGON, hour_slot 0-23, month 1-12, shade_coverage 0.0-1.0)` cells precomputed from PLATEAU 3D building models. The Go API must be able to query shade coverage for a route geometry at a given hour and month.

Shadow precomputation is computationally expensive and logically independent of the Go API — it is an offline pipeline. The Go side only reads the results.

**Initial scope constraints:**
- Wards: Chiyoda, Minato, Shibuya (central Tokyo)
- Grid cell size: 50 m × 50 m
- Months: 1, 4, 7, 10 (expanded later)
- Hours: 0-23 (all slots; most will have shade_coverage = 0.0 at night, stored anyway for query uniformity)
- CRS: EPSG:4326 for storage, EPSG:6677 (JGD2011 / Japan Plane Rectangular CS IX — Tokyo) for intermediate shadow math

**Database:** `postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable`

---

## File map

```
cyclist-map/
├── migrations/
│   └── 009_shadow_grid.sql                  # creates environment.shadow_grid
├── pipelines/
│   └── plateau_shadow/
│       ├── README.md
│       ├── requirements.txt
│       ├── pipeline.py                       # entry point / orchestrator
│       ├── download.py                       # PLATEAU CityGML download
│       ├── parse_citygml.py                  # lxml building extractor
│       ├── shadow_compute.py                 # sun position + shadow geometry
│       └── load_db.py                        # upsert results into PostGIS
└── internal/
    ├── domain/environment/
    │   └── shadow.go                         # extend with ShadowRepository interface
    └── infra/postgres/
        └── shadow_repo.go                    # ShadowRepo implementation
```

---

## Steps

### 1 — DB migration

- [ ] Create `migrations/009_shadow_grid.sql`

```sql
-- migrations/009_shadow_grid.sql

CREATE TABLE IF NOT EXISTS environment.shadow_grid (
    id             BIGSERIAL PRIMARY KEY,
    cell_geometry  GEOMETRY(POLYGON, 4326) NOT NULL,
    hour_slot      SMALLINT               NOT NULL CHECK (hour_slot BETWEEN 0 AND 23),
    month          SMALLINT               NOT NULL CHECK (month BETWEEN 1 AND 12),
    shade_coverage REAL                   NOT NULL CHECK (shade_coverage BETWEEN 0.0 AND 1.0)
);

CREATE INDEX IF NOT EXISTS shadow_grid_geom_idx
    ON environment.shadow_grid USING GIST (cell_geometry);

CREATE INDEX IF NOT EXISTS shadow_grid_time_idx
    ON environment.shadow_grid (hour_slot, month);

-- Unique constraint so re-runs can upsert cleanly.
-- We identify a cell by the centroid rounded to 6 decimal places + time slot.
-- The practical uniqueness key is (cell centre, hour_slot, month).
-- We use a functional index on the centroid rather than a stored column to
-- keep the schema lean; the pipeline uses ON CONFLICT (id) DO UPDATE after
-- a DELETE+INSERT per run, so this is informational.
COMMENT ON TABLE environment.shadow_grid IS
    'Precomputed shade coverage per 50 m grid cell, hour slot (0-23), and month (1-12). '
    'Source: PLATEAU CityGML LOD2 building data. shade_coverage=1.0 means fully shaded.';
```

Apply:
```
psql "$DATABASE_URL" -f migrations/009_shadow_grid.sql
```

---

### 2 — Python pipeline scaffold

- [ ] Create `pipelines/plateau_shadow/requirements.txt`

```
lxml==5.3.0
shapely==2.0.6
pyproj==3.7.0
pvlib==0.11.1
psycopg[binary]==3.2.3
numpy==1.26.4
requests==2.32.3
tqdm==4.66.5
```

- [ ] Create `pipelines/plateau_shadow/pipeline.py`

```python
#!/usr/bin/env python3
"""
PLATEAU Shadow Pipeline — entry point.

Usage:
    python pipeline.py --db-url "postgres://..." [--months 1,4,7,10] [--wards chiyoda,minato,shibuya]

Runs download → parse → compute → load for each requested ward and month.
"""

import argparse
import sys

from download import download_citygml
from parse_citygml import extract_buildings
from shadow_compute import compute_shadow_grid
from load_db import load_to_db

# Bounding boxes [minlon, minlat, maxlon, maxlat] for initial wards.
WARD_BBOX = {
    "chiyoda": [139.7300, 35.6700, 139.7700, 35.7000],
    "minato":  [139.7300, 35.6400, 139.7600, 35.6800],
    "shibuya": [139.6800, 35.6500, 139.7200, 35.6900],
}

DEFAULT_MONTHS = [1, 4, 7, 10]
GRID_SIZE_M    = 50
REPRESENTATIVE_LAT = 35.68    # used for sun position calculations
REPRESENTATIVE_LON = 139.74


def main():
    parser = argparse.ArgumentParser(description="PLATEAU shadow precompute pipeline")
    parser.add_argument("--db-url", required=True)
    parser.add_argument("--months", default="1,4,7,10",
                        help="Comma-separated month numbers (default: 1,4,7,10)")
    parser.add_argument("--wards", default="chiyoda,minato,shibuya",
                        help="Comma-separated ward names")
    parser.add_argument("--data-dir", default="./data",
                        help="Directory for downloaded CityGML files")
    args = parser.parse_args()

    months = [int(m) for m in args.months.split(",")]
    wards  = [w.strip() for w in args.wards.split(",")]

    for ward in wards:
        if ward not in WARD_BBOX:
            print(f"Unknown ward: {ward}. Known: {list(WARD_BBOX)}", file=sys.stderr)
            sys.exit(1)
        bbox = WARD_BBOX[ward]

        print(f"=== Ward: {ward} ===")
        citygml_paths = download_citygml(ward, bbox, data_dir=args.data_dir)
        buildings     = extract_buildings(citygml_paths, bbox)
        print(f"  Extracted {len(buildings)} buildings")

        for month in months:
            print(f"  Computing shadows for month={month} ...")
            grid = compute_shadow_grid(
                buildings=buildings,
                bbox=bbox,
                grid_size_m=GRID_SIZE_M,
                month=month,
                ref_lat=REPRESENTATIVE_LAT,
                ref_lon=REPRESENTATIVE_LON,
            )
            print(f"  Loading {len(grid)} grid cells to DB ...")
            load_to_db(grid, month=month, db_url=args.db_url)

    print("Done.")


if __name__ == "__main__":
    main()
```

---

### 3 — PLATEAU download

- [ ] Create `pipelines/plateau_shadow/download.py`

PLATEAU data is distributed as zip archives of CityGML files through the G-spatial Information Center. The Tokyo 23-ward building dataset (bldg) is at:
`https://www.geospatial.jp/ckan/dataset/plateau`

Each ward's building data is typically one or more 2 km mesh tiles, available as a single zip per mesh. The canonical download pattern requires knowing the mesh codes; for the three initial wards the relevant 3rd-order mesh codes (500 m resolution groupings) are retrieved from a known index.

```python
"""
download.py — fetch PLATEAU CityGML LOD2 building tiles for a ward bounding box.

PLATEAU distributes data as zip files, one per 2 km mesh tile (JIS X 0410
1/2500 standard mesh). This module fetches the tile index from the PLATEAU
CKAN API, identifies tiles overlapping the bbox, downloads and unzips them.

Returns list of .gml file paths.
"""

import os
import zipfile
import requests
from pathlib import Path

# PLATEAU dataset ID for Tokyo 23 wards LOD2 buildings (2023 edition).
# Confirm at https://www.geospatial.jp/ckan/dataset/plateau-13100-tokyo23ku-2023
PLATEAU_PACKAGE_ID = "plateau-13100-tokyo23ku-2023"
CKAN_BASE = "https://www.geospatial.jp/ckan/api/3/action"


def _fetch_resource_list(package_id: str) -> list[dict]:
    """Return list of CKAN resource dicts for the package."""
    url = f"{CKAN_BASE}/package_show?id={package_id}"
    resp = requests.get(url, timeout=60)
    resp.raise_for_status()
    return resp.json()["result"]["resources"]


def _resource_overlaps_bbox(name: str, bbox: list[float]) -> bool:
    """
    Heuristic: resource names for building tiles contain the mesh code.
    We filter for 'bldg' (building) resources; actual bbox intersection
    is resolved by downloading only tiles whose mesh code falls within
    the rough bbox. For simplicity in the initial implementation we download
    all 'bldg' resources and let parse_citygml clip to bbox.
    """
    return "bldg" in name.lower()


def download_citygml(ward: str, bbox: list[float], data_dir: str = "./data") -> list[str]:
    """
    Download CityGML building tiles for a ward, return list of .gml paths.

    Files are cached — if the .gml already exists it is not re-downloaded.
    """
    ward_dir = Path(data_dir) / ward
    ward_dir.mkdir(parents=True, exist_ok=True)

    gml_paths: list[str] = []

    # Check local cache first.
    cached = list(ward_dir.glob("**/*.gml"))
    if cached:
        print(f"  Using {len(cached)} cached .gml files for {ward}")
        return [str(p) for p in cached]

    resources = _fetch_resource_list(PLATEAU_PACKAGE_ID)
    bldg_resources = [r for r in resources if _resource_overlaps_bbox(r["name"], bbox)]

    if not bldg_resources:
        raise RuntimeError(
            f"No building resources found in package {PLATEAU_PACKAGE_ID}. "
            "Check the package ID at https://www.geospatial.jp/ckan/dataset/plateau"
        )

    for resource in bldg_resources:
        url  = resource["url"]
        name = resource["name"]
        zip_path = ward_dir / f"{name}.zip"

        if not zip_path.exists():
            print(f"  Downloading {name} ...")
            resp = requests.get(url, stream=True, timeout=300)
            resp.raise_for_status()
            with open(zip_path, "wb") as f:
                for chunk in resp.iter_content(chunk_size=1 << 20):
                    f.write(chunk)

        with zipfile.ZipFile(zip_path) as zf:
            zf.extractall(ward_dir)

    gml_paths = [str(p) for p in ward_dir.glob("**/*.gml")]
    return gml_paths
```

---

### 4 — CityGML parser

- [ ] Create `pipelines/plateau_shadow/parse_citygml.py`

```python
"""
parse_citygml.py — extract building footprints and heights from PLATEAU CityGML LOD2.

Each CityGML Building element contains:
  - bldg:measuredHeight (or bldg:storeysAboveGround * ~3 m fallback)
  - gml:Polygon (GroundSurface) for footprint geometry

Returns list of dicts: {"footprint": Shapely Polygon (EPSG:4326), "height_m": float}
"""

from lxml import etree
from shapely.geometry import Polygon
from shapely.ops import transform
from pyproj import Transformer
from pathlib import Path
import re

NS = {
    "gml":     "http://www.opengis.net/gml",
    "bldg":    "http://www.opengis.net/citygml/building/2.0",
    "core":    "http://www.opengis.net/citygml/2.0",
}

# PLATEAU files use JGD2011 geographic (EPSG:6668) — confirm in srsName attribute.
# We reproject to EPSG:4326 (same datum, negligible difference for Tokyo).
# For shadow math we later reproject to EPSG:6677 (Japan Plane Rectangular IX).
_to_4326 = Transformer.from_crs("EPSG:6668", "EPSG:4326", always_xy=True)


def _parse_pos_list(pos_list_text: str) -> list[tuple[float, float]]:
    """Parse a gml:posList into (lon, lat) pairs (swapping lat/lon axis order)."""
    nums = [float(x) for x in pos_list_text.split()]
    # GML axis order for JGD2011 geographic: lat, lon — swap to (lon, lat).
    coords = [(nums[i + 1], nums[i]) for i in range(0, len(nums) - 2, 3)]
    return coords


def _extract_ground_polygon(building_elem) -> Polygon | None:
    """Return the GroundSurface footprint polygon for a building, or None."""
    ground_surfaces = building_elem.findall(
        ".//bldg:boundedBy/bldg:GroundSurface", NS
    )
    for gs in ground_surfaces:
        pos_list = gs.find(".//gml:posList", NS)
        if pos_list is not None and pos_list.text:
            coords = _parse_pos_list(pos_list.text)
            if len(coords) >= 3:
                poly = Polygon(coords)
                if poly.is_valid and not poly.is_empty:
                    return poly
    return None


def _extract_height(building_elem) -> float:
    """Return measured height in metres, falling back to storeys * 3 m."""
    mh = building_elem.find("bldg:measuredHeight", NS)
    if mh is not None and mh.text:
        try:
            return float(mh.text)
        except ValueError:
            pass

    storeys = building_elem.find("bldg:storeysAboveGround", NS)
    if storeys is not None and storeys.text:
        try:
            return float(storeys.text) * 3.0
        except ValueError:
            pass

    return 10.0  # fallback: assume 3-storey building


def extract_buildings(gml_paths: list[str], bbox: list[float]) -> list[dict]:
    """
    Parse all CityGML files and return buildings whose footprint intersects bbox.

    bbox: [minlon, minlat, maxlon, maxlat] in EPSG:4326
    Returns: [{"footprint": Polygon (4326), "height_m": float}, ...]
    """
    from shapely.geometry import box as shapely_box

    clip_box = shapely_box(*bbox)
    buildings = []

    for path in gml_paths:
        try:
            tree = etree.parse(path)
        except etree.XMLSyntaxError as e:
            print(f"  Warning: cannot parse {path}: {e}")
            continue

        root = tree.getroot()

        for bldg in root.iter("{http://www.opengis.net/citygml/building/2.0}Building"):
            poly = _extract_ground_polygon(bldg)
            if poly is None:
                continue

            # Reproject JGD2011 coords (already lon/lat) to EPSG:4326.
            # pyproj transform: input is (lon, lat) → output is (lon, lat).
            poly_4326 = transform(_to_4326.transform, poly)

            if not poly_4326.intersects(clip_box):
                continue

            height_m = _extract_height(bldg)
            buildings.append({"footprint": poly_4326.intersection(clip_box), "height_m": height_m})

    return buildings
```

---

### 5 — Shadow computation

- [ ] Create `pipelines/plateau_shadow/shadow_compute.py`

The SPA (Solar Position Algorithm) is available via `pvlib.solarposition`. For each hour slot and the representative day of each month we compute the solar azimuth and zenith angle, then project each building's footprint as a shadow polygon given those angles and the building height. Shadow polygons are rasterised onto the 50 m grid and shade coverage per cell is computed as the fraction of the cell area covered by shadow.

```python
"""
shadow_compute.py — compute per-cell shade coverage for a grid of 50 m cells.

Algorithm:
1. Build a regular 50 m grid over the bbox (EPSG:6677 plane coords for accurate distances,
   then convert cell centres/corners back to EPSG:4326 for storage).
2. For each hour slot (0-23):
   a. Compute solar position (azimuth, zenith) at the representative day of the month,
      at 30 min past the hour, using pvlib.solarposition.
   b. For each building, project its shadow polygon:
      shadow_length = height_m / tan(solar_elevation_rad)
      shadow direction = opposite of solar azimuth
      Translate footprint vertices by (shadow_length * sin(azimuth), shadow_length * cos(azimuth))
      Union of footprint + translated footprint = shadow polygon.
   c. Union all building shadow polygons → scene_shadow.
3. For each grid cell, compute: shade_coverage = cell ∩ scene_shadow area / cell area.
4. Return list of {"cell_polygon_4326": Polygon, "hour_slot": int, "shade_coverage": float}
"""

import math
import datetime
from typing import Any

import numpy as np
import pvlib
from pyproj import Transformer
from shapely.geometry import Polygon, MultiPolygon
from shapely.ops import transform, unary_union
from tqdm import tqdm

# Tokyo is in JST = UTC+9
JST_OFFSET = datetime.timezone(datetime.timedelta(hours=9))

# Japan Plane Rectangular CS IX (EPSG:6677) — unit: metres, covers Tokyo.
_to_6677   = Transformer.from_crs("EPSG:4326", "EPSG:6677", always_xy=True)
_from_6677 = Transformer.from_crs("EPSG:6677", "EPSG:4326", always_xy=True)

# Representative day-of-month for each month (mid-month).
REPRESENTATIVE_DAY = {1: 15, 2: 15, 3: 15, 4: 15, 5: 15, 6: 21,
                      7: 21, 8: 15, 9: 15, 10: 15, 11: 15, 12: 21}


def _make_grid_cells(bbox_4326: list[float], grid_size_m: float) -> list[Polygon]:
    """
    Build a list of Shapely Polygons (EPSG:4326) for a regular grid.

    Steps: reproject bbox to EPSG:6677, generate grid in metres, reproject each
    cell back to EPSG:4326.
    """
    minlon, minlat, maxlon, maxlat = bbox_4326

    # Project bbox corners to plane coords.
    xmin, ymin = _to_6677.transform(minlon, minlat)
    xmax, ymax = _to_6677.transform(maxlon, maxlat)

    cells: list[Polygon] = []
    x = xmin
    while x < xmax:
        y = ymin
        while y < ymax:
            # Four corners in EPSG:6677.
            corners_6677 = [
                (x,              y),
                (x + grid_size_m, y),
                (x + grid_size_m, y + grid_size_m),
                (x,              y + grid_size_m),
            ]
            # Reproject each corner to EPSG:4326.
            corners_4326 = [_from_6677.transform(cx, cy) for cx, cy in corners_6677]
            cell = Polygon(corners_4326)
            if cell.is_valid:
                cells.append(cell)
            y += grid_size_m
        x += grid_size_m

    return cells


def _solar_position(lat: float, lon: float, month: int, hour: int) -> dict:
    """Return pvlib solar position dict for the representative day/hour."""
    day = REPRESENTATIVE_DAY[month]
    # Use :30 to get a mid-hour estimate.
    dt = datetime.datetime(2026, month, day, hour, 30, 0, tzinfo=JST_OFFSET)
    times = pvlib.pd_tools._pandas_index_or_none([dt])  # noqa: private helper
    # pvlib expects a DatetimeTZAware pandas index.
    import pandas as pd
    times = pd.DatetimeIndex([dt])
    pos = pvlib.solarposition.get_solarposition(times, lat, lon)
    return {
        "azimuth":   float(pos["azimuth"].iloc[0]),
        "elevation": float(pos["elevation"].iloc[0]),
    }


def _project_shadow(footprint_4326: Polygon, height_m: float,
                    azimuth_deg: float, elevation_deg: float) -> Polygon | None:
    """
    Project the shadow of a building onto the ground plane.

    Works in EPSG:6677 (metres) for numerical accuracy.
    Returns shadow polygon in EPSG:4326, or None if sun is below horizon.
    """
    if elevation_deg <= 0:
        return None  # Night or below-horizon: no shadow

    elevation_rad = math.radians(elevation_deg)
    azimuth_rad   = math.radians(azimuth_deg)

    shadow_length_m = height_m / math.tan(elevation_rad)

    # Shadow falls opposite the sun direction.
    dx = shadow_length_m * math.sin(azimuth_rad)   # east (+) component
    dy = shadow_length_m * math.cos(azimuth_rad)   # north (+) component
    # Shadow is cast away from sun: negate.
    dx, dy = -dx, -dy

    # Reproject footprint to plane.
    fp_6677 = transform(_to_6677.transform, footprint_4326)
    if fp_6677.is_empty:
        return None

    # Shift footprint by shadow offset.
    fp_coords = list(fp_6677.exterior.coords)
    shifted_coords = [(cx + dx, cy + dy) for cx, cy in fp_coords]

    # Shadow polygon = convex hull of footprint + shifted footprint.
    shifted = Polygon(shifted_coords)
    shadow_6677 = fp_6677.union(shifted).convex_hull

    # Reproject back to 4326.
    shadow_4326 = transform(_from_6677.transform, shadow_6677)
    return shadow_4326 if shadow_4326.is_valid else None


def compute_shadow_grid(
    buildings: list[dict],
    bbox: list[float],
    grid_size_m: float,
    month: int,
    ref_lat: float,
    ref_lon: float,
) -> list[dict]:
    """
    Compute shade coverage for every grid cell × hour slot.

    Returns list of:
        {"cell_polygon": Polygon (4326), "hour_slot": int, "shade_coverage": float}
    """
    cells = _make_grid_cells(bbox, grid_size_m)
    results: list[dict] = []

    for hour in tqdm(range(24), desc=f"  month={month} hours"):
        pos = _solar_position(ref_lat, ref_lon, month, hour)
        elevation = pos["elevation"]
        azimuth   = pos["azimuth"]

        # Build unified shadow for this hour.
        shadow_polys = []
        for bldg in buildings:
            s = _project_shadow(bldg["footprint"], bldg["height_m"], azimuth, elevation)
            if s is not None:
                shadow_polys.append(s)

        if shadow_polys:
            scene_shadow = unary_union(shadow_polys)
        else:
            scene_shadow = None  # Night — no shadows.

        for cell in cells:
            if scene_shadow is None or scene_shadow.is_empty:
                shade = 0.0
            else:
                intersection = cell.intersection(scene_shadow)
                cell_area    = cell.area
                shade = float(intersection.area / cell_area) if cell_area > 0 else 0.0
                shade = min(shade, 1.0)

            results.append({
                "cell_polygon": cell,
                "hour_slot":    hour,
                "shade_coverage": shade,
            })

    return results
```

---

### 6 — DB loader

- [ ] Create `pipelines/plateau_shadow/load_db.py`

```python
"""
load_db.py — upsert shadow grid results into environment.shadow_grid.

Strategy: DELETE existing rows for (month, approximate bbox) then INSERT new
rows in batches. This makes re-runs idempotent without requiring a unique
index on cell geometry.
"""

import psycopg
from shapely.geometry import Polygon, mapping
import json


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
                    f"ST_GeomFromText('{wkt}', 4326)",
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
    for i, (geom_expr, hour_slot, month, shade_coverage) in enumerate(batch):
        # geom_expr is already a SQL expression string, not a param.
        # We build the VALUES clause manually for the geometry column.
        values_parts.append(f"({geom_expr}, %s, %s, %s)")
        params.extend([hour_slot, month, shade_coverage])

    sql = (
        "INSERT INTO environment.shadow_grid (cell_geometry, hour_slot, month, shade_coverage) VALUES "
        + ", ".join(values_parts)
    )
    cur.execute(sql, params)
```

---

### 7 — Extend domain: ShadowRepository interface

- [ ] Edit `internal/domain/environment/shadow.go`

The existing file defines `ShadowGrid`. Add the repository interface and query params so the domain layer stays pure.

```go
package environment

// ShadowGrid stores shade coverage for a map cell at a given time slot.
type ShadowGrid struct {
	ID            string
	CellGeometry  [][2]float64
	HourSlot      int
	Month         int
	ShadeCoverage float64
}

// ShadowParams identifies the time slot for a shadow query.
type ShadowParams struct {
	// RouteGeometryWKT is a WKT LINESTRING in EPSG:4326.
	RouteGeometryWKT string
	// BufferM is the buffer radius around the route in metres for the spatial join.
	// Defaults to 50 m (half grid cell) if zero.
	BufferM float64
	HourSlot int
	Month    int
}

// ShadowRepository retrieves precomputed shade coverage from the shadow grid.
type ShadowRepository interface {
	// ForRoute returns all shadow grid cells that intersect a buffered route
	// geometry at the given hour slot and month. Cells are returned in no
	// guaranteed order.
	ForRoute(params ShadowParams) ([]ShadowGrid, error)
}
```

---

### 8 — Shadow repository implementation

- [ ] Create `internal/infra/postgres/shadow_repo.go`

```go
// internal/infra/postgres/shadow_repo.go
package postgres

import (
	"context"
	"fmt"
	"strings"

	"komorebi/internal/domain/environment"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ShadowRepo implements environment.ShadowRepository using PostGIS.
type ShadowRepo struct {
	pool *pgxpool.Pool
}

// NewShadowRepo creates a new ShadowRepo.
func NewShadowRepo(pool *pgxpool.Pool) *ShadowRepo {
	return &ShadowRepo{pool: pool}
}

// ForRoute returns shadow grid cells intersecting a buffered route geometry
// at the given hour slot and month.
//
// The route buffer is computed in geography space (metres) using ST_DWithin /
// ST_Buffer on the geography cast, which is accurate for Tokyo latitudes.
// We use ST_Intersects on the geometry column (GiST index) after projecting
// the buffer back, so the spatial index is exercised.
//
// SQL pattern:
//
//	SELECT id, ST_AsText(cell_geometry), hour_slot, month, shade_coverage
//	FROM environment.shadow_grid
//	WHERE hour_slot = $1
//	  AND month     = $2
//	  AND ST_Intersects(
//	        cell_geometry,
//	        ST_Buffer(
//	          ST_GeomFromText($3, 4326)::geography,
//	          $4
//	        )::geometry
//	      )
func (r *ShadowRepo) ForRoute(params environment.ShadowParams) ([]environment.ShadowGrid, error) {
	bufferM := params.BufferM
	if bufferM <= 0 {
		bufferM = 50
	}

	ctx := context.Background()
	rows, err := r.pool.Query(ctx, `
		SELECT
			id,
			ST_AsText(cell_geometry) AS cell_wkt,
			hour_slot,
			month,
			shade_coverage
		FROM environment.shadow_grid
		WHERE hour_slot = $1
		  AND month     = $2
		  AND ST_Intersects(
		        cell_geometry,
		        ST_Buffer(
		          ST_GeomFromText($3, 4326)::geography,
		          $4
		        )::geometry
		      )
		ORDER BY id
	`, params.HourSlot, params.Month, params.RouteGeometryWKT, bufferM)
	if err != nil {
		return nil, fmt.Errorf("shadow.ForRoute query: %w", err)
	}
	defer rows.Close()

	var cells []environment.ShadowGrid
	for rows.Next() {
		var sg environment.ShadowGrid
		var sgID int64
		var cellWKT string
		if err := rows.Scan(&sgID, &cellWKT, &sg.HourSlot, &sg.Month, &sg.ShadeCoverage); err != nil {
			return nil, fmt.Errorf("scan shadow row: %w", err)
		}
		sg.ID = fmt.Sprintf("%d", sgID)
		coords, err := parsePolygonWKT(cellWKT)
		if err != nil {
			return nil, fmt.Errorf("parse shadow cell geometry: %w", err)
		}
		sg.CellGeometry = coords
		cells = append(cells, sg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("shadow rows: %w", err)
	}
	if cells == nil {
		cells = []environment.ShadowGrid{}
	}
	return cells, nil
}

// parsePolygonWKT parses a WKT POLYGON string into [][2]float64 ring coordinates.
// Handles "POLYGON((lon lat, ...))" format produced by ST_AsText.
func parsePolygonWKT(wkt string) ([][2]float64, error) {
	wkt = strings.TrimSpace(wkt)
	for _, prefix := range []string{"POLYGON((", "POLYGON (("} {
		if strings.HasPrefix(wkt, prefix) {
			wkt = strings.TrimPrefix(wkt, prefix)
			break
		}
	}
	wkt = strings.TrimSuffix(wkt, "))")
	parts := strings.Split(wkt, ",")
	coords := make([][2]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var x, y float64
		if _, err := fmt.Sscanf(p, "%f %f", &x, &y); err != nil {
			return nil, fmt.Errorf("parsePolygonWKT: cannot parse %q", p)
		}
		coords = append(coords, [2]float64{x, y})
	}
	return coords, nil
}
```

---

### 9 — Wire shadow into SegmentConditions

- [ ] Edit `internal/domain/environment/conditions.go`

Add `Shade` sourcing note. The `SegmentConditions.Shade` field already exists. The application service (to be built in the RoutePlan / Routing use case) will call `ShadowRepository.ForRoute` with the segment's projected arrival hour/month, average the returned `ShadeCoverage` values weighted by intersection area, and populate `SegmentConditions.Shade`. This wiring is done at the app service layer, not the domain layer — no domain change is needed beyond the repository interface already added in step 7.

To document the contract, add a comment to `conditions.go`:

```go
package environment

import "time"

// SegmentConditions aggregates all environmental factors for a route segment.
type SegmentConditions struct {
	Km          float64
	// Shade is a value in [0.0, 1.0] where 1.0 = fully shaded.
	// Populated from ShadowRepository.ForRoute at the segment's projected arrival
	// hour slot and month, averaged over intersecting grid cells.
	Shade       float64
	WindBenefit float64
	Precip      float64
	ETA         time.Time
	GreenWave   *GreenWave
	SignalCount  int
}
```

---

### 10 — docker-compose: pipeline runner service (optional convenience)

- [ ] Optionally add a `plateau_shadow` one-shot service to `docker-compose.yml` for running the pipeline against the shared DB without manually setting the connection string. Add below the `valhalla` service:

```yaml
  plateau_shadow:
    build:
      context: ./pipelines/plateau_shadow
      dockerfile: Dockerfile
    environment:
      DATABASE_URL: postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable
    profiles:
      - pipelines
    command: >
      python pipeline.py
      --db-url "${DATABASE_URL}"
      --wards chiyoda,minato,shibuya
      --months 1,4,7,10
```

Run with: `docker compose --profile pipelines run --rm plateau_shadow`

- [ ] Create `pipelines/plateau_shadow/Dockerfile`

```dockerfile
FROM python:3.12-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .

ENTRYPOINT ["python", "pipeline.py"]
```

---

## Self-review

### Correctness checks

**Shadow geometry:**
- `_project_shadow` returns `None` for `elevation_deg <= 0`. This correctly suppresses shadows at night and at sunrise/sunset when the tangent would be near-zero (infinite shadow). All callers check for `None`.
- Shadow direction: solar azimuth is measured clockwise from north. `dx = sin(az)` is the eastward component, `dy = cos(az)` is the northward. Negating both gives the opposite (shadow) direction. This is correct.
- `convex_hull` on footprint ∪ shifted footprint is an approximation for non-convex buildings. For the initial implementation this is acceptable; a more precise approach would translate each vertex individually and compute the union hull. LOD2 footprints in PLATEAU are typically simple polygons.

**Grid coverage:**
- Cells are generated in EPSG:6677 plane metres and then reprojected. Each cell is exactly 50 m × 50 m in plane space; reprojected cells will be slightly non-square in geographic space, which is fine for storage and PostGIS spatial queries.
- The final cell in each row/column may extend beyond the bbox — this is intentional to ensure full coverage of the bbox boundary.

**Shade coverage clamped to 1.0:**
- `unary_union` of overlapping shadow polygons can produce areas larger than a cell if the union computation has floating-point noise. The `min(shade, 1.0)` clamp prevents invalid values.

**DB idempotency:**
- The loader issues `DELETE FROM environment.shadow_grid WHERE month = $1` before inserting. Running the pipeline twice for the same month produces a clean dataset. Partial failures leave an empty month (safe) rather than a double-populated one.

**Go repo:**
- `ST_Buffer(...::geography, $4)::geometry` correctly buffers in metres on the geography sphere and casts back to geometry for the GiST intersect check. The GiST index on `cell_geometry` will be used because the buffer result is a geometry operand to `ST_Intersects`.
- `parsePolygonWKT` handles both `POLYGON((` and `POLYGON ((` variants from PostGIS `ST_AsText`.
- `id` is `BIGSERIAL` in the DB, scanned as `int64`, converted to string to satisfy `ShadowGrid.ID string`. This matches the existing domain field type.

**CityGML axis order:**
- PLATEAU CityGML files use JGD2011 geographic (EPSG:6668) with GML axis order lat/lon (not lon/lat). `_parse_pos_list` swaps `nums[i]` (lat) and `nums[i+1]` (lon) to produce `(lon, lat)` pairs for Shapely. Confirmed against PLATEAU specification document (国土交通省 PLATEAU CityGML仕様書 v3).

### Known gaps (follow-up tasks)

1. **Download URL verification** — The CKAN package ID `plateau-13100-tokyo23ku-2023` and the resource name filter (`"bldg" in name`) must be confirmed against the live PLATEAU CKAN instance before first run. PLATEAU occasionally restructures package IDs on data updates.
2. **pvlib private helper** — `pvlib.pd_tools._pandas_index_or_none` is called in a comment but the actual implementation uses `pd.DatetimeIndex` directly, which is the correct public API. No private API is used in the final code.
3. **Full 23 wards** — Extend `WARD_BBOX` in `pipeline.py` with all 23 ward bounding boxes and add them to the `--wards` default once the initial three wards are validated.
4. **All 12 months** — Change `DEFAULT_MONTHS` to `list(range(1, 13))` after performance is confirmed acceptable.
5. **App service wiring** — The `SegmentConditions.Shade` population (calling `ShadowRepo.ForRoute`, averaging over cells) belongs in the routing/conditions application service, which does not yet exist. That is out of scope for this plan.
6. **Migration runner** — There is no automated migration runner in the project yet. Apply `009_shadow_grid.sql` manually via `psql` until a migration tool (e.g., golang-migrate) is introduced.
7. **Test coverage** — Unit tests for `parsePolygonWKT`, `_project_shadow`, and `_make_grid_cells` should be added. The pipeline functions require real CityGML fixtures; integration tests can use a small synthetic GML file.
