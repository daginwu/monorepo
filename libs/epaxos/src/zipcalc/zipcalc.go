package main

import (
	"fmt"

	"github.com/daginwu/monorepo/libs/epaxos/src/zipfian"
)

func main() {
	for _, n := range []uint64{2, 1e9, 1e5, 1e4, 1e10} {
		for _, theta := range []float64{0.99, 0.95, 0.90, 0.85, 0.80, 0.75, 0.70, 0.65, 0.60, 0.55, 0.50} {
			fmt.Printf("N: %d, Theta: %f, Zeta: %.55f\n", n, theta, zipfian.Zeta(n, theta))
		}
	}
}
