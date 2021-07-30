package poisson

import (
	"math"
	"math/rand"
	"time"
)

// Simulates a Poisson distribution
type Poisson struct {
	// The average number of microseconds between arrivals
	rate   int
	random *rand.Rand
}

func NewPoisson(rate int) *Poisson {
	return &Poisson{
		rate,
		rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// The number of microseconds until the next arrival
func (p *Poisson) NextArrival() time.Duration {
	return time.Microsecond * time.Duration(-1*math.Log(1.0-p.random.Float64())*float64(p.rate))
}
