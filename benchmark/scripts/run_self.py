#!/usr/bin/env python3
import argparse
import csv
import json
import subprocess
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
FIELDS = [
    "dataset", "tool", "tool_version", "tool_revision", "mode",
    "eoo_raw_km2", "eoo_assessment_km2", "aoo_km2", "occupied_cells",
    "input_records", "unique_coordinates", "error", "settings", "notes",
]


def source_revision():
    try:
        sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
        dirty = subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True).strip()
        return sha + ("+dirty" if dirty else "")
    except (OSError, subprocess.CalledProcessError):
        return "unknown"


def run_one(binary, dataset, projection, raw_dir, revision):
    out = raw_dir / f"{dataset.stem}-{projection}.json"
    if out.exists():
        out.unlink()
    cmd = [
        str(binary), "calc", "--in", str(dataset), "--projection", projection,
        "--out", str(out),
    ]
    proc = subprocess.run(cmd, text=True, capture_output=True)
    row = {
        "dataset": dataset.stem,
        "tool": "go-iucn-eoo-aoo",
        "tool_revision": revision,
        "mode": projection,
        "settings": json.dumps({"projection": projection, "cell_size_km": 2}, sort_keys=True),
        "notes": f"measured {datetime.now(timezone.utc).isoformat()}",
    }
    if proc.returncode != 0:
        row["error"] = (proc.stderr or proc.stdout).strip()
        return row
    result = json.loads(out.read_text(encoding="utf-8"))
    row.update({
        "tool_version": result.get("software_version", ""),
        "eoo_raw_km2": result.get("eoo", {}).get("raw_area_km2", ""),
        "eoo_assessment_km2": result.get("eoo", {}).get("assessment_area_km2", ""),
        "aoo_km2": result.get("aoo", {}).get("area_km2", ""),
        "occupied_cells": result.get("aoo", {}).get("occupied_cells", ""),
        "input_records": result.get("input_records", ""),
        "unique_coordinates": result.get("unique_coordinates", ""),
        "error": "",
    })
    return row


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--binary", required=True, type=Path)
    p.add_argument("--datasets", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    args = p.parse_args()

    binary = args.binary.resolve()
    if not binary.is_file():
        raise SystemExit(f"calculator binary not found: {binary}")
    datasets = args.datasets.resolve()
    revision = source_revision()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    raw_dir = args.output.parent / "raw" / "go"
    raw_dir.mkdir(parents=True, exist_ok=True)
    rows = []
    for dataset in sorted(datasets.glob("*.csv")):
        for projection in ("local-laea", "iucn-cea"):
            rows.append(run_one(binary, dataset, projection, raw_dir, revision))

    with args.output.open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=FIELDS)
        w.writeheader()
        w.writerows(rows)
    print(f"wrote {len(rows)} rows to {args.output}")


if __name__ == "__main__":
    main()
