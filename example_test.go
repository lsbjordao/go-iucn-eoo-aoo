package eooaoo_test

import (
	"context"
	"fmt"

	eooaoo "github.com/lsbjordao/go-iucn-eoo-aoo"
)

func ExampleCalculate() {
	points := []eooaoo.Point{
		{X: -43.2, Y: -22.9, ID: "demo-001", Taxon: "Mimosa demonstrativa"},
	}

	result, err := eooaoo.Calculate(context.Background(), points, eooaoo.Config{})
	if err != nil {
		panic(err)
	}

	fmt.Println(result.AOO.AreaKM2)
	fmt.Println(result.EOO.RawAreaKM2 == nil)
	// Output:
	// 4
	// true
}
