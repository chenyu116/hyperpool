package hyperpool

import (
	"sync"
	"sync/atomic"
	"time"
)

const max = 1<<31 - 1

type Pool struct {
	New   func() interface{}
	pool  sync.Pool
	gets  int32
	Limit int32
}

// wait is waiting the time to re get from the pool
func (p *Pool) Get(wait ...time.Duration) (x interface{}) {
	x = p.pool.Get()
	if x == nil && len(wait) > 0 {
		if wait[0] > 0 {
			<-time.After(wait[0])
			x = p.pool.Get()
		}
	}
	if x == nil && p.New != nil {
		if p.Limit == 0 || p.gets < p.Limit {
			x = p.New()
			if p.Limit > 0 && p.gets < max {
				atomic.AddInt32(&p.gets, 1)
			}
		}
	}
	return
}

func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	p.pool.Put(x)
	if p.New != nil && p.Limit > 0 && p.gets > 0 {
		atomic.AddInt32(&p.gets, -1)
	}
}
