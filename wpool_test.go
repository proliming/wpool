// Description:
// Author: liming.one@bytedance.com
package wpool

import (
	"fmt"
	"testing"
	"time"
)

type myTask struct {
	num int
}

func (t *myTask) Run() error {
	time.Sleep(1 * time.Second)
	fmt.Printf("%d task done !\n", t.num)
	return nil
}

func (t *myTask) Stop() {
	//fmt.Printf("stopping task %d", t.num)
}

func TestWorkerPool_Submit(t *testing.T) {
	p := New()

	p.Start()

	ch := time.Tick(500 * time.Millisecond)
	i := 0
	for t := range ch {
		_ = t
		i++
		p.Submit(&myTask{i})
	}
	//p.Stop()

}
