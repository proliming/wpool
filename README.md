# wpool
A worker pool for golang

## Usage

Step 1: Create a Task before you start using it.

```go
// An interface like Runable in JAVA
type Task interface {
	Run() error
}

type myTask struct {
}

func (t *myTask) Run() error {
// do something here
  return nil
}

```
Step2: Create the workPool
```go
p:=wpool.New()   // create a workerPool with directExecutor and default max workers ..

// then start it
p.Start()

// submit some tasks
p.Submit(&myTask{})

// wait then stop
p.WaitThenStop()

```
Custom workPool

```go
func myExecutor(r Task) error {
	return r.Run()
}

p:=wpool.NewWith(myExecutor, 1024, math.MaxInt64*time.Nanosecond, wpool.BlockWhenNoWorker)
p.Start()
p.Submit(...)
p.Stop()

```

## Benchmarks
On my macOS High Sierra: 2.5 GHz Intel Core i7, 4 Core

```go 
goos: darwin
goarch: amd64
BenchmarkWorkerPool_SubmitOneWorkerNoIdleOneCPU-8          	     100	  36199276 ns/op	      13 B/op	       1 allocs/op
BenchmarkWorkerPool_SubmitOneWorkerNoIdleMoreCPU-8         	  200000	     10273 ns/op	       8 B/op	       1 allocs/op
BenchmarkWorkerPool_Submit16WorkerNoIdleOneCPU-8           	   50000	     34704 ns/op	      13 B/op	       1 allocs/op
BenchmarkWorkerPool_Submit1024WorkerNoIdleOneCPU-8         	   50000	     33809 ns/op	      13 B/op	       1 allocs/op
BenchmarkWorkerPool_Submit16WorkerNoIdleMoreCPU-8          	 3000000	       543 ns/op	       8 B/op	       1 allocs/op
BenchmarkWorkerPool_Submit1024WorkerNoIdleMoreCPU-8        	 3000000	       535 ns/op	       8 B/op	       1 allocs/op
BenchmarkWorkerPool_SubmitOneWorkerDefaultIdleMoreCPU-8    	  100000	     10824 ns/op	       8 B/op	       1 allocs/op
BenchmarkWorkerPool_Submit1024WorkerDefaultIdleMoreCPU-8   	 3000000	       589 ns/op	       8 B/op	       1 allocs/op
PASS
```
