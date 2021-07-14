# Throttle

## Objective
In Go, developers can **easily write multi-threads code by using Go Routine**. In my opinion, the native thread pool in Go is too low level to use. So, this library provides a method to **using threads in an abstract way**.

## Implementation
### Declare thread pool
```go
// Number of thread you need in this pool
th := NewThrottle(3)
```
### Regist/Deregist work 
``` go
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
```
### Example result
``` bash
...
Processing item number = 91
Processing item number = 92
Processing item number = 93
Processing item number = 94
Processing item number = 95
Processing item number = 96
Processing item number = 97
Processing item number = 98
Processing item number = 99
--- PASS: TestThrottle (0.00s)
PASS
ok  	github.com/daginwu/monorepo/libs/throttle	(cached)
```

## Reference
[Real World Concurrent Programming in Go | Workshop | Go Systems Conf SF 2020](https://www.youtube.com/watch?v=r7qm6ZrxYIE)
