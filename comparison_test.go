package eooaoo

import (
	"context"
	"testing"
)

func TestCompareProjections(t *testing.T) {
	p := []Point{
		{X: -43.2, Y: -22.9, Taxon: "Example species"},
		{X: -44.3, Y: -21.5, Taxon: "Example species"},
		{X: -42.9, Y: -20.8, Taxon: "Example species"},
		{X: -45.0, Y: -22.0, Taxon: "Example species"},
	}
	c, err := CompareProjections(context.Background(), p, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if c.LocalLAEA.Projection.Strategy != ProjectionLocalLAEA || c.IUCNCEA.Projection.Strategy != ProjectionIUCNCEA {
		t.Fatal("projection strategies not recorded")
	}
	if c.InputSHA256 == "" || c.InputSHA256 != c.LocalLAEA.InputSHA256 || c.InputSHA256 != c.IUCNCEA.InputSHA256 {
		t.Fatal("comparison input provenance mismatch")
	}
	if c.LocalLAEA.AOO.AreaKM2 <= 0 || c.IUCNCEA.AOO.AreaKM2 <= 0 {
		t.Fatal("invalid AOO in comparison")
	}
}

func TestCompareRejectsFixedCenter(t *testing.T) {
	center := [2]float64{-43, -22}
	_, err := CompareProjections(context.Background(), []Point{{X: -43, Y: -22}}, Config{Center: &center})
	if err == nil {
		t.Fatal("expected fixed-center comparison error")
	}
}
