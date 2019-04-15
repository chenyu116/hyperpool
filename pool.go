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

	sync.Mutex

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
	h.Lock()
	last := len(h.pools) - 1
	if last >= 0 {
		x = h.pools[last]
		h.pools = h.pools[:last]
	}
	h.Unlock()
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

	h.Lock()
	h.pools = append(h.pools, x)
	h.Unlock()
}

func (h *Pool) clean() {
	ticker := time.NewTicker(time.Second * 10)
	for range ticker.C {
		if h.free < atomic.LoadUint32(&h.lastPut) {
			if atomic.LoadInt32(&h.cleaning) == 1 {
				continue
			}
			h.Lock()
			atomic.StoreInt32(&h.cleaning, 1)
			for i := 0; i < len(h.pools); i++ {
				if h.pools[i] != nil && h.Close != nil {
					h.Close(h.pools[i])
				}
				h.pools[i] = nil
			}
			h.pools = []interface{}{}
			h.Unlock()
			atomic.StoreUint32(&h.lastPut, 0)
			atomic.StoreInt32(&h.cleaning, 0)
			continue
		}
		atomic.AddUint32(&h.lastPut, 10)
	}
}

func NewPool(free uint32) *Pool {
	h := new(Pool)

	if free == 0 {
		free = 60
	}
	h.free = free
	go h.clean()

	return h
}
