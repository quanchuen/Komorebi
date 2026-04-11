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

NS = {
    "gml":  "http://www.opengis.net/gml",
    "bldg": "http://www.opengis.net/citygml/building/2.0",
    "core": "http://www.opengis.net/citygml/2.0",
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
