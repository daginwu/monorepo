package throttle

import (
	"fmt"
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	// Create a new throttle variable.
	th := NewThrottle(3)

	for i := 0; i < 100; i++ {
		// Gate keeper. Do not let more than 3 goroutines to start.
		th.Do()

		fmt.Printf("Processing item number = %+v\n", i)
		go doWork(th)
	}

	// Wait for all the jobs to finish.
	th.Finish()
}
func doWork(th *Throttle) {
	// Mark this job as done.
	defer th.Done(nil)

	// Simulate some work.
	time.Sleep(time.Microsecond)
}
