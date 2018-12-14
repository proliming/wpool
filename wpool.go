// Author: liming.one@bytedance.com
package wpool

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

var errNoWorkerAvailable = errors.New("no worker available for now")

type RejectedStrategy int

const (
	BlockWhenNoWorker RejectedStrategy = iota
	RejectWhenNoWorker
)

var defaultMaxWorkersCount = runtime.NumCPU()
var defaultMaxIdleWorkerDuration = 8 * time.Second
var workerChanCap = func() int {
	// Use blocking worker if GOMAXPROCS=1.
	// This immediately switches Submit to Exec, which results
	// in higher performance (under go1.5 at least).
	if runtime.GOMAXPROCS(0) == 1 {
		return 0
	}
	// Use non-blocking worker if GOMAXPROCS>1,
	// since otherwise the Submit caller (Acceptor) may lag accepting
	// new connections if Exec is CPU-bound.
	return 1
}()

// An Executor execute the task
type Executor func(r Task) error

func directExecutor(r Task) error {
	return r.Run()
}

// A worker performs some work
type Task interface {
	Run() error
}

// workerPool serves incoming Task via a pool of workers in FILO order
// The most recently stopped worker will serve the next Task.
// Such a scheme keeps CPU caches hot (in theory).
type workerPool struct {
	rejectedStrategy      RejectedStrategy
	executorFunc          Executor // executor to exec Tasks
	maxWorkersCount       int
	logAllErrors          bool
	maxIdleWorkerDuration time.Duration
	lock                  sync.Mutex
	workersCount          int
	mustStop              bool
	availableWorkers      []*worker // holder for available workers
	stopCh                chan struct{}
	innerWorkersPool      sync.Pool // inner pool
	wg                    sync.WaitGroup
}

// A worker hold a queue of Task
type worker struct {
	lastUseTime time.Time
	queue       chan Task // task queue
}

// Create a default worker pool
func New() *workerPool {
	return NewWith(directExecutor, defaultMaxWorkersCount, defaultMaxIdleWorkerDuration, BlockWhenNoWorker)
}

// Create with custom parameters
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

func (pool *workerPool) Start() {
	if pool.stopCh != nil {
		panic("workerPool already started")
	}
	pool.stopCh = make(chan struct{})
	stopCh := pool.stopCh

	// start an goroutine for cleaning invalid workers
	go func() {
		var workers []*worker
		for {
			pool.clean(&workers)
			select {
			case <-stopCh:
				return
			default:
				time.Sleep(pool.getMaxIdleWorkerDuration())
			}
		}
	}()
}

// Submit an Task
// The caller goroutine will blocking or got an error when there is no workers available
func (pool *workerPool) Submit(t Task) error {
	worker := pool.getWorker()
	if worker == nil {
		switch pool.rejectedStrategy {
		case RejectWhenNoWorker:
			return errNoWorkerAvailable
		case BlockWhenNoWorker:
			for {
				worker = pool.getWorker()
				if worker != nil {
					break
				}
			}
		}
	}
	pool.wg.Add(1)
	worker.queue <- t
	return nil
}

// Stop the pool right now
// Does not wait for actively executing tasks to terminate
func (pool *workerPool) Stop() {
	if pool.stopCh == nil {
		panic("workerPool wasn't started")
	}
	close(pool.stopCh)
	pool.stopCh = nil

	// Stop all the workers waiting for incoming Tasks.
	// Do not wait for busy workers - they will stop after
	// executing the Task and noticing pool.mustStop = true.
	pool.lock.Lock()
	workers := pool.availableWorkers
	for i, w := range workers {
		w.queue <- nil
		workers[i] = nil
	}
	pool.availableWorkers = workers[:0]
	pool.mustStop = true
	pool.lock.Unlock()
}

// Stop when all tasks stopped
// Blocks until all tasks have completed execution.
func (pool *workerPool) WaitThenStop() {
	pool.wg.Wait()
	pool.Stop()
}

func (pool *workerPool) getMaxIdleWorkerDuration() time.Duration {
	if pool.maxIdleWorkerDuration <= 0 {
		return 10 * time.Second
	}
	return pool.maxIdleWorkerDuration
}

func (pool *workerPool) clean(invalidWorkers *[]*worker) {
	maxIdleWorkerDuration := pool.getMaxIdleWorkerDuration()

	// Clean least recently used workers if they didn't executes Task
	// for more than maxIdleWorkerDuration.
	currentTime := time.Now()

	pool.lock.Lock()
	workers := pool.availableWorkers
	n := len(workers)
	i := 0
	for i < n && currentTime.Sub(workers[i].lastUseTime) > maxIdleWorkerDuration {
		i++
	}
	*invalidWorkers = append((*invalidWorkers)[:0], workers[:i]...)
	if i > 0 {
		m := copy(workers, workers[i:])
		for i = m; i < n; i++ {
			workers[i] = nil
		}
		pool.availableWorkers = workers[:m]
	}
	pool.lock.Unlock()

	// Notify obsolete workers to stop.
	// This notification must be outside the pool.lock, since worker.ch
	// may be blocking and may consume a lot of time if many workers
	// are located on non-local CPUs.
	tmp := *invalidWorkers
	for i, ch := range tmp {
		ch.queue <- nil
		tmp[i] = nil
	}
}

func (pool *workerPool) getWorker() *worker {
	var w *worker
	createWorker := false

	pool.lock.Lock()
	workers := pool.availableWorkers
	n := len(workers) - 1
	if n < 0 {
		if pool.workersCount < pool.maxWorkersCount {
			createWorker = true
			pool.workersCount++
		}
	} else {
		w = workers[n]
		workers[n] = nil
		pool.availableWorkers = workers[:n]
	}
	pool.lock.Unlock()

	if w == nil {
		if !createWorker {
			return nil
		}
		pooledWorker := pool.innerWorkersPool.Get()
		if pooledWorker == nil {
			pooledWorker = &worker{
				queue: make(chan Task, workerChanCap),
			}
		}
		w = pooledWorker.(*worker)
		go func() {
			pool.exec(w)
			pool.innerWorkersPool.Put(pooledWorker)
		}()
	}
	return w
}

func (pool *workerPool) release(w *worker) bool {
	w.lastUseTime = time.Now()
	pool.lock.Lock()
	if pool.mustStop {
		pool.lock.Unlock()
		return false
	}
	pool.availableWorkers = append(pool.availableWorkers, w)
	pool.lock.Unlock()
	return true
}

func (pool *workerPool) exec(worker *worker) {
	var task Task
	var err error
	for task = range worker.queue {
		if task == nil {
			break
		}
		if err = pool.executorFunc(task); err != nil {
			if pool.logAllErrors {
				fmt.Printf("error when executing task %s", err.Error())
			}
		}
		pool.wg.Done()
		task = nil
		if !pool.release(worker) {
			break
		}
	}

	pool.lock.Lock()
	pool.workersCount--
	pool.lock.Unlock()
}
