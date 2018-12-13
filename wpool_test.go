// Description:
// Author: liming.one@bytedance.com
package wpool

import (
	"math"
	"runtime"
	"testing"
	"time"
)

type myTask struct {
	num int
}

func (t *myTask) Run() error {
	return nil
}

func TestWorkerPool_Submit(t *testing.T) {
	//p := New()
	p := NewWith(directExecutor, 2, 1*time.Second, BlockWhenNoWorker)
	p.Start()
	for i := 0; i < 100; i++ {
		p.Submit(&myTask{i})
	}
	p.WaitAndStop()
	//p.Stop()
}

func BenchmarkWorkerPool_SubmitOneWorkerNoIdleOneCPU(b *testing.B) {
	n := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(n)
	p := NewWith(directExecutor, 1, math.MaxInt64*time.Nanosecond, BlockWhenNoWorker)
	b.ResetTimer()
	b.ReportAllocs()
	p.Start()
	for i := 0; i < b.N; i++ {
		p.Submit(&myTask{num: i})
	}
	p.WaitAndStop()
}
func BenchmarkWorkerPool_SubmitOneWorkerNoIdleMoreCPU(b *testing.B) {
	n := runtime.GOMAXPROCS(runtime.NumCPU())
	defer runtime.GOMAXPROCS(n)
	p := NewWith(directExecutor, 1, math.MaxInt64*time.Nanosecond, BlockWhenNoWorker)
	b.ResetTimer()
	b.ReportAllocs()
	p.Start()
	for i := 0; i < b.N; i++ {
		p.Submit(&myTask{num: i})
	}
	p.WaitAndStop()
}

func BenchmarkWorkerPool_Submit16WorkerNoIdleOneCPU(b *testing.B) {
	n := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(n)
	p := NewWith(directExecutor, 1024, math.MaxInt64*time.Nanosecond, BlockWhenNoWorker)
	b.ResetTimer()
	b.ReportAllocs()
	p.Start()
	for i := 0; i < b.N; i++ {
		p.Submit(&myTask{num: i})
	}
	p.WaitAndStop()
}
func BenchmarkWorkerPool_Submit1024WorkerNoIdleOneCPU(b *testing.B) {
	n := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(n)
	p := NewWith(directExecutor, 1024, math.MaxInt64*time.Nanosecond, BlockWhenNoWorker)
	b.ResetTimer()
	b.ReportAllocs()
	p.Start()
	for i := 0; i < b.N; i++ {
		p.Submit(&myTask{num: i})
	}
	p.WaitAndStop()
}

func BenchmarkWorkerPool_Submit16WorkerNoIdleMoreCPU(b *testing.B) {
	n := runtime.GOMAXPROCS(runtime.NumCPU())
	defer runtime.GOMAXPROCS(n)
	p := NewWith(directExecutor, 1024, math.MaxInt64*time.Nanosecond, BlockWhenNoWorker)
	b.ResetTimer()
	b.ReportAllocs()
	p.Start()
	for i := 0; i < b.N; i++ {
		p.Submit(&myTask{num: i})
	}
	p.WaitAndStop()
}

func BenchmarkWorkerPool_Submit1024WorkerNoIdleMoreCPU(b *testing.B) {
	n := runtime.GOMAXPROCS(runtime.NumCPU())
	defer runtime.GOMAXPROCS(n)
	p := NewWith(directExecutor, 1024, math.MaxInt64*time.Nanosecond, BlockWhenNoWorker)
	b.ResetTimer()
	b.ReportAllocs()
	p.Start()
	for i := 0; i < b.N; i++ {
		p.Submit(&myTask{num: i})
	}
	p.WaitAndStop()
}
