package cycles

import (
	"time"
)

/**
 * Return the current value of the fine-grain CPU cycle counter
 * (accessed via the RDTSC instruction).
 */
func Rdtsc() uint64

/**
 * Return the current value of the fine-grain CPU cycle counter
 * (accessed via the RDTSCP instruction).
 */
func Rdtscp() uint64

var MockCyclesPerSec float64
var cyclesPerSec float64

/**
 * Perform once-only overall initialization for the Cycles class, such
 * as calibrating the clock frequency.  This method is invoked automatically
 * during initialization, but it may be invoked explicitly by other modules
 * to ensure that initialization occurs before those modules initialize
 * themselves.
 */
func Init() {
	// Compute the frequency of the fine-grained CPU timer: to do this,
	// take parallel time readings using both rdtsc and gettimeofday.
	// After 10ms have elapsed, take the ratio between these readings.

	var startTime, stopTime time.Time
	var startCycles, stopCycles uint64
	var micros int64
	var oldCycles float64

	// There is one tricky aspect, which is that we could get interrupted
	// between calling gettimeofday and reading the cycle counter, in which
	// case we won't have corresponding readings.  To handle this (unlikely)
	// case, compute the overall result repeatedly, and wait until we get
	// two successive calculations that are within 0.1% of each other.
	oldCycles = 0
	for {
		startTime = time.Now()
		startCycles = Rdtsc()
		for {
			stopTime = time.Now()
			stopCycles = Rdtsc()
			micros = stopTime.Sub(startTime).Microseconds()
			if micros > 10000 {
				cyclesPerSec = float64(stopCycles - startCycles)
				cyclesPerSec = 1000000.0 * cyclesPerSec / float64(micros)
				break
			}
		}
		delta := cyclesPerSec / 1000.0
		if oldCycles > (cyclesPerSec-delta) &&
			oldCycles < (cyclesPerSec+delta) {
			return
		}
		oldCycles = cyclesPerSec
	}
}

func PerSecond() float64 {
	if MockCyclesPerSec > 0 {
		return MockCyclesPerSec
	}

	if cyclesPerSec == 0 {
		Init()
	}

	return cyclesPerSec
}

func ToSeconds(cycles uint64) float64 {
	return float64(cycles) / PerSecond()
}

func ToNanoseconds(cycles uint64) float64 {
	return 1e09 * ToSeconds(cycles)
}
