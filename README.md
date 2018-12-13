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
func NewWith(executor Executor, maxWorkers int, maxIdleTime time.Duration, rejectStrategy RejectedStrategy) *workerPool {
	if executor == nil {
		executor = directExecutor
	}
	if maxWorkers <= 0 {
		panic("max workers must > 0")
	}
	return &workerPool{
		executorFunc:          executor,
		maxWorkersCount:       maxWorkers,
		maxIdleWorkerDuration: maxIdleTime,
		rejectedStrategy:      rejectStrategy,
	}
}

func myExecutor(r Task) error {
	return r.Run()
}

p:=wpool.NewWith(myExecutor, 1024, math.MaxInt64*time.Nanosecond, wpool.BlockWhenNoWorker)
p.Start()
p.Submit(...)
p.Stop()

```
