// Description:
// Author: liming.one@bytedance.com
package wpool

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// A WorkerState represents the state of a Task
type WorkerState int

const (
	StateNew WorkerState = iota
	StateRunning
	StateStopped
)

var DefaultMaxWorkersCount = runtime.NumCPU()
var DefaultMaxIdleWorkerDuration = 8 * time.Second

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
	Stop()
}

type TaskListener func(Task, WorkerState)

func noOpTaskListener(t Task, stat WorkerState) {
}

// workerPool serves incoming Task via a pool of workers
// in FILO order, i.e. the most recently stopped worker will serve the next
// incoming connection.
//
// Such a scheme keeps CPU caches hot (in theory).
type workerPool struct {
	Exec            Executor
	MaxWorkersCount int

	LogAllErrors bool

	MaxIdleWorkerDuration time.Duration

	lock         sync.Mutex
	workersCount int
	mustStop     bool

	availableWorkers []*worker

	stopCh chan struct{}

	workersPool sync.Pool

	notifyStateChange TaskListener
}

func New() *workerPool {
	return NewWith(directExecutor, DefaultMaxWorkersCount, DefaultMaxIdleWorkerDuration, noOpTaskListener)
}

func NewWith(executor Executor, maxWorkers int, maxIdleTime time.Duration, listener TaskListener) *workerPool {
	return &workerPool{
		Exec:                  executor,
		MaxWorkersCount:       maxWorkers,
		MaxIdleWorkerDuration: maxIdleTime,
		notifyStateChange:     listener,
	}
}

type worker struct {
	lastUseTime time.Time
	queue       chan Task
}

func (pool *workerPool) Start() {
	if pool.stopCh != nil {
		panic("BUG: workerPool already started")
	}
	pool.stopCh = make(chan struct{})
	stopCh := pool.stopCh
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

func (pool *workerPool) Stop() {
	if pool.stopCh == nil {
		panic("workerPool wasn't started")
	}
	close(pool.stopCh)
	pool.stopCh = nil

	// Stop all the workers waiting for incoming connections.
	// Do not wait for busy workers - they will stop after
	// serving the connection and noticing pool.mustStop = true.
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

func (pool *workerPool) getMaxIdleWorkerDuration() time.Duration {
	if pool.MaxIdleWorkerDuration <= 0 {
		return 10 * time.Second
	}
	return pool.MaxIdleWorkerDuration
}

func (pool *workerPool) clean(invalidWorkers *[]*worker) {
	maxIdleWorkerDuration := pool.getMaxIdleWorkerDuration()

	// Clean least recently used workers if they didn't serve connections
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

func (pool *workerPool) Submit(t Task) bool {
	worker := pool.getWorker()
	if worker == nil {
		return false
	}
	worker.queue <- t
	pool.notifyStateChange(t, StateNew)
	return true
}

func (pool *workerPool) getWorker() *worker {
	var w *worker
	createWorker := false

	pool.lock.Lock()
	workers := pool.availableWorkers
	n := len(workers) - 1
	if n < 0 {
		if pool.workersCount < pool.MaxWorkersCount {
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
		vch := pool.workersPool.Get()
		if vch == nil {
			vch = &worker{
				queue: make(chan Task, workerChanCap),
			}
		}
		w = vch.(*worker)
		go func() {
			pool.exec(w)
			pool.workersPool.Put(vch)
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
		pool.notifyStateChange(task, StateRunning)
		if err = pool.Exec(task); err != nil {
			if pool.LogAllErrors {
				fmt.Printf("error when serving runnable %s", err.Error())
			}
		}
		task.Stop()
		pool.notifyStateChange(task, StateStopped)
		task = nil
		if !pool.release(worker) {
			break
		}
	}

	pool.lock.Lock()
	pool.workersCount--
	pool.lock.Unlock()
}
