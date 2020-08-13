package hyperpool

import (
	"time"
)

type Pool struct {
	New   func() interface{}
	poolCache  []interface{}
	resetAfter time.Duration
	cacheSize  int
	bits       uint64
}

func (p *Pool) Get() (x interface{}) {
	pid := p.pin()
	if pid != p.cacheSize {
		x = p.poolCache[pid]
		if x != nil {
			p.poolCache[pid] = nil
		}
	}
	if x == nil && p.New != nil {
		x = p.New()
	}
	return
}

func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	pid := p.unpin()
	if pid == p.cacheSize {
		return
	}
	p.poolCache[pid] = x
	p.bits |= 1 << pid
}

func (p *Pool) reset() {
	resetTicker := time.NewTicker(p.resetAfter)
	var lastBits uint64 = 0
	checkTimes := 0
	checkLimit := 3
	for range resetTicker.C {
		if lastBits == 0 || lastBits != p.bits {
			lastBits = p.bits
			continue
		}
		if lastBits == p.bits {
			if checkTimes < checkLimit {
				checkTimes++
				continue
			}
			checkTimes = 0
			p.bits = 0
			p.poolCache = make([]interface{}, p.cacheSize)
		}
	}
}
func (p *Pool) unpin() (pid int) {
	for i := 0; i < p.cacheSize; i++ {
		if 1<<i&p.bits == 0 {
			return i
		}
	}
	return p.cacheSize
}
func (p *Pool) pin() (pid int) {
	for i := 0; i < p.cacheSize; i++ {
		if 1<<i&p.bits != 0 {
			p.bits &^= 1 << i
			return i
		}
	}
	return p.cacheSize
}

func NewPool(cacheSize int, resetAfter time.Duration, n func() interface{}) *Pool {
	p := new(Pool)
	p.resetAfter = resetAfter
	if p.resetAfter == 0 {
		p.resetAfter = time.Second * 120
	}

	p.cacheSize = cacheSize
	if p.cacheSize >= 64 {
		p.cacheSize = 64
	}
	p.poolCache = make([]interface{}, p.cacheSize)
	p.bits = 0
	p.New = n
	go p.reset()

	return p
}
