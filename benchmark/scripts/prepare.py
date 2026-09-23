#!/usr/bin/env python3
import csv
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
WORK = ROOT / "benchmark" / "work"
DATASETS = WORK / "datasets"
MANUAL_INPUT = WORK / "manual-input"
MANUAL_RESULTS = ROOT / "benchmark" / "results" / "manual"
FIELDS = [
    "dataset", "tool", "tool_version", "tool_revision", "mode",
    "eoo_raw_km2", "eoo_assessment_km2", "aoo_km2", "occupied_cells",
    "input_records", "unique_coordinates", "error", "settings", "notes",
]


def write_case(name, points):
    rows = []
    for i, p in enumerate(points, 1):
        rows.append({
            "id": p.get("id") or f"{name}-{i:03d}",
            "taxon": p.get("taxon") or name,
            "lon": p["lon"],
            "lat": p["lat"],
        })

    DATASETS.mkdir(parents=True, exist_ok=True)
    with (DATASETS / f"{name}.csv").open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["id", "taxon", "lon", "lat"])
        w.writeheader()
        w.writerows(rows)

    with (DATASETS / f"{name}.json").open("w", encoding="utf-8") as f:
        json.dump(rows, f, ensure_ascii=False, indent=2)
        f.write("\n")

    MANUAL_INPUT.mkdir(parents=True, exist_ok=True)
    with (MANUAL_INPUT / f"{name}.csv").open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["id", "taxon", "longitude", "latitude"])
        w.writeheader()
        for row in rows:
            w.writerow({
                "id": row["id"],
                "taxon": row["taxon"],
                "longitude": row["lon"],
                "latitude": row["lat"],
            })


def load_brazil_example():
    path = ROOT / "examples" / "brazil.csv"
    with path.open(newline="", encoding="utf-8") as f:
        rows = list(csv.DictReader(f))
    points = [
        {
            "id": r.get("id", ""),
            "taxon": r.get("taxon", "Brazil example"),
            "lon": float(r["lon"]),
            "lat": float(r["lat"]),
        }
        for r in rows
    ]
    write_case("brazil-example", points)


def sync_manual_template(path, tool):
    path.parent.mkdir(parents=True, exist_ok=True)
    existing = {}
    if path.exists():
        with path.open(newline="", encoding="utf-8") as f:
            for row in csv.DictReader(f):
                dataset = row.get("dataset", "").strip()
                if dataset:
                    existing[dataset] = {k: row.get(k, "") for k in FIELDS}

    datasets = sorted(p.stem for p in DATASETS.glob("*.csv"))
    with path.open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=FIELDS)
        w.writeheader()
        for dataset in datasets:
            row = existing.get(dataset, {"dataset": dataset, "tool": tool})
            row["dataset"] = dataset
            row["tool"] = row.get("tool") or tool
            w.writerow({k: row.get(k, "") for k in FIELDS})


def clean_files(directory):
    if not directory.exists():
        return
    for old in directory.glob("*"):
        if old.is_file():
            old.unlink()


def main():
    DATASETS.mkdir(parents=True, exist_ok=True)
    clean_files(DATASETS)
    clean_files(MANUAL_INPUT)

    cases_path = ROOT / "testdata" / "reference" / "cases.json"
    cases = json.loads(cases_path.read_text(encoding="utf-8"))
    for case in cases:
        write_case(case["name"], case["points"])
    load_brazil_example()

    sync_manual_template(MANUAL_RESULTS / "geocat.csv", "GeoCAT")
    sync_manual_template(MANUAL_RESULTS / "iucn-eoo-calculator.csv", "IUCN EOO Calculator")

    count = len(list(DATASETS.glob("*.csv")))
    print(f"prepared {count} benchmark datasets in {DATASETS.relative_to(ROOT)}")
    print(f"manual upload inputs: {MANUAL_INPUT.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
