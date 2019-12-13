package hyperpool

import (
	"sync"
	"sync/atomic"
	"time"
)

type Pool struct {
	pools    []interface{}
	free     uint32
	lastPut  uint32
	cleaning int32

	mu sync.Mutex

	New   func() interface{}
	Close func(x interface{})
}

func (h *Pool) Get() (x interface{}) {
	if atomic.LoadInt32(&h.cleaning) == 1 {
		if h.New != nil {
			x = h.New()
		}
		return
	}
	h.mu.Lock()
	last := len(h.pools) - 1
	if last >= 0 {
		x = h.pools[last]
		h.pools = h.pools[:last]
	}
	h.mu.Unlock()
	if x == nil && h.New != nil {
		x = h.New()
	}
	return
}

func (h *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	if atomic.LoadInt32(&h.cleaning) == 1 {
		if h.Close != nil {
			h.Close(x)
		}
		return
	}

	atomic.StoreUint32(&h.lastPut, 0)

	h.mu.Lock()
	h.pools = append(h.pools, x)
	h.mu.Unlock()
}

func (h *Pool) clean() {
	ticker := time.NewTicker(time.Second * 10)
	for range ticker.C {
		if atomic.LoadInt32(&h.cleaning) == 1 {
			continue
		}
		atomic.StoreInt32(&h.cleaning, 1)
		if h.free < h.lastPut {
			h.mu.Lock()
			for i := 0; i < len(h.pools); i++ {
				if h.pools[i] != nil && h.Close != nil {
					h.Close(h.pools[i])
				}
				h.pools[i] = nil
			}
			h.pools = []interface{}{}
			h.mu.Unlock()
			h.lastPut = 0
		} else {
			h.lastPut += 10
		}
		atomic.StoreInt32(&h.cleaning, 0)
	}
}

func NewPool(free uint32) *Pool {
	h := new(Pool)

	if free == 0 {
		free = 120
	}
	h.free = free
	go h.clean()

	return h
}
