package main

import (
	"context"
	"fmt"
	geo "github.com/lsbjordao/go-iucn-eoo-aoo"
	"log"
)

func main() {
	result, err := geo.Calculate(context.Background(), []geo.Point{
		{X: -43.2, Y: -22.9}, {X: -44.3, Y: -21.5}, {X: -42.9, Y: -20.8},
	}, geo.Config{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("EOO para avaliação: %.6f km²; AOO: %.0f km²\n",
		result.EOO.AssessmentAreaKM2, result.AOO.AreaKM2)
}
