package hyperpool

import (
	"sync/atomic"
	"time"
)

// every queue has 8 cache slots
var cacheQueueSize uint32

type Pool struct {
	New        func() interface{}
	Close      func(i interface{})
	resetAfter time.Duration
	lastPut    uint32
	free       int64
	shared     poolChain
}

func (p *Pool) Get() (x interface{}) {
	x, _ = p.shared.popHead()
	return
}

func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	p.shared.pushHead(x)
	// check if still using the pool
	atomic.StoreUint32(&p.lastPut, 1)
}

func (p *Pool) Len() int64 {
	return p.free
}
func (p *Pool) reset() {
	resetTicker := time.NewTicker(p.resetAfter)
	checkTimes := 0
	checkLimit := 3
	resetting := false
	for range resetTicker.C {
		if p.lastPut == 1 {
			atomic.StoreUint32(&p.lastPut, 0)
			continue
		}
		if resetting {
			continue
		}
		if p.lastPut == 0 {
			if checkTimes < checkLimit {
				checkTimes++
				continue
			}
			resetting = true
			for {
				x := p.Get()
				if x != nil && p.Close != nil {
					p.Close(x)
					continue
				}
				if x == nil {
					break
				}
			}
			for i := 0; i < int(cacheQueueSize*initSize); i++ {
				p.Put(p.New())
			}
			checkTimes = 0
			resetting = false
		}
	}
}

func NewPool(queueSize uint32, resetAfter time.Duration, n func() interface{}) *Pool {
	p := new(Pool)
	p.resetAfter = resetAfter
	if p.resetAfter == 0 {
		p.resetAfter = time.Second * 120
	}
	cacheQueueSize = queueSize

	for i := 0; i < int(cacheQueueSize*8); i++ {
		p.Put(n())
	}
	p.New = n
	go p.reset()

	return p
}
