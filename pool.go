package hyperpool

import (
	"sync"
	"sync/atomic"
)

type Pool struct {
	New   func() interface{}
	pool  sync.Pool
	gets  int32
	Limit int32
}

func (p *Pool) Get() (x interface{}) {
	x = p.pool.Get()
	if p.Limit == 0 || (x == nil && p.New != nil && p.gets < p.Limit) {
		x = p.New()
		atomic.AddInt32(&p.gets, 1)
	}
	return
}

func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	p.pool.Put(x)
	atomic.AddInt32(&p.gets, -1)
}
