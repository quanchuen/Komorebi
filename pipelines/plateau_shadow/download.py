"""
download.py — fetch PLATEAU CityGML LOD2 building files for a ward.

The 2023 PLATEAU edition is distributed as one CityGML archive per ward on
the geospatial.jp CKAN (package `plateau-131XX-<ward>-ku-2023`); the older
per-mesh-tile packages this pipeline originally used were removed. This
module resolves the ward's package, downloads its newest CityGML archive
(≈2 GB per ward — cached), extracts only the building files
(`udx/bldg/*.gml`), deletes the archive to reclaim disk, and returns the
.gml paths. Spatial clipping to the ward bbox happens in parse_citygml.
"""

import re
import zipfile
from pathlib import Path

import requests
from tqdm import tqdm

CKAN_BASE = "https://www.geospatial.jp/ckan/api/3/action"
PLATEAU_EDITION = "2023"

# JIS ward codes for the Tokyo 23 special wards.
WARD_CODES = {
    "chiyoda": "13101",
    "chuo": "13102",
    "minato": "13103",
    "shinjuku": "13104",
    "bunkyo": "13105",
    "taito": "13106",
    "sumida": "13107",
    "koto": "13108",
    "shinagawa": "13109",
    "meguro": "13110",
    "ota": "13111",
    "setagaya": "13112",
    "shibuya": "13113",
    "nakano": "13114",
    "suginami": "13115",
    "toshima": "13116",
    "kita": "13117",
    "arakawa": "13118",
    "itabashi": "13119",
    "nerima": "13120",
    "adachi": "13121",
    "katsushika": "13122",
    "edogawa": "13123",
}


def _package_id(ward: str) -> str:
    code = WARD_CODES.get(ward)
    if code is None:
        raise RuntimeError(f"Unknown ward {ward!r}. Known: {sorted(WARD_CODES)}")
    return f"plateau-{code}-{ward}-ku-{PLATEAU_EDITION}"


def _fetch_resource_list(package_id: str) -> list[dict]:
    """Return list of CKAN resource dicts for the package."""
    url = f"{CKAN_BASE}/package_show?id={package_id}"
    resp = requests.get(url, timeout=60)
    resp.raise_for_status()
    return resp.json()["result"]["resources"]


def _newest_citygml_resource(resources: list[dict]) -> dict:
    """
    Pick the highest-versioned CityGML archive. Resource names look like
    'CityGML（v4）' / 'CityGML（v3）' (full-width parentheses).
    """

    def version(resource: dict) -> int:
        match = re.search(r"v(\d+)", resource["name"])
        return int(match.group(1)) if match else 0

    citygml = [r for r in resources if r["name"].startswith("CityGML")]
    if not citygml:
        raise RuntimeError("Package has no CityGML resource; check it on geospatial.jp")
    return max(citygml, key=version)


def _download(url: str, dest: Path) -> None:
    # Download to a temp name and rename on success, so an interrupted run
    # never leaves a truncated file that a later run would trust as cached.
    part = dest.with_suffix(dest.suffix + ".part")
    resp = requests.get(url, stream=True, timeout=300)
    resp.raise_for_status()
    total = int(resp.headers.get("content-length", 0))
    with open(part, "wb") as f, tqdm(
        total=total, unit="B", unit_scale=True, desc=dest.name
    ) as bar:
        for chunk in resp.iter_content(chunk_size=1 << 20):
            f.write(chunk)
            bar.update(len(chunk))
    part.rename(dest)


def download_citygml(ward: str, bbox: list[float], data_dir: str = "./data") -> list[str]:
    """
    Download CityGML building files for a ward, return list of .gml paths.

    Extracted files are cached — if .gml files already exist for the ward
    nothing is downloaded.
    """
    ward_dir = Path(data_dir) / ward
    ward_dir.mkdir(parents=True, exist_ok=True)

    cached = list(ward_dir.glob("**/*.gml"))
    if cached:
        print(f"  Using {len(cached)} cached .gml files for {ward}")
        return [str(p) for p in cached]

    package_id = _package_id(ward)
    resource = _newest_citygml_resource(_fetch_resource_list(package_id))
    zip_path = ward_dir / f"{package_id}.zip"

    if not zip_path.exists():
        print(f"  Downloading {resource['name']} for {ward} ({resource['url']})")
        _download(resource["url"], zip_path)

    try:
        zipfile.ZipFile(zip_path).close()
    except zipfile.BadZipFile:
        # Pre-atomic-rename runs could leave a truncated archive behind.
        print(f"  Cached {zip_path.name} is corrupt; re-downloading")
        zip_path.unlink()
        _download(resource["url"], zip_path)

    # The archive carries far more than buildings (terrain, textures, other
    # themes); extract only the building geometry files. Match with or
    # without a leading directory component — ward archives differ.
    with zipfile.ZipFile(zip_path) as zf:
        members = [
            m for m in zf.namelist() if "udx/bldg" in m and m.endswith(".gml")
        ]
        if not members:
            sample = "\n    ".join(zf.namelist()[:15])
            raise RuntimeError(
                f"No udx/bldg .gml files in {zip_path.name}; first entries:\n    {sample}"
            )
        print(f"  Extracting {len(members)} building files for {ward}")
        for member in members:
            zf.extract(member, ward_dir)

    zip_path.unlink()  # ~2 GB per ward; the .gml files are the durable cache

    gml_paths = [str(p) for p in ward_dir.glob("**/*.gml")]
    return gml_paths
