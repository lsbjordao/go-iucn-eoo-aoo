#!/usr/bin/env python3
import csv
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RESULTS = ROOT / "benchmark" / "results"
FIELDS = [
    "dataset", "tool", "tool_version", "tool_revision", "mode",
    "eoo_raw_km2", "eoo_assessment_km2", "aoo_km2", "occupied_cells",
    "input_records", "unique_coordinates", "error", "settings", "notes",
]
DELTA_FIELDS = [
    "delta_eoo_vs_go_iucn_cea_km2", "delta_eoo_vs_go_iucn_cea_percent",
    "delta_aoo_vs_go_iucn_cea_km2", "delta_aoo_vs_go_iucn_cea_percent",
]


def as_float(value):
    if value is None or str(value).strip() == "":
        return None
    try:
        return float(value)
    except ValueError:
        return None


def read_rows(path):
    if not path.exists():
        return []
    with path.open(newline="", encoding="utf-8") as f:
        out = []
        for row in csv.DictReader(f):
            if row.get("tool") in {"GeoCAT", "IUCN EOO Calculator"}:
                measured = any(str(row.get(k, "")).strip() for k in ("eoo_raw_km2", "aoo_km2", "error"))
                if not measured:
                    continue
            out.append({k: row.get(k, "") for k in FIELDS})
        return out


def delta(value, baseline):
    if value is None or baseline is None:
        return ("", "")
    d = value - baseline
    pct = "" if baseline == 0 else d / baseline * 100
    return (d, pct)


def fmt(value):
    if value == "" or value is None:
        return ""
    if isinstance(value, float):
        return f"{value:.12g}"
    return str(value)


def main():
    sources = [
        RESULTS / "go.csv",
        RESULTS / "vicentecalfo.csv",
        RESULTS / "conr.csv",
        RESULTS / "manual" / "geocat.csv",
        RESULTS / "manual" / "iucn-eoo-calculator.csv",
    ]
    rows = []
    for source in sources:
        rows.extend(read_rows(source))
    if not rows:
        raise SystemExit("no benchmark results found; run make benchmark-prepare and one or more benchmark runners")

    baseline = {}
    for row in rows:
        if row["tool"] == "go-iucn-eoo-aoo" and row["mode"] == "iucn-cea" and not row["error"]:
            baseline[row["dataset"]] = {
                "eoo": as_float(row["eoo_raw_km2"]),
                "aoo": as_float(row["aoo_km2"]),
            }

    enriched = []
    for row in rows:
        base = baseline.get(row["dataset"], {})
        deoo, peoo = delta(as_float(row["eoo_raw_km2"]), base.get("eoo"))
        daoo, paoo = delta(as_float(row["aoo_km2"]), base.get("aoo"))
        item = dict(row)
        item.update({
            "delta_eoo_vs_go_iucn_cea_km2": deoo,
            "delta_eoo_vs_go_iucn_cea_percent": peoo,
            "delta_aoo_vs_go_iucn_cea_km2": daoo,
            "delta_aoo_vs_go_iucn_cea_percent": paoo,
        })
        enriched.append(item)

    enriched.sort(key=lambda r: (r["dataset"], r["tool"], r["mode"]))
    RESULTS.mkdir(parents=True, exist_ok=True)
    out_csv = RESULTS / "comparison.csv"
    with out_csv.open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=FIELDS + DELTA_FIELDS)
        w.writeheader()
        for row in enriched:
            w.writerow({k: fmt(row.get(k, "")) for k in FIELDS + DELTA_FIELDS})

    out_md = RESULTS / "comparison.md"
    with out_md.open("w", encoding="utf-8") as f:
        f.write("# EOO/AOO reference comparison\n\n")
        f.write("Deltas use `go-iucn-eoo-aoo` `iucn-cea` as an inspection baseline only; they are not accuracy scores.\n\n")
        current = None
        for row in enriched:
            if row["dataset"] != current:
                current = row["dataset"]
                f.write(f"## {current}\n\n")
                f.write("| Tool | Mode | Raw EOO km² | AOO km² | Δ EOO % | Δ AOO % | Error |\n")
                f.write("|---|---|---:|---:|---:|---:|---|\n")
            f.write(
                "| {tool} | {mode} | {eoo} | {aoo} | {deoo} | {daoo} | {error} |\n".format(
                    tool=row["tool"].replace("|", "\\|"),
                    mode=row["mode"].replace("|", "\\|"),
                    eoo=fmt(row["eoo_raw_km2"]),
                    aoo=fmt(row["aoo_km2"]),
                    deoo=fmt(row["delta_eoo_vs_go_iucn_cea_percent"]),
                    daoo=fmt(row["delta_aoo_vs_go_iucn_cea_percent"]),
                    error=row["error"].replace("|", "\\|").replace("\n", " "),
                )
            )
        f.write("\n")

    print(f"combined {len(enriched)} measured rows")
    print(f"wrote {out_csv.relative_to(ROOT)}")
    print(f"wrote {out_md.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
