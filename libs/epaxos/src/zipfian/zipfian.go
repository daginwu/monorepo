package zipfian

import (
	"math"
	"math/rand"
	"time"
)

/**
 * Used to generate zipfian distributed random numbers where the distribution is
 * skewed toward the lower integers; e.g. 0 will be the most popular, 1 the next
 * most popular, etc.
 *
 * This class implements the core algorithm from YCSB's ZipfianGenerator; it, in
 * turn, uses the algorithm from "Quickly Generating Billion-Record Synthetic
 * Databases", Jim Gray et al, SIGMOD 1994.
 */
type ZipfianGenerator struct {
	n     float64 // Range of numbers to be generated.
	theta float64 // Parameter of the zipfian distribution.
	alpha float64 // Special intermediate result used for generation.
	zetan float64 // Special intermediate result used for generation.
	eta   float64 // Special intermediate result used for generation.
	r     *rand.Rand
}

/**
 * Construct a generator.  This may be expensive if n is large.
 *
 * \param n
 *      The generator will output random numbers between 0 and n-1.
 * \param theta
 *      The zipfian parameter where 0 < theta < 1 defines the skew; the
 *      smaller the value the more skewed the distribution will be. Default
 *      value of 0.99 comes from the YCSB default value.
 */
func NewZipfianGenerator(n uint64, theta float64) ZipfianGenerator {
	zetan := Zeta(n, theta)
	return ZipfianGenerator{
		n:     float64(n),
		theta: theta,
		alpha: (1.0 / (1.0 - theta)),
		zetan: zetan,
		eta: (1.0 - math.Pow(2.0/float64(n), 1.0-theta)) /
			(1.0 - Zeta(2.0, theta)/zetan),
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

/**
 * Return the zipfian distributed random number between 0 and n-1.
 * Partially inspired by https://github.com/cockroachdb/cockroach/blob/2eebbddbb133eea7102a47fbe7f5d13ec6f8f670/pkg/workload/ycsb/zipfgenerator.go
 */
func (z ZipfianGenerator) NextNumber() uint64 {
	u := z.r.Float64()
	uz := u * z.zetan
	if uz < 1 {
		return 0
	}
	if uz < 1+math.Pow(0.5, z.theta) {
		return 1
	}
	return 0 + uint64(float64(z.n)*math.Pow(z.eta*u-z.eta+1.0, z.alpha))
}

/**
 * Returns the nth harmonic number with parameter theta; e.g. H_{n,theta}.
 */
func Zeta(n uint64, theta float64) float64 {
	// Some of these take quite a while to compute, so return
	// common results immediately
	if theta == 0.99 {
		if n == 1e9 {
			return 23.60336399999999912324710749089717864990234375
		}
	}
	if theta == 0.95 {
		if n == 1e9 {
			return 36.94122142977597178514770348556339740753173828125
		}
	}
	if theta == 0.90 {
		if n == 1e9 {
			return 70.0027094570042294208178645931184291839599609375
		}
	}
	if theta == 0.85 {
		if n == 1e9 {
			return 143.14759472538497675486723892390727996826171875
		}
	}
	if theta == 0.80 {
		if n == 1e9 {
			return 311.04113385576732753179385326802730560302734375
		}
	}
	if theta == 0.75 {
		if n == 1e9 {
			return 707.87047871782715446897782385349273681640625
		}
	}
	if theta == 0.70 {
		if n == 1e9 {
			return 1667.845723895596393049345351755619049072265625
		}
	}
	if theta == 0.65 {
		if n == 1e9 {
			return 4033.51556666213946300558745861053466796875
		}
	}
	if theta == 0.60 {
		if n == 1e9 {
			return 9950.726604378511183313094079494476318359375
		}
	}
	if theta == 0.55 {
		if n == 1e9 {
			return 24932.0647149890646687708795070648193359375
		}
	}
	if theta == 0.50 {
		if n == 1e9 {
			return 63244.092864672114956192672252655029296875
		}
	}

	var sum float64 = 0
	var i float64
	for i = 0; i < float64(n); i++ {
		sum = sum + 1.0/(math.Pow(i+1.0, theta))
	}

	return sum
}
