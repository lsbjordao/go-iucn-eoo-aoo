package eooaoo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// CurationManifestVersion is the manifest format accepted by ApplyCuration.
const CurationManifestVersion = "1"

// CurationDecision selects one record by zero-based input index and states
// what the curator expects to find there, so a manifest written for one file
// cannot silently apply to another. At least one of RecordID or Lon/Lat is
// required; every field given must match exactly.
type CurationDecision struct {
	InputIndex    int      `json:"input_index"`
	RecordID      *string  `json:"record_id,omitempty"`
	Lon           *float64 `json:"lon,omitempty"`
	Lat           *float64 `json:"lat,omitempty"`
	Action        string   `json:"action"`
	Justification string   `json:"justification"`
}

// CurationManifest is a versioned, reviewable list of explicit exclusions for
// one exact input, identified by the same hash Calculate records.
type CurationManifest struct {
	ManifestVersion string             `json:"manifest_version"`
	InputSHA256     string             `json:"input_sha256"`
	Description     string             `json:"description,omitempty"`
	Curator         string             `json:"curator,omitempty"`
	Decisions       []CurationDecision `json:"decisions"`
}

// ExcludedRecord documents one record removed by a manifest decision.
type ExcludedRecord struct {
	InputIndex    int     `json:"input_index"`
	RecordID      string  `json:"record_id,omitempty"`
	Lon           float64 `json:"lon"`
	Lat           float64 `json:"lat"`
	Justification string  `json:"justification"`
	Record        Point   `json:"record"`
}

// CurationLog links the original and derived record sets.
type CurationLog struct {
	SchemaVersion   string           `json:"schema_version"`
	SoftwareVersion string           `json:"software_version"`
	ManifestSHA256  string           `json:"manifest_sha256"`
	InputSHA256     string           `json:"input_sha256"`
	OutputSHA256    string           `json:"output_sha256"`
	InputRecords    int              `json:"input_records"`
	OutputRecords   int              `json:"output_records"`
	Excluded        []ExcludedRecord `json:"excluded"`
	Note            string           `json:"note"`
}

// ApplyCuration verifies every decision against points before applying any of
// them and returns a new slice; points is not modified. The input hash must
// match, each selector must identify its record unambiguously, and every
// exclusion needs a justification.
func ApplyCuration(points []Point, m CurationManifest) ([]Point, CurationLog, error) {
	log := CurationLog{SchemaVersion: "1", SoftwareVersion: Version, InputRecords: len(points), Excluded: []ExcludedRecord{}}
	log.Note = "Derived record set. The original input is unchanged; reproduce with the same manifest and input hash."
	if m.ManifestVersion != CurationManifestVersion {
		return nil, log, fmt.Errorf("manifest_version %q is not supported (want %q)", m.ManifestVersion, CurationManifestVersion)
	}
	mb, err := json.Marshal(m)
	if err != nil {
		return nil, log, err
	}
	ms := sha256.Sum256(mb)
	log.ManifestSHA256 = hex.EncodeToString(ms[:])
	h, err := InputHash(points)
	if err != nil {
		return nil, log, err
	}
	log.InputSHA256 = h
	if !strings.EqualFold(m.InputSHA256, h) {
		return nil, log, fmt.Errorf("manifest was written for input %s but this input hashes to %s", m.InputSHA256, h)
	}
	if len(m.Decisions) == 0 {
		return nil, log, fmt.Errorf("manifest has no decisions")
	}
	drop := map[int]CurationDecision{}
	for k, d := range m.Decisions {
		where := fmt.Sprintf("decision %d (input_index %d)", k, d.InputIndex)
		if d.InputIndex < 0 || d.InputIndex >= len(points) {
			return nil, log, fmt.Errorf("%s: index out of range", where)
		}
		if _, dup := drop[d.InputIndex]; dup {
			return nil, log, fmt.Errorf("%s: ambiguous, index selected more than once", where)
		}
		if d.RecordID == nil && d.Lon == nil && d.Lat == nil {
			return nil, log, fmt.Errorf("%s: ambiguous, give record_id and/or lon/lat expected at that index", where)
		}
		if (d.Lon == nil) != (d.Lat == nil) {
			return nil, log, fmt.Errorf("%s: lon and lat must be given together", where)
		}
		if d.Action != "exclude" {
			return nil, log, fmt.Errorf("%s: action %q is not supported (only exclude)", where, d.Action)
		}
		if strings.TrimSpace(d.Justification) == "" {
			return nil, log, fmt.Errorf("%s: justification is required", where)
		}
		p := points[d.InputIndex]
		if d.RecordID != nil && *d.RecordID != p.ID {
			return nil, log, fmt.Errorf("%s: expected record_id %q, found %q", where, *d.RecordID, p.ID)
		}
		if d.Lon != nil && (*d.Lon != p.X || *d.Lat != p.Y) {
			return nil, log, fmt.Errorf("%s: expected coordinates %v,%v, found %v,%v", where, *d.Lon, *d.Lat, p.X, p.Y)
		}
		drop[d.InputIndex] = d
	}
	out := make([]Point, 0, len(points)-len(drop))
	for i, p := range points {
		if d, ok := drop[i]; ok {
			log.Excluded = append(log.Excluded, ExcludedRecord{InputIndex: i, RecordID: p.ID, Lon: p.X, Lat: p.Y, Justification: d.Justification, Record: p})
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, log, fmt.Errorf("manifest would exclude every record")
	}
	if log.OutputSHA256, err = InputHash(out); err != nil {
		return nil, log, err
	}
	log.OutputRecords = len(out)
	return out, log, nil
}
