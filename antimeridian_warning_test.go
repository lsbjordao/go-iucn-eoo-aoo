package eooaoo

import (
	"context"
	"strings"
	"testing"
)

func TestIUCNCEAAntimeridianWarning(t *testing.T) {
	points := []Point{
		{X: 179.9, Y: 10},
		{X: -179.9, Y: 10},
		{X: -179.9, Y: 10.1},
		{X: 179.9, Y: 10.1},
	}

	result, err := Calculate(context.Background(), points, Config{ProjectionStrategy: ProjectionIUCNCEA})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, warning := range result.Warnings {
		if strings.HasPrefix(warning, "ANTIMERIDIAN_CEA:") && strings.Contains(warning, "world-spanning planar hull") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ANTIMERIDIAN_CEA warning, got %v", result.Warnings)
	}
}
