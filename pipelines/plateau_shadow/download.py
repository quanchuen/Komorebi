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
