package poisson

import (
	"math"
	"testing"
)

const POISSON_PARAM = 40
const COUNT = 100000

func TestPoisson(t *testing.T) {
	p := NewPoisson(POISSON_PARAM)
	var sum int64 = 0
	for i := 0; i < COUNT; i++ {
		sum += p.NextArrival().Microseconds()
	}
	avg := float64(sum) / float64(COUNT)
	if math.Abs(avg-POISSON_PARAM) > 1 {
		t.Errorf("Expected Poisson avg to be %d, got %f", POISSON_PARAM, avg)
	}
}
