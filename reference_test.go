package eooaoo

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
)

type referenceExpectation struct {
	AOOAreaKM2        *float64 `json:"aoo_area_km2,omitempty"`
	EOOStatus         string   `json:"eoo_status,omitempty"`
	RawEOONull        *bool    `json:"raw_eoo_null,omitempty"`
	AdjustedToAOO     *bool    `json:"adjusted_to_aoo,omitempty"`
	UniqueCoordinates *int     `json:"unique_coordinates,omitempty"`
	DuplicateRecords  *int     `json:"duplicate_records,omitempty"`
	MaxRawEOOKM2      *float64 `json:"max_raw_eoo_km2,omitempty"`
}

type referenceCase struct {
	Name   string               `json:"name"`
	Points []Point              `json:"points"`
	Config Config               `json:"config"`
	Expect referenceExpectation `json:"expect"`
}

func TestReferenceCorpus(t *testing.T) {
	b, err := os.ReadFile("testdata/reference/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []referenceCase
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("reference corpus is empty")
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			r, err := Calculate(context.Background(), tc.Points, tc.Config)
			if err != nil {
				t.Fatal(err)
			}
			if tc.Expect.AOOAreaKM2 != nil && math.Abs(r.AOO.AreaKM2-*tc.Expect.AOOAreaKM2) > 1e-9 {
				t.Fatalf("AOO %.12g != expected %.12g", r.AOO.AreaKM2, *tc.Expect.AOOAreaKM2)
			}
			if tc.Expect.EOOStatus != "" && r.EOO.Status != tc.Expect.EOOStatus {
				t.Fatalf("EOO status %q != expected %q", r.EOO.Status, tc.Expect.EOOStatus)
			}
			if tc.Expect.RawEOONull != nil && (r.EOO.RawAreaKM2 == nil) != *tc.Expect.RawEOONull {
				t.Fatalf("raw EOO null=%v != expected %v", r.EOO.RawAreaKM2 == nil, *tc.Expect.RawEOONull)
			}
			if tc.Expect.AdjustedToAOO != nil && r.EOO.AdjustedToAOO != *tc.Expect.AdjustedToAOO {
				t.Fatalf("adjusted_to_aoo=%v != expected %v", r.EOO.AdjustedToAOO, *tc.Expect.AdjustedToAOO)
			}
			if tc.Expect.UniqueCoordinates != nil && r.UniqueCoordinates != *tc.Expect.UniqueCoordinates {
				t.Fatalf("unique coordinates=%d != expected %d", r.UniqueCoordinates, *tc.Expect.UniqueCoordinates)
			}
			if tc.Expect.DuplicateRecords != nil && r.DuplicateRecords != *tc.Expect.DuplicateRecords {
				t.Fatalf("duplicate records=%d != expected %d", r.DuplicateRecords, *tc.Expect.DuplicateRecords)
			}
			if tc.Expect.MaxRawEOOKM2 != nil {
				if r.EOO.RawAreaKM2 == nil || *r.EOO.RawAreaKM2 > *tc.Expect.MaxRawEOOKM2 {
					t.Fatalf("raw EOO %v exceeds expected maximum %.12g", r.EOO.RawAreaKM2, *tc.Expect.MaxRawEOOKM2)
				}
			}
		})
	}
}
