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
