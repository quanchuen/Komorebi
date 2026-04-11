"""
shadow_compute.py — compute per-cell shade coverage for a grid of 50 m cells.

Algorithm:
1. Build a regular 50 m grid over the bbox (EPSG:6677 plane coords for accurate
   distances, then convert cell centres/corners back to EPSG:4326 for storage).
2. For each hour slot (0-23):
   a. Compute solar position (azimuth, zenith) at the representative day of the
      month, at 30 min past the hour, using pvlib.solarposition.
   b. For each building, project its shadow polygon:
      shadow_length = height_m / tan(solar_elevation_rad)
      shadow direction = opposite of solar azimuth
      Translate footprint vertices by (shadow_length * sin(azimuth),
                                       shadow_length * cos(azimuth))
      Union of footprint + translated footprint = shadow polygon.
   c. Union all building shadow polygons → scene_shadow.
3. For each grid cell, compute:
      shade_coverage = cell ∩ scene_shadow area / cell area
4. Return list of {"cell_polygon": Polygon, "hour_slot": int, "shade_coverage": float}
"""

import math
import datetime
from typing import Any

import numpy as np
import pandas as pd
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

# Representative day-of-month for each month (mid-month, or solstice for 6/12).
REPRESENTATIVE_DAY = {
    1: 15, 2: 15, 3: 15, 4: 15, 5: 15, 6: 21,
    7: 21, 8: 15, 9: 15, 10: 15, 11: 15, 12: 21,
}


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
                (x,               y),
                (x + grid_size_m, y),
                (x + grid_size_m, y + grid_size_m),
                (x,               y + grid_size_m),
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
                "cell_polygon":  cell,
                "hour_slot":     hour,
                "shade_coverage": shade,
            })

    return results
